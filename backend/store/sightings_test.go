package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

func TestSightings(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	now := time.Date(2026, 12, 12, 23, 0, 0, 0, time.UTC)
	add := func(name string) User {
		inv, _ := s.CreateInvite(ctx, Invite{CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
		u, err := s.Redeem(ctx, inv, NewHandle(), name, []byte(name), []byte(`{}`), now)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	mom, evan := add("Mom"), add("Evan")

	s.SaveSighting(ctx, mom.ID, "2026-12-12", 5, "gorgeous in the fog", now)
	s.SaveSighting(ctx, evan.ID, "2026-12-12", 0, "", now.Add(time.Minute))
	s.SaveSighting(ctx, mom.ID, "2026-12-10", 3.5, "", now)
	s.SaveSighting(ctx, evan.ID, "2026-12-10", 4, "", now)
	if half, _ := s.NightSightings(ctx, "2026-12-10"); len(half) != 2 || half[0].Rating != 3.5 {
		t.Errorf("half stars = %+v", half)
	}
	if st, _ := s.NightStats(ctx, "2026-12-10"); st != (NightStats{Seen: 2, Rated: 2, Average: 3.75}) {
		t.Errorf("half star stats = %+v", st)
	}

	night, err := s.NightSightings(ctx, "2026-12-12")
	if err != nil || len(night) != 2 || night[0].Name != "Mom" || night[0].Rating != 5 || night[1].Rating != 0 {
		t.Errorf("night = %+v %v", night, err)
	}
	stats, _ := s.NightStats(ctx, "2026-12-12")
	if stats != (NightStats{Seen: 2, Rated: 1, Average: 5}) {
		t.Errorf("stats = %+v", stats)
	}
	if empty, _ := s.NightStats(ctx, "2026-12-01"); empty != (NightStats{}) {
		t.Errorf("empty night = %+v", empty)
	}

	// updating keeps one row and clears the rating
	s.SaveSighting(ctx, mom.ID, "2026-12-12", 0, "changed my mind", now.Add(time.Hour))
	mine, _ := s.UserSightings(ctx, mom.ID, "2026-12-01", "2026-12-31")
	if len(mine) != 2 || mine[0].Date != "2026-12-12" || mine[0].Rating != 0 || mine[0].Note != "changed my mind" ||
		!mine[0].UpdatedAt.After(mine[0].CreatedAt) {
		t.Errorf("mine = %+v", mine)
	}
	s.DeleteSighting(ctx, mom.ID, "2026-12-10")
	if counts, _ := s.SightingCounts(ctx); counts[mom.ID] != 1 || counts[evan.ID] != 2 {
		t.Errorf("counts = %v", counts)
	}
}

// A whole-star rating from before half stars (migration 007) comes through.
func TestHalfStarsMigration(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "p.db")
	s, err := Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 12, 12, 23, 0, 0, 0, time.UTC)
	inv, _ := s.CreateInvite(ctx, Invite{CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
	u, err := s.Redeem(ctx, inv, NewHandle(), "Mom", []byte("Mom"), []byte(`{}`), now)
	if err != nil {
		t.Fatal(err)
	}
	// back to how 001 left it, with a rating of 4 in the old column
	for _, q := range []string{
		`alter table sightings drop column half_stars`,
		`delete from migrations where version = 7`,
	} {
		if _, err := s.DB.ExecContext(ctx, q); err != nil {
			t.Fatal(q, err)
		}
	}
	if _, err := s.DB.ExecContext(ctx, `insert into sightings (user_id, date, rating, note, created_at, updated_at)
		values (?, '2026-12-10', 4, '', '', '')`, u.ID); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got, _ := s.NightSightings(ctx, "2026-12-10"); len(got) != 1 || got[0].Rating != 4 {
		t.Errorf("after migrating = %+v", got)
	}
}

func TestOverrides(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	now := time.Now()
	o := Override{Date: "2026-12-24", Colors: []schedule.ColorKey{schedule.Red, schedule.Green}, Cause: "Christmas Eve",
		Kind: schedule.KindHoliday, Note: "site had it wrong"}
	if err := s.SaveOverride(ctx, o, now); err != nil {
		t.Fatal(err)
	}
	o.Cause = "Christmas Eve!"
	s.SaveOverride(ctx, o, now)
	list, _ := s.Overrides(ctx)
	if len(list) != 1 || list[0].Cause != "Christmas Eve!" || len(list[0].Colors) != 2 {
		t.Errorf("overrides = %+v", list)
	}
	ls, _ := s.Lightings(ctx, "2026-12-24", "2026-12-24")
	if len(ls) != 1 || ls[0].Program != schedule.ProgramOverride {
		t.Errorf("lightings = %+v", ls)
	}
	if ok, _ := s.DeleteOverride(ctx, "2026-12-24"); !ok {
		t.Error("delete reported nothing")
	}
	if ok, _ := s.DeleteOverride(ctx, "2026-12-24"); ok {
		t.Error("deleted twice")
	}
}
