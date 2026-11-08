package store

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

const src = "https://www.prudentialcenter.com/experiences/events/31-nights-of-light/"

func night(d schedule.Date, cause string, colors ...schedule.ColorKey) schedule.Lighting {
	return schedule.Lighting{Date: d, Cause: cause, Colors: colors, Kind: schedule.KindNonprofit,
		Program: schedule.ProgramNights, Scheduled: true, SourceURL: src}
}

func TestApplyPage(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

	first := []schedule.Lighting{
		night("2026-12-01", "Life is Good", schedule.Gold, schedule.Maroon),
		night("2026-12-02", "Food Rescue", schedule.Orange),
		night("2026-12-03", "Public Garden", schedule.White, schedule.Green),
	}
	got, err := s.ApplyPage(ctx, src, 2026, first, now)
	if err != nil || got != (Applied{Added: 3}) {
		t.Fatalf("first apply = %+v, %v", got, err)
	}
	// unchanged
	if got, _ := s.ApplyPage(ctx, src, 2026, first, now); got != (Applied{}) {
		t.Fatalf("same page again = %+v", got)
	}
	// one recolored, one dropped, one new
	second := []schedule.Lighting{
		night("2026-12-01", "Life is Good", schedule.Gold),
		night("2026-12-03", "Public Garden", schedule.White, schedule.Green),
		night("2026-12-04", "Hanukkah", schedule.Blue),
	}
	if got, _ := s.ApplyPage(ctx, src, 2026, second, now); got != (Applied{Added: 1, Changed: 1, Removed: 1}) {
		t.Fatalf("second apply = %+v", got)
	}
	ls, err := s.Lightings(ctx, "2026-12-01", "2026-12-31")
	if err != nil {
		t.Fatal(err)
	}
	var dates []schedule.Date
	for _, l := range ls {
		dates = append(dates, l.Date)
	}
	if !slices.Equal(dates, []schedule.Date{"2026-12-01", "2026-12-03", "2026-12-04"}) {
		t.Fatalf("dates = %v", dates)
	}
	if !slices.Equal(ls[0].Colors, []schedule.ColorKey{schedule.Gold}) || !ls[0].Scheduled {
		t.Errorf("recolored night = %+v", ls[0])
	}
	// the first version comes back: two restored or recolored, Hanukkah gone
	if got, _ := s.ApplyPage(ctx, src, 2026, first, now); got != (Applied{Changed: 2, Removed: 1}) {
		t.Errorf("restored = %+v", got)
	}
	if n, _ := s.CountForSource(ctx, src, 2026); n != 3 {
		t.Errorf("count = %d", n)
	}
	if d, _ := s.PublishedThrough(ctx); d != "2026-12-03" {
		t.Errorf("published through %s", d)
	}
}

func TestApplyPageKeepsOtherYears(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	now := time.Now()
	s.ApplyPage(ctx, src, 2025, []schedule.Lighting{night("2025-12-01", "Old", schedule.Red)}, now)
	// the same page now lists next year's nights
	got, _ := s.ApplyPage(ctx, src, 2026, []schedule.Lighting{night("2026-12-01", "New", schedule.Blue)}, now)
	if got.Removed != 0 {
		t.Errorf("a new year removed last year's nights: %+v", got)
	}
	if ls, _ := s.Lightings(ctx, "2025-01-01", "2026-12-31"); len(ls) != 2 {
		t.Errorf("got %d nights, want both years", len(ls))
	}
}

func TestOverridesComeFirst(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	s.ApplyPage(ctx, src, 2026, []schedule.Lighting{night("2026-12-10", "Dress for Success", schedule.Red)}, time.Now())
	ts := fmtTime(time.Now())
	if _, err := s.DB.ExecContext(ctx, `insert into overrides (date, colors, cause, created_at, updated_at)
		values ('2026-12-10', '["blue"]', 'Fixed', ?, ?)`, ts, ts); err != nil {
		t.Fatal(err)
	}
	ls, _ := s.Lightings(ctx, "2026-12-10", "2026-12-10")
	if len(ls) != 2 || ls[0].Program != schedule.ProgramOverride {
		t.Fatalf("got %+v", ls)
	}
	if m := schedule.Merge(ls); m.Cause != "Fixed" {
		t.Errorf("merged = %+v", m)
	}
}

func TestSourcesAndRuns(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	if _, ok, err := s.Source(ctx, src); ok || err != nil {
		t.Fatalf("new source ok=%v err=%v", ok, err)
	}
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	in := Source{URL: src, Title: "31 Nights of Light", Year: 2026, Program: schedule.ProgramNights,
		LastMod: "2026-11-06T13:58:06+00:00", ETag: `"abc"`, Items: 31, CheckedAt: now, ChangedAt: now}
	if err := s.SaveSource(ctx, in); err != nil {
		t.Fatal(err)
	}
	out, ok, err := s.Source(ctx, src)
	if !ok || err != nil || out != in {
		t.Fatalf("source = %+v %v %v", out, ok, err)
	}
	if all, _ := s.Sources(ctx); len(all) != 1 {
		t.Errorf("sources = %d", len(all))
	}

	if last, _ := s.LastSuccess(ctx); !last.IsZero() {
		t.Errorf("last success before any run = %v", last)
	}
	run := &ScrapeRun{StartedAt: now, FinishedAt: now.Add(time.Minute), Pages: 3, Warnings: []string{"w"}}
	if err := s.SaveRun(ctx, run); err != nil || run.ID == 0 {
		t.Fatalf("save run: %v", err)
	}
	s.SaveRun(ctx, &ScrapeRun{StartedAt: now, FinishedAt: now.Add(2 * time.Minute), Error: "boom"})
	runs, _ := s.Runs(ctx, 10)
	if len(runs) != 2 || runs[1].Warnings[0] != "w" || runs[0].Error != "boom" {
		t.Errorf("runs = %+v", runs)
	}
	if last, _ := s.LastSuccess(ctx); !last.Equal(now.Add(time.Minute)) {
		t.Errorf("last success = %v", last)
	}
}
