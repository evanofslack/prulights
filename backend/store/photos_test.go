package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPhotos(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	now := time.Date(2026, 12, 12, 23, 0, 0, 0, time.UTC)
	inv, _ := s.CreateInvite(ctx, Invite{CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
	mom, err := s.Redeem(ctx, inv, NewHandle(), "Mom", []byte("Mom"), []byte(`{}`), now)
	if err != nil {
		t.Fatal(err)
	}
	photo := func(sha string) Photo {
		return Photo{PhotoInfo: PhotoInfo{SHA256: sha, Width: 1600, Height: 1200}, Full: []byte("full " + sha), Thumb: []byte("thumb " + sha)}
	}

	// a photo of a night not logged yet logs it; one that's logged keeps its stars
	s.SaveSighting(ctx, mom.ID, "2026-12-10", 4, "nice", now)
	if err := s.SavePhoto(ctx, mom.ID, "2026-12-10", photo("aaa"), now); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePhoto(ctx, mom.ID, "2026-12-12", photo("bbb"), now); err != nil {
		t.Fatal(err)
	}
	mine, _ := s.UserSightings(ctx, mom.ID, "2026-12-01", "2026-12-31")
	if len(mine) != 2 || mine[0].Photo == nil || mine[0].Photo.SHA256 != "bbb" || mine[1].Rating != 4 ||
		mine[1].Photo == nil || mine[1].Photo.Width != 1600 {
		t.Fatalf("mine = %+v", mine)
	}

	// one per night: a second replaces the first
	s.SavePhoto(ctx, mom.ID, "2026-12-12", photo("ccc"), now)
	if _, err := s.PhotoImage(ctx, "bbb", false); !errors.Is(err, ErrNotFound) {
		t.Errorf("replaced photo still served: %v", err)
	}
	if b, _ := s.PhotoImage(ctx, "ccc", true); string(b) != "thumb ccc" {
		t.Errorf("thumb = %q", b)
	}

	if ok, _ := s.DeletePhoto(ctx, mom.ID, "2026-12-12"); !ok {
		t.Error("delete found nothing")
	}
	night, _ := s.NightSightings(ctx, "2026-12-12")
	if len(night) != 1 || night[0].Photo != nil {
		t.Errorf("after delete = %+v", night)
	}

	// taking back the sighting takes the photo with it
	s.DeleteSighting(ctx, mom.ID, "2026-12-10")
	if _, err := s.PhotoImage(ctx, "aaa", false); !errors.Is(err, ErrNotFound) {
		t.Errorf("photo outlived its sighting: %v", err)
	}
}
