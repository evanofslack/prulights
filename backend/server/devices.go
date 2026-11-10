package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/evanofslack/prulights/metrics"
	"github.com/evanofslack/prulights/render"
	"github.com/evanofslack/prulights/schedule"
	"github.com/evanofslack/prulights/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
)

const (
	// bundleNights is how many nights a bundle covers, starting tonight.
	bundleNights = 7
	// lowBatteryMV is when the frame starts drawing its low-battery mark.
	lowBatteryMV = 3500
	// registerRateLimit is new frames per client per minute.
	registerRateLimit = 10
	maxEvents         = 200
	maxDetailLen      = 200
	maxDeviceNameLen  = 40
	// frameInviteTTL is how long a frame stays marked for a new person.
	frameInviteTTL = 30 * 24 * time.Hour
	// portalURL is the frame's setup page on its own hotspot.
	portalURL = "http://192.168.4.1"
	// Check-in intervals: unclaimed frames wait to be claimed, frames on USB
	// check hourly, and frames on battery once a day, a little after noon.
	unclaimedCheckin = 2 * time.Minute
	usbCheckin       = time.Hour
	dailyCheckinHour = 12
)

// dayViews are the views a frame can show for a night; the button cycles
// through the ones its owner picks.
var dayViews = []render.View{render.ViewTonight, render.ViewWeek, render.ViewMonth, render.ViewLog}

// Themes: dark is the default.
const (
	themeDark  = "dark"
	themeAuto  = "auto"
	themeLight = "light"
)

func defaultSettings() DeviceSettings {
	return DeviceSettings{DefaultView: string(render.ViewTonight),
		Views: []string{string(render.ViewTonight), string(render.ViewWeek), string(render.ViewMonth), string(render.ViewLog)},
		Theme: themeDark, Scene: string(render.SceneSkyline), Words: string(render.WordsCause)}
}

// withDefaults fills in the choices that were left out (older settings, or
// a client that doesn't know them).
func (set DeviceSettings) withDefaults() DeviceSettings {
	def := defaultSettings()
	if set.Theme == "" {
		set.Theme = def.Theme
	}
	if set.Scene == "" {
		set.Scene = def.Scene
	}
	if set.Words == "" {
		set.Words = def.Words
	}
	return set
}

// look is how the settings draw, by day (light) or by night.
func (set DeviceSettings) look(light bool) render.Look {
	return render.Look{Light: light, Scene: render.Scene(set.Scene), Words: render.Words(set.Words)}
}

// settingsOf reads stored settings, filling in what's missing.
func settingsOf(raw []byte) DeviceSettings {
	set := defaultSettings()
	var got DeviceSettings
	if json.Unmarshal(raw, &got) == nil {
		if got.DefaultView != "" {
			set.DefaultView = got.DefaultView
		}
		if len(got.Views) > 0 {
			set.Views = got.Views
		}
		set.QuietStart, set.QuietEnd, set.Paused = got.QuietStart, got.QuietEnd, got.Paused
		set.Theme, set.Scene, set.Words = got.Theme, got.Scene, got.Words
	}
	return set.withDefaults()
}

var clockRE = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

func validSettings(set DeviceSettings) string {
	if len(set.Views) == 0 {
		return "pick at least one view"
	}
	for i, v := range set.Views {
		if !slices.Contains(dayViews, render.View(v)) {
			return "unknown view " + v
		}
		if slices.Contains(set.Views[:i], v) {
			return "a view is listed twice"
		}
	}
	if !slices.Contains(dayViews, render.View(set.DefaultView)) {
		return "unknown default view"
	}
	set = set.withDefaults()
	if !slices.Contains([]string{themeDark, themeAuto, themeLight}, set.Theme) {
		return "theme is dark, auto or light"
	}
	if !slices.Contains(render.Scenes, render.Scene(set.Scene)) {
		return "scene is tower, skyline or river"
	}
	if !slices.Contains(render.WordsList, render.Words(set.Words)) {
		return "words is none, color or cause"
	}
	if (set.QuietStart == "") != (set.QuietEnd == "") {
		return "quiet hours need a start and an end"
	}
	if set.QuietStart != "" && (!clockRE.MatchString(set.QuietStart) || !clockRE.MatchString(set.QuietEnd) || set.QuietStart == set.QuietEnd) {
		return "quiet hours are like 22:00 to 07:00"
	}
	return ""
}

// quiet is whether a Boston clock time falls in the quiet hours.
func (set DeviceSettings) quiet(t time.Time) bool {
	if set.QuietStart == "" {
		return false
	}
	now := t.In(schedule.Boston).Format("15:04")
	if set.QuietStart < set.QuietEnd {
		return now >= set.QuietStart && now < set.QuietEnd
	}
	return now >= set.QuietStart || now < set.QuietEnd // overnight
}

// daylight is when a night's "light" pictures apply, for theme "auto":
// from sunrise until the crown lights up.
func daylight(nights []schedule.Lighting, day schedule.Date) (from, until time.Time) {
	t := day.Time()
	from = t.Add(7 * time.Hour)
	if rise, ok := schedule.Sunrise(t.Year(), t.Month(), t.Day(), schedule.TowerLat, schedule.TowerLon); ok {
		from = rise
	}
	until, _ = schedule.LightsOn(day, schedule.ProgramUnlisted)
	for _, l := range nights {
		if l.Date == day && !l.LightsOn.IsZero() {
			until = l.LightsOn
		}
	}
	return from, until
}

// lightAt is whether the frame draws on paper at a moment.
func (set DeviceSettings) lightAt(now time.Time, nights []schedule.Lighting, tonight schedule.Date) bool {
	switch set.Theme {
	case themeLight:
		return true
	case themeAuto:
		from, until := daylight(nights, tonight)
		return !now.Before(from) && now.Before(until)
	}
	return false
}

// mountDevice adds the frames' api under /api/device. Frames authenticate
// with a bearer token, not a cookie, so there's no origin check.
func (s *Server) mountDevice(r chi.Router) {
	r.Route("/v1", func(r chi.Router) {
		r.With(httprate.Limit(registerRateLimit, time.Minute, httprate.WithKeyFuncs(s.clientKey),
			httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
				writeError(w, http.StatusTooManyRequests, "too many new frames, wait a minute")
			}))).Post("/register", s.registerDevice)
		r.Group(func(r chi.Router) {
			r.Use(s.withDevice)
			r.Get("/state", s.getDeviceState)
			r.Get("/bundle", s.getBundle)
			r.Get("/images/{file}", s.getImage)
			r.Post("/events", s.postEvents)
		})
	})
	r.With(s.withDevice).Get("/compat/current.png", s.getCompatPNG)
}

// mountDevices adds claiming and managing frames, inside mountAccount's group.
func (s *Server) mountDevices(r chi.Router) {
	r.With(httprate.Limit(authRateLimit, time.Minute, httprate.WithKeyFuncs(s.clientKey),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusTooManyRequests, "too many tries, wait a minute")
		}))).Get("/devices/claim/{code}", s.getClaimStatus)
	r.Post("/devices/claim", s.requireUser(s.claimDevice))
	r.Get("/me/devices", s.requireUser(s.getMyDevices))
	r.Patch("/me/devices/{id}", s.requireUser(s.patchDevice))
	r.Delete("/me/devices/{id}", s.requireUser(s.unclaimDevice))
	r.Get("/me/devices/{id}/preview.png", s.requireUser(s.getDevicePreview))
}

func (s *Server) mountAdminDevices(r chi.Router) {
	r.Get("/devices", s.getAllDevices)
	r.Post("/devices", s.prepareFrame)
	r.Delete("/devices/{id}", s.deleteDevice)
	r.Post("/devices/{id}/invite", s.markFrame)
	r.Delete("/devices/{id}/invite", s.unmarkFrame)
}

// --- which frame is asking

type deviceKey struct{}

func deviceFrom(ctx context.Context) store.Device {
	d, _ := ctx.Value(deviceKey{}).(store.Device)
	return d
}

// withDevice reads "Authorization: Bearer <token>" (or ?token=, for firmware
// that can only fetch a url) and puts the frame in the context.
func (s *Server) withDevice(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			token = r.URL.Query().Get("token")
		}
		if token == "" {
			writeError(w, http.StatusUnauthorized, "send the frame's token")
			return
		}
		d, err := s.store.DeviceByToken(r.Context(), strings.TrimSpace(token))
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				s.internalError(w, err)
				return
			}
			// the frame should register again
			writeError(w, http.StatusUnauthorized, "unknown frame")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), deviceKey{}, d)))
	})
}

func (s *Server) claimURL(code string) string {
	return strings.TrimRight(s.cfg.BaseURL, "/") + "/claim?code=" + code
}

func (s *Server) registerDevice(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if r.ContentLength != 0 && !decode(w, r, &req) {
		return
	}
	ssid, pass := truncate(req.APSSID, 32), truncate(req.APPassword, 63)
	d, token, err := s.store.CreateDevice(r.Context(), truncate(req.Firmware, 40), ssid, pass, s.now())
	if err != nil {
		s.internalError(w, err)
		return
	}
	slog.Info("frame registered", "device", d.ID, "fw", d.Firmware)
	writeJSON(w, http.StatusCreated, RegisterResponse{DeviceID: d.ID, Token: token, ClaimCode: d.ClaimCode, ClaimURL: s.claimURL(d.ClaimCode)})
}

func (s *Server) getDeviceState(w http.ResponseWriter, r *http.Request) {
	d := deviceFrom(r.Context())
	if err := s.store.CheckIn(r.Context(), d.ID, s.now(), 0, "", ""); err != nil {
		s.internalError(w, err)
		return
	}
	s.writeState(w, r, d)
}

func (s *Server) writeState(w http.ResponseWriter, r *http.Request, d store.Device) {
	b, err := s.bundleFor(r, d)
	if err != nil {
		s.internalError(w, err)
		return
	}
	now := s.now()
	set := settingsOf(d.Settings)
	st := DeviceState{Claimed: d.Claimed(), Name: d.Name, BundleVersion: b.Version,
		NextCheckinS: int(nextCheckin(now, d, set).Seconds()), ServerTime: now.UTC(),
		TZ: schedule.Boston.String(), Tonight: schedule.Tonight(now), Settings: set}
	if !d.Claimed() {
		st.ClaimCode, st.ClaimURL = d.ClaimCode, s.claimURL(d.ClaimCode)
	}
	metrics.DeviceCheckins.Inc()
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, st)
}

// nextCheckin is how long a frame should sleep before checking in again.
func nextCheckin(now time.Time, d store.Device, set DeviceSettings) time.Duration {
	if !d.Claimed() {
		return unclaimedCheckin
	}
	if d.Power == "usb" && !set.Paused {
		return usbCheckin
	}
	t := now.In(schedule.Boston)
	next := time.Date(t.Year(), t.Month(), t.Day(), dailyCheckinHour, 5, 0, 0, schedule.Boston)
	if !next.After(t) {
		next = next.AddDate(0, 0, 1)
	}
	return next.Sub(t).Round(time.Second)
}

func (s *Server) getBundle(w http.ResponseWriter, r *http.Request) {
	b, err := s.bundleFor(r, deviceFrom(r.Context()))
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if have := r.URL.Query().Get("have"); have != "" && have == b.Version {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

var imageFileRE = regexp.MustCompile(`^([0-9a-f]{64})\.bin\.gz$`)

// getImage serves a rendered frame. It never changes, so it can be cached
// for good.
func (s *Server) getImage(w http.ResponseWriter, r *http.Request) {
	m := imageFileRE.FindStringSubmatch(chi.URLParam(r, "file"))
	if m == nil {
		writeError(w, http.StatusNotFound, "no such image")
		return
	}
	bin, err := s.store.Image(r.Context(), m[1])
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such image")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Length", strconv.Itoa(len(bin)))
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+m[1]+`"`)
	w.Write(bin)
}

var eventTypeRE = regexp.MustCompile(`^[a-z_]{1,24}$`)

// postEvents is a frame's check-in: its battery, and what it queued since
// the last one. A "seen" logs the night for the frame's owner, within the
// same window as the web, and keeps any stars or note already given.
func (s *Server) postEvents(w http.ResponseWriter, r *http.Request) {
	d := deviceFrom(r.Context())
	var req EventsRequest
	if !decode(w, r, &req) {
		return
	}
	if len(req.Events) > maxEvents {
		writeError(w, http.StatusRequestEntityTooLarge, "send at most 200 events at a time")
		return
	}
	if req.Power != "" && req.Power != "usb" && req.Power != "battery" {
		writeError(w, http.StatusBadRequest, "power is usb or battery")
		return
	}
	now := s.now()
	tonight := schedule.Tonight(now)
	events := make([]store.DeviceEvent, 0, len(req.Events))
	for _, e := range req.Events {
		if !eventTypeRE.MatchString(e.Type) {
			writeError(w, http.StatusBadRequest, "bad event type")
			return
		}
		if e.At.IsZero() || e.At.After(now) {
			e.At = now // a frame's clock can drift or be unset
		}
		if e.Date == "" {
			e.Date = schedule.Tonight(e.At)
		} else if _, err := schedule.ParseDate(string(e.Date)); err != nil {
			writeError(w, http.StatusBadRequest, "event dates are YYYY-MM-DD")
			return
		}
		events = append(events, store.DeviceEvent{Type: e.Type, At: e.At, Date: e.Date, Detail: truncate(e.Detail, maxDetailLen)})
	}
	if err := s.store.AddEvents(r.Context(), d.ID, events, now); err != nil {
		s.internalError(w, err)
		return
	}
	for _, e := range events {
		if e.Type != "seen" || !d.Claimed() || !canLog(e.Date, tonight) {
			continue
		}
		fresh, err := s.store.MarkSeen(r.Context(), d.UserID, e.Date, now)
		if err != nil {
			s.internalError(w, err)
			return
		}
		if fresh {
			slog.Info("sighting from frame", "user", d.UserID, "device", d.ID, "date", e.Date)
		}
	}
	if err := s.store.CheckIn(r.Context(), d.ID, now, req.BatteryMV, req.Power, truncate(req.Firmware, 40)); err != nil {
		s.internalError(w, err)
		return
	}
	if req.APSSID != "" && req.APPassword != "" && (req.APSSID != d.APSSID || req.APPassword != d.APPassword) {
		if err := s.store.SetHotspot(r.Context(), d.ID, truncate(req.APSSID, 32), truncate(req.APPassword, 63)); err != nil {
			s.internalError(w, err)
			return
		}
	}
	d, err := s.store.DeviceByID(r.Context(), d.ID)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.writeState(w, r, d)
}

// getCompatPNG is for a first test with the frame's stock firmware, which
// can only fetch one picture from a url: what the frame should show now, in
// the panel's ideal colors (so the firmware's own dithering leaves it alone).
// ?landscape=1 turns it to the panel's 800×480; ?palette=measured previews.
func (s *Server) getCompatPNG(w http.ResponseWriter, r *http.Request) {
	d := deviceFrom(r.Context())
	now := s.now()
	if err := s.store.CheckIn(r.Context(), d.ID, now, 0, "", ""); err != nil {
		s.internalError(w, err)
		return
	}
	tonight := schedule.Tonight(now)
	data, err := s.frameData(r, d, tonight)
	if err != nil {
		s.internalError(w, err)
		return
	}
	set := settingsOf(d.Settings)
	view := render.View(set.DefaultView)
	switch {
	case !d.Claimed():
		view = render.ViewSetup
	case set.quiet(now):
		view = render.ViewGoodnight
	}
	data.Look = set.look(set.lightAt(now, data.Nights, tonight) && view != render.ViewGoodnight)
	f, err := render.Render(view, data)
	if err != nil {
		s.internalError(w, err)
		return
	}
	pal := render.Ideal
	if r.URL.Query().Get("palette") == render.Measured.Name {
		pal = render.Measured
	}
	var body []byte
	if r.URL.Query().Get("landscape") == "1" {
		body = f.PanelPNG(pal)
	} else {
		body = f.PNG(pal)
	}
	etag := `"` + f.SHA256()[:24] + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, no-cache")
	if strings.Contains(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(body)
}

// --- what a frame draws

// frameData gathers what a frame's views draw from: the nights around
// tonight and, once claimed, its owner's sightings and log.
func (s *Server) frameData(r *http.Request, d store.Device, tonight schedule.Date) (render.Data, error) {
	// the month views of every night in the bundle, and the week after the last
	from := firstOfMonth(tonight)
	if back := tonight.AddDays(-logWindow); back < from {
		from = back
	}
	last := tonight.AddDays(bundleNights) // the stale picture's night
	to := firstOfMonth(last).Time().AddDate(0, 1, -1)
	toDate := schedule.DateOf(to)
	if after := last.AddDays(6); after > toDate {
		toDate = after
	}
	nights, err := s.lightings(r, from, toDate)
	if err != nil {
		return render.Data{}, err
	}
	data := render.Data{Tonight: tonight, Nights: nights, BaseURL: s.cfg.BaseURL,
		LowBattery: d.Power == "battery" && d.BatteryMV > 0 && d.BatteryMV < lowBatteryMV,
		APSSID:     d.APSSID, APPassword: d.APPassword, PortalURL: portalURL}
	if !d.Claimed() {
		data.ClaimCode, data.ClaimURL = d.ClaimCode, s.claimURL(d.ClaimCode)
		return data, nil
	}
	logged, err := s.myNights(r, d.UserID)
	if err != nil {
		return render.Data{}, err
	}
	data.Sightings = map[schedule.Date]render.Sighting{}
	st := statsOf(logged, tonight)
	stats := &render.Stats{Seen: st.Seen, Rated: st.Rated, AvgRating: st.Average}
	for _, f := range st.Favorites {
		stats.Favorites = append(stats.Favorites, render.ColorCount{Color: f.Color, Count: f.Count})
	}
	for _, n := range logged {
		if n.Date >= from {
			data.Sightings[n.Date] = render.Sighting{Seen: true, Rating: n.Rating}
		}
		if len(stats.Nights) < 8 {
			stats.Nights = append(stats.Nights, render.RatedNight{Date: n.Date, Colors: n.Lighting.Colors,
				Cause: n.Lighting.Cause, Rating: n.Rating})
		}
		if n.Rating > 0 && len(stats.Recent) < 4 {
			stats.Recent = append(stats.Recent, render.RatedNight{Date: n.Date, Colors: n.Lighting.Colors,
				Cause: n.Lighting.Cause, Rating: n.Rating})
		}
	}
	data.Stats = stats
	return data, nil
}

func firstOfMonth(d schedule.Date) schedule.Date { return schedule.Date(string(d)[:8] + "01") }

// bundleViews are the views a frame needs each night: the ones it cycles
// through, its default, and "good night" when it has quiet hours.
func bundleViews(set DeviceSettings) []render.View {
	var out []render.View
	for _, v := range append([]string{set.DefaultView}, set.Views...) {
		if !slices.Contains(out, render.View(v)) && slices.Contains(dayViews, render.View(v)) {
			out = append(out, render.View(v))
		}
	}
	if set.QuietStart != "" {
		out = append(out, render.ViewGoodnight)
	}
	return out
}

// bundles caches built bundles by what went into them, so a frame checking
// in with nothing changed costs a couple of queries and no drawing.
type bundles struct {
	mu    sync.Mutex
	m     map[string]*Bundle
	order []string
}

const bundleCacheSize = 128

func (c *bundles) get(key string) *Bundle {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[key]
}

func (c *bundles) put(key string, b *Bundle) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]*Bundle{}
	}
	if _, ok := c.m[key]; !ok {
		c.order = append(c.order, key)
	}
	c.m[key] = b
	for len(c.order) > bundleCacheSize {
		delete(c.m, c.order[0])
		c.order = c.order[1:]
	}
}

// bundleFor draws (or finds already drawn) every picture a frame needs for
// the next week. Its version is a hash of the pictures, so it changes
// exactly when one of them would look different.
func (s *Server) bundleFor(r *http.Request, d store.Device) (*Bundle, error) {
	tonight := schedule.Tonight(s.now())
	data, err := s.frameData(r, d, tonight)
	if err != nil {
		return nil, err
	}
	set := settingsOf(d.Settings)
	views := bundleViews(set)
	in, err := json.Marshal(struct {
		Data     render.Data
		Views    []render.View
		Settings DeviceSettings
		Claimed  bool
	}{data, views, set, d.Claimed()})
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(in)
	key := hex.EncodeToString(sum[:])
	if b := s.bundles.get(key); b != nil {
		metrics.DeviceBundles.WithLabelValues("cached").Inc()
		return b, nil
	}

	// a picture for a view of a night, in a theme: "" for frames that
	// don't change with the daylight
	b := &Bundle{Tonight: tonight, Items: []BundleItem{}}
	add := func(v render.View, day schedule.Date, stale bool, theme string) error {
		dd := data
		dd.Tonight = day
		dd.Look = set.look(theme == themeLight || (theme == "" && set.Theme == themeLight))
		if stale {
			dd.StaleSince = tonight
		}
		f, err := render.Render(v, dd)
		if err != nil {
			return err
		}
		bin, sha := f.PanelBin(), f.SHA256()
		if err := s.store.SaveImage(r.Context(), sha, bin, s.now()); err != nil {
			return err
		}
		it := BundleItem{Date: day, View: string(v), Stale: stale, SHA256: sha, Size: len(bin),
			URL: "/api/device/v1/images/" + sha + ".bin.gz", Theme: theme}
		if theme == themeLight {
			from, until := daylight(data.Nights, day)
			it.From, it.Until = &from, &until
		}
		b.Items = append(b.Items, it)
		return nil
	}
	// a night's view: twice for "auto" frames, by night and by day
	addNight := func(v render.View, day schedule.Date, stale bool) error {
		if set.Theme != themeAuto {
			return add(v, day, stale, "")
		}
		if err := add(v, day, stale, themeDark); err != nil {
			return err
		}
		if v == render.ViewGoodnight || stale { // quiet hours are night; so is running out
			return nil
		}
		return add(v, day, stale, themeLight)
	}
	if !d.Claimed() {
		// the welcome screen, a night at a time, until someone claims it
		for i := range bundleNights {
			if err := add(render.ViewSetup, tonight.AddDays(i), false, ""); err != nil {
				return nil, err
			}
		}
	} else {
		for i := range bundleNights {
			for _, v := range views {
				if err := addNight(v, tonight.AddDays(i), false); err != nil {
					return nil, err
				}
			}
		}
		if err := addNight(render.View(set.DefaultView), tonight.AddDays(bundleNights), true); err != nil {
			return nil, err
		}
	}
	// the Wi-Fi screen, for when no network answers: it has no night
	wifiTheme := ""
	if set.Theme == themeAuto {
		wifiTheme = themeDark
	}
	if err := add(render.ViewWifi, tonight, false, wifiTheme); err != nil {
		return nil, err
	}
	b.Items[len(b.Items)-1].Date = ""
	h := sha256.New()
	for _, it := range b.Items {
		h.Write([]byte(string(it.Date) + " " + it.View + " " + strconv.FormatBool(it.Stale) + " " + it.Theme + " " + it.SHA256 + "\n"))
		if it.From != nil {
			h.Write([]byte(it.From.UTC().Format(time.RFC3339) + " " + it.Until.UTC().Format(time.RFC3339) + "\n"))
		}
	}
	b.Version = hex.EncodeToString(h.Sum(nil))[:16]
	s.bundles.put(key, b)
	metrics.DeviceBundles.WithLabelValues("rendered").Inc()
	return b, nil
}

// --- claiming and managing frames, signed in

func deviceInfoOf(d store.Device) DeviceInfo {
	info := DeviceInfo{ID: d.ID, Name: d.Name, ClaimedAt: d.ClaimedAt, BatteryMV: d.BatteryMV, Power: d.Power,
		Firmware: d.Firmware, Settings: settingsOf(d.Settings)}
	if !d.CheckinAt.IsZero() {
		t := d.CheckinAt
		info.CheckinAt = &t
	}
	return info
}

func cleanDeviceName(name string) (string, bool) {
	name = strings.Join(strings.Fields(name), " ")
	return name, utf8.RuneCountInString(name) <= maxDeviceNameLen
}

func (s *Server) claimDevice(w http.ResponseWriter, r *http.Request) {
	var req ClaimRequest
	if !decode(w, r, &req) {
		return
	}
	name, ok := cleanDeviceName(req.Name)
	if !ok {
		writeError(w, http.StatusBadRequest, "keep the name to 40 characters")
		return
	}
	if name == "" {
		name = "Frame"
	}
	me := userFrom(r.Context())
	d, err := s.store.Claim(r.Context(), req.Code, me.ID, name, s.now())
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "that code isn't on a frame waiting to be set up")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	slog.Info("frame claimed", "device", d.ID, "user", me.ID)
	writeJSON(w, http.StatusOK, deviceInfoOf(d))
}

func (s *Server) getMyDevices(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.UserDevices(r.Context(), userFrom(r.Context()).ID)
	if err != nil {
		s.internalError(w, err)
		return
	}
	out := make([]DeviceInfo, len(list))
	for i, d := range list {
		out[i] = deviceInfoOf(d)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

func idParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "no such frame")
		return 0, false
	}
	return id, true
}

// myDevice is a frame the signed-in user owns.
func (s *Server) myDevice(w http.ResponseWriter, r *http.Request) (store.Device, bool) {
	id, ok := idParam(w, r)
	if !ok {
		return store.Device{}, false
	}
	d, err := s.store.DeviceByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && d.UserID != userFrom(r.Context()).ID) {
		writeError(w, http.StatusNotFound, "no such frame")
		return store.Device{}, false
	}
	if err != nil {
		s.internalError(w, err)
		return store.Device{}, false
	}
	return d, true
}

func (s *Server) patchDevice(w http.ResponseWriter, r *http.Request) {
	d, ok := s.myDevice(w, r)
	if !ok {
		return
	}
	var req DeviceUpdate
	if !decode(w, r, &req) {
		return
	}
	name, set := d.Name, settingsOf(d.Settings)
	if req.Name != nil {
		n, ok := cleanDeviceName(*req.Name)
		if !ok || n == "" {
			writeError(w, http.StatusBadRequest, "give the frame a name of up to 40 characters")
			return
		}
		name = n
	}
	if req.Settings != nil {
		if msg := validSettings(*req.Settings); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		set = req.Settings.withDefaults()
	}
	raw, _ := json.Marshal(set)
	if err := s.store.UpdateDevice(r.Context(), d.ID, d.UserID, name, raw); err != nil {
		s.internalError(w, err)
		return
	}
	d.Name, d.Settings = name, raw
	writeJSON(w, http.StatusOK, deviceInfoOf(d))
}

// getDevicePreview draws one of the frame's views, as the panel would show
// it, with choices that may not be saved yet: ?view=tonight&light=1
// &scene=river&words=color. The settings page shows it as you choose.
func (s *Server) getDevicePreview(w http.ResponseWriter, r *http.Request) {
	d, ok := s.myDevice(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	view := render.View(q.Get("view"))
	if view == "" {
		view = render.ViewTonight
	}
	if !slices.Contains(dayViews, view) && view != render.ViewGoodnight {
		writeError(w, http.StatusBadRequest, "unknown view")
		return
	}
	set := settingsOf(d.Settings)
	if v := q.Get("scene"); v != "" {
		set.Scene = v
	}
	if v := q.Get("words"); v != "" {
		set.Words = v
	}
	if msg := validSettings(set); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	data, err := s.frameData(r, d, schedule.Tonight(s.now()))
	if err != nil {
		s.internalError(w, err)
		return
	}
	data.Look = set.look(q.Get("light") == "1")
	f, err := render.Render(view, data)
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Write(f.PNG(render.Measured))
}

// unclaimDevice lets go of a frame. It goes back to its setup screen on its
// next sync, ready for someone to claim it again.
func (s *Server) unclaimDevice(w http.ResponseWriter, r *http.Request) {
	d, ok := s.myDevice(w, r)
	if !ok {
		return
	}
	if err := s.store.Unclaim(r.Context(), d.ID, d.UserID); err != nil {
		s.internalError(w, err)
		return
	}
	slog.Info("frame unclaimed", "device", d.ID, "user", d.UserID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getAllDevices(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Devices(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	out := make([]AdminDevice, len(list))
	for i, d := range list {
		out[i] = AdminDevice{DeviceInfo: deviceInfoOf(d), Owner: d.Owner, ClaimCode: d.ClaimCode, CreatedAt: d.CreatedAt}
		if d.Invites(s.now()) {
			t := d.InviteUntil
			out[i].InviteUntil = &t
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// deleteDevice forgets a frame entirely: its token stops working and it
// has to register again.
func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	err := s.store.DeleteDevice(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such frame")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	slog.Info("frame deleted", "device", id, "by", userFrom(r.Context()).ID)
	w.WriteHeader(http.StatusNoContent)
}

// getClaimStatus says whether a code is on a frame waiting to be claimed, and
// whether it would also make an account, so /claim knows what to offer.
func (s *Server) getClaimStatus(w http.ResponseWriter, r *http.Request) {
	d, err := s.store.DeviceByClaimCode(r.Context(), chi.URLParam(r, "code"))
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, ClaimStatus{Waiting: err == nil, Invites: err == nil && d.Invites(s.now())})
}

// prepareFrame makes a frame's identity before it's flashed: an unclaimed
// device, marked for a new person when asked. The token is shown only here.
func (s *Server) prepareFrame(w http.ResponseWriter, r *http.Request) {
	var req PrepareFrame
	if r.ContentLength != 0 && !decode(w, r, &req) {
		return
	}
	cfg, err := s.PrepareFrame(r.Context(), userFrom(r.Context()).ID, req.Invite)
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, cfg)
}

// PrepareFrame makes a new frame's identity, for the admin page and the
// "prulights frame new" command. byUserID is who marks it, when invite.
func (s *Server) PrepareFrame(ctx context.Context, byUserID int64, invite bool) (FrameConfig, error) {
	now := s.now()
	d, token, err := s.store.CreateDevice(ctx, "", "", "", now)
	if err != nil {
		return FrameConfig{}, err
	}
	if invite {
		if err := s.store.MarkForInvite(ctx, d.ID, byUserID, now.Add(frameInviteTTL)); err != nil {
			return FrameConfig{}, err
		}
	}
	slog.Info("frame prepared", "device", d.ID, "invite", invite)
	return FrameConfig{Version: 1, Server: strings.TrimRight(s.cfg.BaseURL, "/"), DeviceID: d.ID, Token: token,
		ClaimCode: d.ClaimCode, AP: FrameAP{SSID: d.APSSID, Password: d.APPassword}}, nil
}

func (s *Server) markFrame(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	err := s.store.MarkForInvite(r.Context(), id, userFrom(r.Context()).ID, s.now().Add(frameInviteTTL))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no unclaimed frame with that id")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unmarkFrame(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	err := s.store.UnmarkInvite(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such frame")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
