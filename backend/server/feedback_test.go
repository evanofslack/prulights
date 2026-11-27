package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestRename(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)
	person(t, s, "Dad", false)
	call(t, s, "PUT", "/api/v1/me/sightings/2026-12-12", `{"rating":4}`, mom)

	// in order: the last rename is the name that sticks
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"name":"   "}`, 400},
		{`{"name":"` + strings.Repeat("a", 41) + `"}`, 400},
		{`{"name":"dad"}`, 409},             // names ignore case
		{`{"name":"mom"}`, 200},             // your own name in new case is fine
		{`{"name":"  Grandma   Jo "}`, 200}, // last
	} {
		if rec := call(t, s, "PUT", "/api/v1/me/name", c.body, mom); rec.Code != c.want {
			t.Errorf("%s = %d, want %d: %s", c.body, rec.Code, c.want, rec.Body.String())
		}
	}
	if me := decodeAs[MeResponse](t, call(t, s, "GET", "/api/v1/me", "", mom).Body.Bytes()); me.Name != "Grandma Jo" {
		t.Errorf("me = %+v", me)
	}
	// what they logged follows them
	night := decodeAs[NightSightings](t, call(t, s, "GET", "/api/v1/sightings/2026-12-12", "", mom).Body.Bytes())
	if len(night.Sightings) != 1 || night.Sightings[0].Name != "Grandma Jo" {
		t.Errorf("sightings = %+v", night)
	}
	if rec := call(t, s, "PUT", "/api/v1/me/name", `{"name":"X"}`, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out = %d", rec.Code)
	}
}

func TestFeedback(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)
	_, evan := person(t, s, "Evan", true)
	// in order: the list is newest first
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"text":"  "}`, 400},
		{`{"text":"` + strings.Repeat("é", 2001) + `"}`, 400},
		{`{"text":"  a bigger font please "}`, 204},
		{`{"text":"show the moon"}`, 204},
	} {
		if rec := call(t, s, "POST", "/api/v1/me/feedback", c.body, mom); rec.Code != c.want {
			t.Errorf("%.30s = %d, want %d", c.body, rec.Code, c.want)
		}
	}
	if rec := call(t, s, "POST", "/api/v1/me/feedback", `{"text":"hi"}`, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out = %d", rec.Code)
	}
	if rec := call(t, s, "GET", "/api/v1/admin/feedback", "", mom); rec.Code != http.StatusForbidden {
		t.Errorf("non-admin = %d", rec.Code)
	}
	list := decodeAs[[]FeedbackInfo](t, call(t, s, "GET", "/api/v1/admin/feedback", "", evan).Body.Bytes())
	if len(list) != 2 || list[1].Text != "a bigger font please" || list[1].Name != "Mom" || list[1].Done {
		t.Fatalf("list = %+v", list)
	}
	// done goes to the bottom
	first := list[0].ID
	if rec := call(t, s, "PUT", "/api/v1/admin/feedback/"+itoa(first), `{"done":true}`, evan); rec.Code != 204 {
		t.Fatalf("done = %d %s", rec.Code, rec.Body.String())
	}
	list = decodeAs[[]FeedbackInfo](t, call(t, s, "GET", "/api/v1/admin/feedback", "", evan).Body.Bytes())
	if list[1].ID != first || !list[1].Done {
		t.Errorf("after done = %+v", list)
	}
	if rec := call(t, s, "DELETE", "/api/v1/admin/feedback/"+itoa(first), "", evan); rec.Code != 204 {
		t.Errorf("delete = %d", rec.Code)
	}
	if rec := call(t, s, "DELETE", "/api/v1/admin/feedback/"+itoa(first), "", evan); rec.Code != 404 {
		t.Errorf("delete again = %d", rec.Code)
	}
}
