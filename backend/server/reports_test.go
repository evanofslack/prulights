package server

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/evanofslack/prulights/schedule"
)

// seeded's clock is 7pm on Dec 12, 2026; the 12th is Blue · Pink · Gold.
func TestReportWindow(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)
	for date, want := range map[string]int{
		"2026-12-12": 200, // tonight
		"2026-12-05": 200, // a week ago
		"2026-12-04": 422, // too long ago
		"2026-12-25": 200, // ahead: the schedule can be wrong too
		"tomorrow":   400,
	} {
		if rec := call(t, s, "PUT", "/api/v1/me/reports/"+date, `{"colors":["red"]}`, mom); rec.Code != want {
			t.Errorf("PUT %s = %d, want %d: %s", date, rec.Code, want, rec.Body.String())
		}
	}
	for body, want := range map[string]int{
		`{"colors":[]}`:        400,
		`{"colors":["plaid"]}`: 400,
		`{"colors":["red","red","red","red","red","red","red"]}`:       400,
		`{"colors":["red"],"note":"` + strings.Repeat("é", 281) + `"}`: 400,
		`{"colors":["red","green"],"note":"  red and green  "}`:        200,
	} {
		if rec := call(t, s, "PUT", "/api/v1/me/reports/2026-12-12", body, mom); rec.Code != want {
			t.Errorf("%.40s = %d, want %d", body, rec.Code, want)
		}
	}
	if rec := call(t, s, "PUT", "/api/v1/me/reports/2026-12-12", `{"colors":["red"]}`, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out = %d", rec.Code)
	}
	if rec := call(t, s, "GET", "/api/v1/reports/2026-12-12", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out read = %d", rec.Code)
	}
}

func TestReportsAccepted(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)
	_, dad := person(t, s, "Dad", false)
	_, evan := person(t, s, "Evan", true)
	call(t, s, "PUT", "/api/v1/me/reports/2026-12-12", `{"colors":["red","green"],"note":"  christmasy  "}`, mom)
	call(t, s, "PUT", "/api/v1/me/reports/2026-12-12", `{"colors":["red"]}`, dad)
	call(t, s, "PUT", "/api/v1/me/reports/2026-12-12", `{"colors":["red","green"]}`, evan)

	night := decodeAs[NightReports](t, call(t, s, "GET", "/api/v1/reports/2026-12-12", "", dad).Body.Bytes())
	if len(night.Reports) != 3 || night.Reports[0].Name != "Mom" || night.Reports[0].Note != "christmasy" ||
		night.Reports[0].Mine || !night.Reports[1].Mine || !night.CanReport || night.Reports[0].Status != "open" {
		t.Errorf("night = %+v", night)
	}

	if rec := call(t, s, "GET", "/api/v1/admin/reports", "", mom); rec.Code != http.StatusForbidden {
		t.Errorf("non-admin = %d", rec.Code)
	}
	list := decodeAs[AdminReports](t, call(t, s, "GET", "/api/v1/admin/reports", "", evan).Body.Bytes())
	if len(list.Nights) != 1 || len(list.Nights[0].Reports) != 3 ||
		!slices.Equal(list.Nights[0].Suggested, []schedule.ColorKey{schedule.Red, schedule.Green}) ||
		!slices.Equal(list.Nights[0].Lighting.Colors, []schedule.ColorKey{schedule.Blue, schedule.Pink, schedule.Gold}) {
		t.Fatalf("admin list = %+v", list)
	}

	// no body: the suggested colors; the cause is kept
	if rec := call(t, s, "POST", "/api/v1/admin/reports/2026-12-12/accept", "", evan); rec.Code != 200 {
		t.Fatalf("accept = %d %s", rec.Code, rec.Body.String())
	}
	l, _ := getJSON[schedule.Lighting](t, s, "/api/v1/lightings/2026-12-12")
	if !slices.Equal(l.Colors, []schedule.ColorKey{schedule.Red, schedule.Green}) || !strings.Contains(l.Cause, "Stepping Strong") {
		t.Errorf("corrected night = %+v", l)
	}
	if list := decodeAs[AdminReports](t, call(t, s, "GET", "/api/v1/admin/reports", "", evan).Body.Bytes()); len(list.Nights) != 0 {
		t.Errorf("still open: %+v", list)
	}
	if rec := call(t, s, "POST", "/api/v1/admin/reports/2026-12-12/accept", "", evan); rec.Code != http.StatusNotFound {
		t.Errorf("accept twice = %d", rec.Code)
	}
	night = decodeAs[NightReports](t, call(t, s, "GET", "/api/v1/reports/2026-12-12", "", dad).Body.Bytes())
	if night.Reports[0].Status != "accepted" {
		t.Errorf("status = %+v", night.Reports[0])
	}
}

func TestReportsDismissed(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)
	_, evan := person(t, s, "Evan", true)
	call(t, s, "PUT", "/api/v1/me/reports/2026-12-10", `{"colors":["green"]}`, mom)

	// an admin can pick other colors than the reports
	call(t, s, "PUT", "/api/v1/me/reports/2026-12-13", `{"colors":["green"]}`, mom)
	if rec := call(t, s, "POST", "/api/v1/admin/reports/2026-12-13/accept", `{"colors":["plaid"]}`, evan); rec.Code != 400 {
		t.Errorf("bad colors = %d", rec.Code)
	}
	if rec := call(t, s, "POST", "/api/v1/admin/reports/2026-12-13/accept", `{"colors":["teal"]}`, evan); rec.Code != 200 {
		t.Errorf("accept teal = %d %s", rec.Code, rec.Body.String())
	}
	if l, _ := getJSON[schedule.Lighting](t, s, "/api/v1/lightings/2026-12-13"); !slices.Equal(l.Colors, []schedule.ColorKey{schedule.Teal}) {
		t.Errorf("13th = %+v", l.Colors)
	}

	if rec := call(t, s, "POST", "/api/v1/admin/reports/2026-12-10/dismiss", "", evan); rec.Code != http.StatusNoContent {
		t.Fatalf("dismiss = %d", rec.Code)
	}
	if l, _ := getJSON[schedule.Lighting](t, s, "/api/v1/lightings/2026-12-10"); !slices.Equal(l.Colors, []schedule.ColorKey{schedule.Red}) {
		t.Errorf("dismissing changed the night: %+v", l.Colors)
	}
	if rec := call(t, s, "POST", "/api/v1/admin/reports/2026-12-10/dismiss", "", evan); rec.Code != http.StatusNotFound {
		t.Errorf("dismiss twice = %d", rec.Code)
	}

	// reporting again opens it again; withdrawing removes it
	call(t, s, "PUT", "/api/v1/me/reports/2026-12-10", `{"colors":["green","red"]}`, mom)
	if list := decodeAs[AdminReports](t, call(t, s, "GET", "/api/v1/admin/reports", "", evan).Body.Bytes()); len(list.Nights) != 1 {
		t.Errorf("reopened: %+v", list)
	}
	if rec := call(t, s, "DELETE", "/api/v1/me/reports/2026-12-10", "", mom); rec.Code != http.StatusNoContent {
		t.Errorf("withdraw = %d", rec.Code)
	}
	if list := decodeAs[AdminReports](t, call(t, s, "GET", "/api/v1/admin/reports", "", evan).Body.Bytes()); len(list.Nights) != 0 {
		t.Errorf("withdrawn: %+v", list)
	}
}
