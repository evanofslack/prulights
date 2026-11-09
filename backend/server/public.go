package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/evanofslack/prulights/schedule"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
)

const (
	// maxRange is the most nights one lightings request returns.
	maxRange = 400
	// defaultRange is how many nights lightings returns without a "to".
	defaultRange = 31
	// upcomingNights is how many nights after tonight /tonight includes.
	upcomingNights = 7
	// attribution credits the source on every response that needs it.
	attribution = "Schedule from the Prudential Center (prudentialcenter.com). prulights is not affiliated with the Prudential Center, BXP or Prudential Financial."
)

// mountPublic adds the open, read-only api: no login, rate limited per
// client, cacheable, and readable from any origin.
func (s *Server) mountPublic(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(httprate.Limit(s.cfg.PublicRateLimit, time.Minute, httprate.WithKeyFuncs(s.clientKey),
			httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
				writeError(w, http.StatusTooManyRequests, "too many requests, slow down")
			})))
		r.Use(publicCORS)
		r.Get("/tonight", s.getTonight)
		r.Get("/lightings", s.getLightings)
		r.Get("/lightings/{date}", s.getLighting)
		r.Get("/lightings/{date}/stats", s.getNightStats)
		r.Get("/palette", s.getPalette)
		r.Get("/meta", s.getMeta)
		r.Get("/calendar.ics", s.getCalendar)
		r.Get("/openapi.yaml", s.getOpenAPIYAML)
		r.Get("/openapi.json", s.getOpenAPIJSON)
	})
}

// publicCORS lets any site read the public api.
func publicCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "ETag")
		next.ServeHTTP(w, r)
	})
}

// clientKey is the client's address for rate limits: the proxy's word for it
// when TRUST_PROXY is set (Cloudflare's CF-Connecting-IP, then the first
// X-Forwarded-For), else the connection's.
func (s *Server) clientKey(r *http.Request) (string, error) {
	return s.clientIP(r), nil
}

func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
			return ip
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			if ip := strings.TrimSpace(first); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// writeCached writes v as json with an ETag, answering a matching
// If-None-Match with 304. maxAge is how long clients and caches may keep it.
func writeCached(w http.ResponseWriter, r *http.Request, v any, maxAge time.Duration) {
	body, err := json.Marshal(v)
	if err != nil {
		slog.Error("encode response", "error", err)
		writeError(w, http.StatusInternalServerError, "encode response")
		return
	}
	body = append(body, '\n')
	writeCachedBytes(w, r, "application/json; charset=utf-8", body, maxAge)
}

func writeCachedBytes(w http.ResponseWriter, r *http.Request, contentType string, body []byte, maxAge time.Duration) {
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:12]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(maxAge.Seconds())))
	if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Write(body)
}

// lightings returns one merged lighting per night from from to to, white
// for the nights nothing lists.
func (s *Server) lightings(r *http.Request, from, to schedule.Date) ([]schedule.Lighting, error) {
	listed, err := s.store.Lightings(r.Context(), from, to)
	if err != nil {
		return nil, err
	}
	out := schedule.Fill(from, to, listed)
	for i := range out {
		if out[i].Scheduled {
			out[i].LightsOn, out[i].LightsOnSource = schedule.LightsOn(out[i].Date, out[i].Program)
		}
	}
	return out, nil
}

func (s *Server) getTonight(w http.ResponseWriter, r *http.Request) {
	tonight := schedule.Tonight(s.now())
	ls, err := s.lightings(r, tonight, tonight.AddDays(upcomingNights))
	if err != nil {
		s.internalError(w, err)
		return
	}
	through, err := s.store.PublishedThrough(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeCached(w, r, TonightResponse{Date: tonight, Lighting: ls[0], Upcoming: ls[1:], PublishedThrough: through}, 5*time.Minute)
}

func (s *Server) getLightings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from := schedule.Tonight(s.now())
	if v := q.Get("from"); v != "" {
		d, err := schedule.ParseDate(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "from: "+err.Error())
			return
		}
		from = d
	}
	to := from.AddDays(defaultRange - 1)
	if v := q.Get("to"); v != "" {
		d, err := schedule.ParseDate(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "to: "+err.Error())
			return
		}
		to = d
	}
	if to < from {
		writeError(w, http.StatusBadRequest, "to is before from")
		return
	}
	if n := from.DaysUntil(to) + 1; n > maxRange {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("at most %d nights per request, asked for %d", maxRange, n))
		return
	}
	fill := true
	if v := q.Get("fill"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "fill: want true or false")
			return
		}
		fill = b
	}
	ls, err := s.lightings(r, from, to)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if !fill {
		listed := ls[:0]
		for _, l := range ls {
			if l.Scheduled {
				listed = append(listed, l)
			}
		}
		ls = listed
	}
	through, err := s.store.PublishedThrough(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	if ls == nil {
		ls = []schedule.Lighting{}
	}
	writeCached(w, r, LightingsResponse{From: from, To: to, Lightings: ls, PublishedThrough: through}, 5*time.Minute)
}

func (s *Server) getLighting(w http.ResponseWriter, r *http.Request) {
	d, err := schedule.ParseDate(chi.URLParam(r, "date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ls, err := s.lightings(r, d, d)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeCached(w, r, ls[0], 5*time.Minute)
}

func (s *Server) getPalette(w http.ResponseWriter, r *http.Request) {
	writeCached(w, r, PaletteResponse{Colors: schedule.Palette()}, 24*time.Hour)
}

func (s *Server) getMeta(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	through, err := s.store.PublishedThrough(ctx)
	if err != nil {
		s.internalError(w, err)
		return
	}
	srcs, err := s.store.Sources(ctx)
	if err != nil {
		s.internalError(w, err)
		return
	}
	out := MetaResponse{
		Tonight:          schedule.Tonight(s.now()),
		Timezone:         schedule.Boston.String(),
		RolloverHour:     schedule.RolloverHour,
		PublishedThrough: through,
		Sources:          []MetaSource{},
		Attribution:      attribution,
		Version:          s.cfg.Version,
		Dev:              s.cfg.Dev,
	}
	if last, err := s.store.LastSuccess(ctx); err == nil && !last.IsZero() {
		out.LastScrape = &last
	}
	for _, src := range srcs {
		out.Sources = append(out.Sources, MetaSource{URL: src.URL, Title: src.Title, Year: src.Year,
			Program: src.Program, Nights: src.Items, CheckedAt: src.CheckedAt, ChangedAt: src.ChangedAt})
	}
	writeCached(w, r, out, time.Minute)
}

// getCalendar is an iCalendar feed of the listed nights from 60 days ago on,
// one all-day event each, for subscribing in a calendar app. Unlisted (white)
// nights are left out.
func (s *Server) getCalendar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	through, err := s.store.PublishedThrough(ctx)
	if err != nil {
		s.internalError(w, err)
		return
	}
	tonight := schedule.Tonight(s.now())
	from := tonight.AddDays(-60)
	if through == "" || through < tonight {
		through = tonight
	}
	if from.DaysUntil(through) > 2*maxRange {
		through = from.AddDays(2 * maxRange)
	}
	listed, err := s.store.Lightings(ctx, from, through)
	if err != nil {
		s.internalError(w, err)
		return
	}
	byDate := map[schedule.Date][]schedule.Lighting{}
	var dates []schedule.Date
	for _, l := range listed {
		if _, ok := byDate[l.Date]; !ok {
			dates = append(dates, l.Date)
		}
		byDate[l.Date] = append(byDate[l.Date], l)
	}

	var b bytes.Buffer
	line := func(s string) { b.WriteString(foldICS(s) + "\r\n") }
	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//prulights//Pru tower lights//EN")
	line("CALSCALE:GREGORIAN")
	line("METHOD:PUBLISH")
	line("X-WR-CALNAME:Pru tower lights")
	line("X-WR-CALDESC:What color is the Prudential Tower tonight?")
	line("X-WR-TIMEZONE:America/New_York")
	line("REFRESH-INTERVAL;VALUE=DURATION:PT12H")
	line("X-PUBLISHED-TTL:PT12H")
	stamp := s.now().UTC().Format("20060102T150405Z")
	for _, d := range dates {
		l := schedule.Merge(byDate[d])
		day := d.Time()
		start, end := day.Format("20060102"), day.AddDate(0, 0, 1).Format("20060102")
		summary := "Pru: " + colorNames(l.Colors)
		if l.Cause != "" {
			summary += " for " + l.Cause
		}
		desc := colorNames(l.Colors)
		if l.Cause != "" {
			desc += " for " + l.Cause
		}
		on, _ := schedule.LightsOn(d, l.Program)
		desc += ". Lights on around " + on.Format("3:04pm") + "."
		if l.CauseURL != "" {
			desc += " " + l.CauseURL
		}
		line("BEGIN:VEVENT")
		line("UID:" + string(d) + "@prulights.evanslack.dev")
		line("DTSTAMP:" + stamp)
		line("DTSTART;VALUE=DATE:" + start)
		line("DTEND;VALUE=DATE:" + end)
		line("SUMMARY:" + escapeICS(summary))
		line("DESCRIPTION:" + escapeICS(desc))
		line("URL:" + s.cfg.BaseURL + "/night?d=" + string(d))
		line("TRANSP:TRANSPARENT")
		line("END:VEVENT")
	}
	line("END:VCALENDAR")
	writeCachedBytes(w, r, "text/calendar; charset=utf-8", b.Bytes(), time.Hour)
}

// colorNames reads colors the way the source writes them: "Blue, Pink & Gold".
func colorNames(keys []schedule.ColorKey) string {
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		if c, ok := schedule.Lookup(k); ok {
			names = append(names, c.Label)
		} else {
			names = append(names, string(k))
		}
	}
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " & " + names[len(names)-1]
}

func escapeICS(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`).Replace(s)
}

// foldICS wraps a content line at 75 octets, as RFC 5545 asks, without
// splitting a utf-8 character.
func foldICS(s string) string {
	if len(s) <= 75 {
		return s
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		size := len(string(r))
		if n+size > 75 {
			b.WriteString("\r\n ")
			n = 1
		}
		b.WriteRune(r)
		n += size
	}
	return b.String()
}

func (s *Server) internalError(w http.ResponseWriter, err error) {
	slog.Error("api", "error", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
