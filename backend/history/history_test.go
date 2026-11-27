package history

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/evanofslack/prulights/schedule"
	"github.com/evanofslack/prulights/store"
)

// Every copy parses cleanly. This is what keeps Apply from failing at startup.
func TestPagesParse(t *testing.T) {
	parsed, _, err := Pages()
	if err != nil {
		t.Fatal(err)
	}
	snaps, _ := Manifest()
	if len(parsed) != len(snaps) {
		t.Fatalf("%d pages parsed, %d in the manifest", len(parsed), len(snaps))
	}
	for i, p := range parsed {
		if len(p.Warnings) > 0 {
			t.Errorf("%s: warnings %q", snaps[i].File(), p.Warnings)
		}
	}
	for i := 1; i < len(snaps); i++ {
		a, b := snaps[i-1], snaps[i]
		if a.URL == b.URL && a.Year == b.Year && a.Snapshot >= b.Snapshot {
			t.Errorf("%s comes before %s; the manifest goes oldest first", a.File(), b.File())
		}
	}
}

func TestApply(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// what the live scraper has: the 2026 page and this December
	live := "https://www.prudentialcenter.com/experiences/events/2026-tower-lightings/"
	nights := "https://www.prudentialcenter.com/experiences/events/31-nights-of-light/"
	l2026 := []schedule.Lighting{{Date: "2026-10-01", Colors: []schedule.ColorKey{"pink"}, Cause: "Ellie Fund", Kind: "nonprofit", Program: schedule.ProgramRegular}}
	d2026 := []schedule.Lighting{{Date: "2026-12-10", Colors: []schedule.ColorKey{"red"}, Cause: "Dress for Success", Kind: "nonprofit", Program: schedule.ProgramNights}}
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	if _, err := st.ApplyPage(ctx, live, 2026, l2026, now); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ApplyPage(ctx, nights, 2026, d2026, now); err != nil {
		t.Fatal(err)
	}

	done, applied, err := Apply(ctx, st, now)
	if err != nil {
		t.Fatal(err)
	}
	// 68 listed in 2025 (one, Feb 4, later moved to Feb 6) and two Decembers
	if !done || applied.Added != 69+31+31 || applied.Removed != 1 {
		t.Errorf("Apply = %v %+v; want done, 131 added, 1 removed", done, applied)
	}

	count := func(from, to schedule.Date) int {
		ls, err := st.Lightings(ctx, from, to)
		if err != nil {
			t.Fatal(err)
		}
		return len(ls)
	}
	if n := count("2025-01-01", "2025-11-30"); n != 68 {
		t.Errorf("Jan-Nov 2025: %d nights, want 68", n)
	}
	if n := count("2025-12-01", "2025-12-31"); n != 31 {
		t.Errorf("Dec 2025: %d nights, want 31", n)
	}
	if n := count("2024-12-01", "2024-12-31"); n != 31 {
		t.Errorf("Dec 2024: %d nights, want 31", n)
	}
	if n := count("2026-01-01", "2026-12-31"); n != 2 {
		t.Errorf("2026: %d nights, want the 2 the scraper had", n)
	}
	if n := count("2025-02-04", "2025-02-04"); n != 0 {
		t.Errorf("Feb 4 2025 was moved to Feb 6 on the site, but it's still listed")
	}

	for _, c := range []struct {
		date   schedule.Date
		cause  string
		colors []schedule.ColorKey
	}{
		{"2025-12-01", "Team Hoyt/The Hoyt Foundation", []schedule.ColorKey{"blue", "white", "red"}},
		{"2024-12-01", "Boston Bruins Foundation", []schedule.ColorKey{"gold"}},
		{"2025-09-11", "9/11 Memorial & Museum “Tribute in Light”", []schedule.ColorKey{"blue"}},
		{"2025-04-04", "Red Sox Opening Home Opener", []schedule.ColorKey{"red", "white", "blue"}},
	} {
		ls, _ := st.Lightings(ctx, c.date, c.date)
		if len(ls) != 1 || ls[0].Cause != c.cause || !slices.Equal(ls[0].Colors, c.colors) {
			t.Errorf("%s = %+v; want %s %v", c.date, ls, c.cause, c.colors)
		}
	}

	// once only
	done, applied, err = Apply(ctx, st, now.Add(time.Hour))
	if err != nil || done || applied != (store.Applied{}) {
		t.Errorf("second Apply = %v %+v %v; want nothing done", done, applied, err)
	}

	// the live scraper's next run of this December leaves the archived ones be
	if a, err := st.ApplyPage(ctx, nights, 2026, d2026, now.Add(2*time.Hour)); err != nil || a.Removed != 0 {
		t.Errorf("live 31 Nights apply = %+v %v; want nothing removed", a, err)
	}
	if n := count("2024-12-01", "2025-12-31"); n != 31+68+31 {
		t.Errorf("after a live scrape: %d archived nights, want 130", n)
	}
}
