package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

func decodeAs[T any](t *testing.T, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	return v
}

// seeded's clock is 7pm on Dec 12, 2026.
func TestSightingWindow(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)
	for date, want := range map[string]int{
		"2026-12-12": 200, // tonight
		"2026-12-05": 200, // a week ago
		"2026-12-04": 422, // too long ago
		"2026-12-13": 422, // not yet
		"tomorrow":   400,
	} {
		if rec := call(t, s, "PUT", "/api/v1/me/sightings/"+date, `{"rating":4}`, mom); rec.Code != want {
			t.Errorf("PUT %s = %d, want %d: %s", date, rec.Code, want, rec.Body.String())
		}
	}
	// just after midnight it's still the 12th, so the 5th is still open
	s.now = func() time.Time { return time.Date(2026, 12, 13, 1, 0, 0, 0, schedule.Boston) }
	if rec := call(t, s, "PUT", "/api/v1/me/sightings/2026-12-05", `{}`, mom); rec.Code != 200 {
		t.Errorf("1am = %d", rec.Code)
	}
	for body, want := range map[string]int{
		`{"rating":6}`:  400,
		`{"rating":-1}`: 400,
		`{"note":"` + strings.Repeat("é", 281) + `"}`: 400,
		`{"note":"` + strings.Repeat("é", 280) + `"}`: 200,
	} {
		if rec := call(t, s, "PUT", "/api/v1/me/sightings/2026-12-12", body, mom); rec.Code != want {
			t.Errorf("%.30s = %d, want %d", body, rec.Code, want)
		}
	}
	if rec := call(t, s, "PUT", "/api/v1/me/sightings/2026-12-12", `{}`, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out = %d", rec.Code)
	}
}

func TestSharedSightings(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)
	_, evan := person(t, s, "Evan", true)
	call(t, s, "PUT", "/api/v1/me/sightings/2026-12-12", `{"rating":5,"note":"  gorgeous in the fog  "}`, mom)
	call(t, s, "PUT", "/api/v1/me/sightings/2026-12-12", `{}`, evan)

	rec := call(t, s, "GET", "/api/v1/sightings/2026-12-12", "", evan)
	night := decodeAs[NightSightings](t, rec.Body.Bytes())
	if len(night.Sightings) != 2 || night.Sightings[0].Name != "Mom" || night.Sightings[0].Note != "gorgeous in the fog" ||
		night.Sightings[0].Mine || !night.Sightings[1].Mine || !night.CanLog {
		t.Errorf("night = %+v", night)
	}
	if rec := call(t, s, "GET", "/api/v1/sightings/2026-12-12", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("names are for invitees only: %d", rec.Code)
	}

	// the public gets totals, no names
	stats, rec := getJSON[NightStatsResponse](t, s, "/api/v1/lightings/2026-12-12/stats")
	if rec.Code != 200 || stats.Seen != 2 || stats.Rated != 1 || stats.Average != 5 || strings.Contains(rec.Body.String(), "Mom") {
		t.Errorf("public stats = %d %s", rec.Code, rec.Body.String())
	}

	if rec := call(t, s, "DELETE", "/api/v1/me/sightings/2026-12-12", "", evan); rec.Code != http.StatusNoContent {
		t.Errorf("delete = %d", rec.Code)
	}
	night = decodeAs[NightSightings](t, call(t, s, "GET", "/api/v1/sightings/2026-12-12", "", evan).Body.Bytes())
	if len(night.Sightings) != 1 {
		t.Errorf("after delete = %+v", night)
	}
}

func TestMyStats(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)
	// Dec 10 red, Dec 11 nothing listed (white), Dec 12 blue/pink/gold, but not
	// tonight yet: the streak counts back from last night
	s.now = func() time.Time { return time.Date(2026, 12, 13, 19, 0, 0, 0, schedule.Boston) }
	call(t, s, "PUT", "/api/v1/me/sightings/2026-12-10", `{"rating":3}`, mom)
	call(t, s, "PUT", "/api/v1/me/sightings/2026-12-11", `{}`, mom)
	call(t, s, "PUT", "/api/v1/me/sightings/2026-12-12", `{"rating":5}`, mom)
	call(t, s, "PUT", "/api/v1/me/sightings/2026-12-08", `{}`, mom)

	st := decodeAs[MyStats](t, call(t, s, "GET", "/api/v1/me/stats", "", mom).Body.Bytes())
	if st.Seen != 4 || st.Rated != 2 || st.Average != 4 || st.Streak != 3 || st.LogFrom != "2026-12-06" {
		t.Errorf("stats = %+v", st)
	}
	// white on the unlisted night doesn't count; red, blue, gold, pink once each, alphabetical
	if len(st.Favorites) != 3 || st.Favorites[0].Color != schedule.Blue || st.Favorites[0].Count != 1 {
		t.Errorf("favorites = %+v", st.Favorites)
	}

	mine := decodeAs[MySightings](t, call(t, s, "GET", "/api/v1/me/sightings", "", mom).Body.Bytes())
	if len(mine.Nights) != 4 || mine.Nights[0].Date != "2026-12-12" || mine.Nights[0].Lighting.Cause == "" ||
		mine.Nights[1].Lighting.Scheduled {
		t.Errorf("my nights = %+v", mine.Nights)
	}

	_, evan := person(t, s, "Evan", false)
	empty := decodeAs[MyStats](t, call(t, s, "GET", "/api/v1/me/stats", "", evan).Body.Bytes())
	if empty.Seen != 0 || empty.Streak != 0 || empty.Favorites == nil {
		t.Errorf("empty stats = %+v", empty)
	}
}

func TestAdmin(t *testing.T) {
	s := seeded(t)
	_, admin := person(t, s, "Evan", true)
	_, mom := person(t, s, "Mom", false)
	call(t, s, "PUT", "/api/v1/me/sightings/2026-12-12", `{}`, mom)

	if rec := call(t, s, "GET", "/api/v1/admin/users", "", mom); rec.Code != http.StatusForbidden {
		t.Errorf("users as mom = %d", rec.Code)
	}
	users := decodeAs[[]UserInfo](t, call(t, s, "GET", "/api/v1/admin/users", "", admin).Body.Bytes())
	if len(users) != 2 || users[1].Name != "Mom" || users[1].Sightings != 1 {
		t.Errorf("users = %+v", users)
	}

	for body, want := range map[string]int{
		`{"colors":[]}`:                                     400,
		`{"colors":["plaid"]}`:                              400,
		`{"colors":["red"],"kind":"party"}`:                 400,
		`{"colors":["red"],"cause_url":"javascript:alert"}`: 400,
		`{"colors":["red","green"],"cause":"Christmas Eve","note":"the site had blue"}`: 200,
	} {
		if rec := call(t, s, "PUT", "/api/v1/admin/overrides/2026-12-24", body, admin); rec.Code != want {
			t.Errorf("override %s = %d, want %d", body, rec.Code, want)
		}
	}
	l, _ := getJSON[schedule.Lighting](t, s, "/api/v1/lightings/2026-12-24")
	if l.Program != schedule.ProgramOverride || l.Cause != "Christmas Eve" || l.Kind != schedule.KindHoliday {
		t.Errorf("corrected night = %+v", l)
	}
	list := decodeAs[[]OverrideInfo](t, call(t, s, "GET", "/api/v1/admin/overrides", "", admin).Body.Bytes())
	if len(list) != 1 || list[0].Note != "the site had blue" {
		t.Errorf("overrides = %+v", list)
	}
	if rec := call(t, s, "DELETE", "/api/v1/admin/overrides/2026-12-24", "", admin); rec.Code != http.StatusNoContent {
		t.Errorf("delete override = %d", rec.Code)
	}
	if rec := call(t, s, "DELETE", "/api/v1/admin/overrides/2026-12-24", "", admin); rec.Code != http.StatusNotFound {
		t.Errorf("delete again = %d", rec.Code)
	}
}

func TestScrapeNow(t *testing.T) {
	s := seeded(t)
	_, admin := person(t, s, "Evan", true)
	if rec := call(t, s, "POST", "/api/v1/admin/scrape", "", admin); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("no scraper = %d", rec.Code)
	}
	release := make(chan struct{})
	done := make(chan struct{})
	s.SetScraper(func(context.Context) error {
		<-release
		close(done)
		return nil
	})
	if rec := call(t, s, "POST", "/api/v1/admin/scrape", "", admin); rec.Code != http.StatusAccepted {
		t.Errorf("scrape now = %d", rec.Code)
	}
	if rec := call(t, s, "POST", "/api/v1/admin/scrape", "", admin); rec.Code != http.StatusConflict {
		t.Errorf("while running = %d", rec.Code)
	}
	st := decodeAs[ScrapeStatus](t, call(t, s, "GET", "/api/v1/admin/scrape", "", admin).Body.Bytes())
	if !st.Running || st.Runs == nil {
		t.Errorf("status = %+v", st)
	}
	close(release)
	<-done
	for range 100 {
		if !s.scraping.Load() {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if s.scraping.Load() {
		t.Error("still running after the scrape returned")
	}
}
