package store

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

func TestReports(t *testing.T) {
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
	red := []schedule.ColorKey{schedule.Red}
	redGreen := []schedule.ColorKey{schedule.Red, schedule.Green}

	if err := s.SaveReport(ctx, mom.ID, "2026-12-12", red, "it was all red", now); err != nil {
		t.Fatal(err)
	}
	s.SaveReport(ctx, evan.ID, "2026-12-12", redGreen, "", now.Add(time.Minute))
	s.SaveReport(ctx, evan.ID, "2026-12-10", red, "", now)

	night, err := s.NightReports(ctx, "2026-12-12")
	if err != nil || len(night) != 2 || night[0].Name != "Mom" || !slices.Equal(night[0].Colors, red) ||
		night[0].Note != "it was all red" || night[0].Status != ReportOpen || !night[0].ResolvedAt.IsZero() {
		t.Fatalf("night = %+v %v", night, err)
	}
	if open, _ := s.OpenReports(ctx); len(open) != 3 || open[0].Date != "2026-12-12" || open[2].Date != "2026-12-10" {
		t.Errorf("open = %+v", open)
	}

	if n, err := s.ResolveReports(ctx, "2026-12-12", ReportAccepted, now.Add(time.Hour)); n != 2 || err != nil {
		t.Errorf("resolve = %d %v", n, err)
	}
	if n, _ := s.ResolveReports(ctx, "2026-12-12", ReportDismissed, now); n != 0 {
		t.Errorf("already resolved, got %d", n)
	}
	if _, err := s.ResolveReports(ctx, "2026-12-10", "maybe", now); err == nil {
		t.Error("unknown status")
	}
	night, _ = s.NightReports(ctx, "2026-12-12")
	if night[0].Status != ReportAccepted || night[0].ResolvedAt.IsZero() {
		t.Errorf("accepted = %+v", night[0])
	}

	// changing a resolved report opens it again, one row
	s.SaveReport(ctx, mom.ID, "2026-12-12", redGreen, "", now.Add(2*time.Hour))
	night, _ = s.NightReports(ctx, "2026-12-12")
	if len(night) != 2 || night[0].Status != ReportOpen || !night[0].ResolvedAt.IsZero() || !slices.Equal(night[0].Colors, redGreen) {
		t.Errorf("reopened = %+v", night[0])
	}

	s.DeleteReport(ctx, evan.ID, "2026-12-10")
	if open, _ := s.OpenReports(ctx); len(open) != 1 || open[0].UserID != mom.ID {
		t.Errorf("after delete = %+v", open)
	}
}
