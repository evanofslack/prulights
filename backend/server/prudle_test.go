package server

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

// seeded's tonight is Dec 12, 2026: blue, pink and gold for Stepping Strong.

func guessAs(t *testing.T, s *Server, token, date, colors string) (PrudleState, int) {
	t.Helper()
	rec := call(t, s, "POST", "/api/v1/me/prudle/"+date+"/guesses", `{"colors":`+colors+`}`, token)
	if rec.Code != http.StatusOK {
		return PrudleState{}, rec.Code
	}
	return decodeAs[PrudleState](t, rec.Body.Bytes()), rec.Code
}

func TestPrudleGame(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)

	rec := call(t, s, "GET", "/api/v1/me/prudle/2026-12-12", "", mom)
	st := decodeAs[PrudleState](t, rec.Body.Bytes())
	if rec.Code != 200 || !st.CanPlay || st.Answer != nil || !strings.HasPrefix(st.Cause, "The Gillian") || st.Tries != 6 {
		t.Fatalf("fresh game = %d %+v", rec.Code, st)
	}
	if strings.Contains(rec.Body.String(), "pink") {
		t.Errorf("the answer leaked: %s", rec.Body.String())
	}

	st, _ = guessAs(t, s, mom, "2026-12-12", `["blue","red","green"]`)
	if st.Turns[0].Count != "exact" || !slices.Equal(st.Found, []schedule.ColorKey{"blue"}) ||
		len(st.Out) != 2 || st.Answer != nil || st.Status != "playing" {
		t.Errorf("first guess = %+v", st)
	}
	for _, c := range []struct {
		body string
		want int
	}{
		{`["blue"]`, 400}, // found
		{`["red"]`, 400},  // out
		{`["mauve"]`, 400},
		{`["a","b","c"]`, 400},
		{`["pink","gold","teal"]`, 400}, // blue rides along: 4
		{`["pink"]`, 200},               // blue and pink: not yet
		{`["gold"]`, 200},               // blue, pink and gold
		{`["teal"]`, 422},               // over
	} {
		if _, code := guessAs(t, s, mom, "2026-12-12", c.body); code != c.want {
			t.Errorf("guess %s = %d, want %d", c.body, code, c.want)
		}
	}
	st = decodeAs[PrudleState](t, call(t, s, "GET", "/api/v1/me/prudle/2026-12-12", "", mom).Body.Bytes())
	if st.Status != "won" || st.CanPlay || len(st.Answer) != 3 || len(st.Turns) != 3 || !st.Turns[2].Picks[0].Carried {
		t.Errorf("won = %+v", st)
	}

	for date, want := range map[string]int{
		"2026-12-13": 422, // not open yet
		"2026-12-11": 404, // not listed: no Prudle
		"2026-11-30": 404, // not December
		"2026-12-10": 422, // past: look, don't play
	} {
		if _, code := guessAs(t, s, mom, date, `["red"]`); code != want {
			t.Errorf("guess %s = %d, want %d", date, code, want)
		}
	}
	// a past night shows its answer
	past := decodeAs[PrudleState](t, call(t, s, "GET", "/api/v1/me/prudle/2026-12-10", "", mom).Body.Bytes())
	if past.CanPlay || !slices.Equal(past.Answer, []schedule.ColorKey{"red"}) {
		t.Errorf("past = %+v", past)
	}
	if rec := call(t, s, "GET", "/api/v1/me/prudle/2026-12-12", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out = %d", rec.Code)
	}
}

func TestPrudleLoseAndPass(t *testing.T) {
	s := seeded(t)
	_, evan := person(t, s, "Evan", false)
	// tonight is blue, pink, gold: five wrong guesses still play, the sixth loses
	for i, c := range []string{"green", "teal", "black", "white", "silver"} {
		st, _ := guessAs(t, s, evan, "2026-12-12", `["`+c+`"]`)
		if st.Status != "playing" || len(st.Turns) != i+1 {
			t.Fatalf("miss %d = %+v", i+1, st)
		}
	}
	st, _ := guessAs(t, s, evan, "2026-12-12", `["rainbow"]`)
	if st.Status != "lost" || len(st.Answer) != 3 {
		t.Errorf("lost = %+v", st)
	}

	_, nana := person(t, s, "Nana", false)
	rec := call(t, s, "POST", "/api/v1/me/prudle/2026-12-12/pass", `{}`, nana)
	st = decodeAs[PrudleState](t, rec.Body.Bytes())
	if rec.Code != 200 || st.Status != "passed" || len(st.Answer) != 3 {
		t.Errorf("pass = %d %+v", rec.Code, st)
	}
	if _, code := guessAs(t, s, nana, "2026-12-12", `["blue"]`); code != 422 {
		t.Errorf("guess after pass = %d", code)
	}
}

func TestPrudleSummary(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)
	sum := decodeAs[PrudleSummary](t, call(t, s, "GET", "/api/v1/me/prudle", "", mom).Body.Bytes())
	if !sum.Season || !sum.Playable || sum.Off || sum.Done || len(sum.Games) != 0 {
		t.Errorf("before = %+v", sum)
	}
	// won last night and tonight: a streak of two
	s.now = func() time.Time { return time.Date(2026, 12, 10, 20, 0, 0, 0, schedule.Boston) }
	guessAs(t, s, mom, "2026-12-10", `["red"]`)
	s.now = func() time.Time { return time.Date(2026, 12, 12, 19, 0, 0, 0, schedule.Boston) }
	sum = decodeAs[PrudleSummary](t, call(t, s, "GET", "/api/v1/me/prudle", "", mom).Body.Bytes())
	if sum.Won != 1 || sum.Streak != 0 || sum.Done {
		t.Errorf("after the 10th = %+v (the 11th broke it)", sum)
	}
	guessAs(t, s, mom, "2026-12-12", `["blue","pink","gold"]`)
	sum = decodeAs[PrudleSummary](t, call(t, s, "GET", "/api/v1/me/prudle", "", mom).Body.Bytes())
	if sum.Won != 2 || sum.Streak != 1 || !sum.Done || len(sum.Games) != 2 {
		t.Errorf("after tonight = %+v", sum)
	}

	if rec := call(t, s, "PUT", "/api/v1/me/prudle/settings", `{"off":true}`, mom); rec.Code != 200 {
		t.Fatalf("settings = %d", rec.Code)
	}
	sum = decodeAs[PrudleSummary](t, call(t, s, "GET", "/api/v1/me/prudle", "", mom).Body.Bytes())
	if !sum.Off {
		t.Error("not off")
	}

	s.now = func() time.Time { return time.Date(2026, 11, 26, 19, 0, 0, 0, schedule.Boston) }
	sum = decodeAs[PrudleSummary](t, call(t, s, "GET", "/api/v1/me/prudle", "", mom).Body.Bytes())
	if sum.Season || sum.Playable {
		t.Errorf("November = %+v", sum)
	}
}

func TestPrudleNight(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)
	_, evan := person(t, s, "Evan", false)
	guessAs(t, s, mom, "2026-12-12", `["blue","red"]`)
	guessAs(t, s, mom, "2026-12-12", `["pink","gold"]`)

	night := func(token string) PrudleNight {
		return decodeAs[PrudleNight](t, call(t, s, "GET", "/api/v1/prudle/2026-12-12", "", token).Body.Bytes())
	}
	if n := night(evan); !n.Locked || len(n.Players) != 0 {
		t.Errorf("before Evan plays = %+v", n)
	}
	guessAs(t, s, evan, "2026-12-12", `["green"]`)
	if n := night(evan); !n.Locked {
		t.Errorf("still playing = %+v", n)
	}
	call(t, s, "POST", "/api/v1/me/prudle/2026-12-12/pass", `{}`, evan)
	rec := call(t, s, "GET", "/api/v1/prudle/2026-12-12", "", evan)
	n := decodeAs[PrudleNight](t, rec.Body.Bytes())
	if n.Locked || len(n.Players) != 2 || n.Players[0].Name != "Mom" || n.Players[0].Mine || !n.Players[1].Mine {
		t.Fatalf("after = %+v", n)
	}
	if !slices.Equal(n.Players[0].Rows[0].Marks, []string{"hit", "close"}) || len(n.Players[0].Rows) != 2 ||
		!slices.Equal(n.Players[0].Rows[1].Marks, []string{"hit", "hit", "hit"}) {
		t.Errorf("rows = %+v", n.Players[0].Rows)
	}
	for _, c := range []string{"blue", "pink", "gold", "green"} {
		if strings.Contains(rec.Body.String(), c) {
			t.Errorf("%s leaked: %s", c, rec.Body.String())
		}
	}
	if n := night(mom); n.Locked {
		t.Error("Mom is done")
	}
	if rec := call(t, s, "GET", "/api/v1/prudle/2026-12-12", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out = %d", rec.Code)
	}
}
