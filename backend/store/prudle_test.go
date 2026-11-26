package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

func TestPrudleGames(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	now := time.Date(2026, 12, 10, 23, 0, 0, 0, time.UTC)
	add := func(name string) User {
		inv, _ := s.CreateInvite(ctx, Invite{CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
		u, err := s.Redeem(ctx, inv, NewHandle(), name, []byte(name), []byte(`{}`), now)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	mom, evan := add("Mom"), add("Evan")

	if _, err := s.PrudleGame(ctx, mom.ID, "2026-12-10"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("before a guess: %v", err)
	}
	g := PrudleGame{UserID: mom.ID, Date: "2026-12-10", Answer: []schedule.ColorKey{"red"},
		Guesses: [][]schedule.ColorKey{{"pink"}}, Status: PrudlePlaying, Misses: 0.5, Turns: 1, CreatedAt: now}
	if err := s.SavePrudleGame(ctx, g); err != nil {
		t.Fatal(err)
	}
	g.Guesses = append(g.Guesses, []schedule.ColorKey{"red"})
	g.Status, g.Turns, g.FinishedAt = PrudleWon, 2, now.Add(time.Minute)
	g.Answer = []schedule.ColorKey{"blue"} // the answer is kept from the first save
	if err := s.SavePrudleGame(ctx, g); err != nil {
		t.Fatal(err)
	}
	got, err := s.PrudleGame(ctx, mom.ID, "2026-12-10")
	if err != nil || got.Status != PrudleWon || len(got.Guesses) != 2 || got.Answer[0] != "red" ||
		got.Name != "Mom" || !got.FinishedAt.Equal(now.Add(time.Minute)) {
		t.Errorf("got %+v %v", got, err)
	}
	s.SavePrudleGame(ctx, PrudleGame{UserID: evan.ID, Date: "2026-12-10", Answer: []schedule.ColorKey{"red"},
		Status: PrudlePlaying, CreatedAt: now})
	if night, _ := s.PrudleNight(ctx, "2026-12-10"); len(night) != 1 || night[0].Name != "Mom" {
		t.Errorf("night = %+v (still playing doesn't show)", night)
	}
	if mine, _ := s.PrudleGames(ctx, mom.ID, "2026-12-01", "2026-12-31"); len(mine) != 1 {
		t.Errorf("mine = %+v", mine)
	}

	if off, err := s.PrudleOff(ctx, mom.ID); off || err != nil {
		t.Errorf("off by default: %v %v", off, err)
	}
	s.SetPrudleOff(ctx, mom.ID, true)
	if off, _ := s.PrudleOff(ctx, mom.ID); !off {
		t.Error("didn't turn off")
	}
}
