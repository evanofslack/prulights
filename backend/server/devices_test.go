package server

import (
	"bytes"
	"compress/gzip"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/evanofslack/prulights/render"
	"github.com/evanofslack/prulights/schedule"
)

// dev makes a request as a frame would: a bearer token, no cookie or origin.
func dev(t *testing.T, s *Server, method, path, body, token string, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func register(t *testing.T, s *Server) RegisterResponse {
	t.Helper()
	rec := dev(t, s, "POST", "/api/device/v1/register", `{"firmware":"emulator"}`, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register = %d %s", rec.Code, rec.Body.String())
	}
	return decodeAs[RegisterResponse](t, rec.Body.Bytes())
}

func stateOf(t *testing.T, s *Server, token string) DeviceState {
	t.Helper()
	rec := dev(t, s, "GET", "/api/device/v1/state", "", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("state = %d %s", rec.Code, rec.Body.String())
	}
	return decodeAs[DeviceState](t, rec.Body.Bytes())
}

func bundleOf(t *testing.T, s *Server, token string) Bundle {
	t.Helper()
	rec := dev(t, s, "GET", "/api/device/v1/bundle", "", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("bundle = %d %s", rec.Code, rec.Body.String())
	}
	return decodeAs[Bundle](t, rec.Body.Bytes())
}

// seeded's clock is 7pm on Dec 12, 2026.
func TestDeviceLifecycle(t *testing.T) {
	s := seeded(t)
	mom, session := person(t, s, "Mom", false)

	if rec := dev(t, s, "GET", "/api/device/v1/state", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token = %d", rec.Code)
	}
	if rec := dev(t, s, "GET", "/api/device/v1/state", "", "nope"); rec.Code != http.StatusUnauthorized {
		t.Errorf("bad token = %d", rec.Code)
	}

	// a new frame shows its setup screen until someone claims it
	reg := register(t, s)
	if reg.Token == "" || !strings.HasSuffix(reg.ClaimURL, "/claim?code="+reg.ClaimCode) {
		t.Fatalf("register = %+v", reg)
	}
	st := stateOf(t, s, reg.Token)
	if st.Claimed || st.ClaimCode != reg.ClaimCode || st.Tonight != "2026-12-12" || st.NextCheckinS != 120 ||
		st.TZ != "America/New_York" || st.BundleVersion == "" {
		t.Errorf("unclaimed state = %+v", st)
	}
	b := bundleOf(t, s, reg.Token)
	// the welcome screen a night at a time, and the Wi-Fi screen
	if len(b.Items) != 8 || b.Items[0].View != "setup" || b.Items[6].Date != "2026-12-18" ||
		b.Items[7].View != "wifi" || b.Items[7].Date != "" || b.Version != st.BundleVersion {
		t.Fatalf("unclaimed bundle = %+v", b)
	}
	if rec := dev(t, s, "GET", "/api/device/v1/bundle?have="+b.Version, "", reg.Token); rec.Code != http.StatusNotModified {
		t.Errorf("bundle with have = %d", rec.Code)
	}

	// the image is the panel's framebuffer, gzipped
	img := dev(t, s, "GET", b.Items[0].URL, "", reg.Token)
	if img.Code != http.StatusOK || img.Header().Get("Content-Type") != "application/gzip" || img.Body.Len() != b.Items[0].Size {
		t.Fatalf("image = %d %s %d", img.Code, img.Header().Get("Content-Type"), img.Body.Len())
	}
	zr, err := gzip.NewReader(img.Body)
	if err != nil {
		t.Fatal(err)
	}
	if raw, _ := io.ReadAll(zr); len(raw) != render.PanelBytes {
		t.Errorf("unpacked image is %d bytes", len(raw))
	}
	if rec := dev(t, s, "GET", "/api/device/v1/images/"+strings.Repeat("0", 64)+".bin.gz", "", reg.Token); rec.Code != http.StatusNotFound {
		t.Errorf("missing image = %d", rec.Code)
	}
	if rec := dev(t, s, "GET", b.Items[0].URL, "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("image without a token = %d", rec.Code)
	}

	// claiming: the code as typed, by someone signed in
	if rec := call(t, s, "POST", "/api/v1/devices/claim", `{"code":"`+reg.ClaimCode+`"}`, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("claim signed out = %d", rec.Code)
	}
	if rec := call(t, s, "POST", "/api/v1/devices/claim", `{"code":"zzz-zzz"}`, session); rec.Code != http.StatusNotFound {
		t.Errorf("claim bad code = %d", rec.Code)
	}
	typed := strings.ToLower(strings.ReplaceAll(reg.ClaimCode, "-", " "))
	rec := call(t, s, "POST", "/api/v1/devices/claim", `{"code":"`+typed+`","name":"Kitchen"}`, session)
	if rec.Code != http.StatusOK {
		t.Fatalf("claim = %d %s", rec.Code, rec.Body.String())
	}
	info := decodeAs[DeviceInfo](t, rec.Body.Bytes())
	if info.Name != "Kitchen" || info.Settings.DefaultView != "tonight" || len(info.Settings.Views) != 4 {
		t.Errorf("claimed = %+v", info)
	}
	if rec := call(t, s, "POST", "/api/v1/devices/claim", `{"code":"`+reg.ClaimCode+`"}`, session); rec.Code != http.StatusNotFound {
		t.Errorf("claim twice = %d", rec.Code)
	}

	// claimed: a week of every view, plus the picture for when it runs out
	st = stateOf(t, s, reg.Token)
	if !st.Claimed || st.Name != "Kitchen" || st.ClaimCode != "" || st.BundleVersion == b.Version {
		t.Errorf("claimed state = %+v", st)
	}
	b = bundleOf(t, s, reg.Token)
	if len(b.Items) != 7*4+2 {
		t.Fatalf("claimed bundle has %d items", len(b.Items))
	}
	if wifi := b.Items[len(b.Items)-1]; wifi.View != "wifi" {
		t.Errorf("last item = %+v, want the Wi-Fi screen", wifi)
	}
	last := b.Items[len(b.Items)-2]
	if b.Items[0].Date != "2026-12-12" || b.Items[0].View != "tonight" || !last.Stale || last.Date != "2026-12-19" {
		t.Errorf("bundle items: first %+v, last %+v", b.Items[0], last)
	}
	if again := bundleOf(t, s, reg.Token); again.Version != b.Version {
		t.Errorf("nothing changed but the version did")
	}

	// a long press logs tonight for mom; too old a night is ignored
	rec = dev(t, s, "POST", "/api/device/v1/events", `{"battery_mv":3400,"power":"battery","firmware":"0.2.0","events":[
		{"type":"long","at":"2026-12-13T00:00:00Z"},
		{"type":"seen","at":"2026-12-13T00:00:01Z"},
		{"type":"seen","at":"2026-12-13T00:00:02Z","date":"2026-12-01"}]}`, reg.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("events = %d %s", rec.Code, rec.Body.String())
	}
	st = decodeAs[DeviceState](t, rec.Body.Bytes())
	if st.BundleVersion == b.Version {
		t.Error("a sighting and a low battery didn't change the bundle")
	}
	// on battery it sleeps until 12:05 tomorrow
	if want := int((17*time.Hour + 5*time.Minute).Seconds()); st.NextCheckinS != want {
		t.Errorf("next check-in on battery = %d, want %d", st.NextCheckinS, want)
	}
	xs, _ := s.store.UserSightings(t.Context(), mom.ID, "2026-11-01", "2026-12-31")
	if len(xs) != 1 || xs[0].Date != "2026-12-12" {
		t.Errorf("mom's sightings = %+v", xs)
	}
	evs, _ := s.store.DeviceEvents(t.Context(), reg.DeviceID, 10)
	if len(evs) != 3 {
		t.Errorf("events = %+v", evs)
	}
	for body, want := range map[string]int{
		`{"events":[{"type":"Bad Type"}]}`:               400,
		`{"power":"solar"}`:                              400,
		`{"events":[{"type":"seen","date":"tomorrow"}]}`: 400,
		`{"events":[` + strings.Repeat(`{"type":"short"},`, 200) + `{"type":"short"}]}`: 413,
	} {
		if rec := dev(t, s, "POST", "/api/device/v1/events", body, reg.Token); rec.Code != want {
			t.Errorf("events %.40s = %d, want %d", body, rec.Code, want)
		}
	}

	// settings: one view and quiet hours
	devPath := "/api/v1/me/devices/" + itoa(reg.DeviceID)
	for body, want := range map[string]int{
		`{"settings":{"default_view":"tonight","views":[]}}`:                                                    400,
		`{"settings":{"default_view":"tonight","views":["setup"]}}`:                                             400,
		`{"settings":{"default_view":"tonight","views":["week","week"]}}`:                                       400,
		`{"settings":{"default_view":"tonight","views":["tonight"],"quiet_start":"22:00"}}`:                     400,
		`{"settings":{"default_view":"tonight","views":["tonight"],"quiet_start":"25:00","quiet_end":"07:00"}}`: 400,
		`{"name":""}`: 400,
		`{"settings":{"default_view":"tonight","views":["tonight"],"theme":"sepia"}}`:  400,
		`{"settings":{"default_view":"tonight","views":["tonight"],"scene":"harbor"}}`: 400,
		`{"settings":{"default_view":"tonight","views":["tonight"],"words":"lots"}}`:   400,
	} {
		if rec := call(t, s, "PATCH", devPath, body, session); rec.Code != want {
			t.Errorf("PATCH %.60s = %d, want %d", body, rec.Code, want)
		}
	}
	rec = call(t, s, "PATCH", devPath, `{"name":"Desk","settings":{"default_view":"tonight","views":["tonight"],"quiet_start":"22:00","quiet_end":"07:00"}}`, session)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d %s", rec.Code, rec.Body.String())
	}
	if b := bundleOf(t, s, reg.Token); len(b.Items) != 7*2+2 {
		t.Errorf("one view and good night: %d items", len(b.Items))
	}
	if list := decodeAs[[]DeviceInfo](t, call(t, s, "GET", "/api/v1/me/devices", "", session).Body.Bytes()); list[0].Settings.Theme != "dark" ||
		list[0].Settings.Scene != "skyline" || list[0].Settings.Words != "cause" {
		t.Errorf("left-out choices = %+v, want the defaults", list[0].Settings)
	}

	// dark at night, paper by day: each night's view twice, the paper one
	// from sunrise to lights-on; good night and the Wi-Fi screen only dark
	rec = call(t, s, "PATCH", devPath, `{"settings":{"default_view":"tonight","views":["tonight"],"quiet_start":"22:00","quiet_end":"07:00","theme":"auto","scene":"river","words":"none"}}`, session)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH auto = %d %s", rec.Code, rec.Body.String())
	}
	b = bundleOf(t, s, reg.Token)
	themes := map[string]int{}
	for _, it := range b.Items {
		themes[it.View+" "+it.Theme]++
		if (it.Theme == "light") != (it.From != nil && it.Until != nil) {
			t.Errorf("%s %s %s: window %v–%v", it.View, it.Date, it.Theme, it.From, it.Until)
		}
		// sunrise around 7; lights on at 5pm (listed) or sunset around 4:12
		if it.From != nil && (it.From.In(schedule.Boston).Hour() != 7 || it.Until.In(schedule.Boston).Hour() < 16 || it.Until.In(schedule.Boston).Hour() > 17) {
			t.Errorf("december daylight %s: %s–%s", it.Date, it.From.In(schedule.Boston), it.Until.In(schedule.Boston))
		}
	}
	if themes["tonight dark"] != 8 || themes["tonight light"] != 7 || themes["goodnight dark"] != 7 ||
		themes["goodnight light"] != 0 || themes["wifi dark"] != 1 {
		t.Errorf("auto bundle = %v", themes)
	}
	for q, want := range map[string]int{"": 200, "?view=month&light=1&scene=tower&words=color": 200,
		"?view=goodnight": 200, "?view=setup": 400, "?scene=harbor": 400} {
		rec := call(t, s, "GET", devPath+"/preview.png"+q, "", session)
		if rec.Code != want || (want == 200 && rec.Header().Get("Content-Type") != "image/png") {
			t.Errorf("preview%s = %d %s", q, rec.Code, rec.Header().Get("Content-Type"))
		}
	}

	_, other := person(t, s, "Evan", false)
	if rec := call(t, s, "GET", devPath+"/preview.png", "", other); rec.Code != http.StatusNotFound {
		t.Errorf("someone else's preview = %d", rec.Code)
	}
	if rec := call(t, s, "PATCH", devPath, `{"name":"Mine"}`, other); rec.Code != http.StatusNotFound {
		t.Errorf("someone else's frame = %d", rec.Code)
	}
	list := decodeAs[[]DeviceInfo](t, call(t, s, "GET", "/api/v1/me/devices", "", session).Body.Bytes())
	if len(list) != 1 || list[0].Name != "Desk" || list[0].BatteryMV != 3400 || list[0].Firmware != "0.2.0" || list[0].CheckinAt == nil {
		t.Errorf("my devices = %+v", list)
	}

	// letting go of it sends it back to setup with a new code
	if rec := call(t, s, "DELETE", devPath, "", session); rec.Code != http.StatusNoContent {
		t.Fatalf("unclaim = %d", rec.Code)
	}
	st = stateOf(t, s, reg.Token)
	if st.Claimed || st.ClaimCode == "" || st.ClaimCode == reg.ClaimCode {
		t.Errorf("after unclaim = %+v", st)
	}
}

func TestCompatPNG(t *testing.T) {
	s := seeded(t)
	reg := register(t, s)
	rec := dev(t, s, "GET", "/api/device/compat/current.png?token="+reg.Token, "", "")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("compat = %d %s", rec.Code, rec.Body.String())
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(rec.Body.Bytes()))
	if err != nil || cfg.Width != render.Width || cfg.Height != render.Height {
		t.Errorf("portrait = %+v %v", cfg, err)
	}
	// ideal colors by default, so the stock firmware's dithering leaves them be
	if img, err := png.Decode(bytes.NewReader(rec.Body.Bytes())); err != nil || img.At(4, 796) != render.Ideal.Colors[render.Black] {
		t.Errorf("night = %v, want the ideal black (%v)", img.At(4, 796), err)
	}
	etag := rec.Header().Get("ETag")
	if rec := dev(t, s, "GET", "/api/device/compat/current.png", "", reg.Token, "If-None-Match", etag); rec.Code != http.StatusNotModified {
		t.Errorf("same picture = %d", rec.Code)
	}
	rec = dev(t, s, "GET", "/api/device/compat/current.png?landscape=1", "", reg.Token)
	if cfg, err := png.DecodeConfig(bytes.NewReader(rec.Body.Bytes())); err != nil || cfg.Width != render.PanelWidth || cfg.Height != render.PanelHeight {
		t.Errorf("landscape = %+v %v", cfg, err)
	}
	if rec := dev(t, s, "GET", "/api/device/compat/current.png", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token = %d", rec.Code)
	}
}

func TestAdminDevices(t *testing.T) {
	s := seeded(t)
	_, admin := person(t, s, "Evan", true)
	_, mom := person(t, s, "Mom", false)
	reg := register(t, s)
	if rec := call(t, s, "GET", "/api/v1/admin/devices", "", mom); rec.Code != http.StatusForbidden {
		t.Errorf("not an admin = %d", rec.Code)
	}
	list := decodeAs[[]AdminDevice](t, call(t, s, "GET", "/api/v1/admin/devices", "", admin).Body.Bytes())
	if len(list) != 1 || list[0].ClaimCode != reg.ClaimCode || list[0].Owner != "" {
		t.Errorf("devices = %+v", list)
	}
	if rec := call(t, s, "DELETE", "/api/v1/admin/devices/"+itoa(reg.DeviceID), "", admin); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", rec.Code)
	}
	if rec := dev(t, s, "GET", "/api/device/v1/state", "", reg.Token); rec.Code != http.StatusUnauthorized {
		t.Errorf("deleted frame's token = %d", rec.Code)
	}
}

func TestQuietHours(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 12, 12, h, m, 0, 0, schedule.Boston) }
	night := DeviceSettings{QuietStart: "22:00", QuietEnd: "07:00"}
	day := DeviceSettings{QuietStart: "13:00", QuietEnd: "15:00"}
	for _, c := range []struct {
		set  DeviceSettings
		t    time.Time
		want bool
	}{
		{night, at(23, 0), true}, {night, at(3, 0), true}, {night, at(7, 0), false}, {night, at(21, 59), false},
		{day, at(14, 0), true}, {day, at(15, 0), false}, {DeviceSettings{}, at(3, 0), false},
	} {
		if got := c.set.quiet(c.t); got != c.want {
			t.Errorf("%s–%s at %s = %v", c.set.QuietStart, c.set.QuietEnd, c.t.Format("15:04"), got)
		}
	}
}

func TestPrepareAndAssign(t *testing.T) {
	s := seeded(t)
	_, admin := person(t, s, "Evan", true)
	kim, member := person(t, s, "Kim", false)
	mom, momSession := person(t, s, "Mom", false)
	views := func(token string) map[string]bool {
		t.Helper()
		out := map[string]bool{}
		for _, it := range bundleOf(t, s, token).Items {
			out[it.View] = true
		}
		return out
	}
	prepare := func(body string) FrameConfig {
		t.Helper()
		rec := call(t, s, "POST", "/api/v1/admin/devices", body, admin)
		if rec.Code != http.StatusCreated {
			t.Fatalf("prepare %s = %d %s", body, rec.Code, rec.Body.String())
		}
		return decodeAs[FrameConfig](t, rec.Body.Bytes())
	}

	if rec := call(t, s, "POST", "/api/v1/admin/devices", `{}`, member); rec.Code != http.StatusForbidden {
		t.Errorf("prepare as a member = %d", rec.Code)
	}
	if rec := call(t, s, "POST", "/api/v1/admin/devices", `{"for_user_id":999}`, admin); rec.Code != http.StatusNotFound {
		t.Errorf("prepare for nobody = %d", rec.Code)
	}

	// a frame prepared for no one waits for its code
	cfg := prepare(`{}`)
	if cfg.Version != 1 || cfg.Token == "" || cfg.Server != "http://localhost:8080" || cfg.ClaimCode == "" ||
		!strings.HasPrefix(cfg.AP.SSID, "prulights-") || cfg.AP.Password == "" {
		t.Fatalf("config = %+v", cfg)
	}
	if st := stateOf(t, s, cfg.Token); st.Claimed || st.ClaimCode != cfg.ClaimCode {
		t.Errorf("prepared frame state = %+v", st)
	}
	status := func(code string) ClaimStatus {
		t.Helper()
		return decodeAs[ClaimStatus](t, call(t, s, "GET", "/api/v1/devices/claim/"+code, "", "").Body.Bytes())
	}
	if got := status(strings.ToLower(cfg.ClaimCode)); !got.Waiting {
		t.Errorf("waiting status = %+v", got)
	}
	if got := status("ZZZ-ZZZ"); got.Waiting {
		t.Errorf("unknown code status = %+v", got)
	}
	if v := views(cfg.Token); !v["setup"] || v["tonight"] {
		t.Errorf("unclaimed bundle views = %v", v)
	}

	// weeks later, the admin gives it to mom
	owner := "/api/v1/admin/devices/" + itoa(cfg.DeviceID) + "/owner"
	if rec := call(t, s, "PUT", owner, `{"user_id":`+itoa(mom.ID)+`}`, member); rec.Code != http.StatusForbidden {
		t.Errorf("assign as a member = %d", rec.Code)
	}
	if rec := call(t, s, "PUT", owner, `{"user_id":999}`, admin); rec.Code != http.StatusNotFound {
		t.Errorf("assign to nobody = %d", rec.Code)
	}
	if rec := call(t, s, "PUT", "/api/v1/admin/devices/999/owner", `{"user_id":`+itoa(mom.ID)+`}`, admin); rec.Code != http.StatusNotFound {
		t.Errorf("assign no frame = %d", rec.Code)
	}
	if rec := call(t, s, "PUT", owner, `{"user_id":`+itoa(mom.ID)+`}`, admin); rec.Code != http.StatusOK {
		t.Fatalf("assign = %d %s", rec.Code, rec.Body.String())
	}
	if st := stateOf(t, s, cfg.Token); !st.Claimed || st.ClaimCode != "" {
		t.Errorf("assigned frame state = %+v", st)
	}
	if v := views(cfg.Token); v["setup"] || !v["tonight"] {
		t.Errorf("assigned bundle views = %v", v)
	}
	if got := status(cfg.ClaimCode); got.Waiting {
		t.Error("an assigned frame's old code still waits")
	}
	mine := decodeAs[[]DeviceInfo](t, call(t, s, "GET", "/api/v1/me/devices", "", momSession).Body.Bytes())
	if len(mine) != 1 || mine[0].ID != cfg.DeviceID {
		t.Errorf("mom's frames = %+v", mine)
	}
	list := decodeAs[[]AdminDevice](t, call(t, s, "GET", "/api/v1/admin/devices", "", admin).Body.Bytes())
	if len(list) != 1 || list[0].Owner != "Mom" || list[0].OwnerID != mom.ID || list[0].ClaimCode != "" {
		t.Errorf("admin list = %+v", list)
	}

	// back to setup, with a new code
	if rec := call(t, s, "DELETE", owner, "", admin); rec.Code != http.StatusNoContent {
		t.Fatalf("release = %d", rec.Code)
	}
	if rec := call(t, s, "DELETE", owner, "", admin); rec.Code != http.StatusNotFound {
		t.Errorf("release twice = %d", rec.Code)
	}
	if st := stateOf(t, s, cfg.Token); st.Claimed || st.ClaimCode == "" || st.ClaimCode == cfg.ClaimCode {
		t.Errorf("released frame state = %+v", st)
	}

	// a frame prepared for someone is theirs from the start
	kims := prepare(`{"for_user_id":` + itoa(kim.ID) + `}`)
	if st := stateOf(t, s, kims.Token); !st.Claimed {
		t.Errorf("frame prepared for kim = %+v", st)
	}
	mine = decodeAs[[]DeviceInfo](t, call(t, s, "GET", "/api/v1/me/devices", "", member).Body.Bytes())
	if len(mine) != 1 || mine[0].ID != kims.DeviceID {
		t.Errorf("kim's frames = %+v", mine)
	}
}

func TestHotspot(t *testing.T) {
	s := seeded(t)
	rec := dev(t, s, "POST", "/api/device/v1/register", `{"firmware":"x","ap_ssid":"frame-1","ap_password":"secret123"}`, "")
	reg := decodeAs[RegisterResponse](t, rec.Body.Bytes())
	d, _ := s.store.DeviceByID(t.Context(), reg.DeviceID)
	if d.APSSID != "frame-1" || d.APPassword != "secret123" {
		t.Errorf("hotspot from register = %q %q", d.APSSID, d.APPassword)
	}
	before := bundleOf(t, s, reg.Token)
	dev(t, s, "POST", "/api/device/v1/events", `{"ap_ssid":"frame-2","ap_password":"another1"}`, reg.Token)
	d, _ = s.store.DeviceByID(t.Context(), reg.DeviceID)
	if d.APSSID != "frame-2" {
		t.Errorf("hotspot after a reflash = %q", d.APSSID)
	}
	after := bundleOf(t, s, reg.Token)
	if before.Items[7].SHA256 == after.Items[7].SHA256 || before.Items[0].SHA256 != after.Items[0].SHA256 {
		t.Error("only the Wi-Fi screen should change with the hotspot")
	}
}
