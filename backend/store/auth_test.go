package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInvitesAndSessions(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	week := now.Add(7 * 24 * time.Hour)

	token, err := s.CreateInvite(ctx, Invite{IsAdmin: true, CreatedAt: now, ExpiresAt: week})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.InviteByToken(ctx, token, now); err != nil {
		t.Fatalf("fresh invite: %v", err)
	}
	if _, err := s.InviteByToken(ctx, "nope", now); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown invite: %v", err)
	}
	if _, err := s.InviteByToken(ctx, token, week); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired invite: %v", err)
	}

	evan, err := s.Redeem(ctx, token, NewHandle(), "Evan", []byte("cred-1"), []byte(`{"id":1}`), now)
	if err != nil || !evan.IsAdmin || evan.Name != "Evan" || len(evan.Handle) != 32 {
		t.Fatalf("redeem = %+v, %v", evan, err)
	}
	if _, err := s.Redeem(ctx, token, NewHandle(), "Again", []byte("cred-2"), nil, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("an invite works once: %v", err)
	}
	if got, err := s.UserByHandle(ctx, evan.Handle); err != nil || got.ID != evan.ID {
		t.Errorf("by handle = %+v %v", got, err)
	}

	// a second person can't take the same name, in any case
	t2, _ := s.CreateInvite(ctx, Invite{CreatedBy: &evan.ID, CreatedAt: now, ExpiresAt: week})
	if _, err := s.Redeem(ctx, t2, NewHandle(), "evan", []byte("cred-3"), []byte(`{}`), now); !errors.Is(err, ErrNameTaken) {
		t.Errorf("duplicate name: %v", err)
	}
	mom, err := s.Redeem(ctx, t2, NewHandle(), "Mom", []byte("cred-3"), []byte(`{}`), now)
	if err != nil || mom.IsAdmin {
		t.Fatalf("mom = %+v %v", mom, err)
	}

	// a re-invite adds a passkey to mom, ignoring the name
	t3, _ := s.CreateInvite(ctx, Invite{CreatedBy: &evan.ID, ForUser: &mom.ID, CreatedAt: now, ExpiresAt: week})
	again, err := s.Redeem(ctx, t3, nil, "whatever", []byte("cred-4"), []byte(`{}`), now)
	if err != nil || again.ID != mom.ID {
		t.Fatalf("re-invite = %+v %v", again, err)
	}
	if creds, _ := s.Credentials(ctx, mom.ID); len(creds) != 2 {
		t.Errorf("mom has %d passkeys", len(creds))
	}
	if users, _ := s.Users(ctx); len(users) != 2 {
		t.Errorf("%d users", len(users))
	}

	// sessions
	sess, err := s.CreateSession(ctx, mom.ID, now, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.SessionByToken(ctx, sess, now.Add(30*time.Minute))
	if err != nil || got.User.ID != mom.ID {
		t.Errorf("session = %+v %v", got, err)
	}
	if _, err := s.SessionByToken(ctx, sess, now.Add(time.Hour)); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired session: %v", err)
	}
	s.ExtendSession(ctx, sess, now.Add(30*time.Minute), time.Hour)
	if _, err := s.SessionByToken(ctx, sess, now.Add(time.Hour)); err != nil {
		t.Errorf("extended session: %v", err)
	}
	s.DeleteSession(ctx, sess)
	if _, err := s.SessionByToken(ctx, sess, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("signed out: %v", err)
	}
	if err := s.DeleteExpired(ctx, now.Add(30*24*time.Hour)); err != nil {
		t.Error(err)
	}
}
