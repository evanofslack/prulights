package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/evanofslack/prulights/schedule"
	"github.com/go-chi/chi/v5"
	"go.yaml.in/yaml/v3"
)

const nightsURL = "https://www.prudentialcenter.com/experiences/events/31-nights-of-light/"

// seeded is a test server holding a few December nights, with the clock at
// 7pm on December 12, 2026.
func seeded(t *testing.T) *Server {
	t.Helper()
	s := newTestServer(t)
	s.now = func() time.Time { return time.Date(2026, 12, 12, 19, 0, 0, 0, schedule.Boston) }
	n := func(d schedule.Date, cause, url string, colors ...schedule.ColorKey) schedule.Lighting {
		return schedule.Lighting{Date: d, Cause: cause, CauseURL: url, Colors: colors, Kind: schedule.Classify(cause),
			Program: schedule.ProgramNights, Scheduled: true, SourceURL: nightsURL}
	}
	_, err := s.store.ApplyPage(context.Background(), nightsURL, 2026, []schedule.Lighting{
		n("2026-12-10", "Dress for Success", "http://boston.dressforsuccess.org", schedule.Red),
		n("2026-12-12", "The Gillian Remy Stepping Strong Center for Trauma Innovation", "http://bwhsteppingstrong.org",
			schedule.Blue, schedule.Pink, schedule.Gold),
		n("2026-12-13", "Read to a Child Inc", "https://readtoachild.org/", schedule.Blue, schedule.Orange, schedule.Green),
		n("2026-12-25", "Christmas", "", schedule.Red, schedule.Green),
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func getJSON[T any](t *testing.T, s *Server, path string) (T, *httptest.ResponseRecorder) {
	t.Helper()
	rec := get(t, s.Handler(), path)
	var v T
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatalf("%s: %v\n%s", path, err, rec.Body.String())
		}
	}
	return v, rec
}

func TestTonight(t *testing.T) {
	s := seeded(t)
	got, rec := getJSON[TonightResponse](t, s, "/api/v1/tonight")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	l := got.Lighting
	if got.Date != "2026-12-12" || !slices.Equal(l.Colors, []schedule.ColorKey{schedule.Blue, schedule.Pink, schedule.Gold}) {
		t.Errorf("tonight = %+v", got)
	}
	if l.LightsOnSource != schedule.LightsOnSource || l.LightsOn.In(schedule.Boston).Hour() != 17 {
		t.Errorf("31 nights should light at 5pm by the source, got %v %s", l.LightsOn, l.LightsOnSource)
	}
	if len(got.Upcoming) != 7 || got.Upcoming[0].Date != "2026-12-13" || got.Upcoming[1].Scheduled {
		t.Errorf("upcoming = %+v", got.Upcoming)
	}
	if got.PublishedThrough != "2026-12-25" {
		t.Errorf("published through %s", got.PublishedThrough)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" || rec.Header().Get("ETag") == "" {
		t.Errorf("headers = %v", rec.Header())
	}

	// at 1am it's still last night
	s.now = func() time.Time { return time.Date(2026, 12, 13, 1, 0, 0, 0, schedule.Boston) }
	if got, _ := getJSON[TonightResponse](t, s, "/api/v1/tonight"); got.Date != "2026-12-12" {
		t.Errorf("at 1am tonight = %s", got.Date)
	}
}

func TestLightings(t *testing.T) {
	s := seeded(t)
	got, rec := getJSON[LightingsResponse](t, s, "/api/v1/lightings?from=2026-12-01&to=2026-12-31")
	if rec.Code != http.StatusOK || len(got.Lightings) != 31 {
		t.Fatalf("status %d, %d nights", rec.Code, len(got.Lightings))
	}
	white := got.Lightings[0]
	if white.Scheduled || white.Kind != schedule.KindDefault || !slices.Equal(white.Colors, []schedule.ColorKey{schedule.White}) {
		t.Errorf("unlisted night = %+v", white)
	}
	if white.LightsOnSource != schedule.LightsOnSunset || white.LightsOn.IsZero() {
		t.Errorf("unlisted night lights on %v from %s", white.LightsOn, white.LightsOnSource)
	}
	if d := got.Lightings[9]; d.Date != "2026-12-10" || d.Cause != "Dress for Success" {
		t.Errorf("dec 10 = %+v", d)
	}

	listed, _ := getJSON[LightingsResponse](t, s, "/api/v1/lightings?from=2026-12-01&to=2026-12-31&fill=false")
	if len(listed.Lightings) != 4 {
		t.Errorf("fill=false gave %d nights", len(listed.Lightings))
	}
	empty, _ := getJSON[LightingsResponse](t, s, "/api/v1/lightings?from=2027-03-01&to=2027-03-02&fill=false")
	if empty.Lightings == nil {
		t.Error("an empty range should be [] not null")
	}

	def, _ := getJSON[LightingsResponse](t, s, "/api/v1/lightings")
	if def.From != "2026-12-12" || len(def.Lightings) != 31 {
		t.Errorf("default range %s..%s, %d nights", def.From, def.To, len(def.Lightings))
	}

	for _, bad := range []string{"?from=12-01-2026", "?to=nope", "?from=2026-12-10&to=2026-12-01",
		"?from=2026-01-01&to=2027-12-31", "?fill=maybe"} {
		if rec := get(t, s.Handler(), "/api/v1/lightings"+bad); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d", bad, rec.Code)
		}
	}
}

func TestLighting(t *testing.T) {
	s := seeded(t)
	got, rec := getJSON[schedule.Lighting](t, s, "/api/v1/lightings/2026-12-25")
	if rec.Code != http.StatusOK || got.Kind != schedule.KindHoliday || got.Cause != "Christmas" {
		t.Errorf("christmas = %d %+v", rec.Code, got)
	}
	got, _ = getJSON[schedule.Lighting](t, s, "/api/v1/lightings/2027-07-04")
	if got.Scheduled || got.Colors[0] != schedule.White {
		t.Errorf("an unlisted night = %+v", got)
	}
	if rec := get(t, s.Handler(), "/api/v1/lightings/tomorrow"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad date = %d", rec.Code)
	}
}

func TestETag(t *testing.T) {
	s := seeded(t)
	first := get(t, s.Handler(), "/api/v1/tonight")
	etag := first.Header().Get("ETag")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tonight", nil)
	req.Header.Set("If-None-Match", etag)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Errorf("If-None-Match = %d %q", rec.Code, rec.Body.String())
	}
	if cc := first.Header().Get("Cache-Control"); cc != "public, max-age=300" {
		t.Errorf("Cache-Control = %q", cc)
	}
}

func TestPaletteAndMeta(t *testing.T) {
	s := seeded(t)
	pal, _ := getJSON[PaletteResponse](t, s, "/api/v1/palette")
	if len(pal.Colors) != len(schedule.Palette()) {
		t.Errorf("palette has %d colors", len(pal.Colors))
	}
	meta, rec := getJSON[MetaResponse](t, s, "/api/v1/meta")
	if rec.Code != http.StatusOK || meta.Tonight != "2026-12-12" || meta.Timezone != "America/New_York" ||
		meta.PublishedThrough != "2026-12-25" || meta.LastScrape != nil || meta.Attribution == "" || meta.Dev {
		t.Errorf("meta = %+v", meta)
	}
}

func TestCalendar(t *testing.T) {
	s := seeded(t)
	rec := get(t, s.Handler(), "/api/v1/calendar.ics")
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/calendar") {
		t.Fatalf("status %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if n := strings.Count(body, "BEGIN:VEVENT"); n != 4 {
		t.Errorf("%d events, want the 4 listed nights", n)
	}
	unfolded := strings.ReplaceAll(body, "\r\n ", "")
	for _, want := range []string{
		"DTSTART;VALUE=DATE:20261212", "DTEND;VALUE=DATE:20261213",
		`SUMMARY:Pru: Blue\, Pink & Gold for The Gillian Remy Stepping Strong Center for Trauma Innovation`,
		"Lights on around 5:00pm.", "URL:http://localhost:8080/night?d=2026-12-12",
	} {
		if !strings.Contains(unfolded, want) {
			t.Errorf("calendar is missing %q", want)
		}
	}
	for _, line := range strings.Split(body, "\r\n") {
		if len(line) > 75 {
			t.Errorf("line longer than 75 octets: %q", line)
		}
	}
}

func TestRateLimit(t *testing.T) {
	s := newTestServer(t)
	s.cfg.PublicRateLimit = 2
	s.cfg.TrustProxy = true
	s = New(s.cfg, s.store)
	call := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/palette", nil)
		req.Header.Set("CF-Connecting-IP", ip)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	if call("1.1.1.1") != 200 || call("1.1.1.1") != 200 || call("1.1.1.1") != http.StatusTooManyRequests {
		t.Error("third request in a minute should be limited")
	}
	if call("2.2.2.2") != 200 {
		t.Error("another client shouldn't be limited")
	}
}

func TestClientIP(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:5555"
	req.Header.Set("X-Forwarded-For", "9.9.9.9, 10.0.0.1")
	if ip := s.clientIP(req); ip != "10.0.0.1" {
		t.Errorf("untrusted proxy: %s", ip)
	}
	s.cfg.TrustProxy = true
	if ip := s.clientIP(req); ip != "9.9.9.9" {
		t.Errorf("trusted proxy: %s", ip)
	}
}

func TestOpenAPI(t *testing.T) {
	s := newTestServer(t)
	rec := get(t, s.Handler(), "/api/v1/openapi.json")
	var doc map[string]any
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &doc) != nil || doc["openapi"] != "3.1.0" {
		t.Fatalf("openapi.json = %d %.200s", rec.Code, rec.Body.String())
	}
	if rec := get(t, s.Handler(), "/api/v1/openapi.yaml"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "openapi: 3.1.0") {
		t.Errorf("openapi.yaml = %d", rec.Code)
	}
}

// Every public route must be in the spec, and every path in the spec must be
// a route.
func TestSpecCoversRoutes(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(openapiYAML, &spec); err != nil {
		t.Fatal(err)
	}
	s := newTestServer(t)
	routes := map[string]bool{}
	chi.Walk(s.router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if p, ok := strings.CutPrefix(route, "/api/v1"); ok && method == http.MethodGet && !isPrivateRoute(p) {
			routes[p] = true
			if _, ok := spec.Paths[p]; !ok {
				t.Errorf("GET /api/v1%s is not in openapi.yaml", p)
			}
		}
		return nil
	})
	for p := range spec.Paths {
		if !routes[p] {
			t.Errorf("openapi.yaml has %s but there's no such route", p)
		}
	}
}

// isPrivateRoute is for routes that need a sign-in or a device token; they're
// not part of the public spec.
func isPrivateRoute(p string) bool {
	first, _, _ := strings.Cut(strings.TrimPrefix(p, "/"), "/")
	return slices.Contains([]string{"me", "auth", "admin", "devices", "sightings"}, first)
}
