package main

import (
	"context"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/evanofslack/prulights/schedule"
	"github.com/evanofslack/prulights/store"
)

func TestParseSeedNames(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
		bad  bool
	}{
		{in: "Logan,Shiv", want: []string{"Logan", "Shiv"}},
		{in: " roman , GREG,, ", want: []string{"Roman", "Greg"}},
		{in: "Logan,Bob", bad: true},
		{in: "Shiv,shiv", bad: true},
		{in: " , ", bad: true},
		{in: "Logan,Kendall,Roman,Shiv,Connor,Tom", bad: true},
	} {
		got, err := parseSeedNames(tc.in)
		if tc.bad != (err != nil) || !slices.Equal(got, tc.want) {
			t.Errorf("parseSeedNames(%q) = %v, %v", tc.in, got, err)
		}
	}
}

func TestSeed(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Date(2026, 11, 26, 22, 0, 0, 0, schedule.Boston)
	tonight := schedule.Tonight(now)

	// someone real, whose own sightings stay out of their Lately
	me, err := st.CreateUser(ctx, "Evan", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveSighting(ctx, me.ID, tonight, 5, "", now); err != nil {
		t.Fatal(err)
	}

	names := []string{"Logan", "Shiv", "Roman", "Greg"}
	run := func(reset bool) []seedSummary {
		out, err := seed(ctx, st, names, reset, rand.New(rand.NewPCG(1, 1)), now)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	first := run(false)
	for i, s := range first {
		if s.Nights == 0 {
			t.Errorf("%s logged nothing", s.Name)
		}
		if i > 0 && s.Nights > first[0].Nights {
			t.Errorf("%s (%d) logged more than %s (%d)", s.Name, s.Nights, first[0].Name, first[0].Nights)
		}
	}
	open, err := st.RecentSightings(ctx, me.ID, tonight.AddDays(-seedWindow), tonight, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) == 0 {
		t.Error("no open sightings to show in Lately")
	}

	count := func() (n int) {
		st.DB.QueryRowContext(ctx, `select count(*) from sightings`).Scan(&n)
		return n
	}
	before := count()
	if again := run(true); !slices.Equal(again, first) {
		t.Errorf("reset run = %v, want %v", again, first)
	}
	if after := count(); after != before {
		t.Errorf("sightings after reset = %d, want %d", after, before)
	}
	if _, err := st.UserByName(ctx, "Evan"); err != nil {
		t.Errorf("reset touched a real person: %v", err)
	}
}
