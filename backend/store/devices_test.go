package store

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestDevices(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	now := time.Date(2026, 12, 12, 23, 0, 0, 0, time.UTC)
	inv, _ := s.CreateInvite(ctx, Invite{CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
	mom, err := s.Redeem(ctx, inv, NewHandle(), "Mom", []byte("mom"), []byte(`{}`), now)
	if err != nil {
		t.Fatal(err)
	}

	d, token, err := s.CreateDevice(ctx, "0.1.0", "", "", now)
	if err != nil || token == "" || d.Claimed() {
		t.Fatalf("create = %+v %q %v", d, token, err)
	}
	if !regexp.MustCompile(`^[A-Z0-9]{3}-[A-Z0-9]{3}$`).MatchString(d.ClaimCode) {
		t.Errorf("claim code %q", d.ClaimCode)
	}
	got, err := s.DeviceByToken(ctx, token)
	if err != nil || got.ID != d.ID || got.Firmware != "0.1.0" || string(got.Settings) != "{}" {
		t.Errorf("by token = %+v %v", got, err)
	}
	if _, err := s.DeviceByToken(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("bad token: %v", err)
	}

	// typed loosely off the frame
	loose := " " + d.ClaimCode[:3] + " " + d.ClaimCode[4:] + " "
	claimed, err := s.Claim(ctx, loose, mom.ID, "Kitchen", now)
	if err != nil || claimed.UserID != mom.ID || claimed.Owner != "Mom" || claimed.Name != "Kitchen" ||
		claimed.ClaimCode != "" || claimed.ClaimedAt.IsZero() {
		t.Fatalf("claim = %+v %v", claimed, err)
	}
	if _, err := s.Claim(ctx, d.ClaimCode, mom.ID, "", now); !errors.Is(err, ErrNotFound) {
		t.Errorf("claim twice: %v", err)
	}

	if err := s.UpdateDevice(ctx, d.ID, mom.ID, "Desk", []byte(`{"paused":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateDevice(ctx, d.ID, mom.ID+1, "Mine", []byte(`{}`)); !errors.Is(err, ErrNotFound) {
		t.Errorf("someone else's frame: %v", err)
	}
	list, _ := s.UserDevices(ctx, mom.ID)
	if len(list) != 1 || list[0].Name != "Desk" || string(list[0].Settings) != `{"paused":true}` {
		t.Errorf("mom's devices = %+v", list)
	}

	if err := s.CheckIn(ctx, d.ID, now, 3900, "battery", ""); err != nil {
		t.Fatal(err)
	}
	s.CheckIn(ctx, d.ID, now.Add(time.Hour), 0, "", "0.2.0")
	got, _ = s.DeviceByID(ctx, d.ID)
	if got.BatteryMV != 3900 || got.Power != "battery" || got.Firmware != "0.2.0" || !got.CheckinAt.Equal(now.Add(time.Hour)) {
		t.Errorf("after check-ins = %+v", got)
	}

	events := []DeviceEvent{{Type: "long", At: now}, {Type: "seen", At: now.Add(time.Second), Date: "2026-12-12"}}
	if err := s.AddEvents(ctx, d.ID, events, now); err != nil {
		t.Fatal(err)
	}
	evs, _ := s.DeviceEvents(ctx, d.ID, 10)
	if len(evs) != 2 || evs[0].Type != "seen" || evs[0].Date != "2026-12-12" {
		t.Errorf("events = %+v", evs)
	}

	if err := s.Unclaim(ctx, d.ID, mom.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = s.DeviceByToken(ctx, token)
	if got.Claimed() || got.ClaimCode == "" || got.ClaimCode == d.ClaimCode || got.Name != "" || string(got.Settings) != "{}" {
		t.Errorf("unclaimed = %+v", got)
	}

	if err := s.DeleteDevice(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeviceByToken(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted device's token still works: %v", err)
	}
}

func TestNormalizeClaimCode(t *testing.T) {
	for in, want := range map[string]string{"k7q9mx": "K7Q-9MX", "K7Q-9MX": "K7Q-9MX", " k7q 9mx": "K7Q-9MX", "abc": "ABC"} {
		if got := NormalizeClaimCode(in); got != want {
			t.Errorf("%q = %q, want %q", in, got, want)
		}
	}
}

func TestDeviceImages(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	now := time.Date(2026, 12, 12, 23, 0, 0, 0, time.UTC)
	s.SaveImage(ctx, "old", []byte("a"), now.Add(-40*24*time.Hour))
	s.SaveImage(ctx, "kept", []byte("b"), now.Add(-40*24*time.Hour))
	s.SaveImage(ctx, "kept", []byte("ignored"), now) // listed again
	if err := s.DeleteExpired(ctx, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Image(ctx, "old"); !errors.Is(err, ErrNotFound) {
		t.Errorf("old image kept: %v", err)
	}
	if b, err := s.Image(ctx, "kept"); err != nil || string(b) != "b" {
		t.Errorf("kept = %q %v", b, err)
	}
}

func TestMarkSeen(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	now := time.Date(2026, 12, 12, 23, 0, 0, 0, time.UTC)
	inv, _ := s.CreateInvite(ctx, Invite{CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
	mom, _ := s.Redeem(ctx, inv, NewHandle(), "Mom", []byte("mom"), []byte(`{}`), now)
	s.SaveSighting(ctx, mom.ID, "2026-12-12", 4, "pretty", now)
	if fresh, err := s.MarkSeen(ctx, mom.ID, "2026-12-12", now); err != nil || fresh {
		t.Errorf("seen again = %v %v", fresh, err)
	}
	if fresh, _ := s.MarkSeen(ctx, mom.ID, "2026-12-11", now); !fresh {
		t.Error("new night not new")
	}
	xs, _ := s.UserSightings(ctx, mom.ID, "2026-12-01", "2026-12-31")
	if len(xs) != 2 || xs[0].Rating != 4 || xs[0].Note != "pretty" {
		t.Errorf("sightings = %+v", xs)
	}
}

func TestFrameInvites(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	now := time.Date(2026, 12, 12, 23, 0, 0, 0, time.UTC)
	inv, _ := s.CreateInvite(ctx, Invite{CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
	evan, _ := s.Redeem(ctx, inv, NewHandle(), "Evan", []byte("evan"), []byte(`{}`), now)

	d, _, err := s.CreateDevice(ctx, "", "", "", now)
	if err != nil || !strings.HasPrefix(d.APSSID, "prulights-") || len(d.APPassword) != 10 {
		t.Fatalf("made-up hotspot = %+v %v", d, err)
	}
	own, _, _ := s.CreateDevice(ctx, "", "frame-1", "secret123", now)
	if own.APSSID != "frame-1" || own.APPassword != "secret123" {
		t.Errorf("the frame's own hotspot = %+v", own)
	}

	redeem := func(name string, at time.Time) (User, Device, error) {
		return s.RedeemFrame(ctx, strings.ToLower(d.ClaimCode), NewHandle(), name, []byte(name), []byte(`{}`), at)
	}
	if _, _, err := redeem("Mom", now); !errors.Is(err, ErrNotFound) {
		t.Errorf("not marked: %v", err)
	}
	if err := s.MarkForInvite(ctx, d.ID, evan.ID, now.Add(30*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _ := s.DeviceByClaimCode(ctx, d.ClaimCode)
	if !got.Invites(now) || got.Invites(now.Add(31*24*time.Hour)) {
		t.Errorf("marked = %+v", got)
	}
	if _, _, err := redeem("Mom", now.Add(31*24*time.Hour)); !errors.Is(err, ErrNotFound) {
		t.Errorf("mark ran out: %v", err)
	}
	if _, _, err := redeem("evan", now); !errors.Is(err, ErrNameTaken) {
		t.Errorf("taken name: %v", err)
	}
	if got, _ := s.DeviceByID(ctx, d.ID); got.Claimed() {
		t.Error("a taken name still claimed the frame")
	}

	mom, frame, err := redeem("Mom", now)
	if err != nil || frame.UserID != mom.ID || frame.Owner != "Mom" || frame.ClaimCode != "" || !frame.InviteUntil.IsZero() {
		t.Fatalf("redeem = %+v %+v %v", mom, frame, err)
	}
	if creds, _ := s.Credentials(ctx, mom.ID); len(creds) != 1 {
		t.Errorf("mom has %d passkeys", len(creds))
	}
	if _, _, err := redeem("Someone", now); !errors.Is(err, ErrNotFound) {
		t.Errorf("redeemed twice: %v", err)
	}

	// claiming a marked frame normally clears the mark
	s.MarkForInvite(ctx, own.ID, evan.ID, now.Add(time.Hour))
	claimed, err := s.Claim(ctx, own.ClaimCode, evan.ID, "Desk", now)
	if err != nil || !claimed.InviteUntil.IsZero() {
		t.Errorf("claim of a marked frame = %+v %v", claimed, err)
	}
	if err := s.MarkForInvite(ctx, own.ID, evan.ID, now.Add(time.Hour)); !errors.Is(err, ErrNotFound) {
		t.Errorf("marking a claimed frame: %v", err)
	}
}
