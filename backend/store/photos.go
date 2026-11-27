package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

// PhotoInfo is what a listing says about a sighting's photo.
type PhotoInfo struct {
	SHA256 string // of the full jpeg
	Width  int
	Height int
}

// Photo is a sighting's photo, ready to serve: the full picture and a thumbnail.
type Photo struct {
	PhotoInfo
	Full  []byte
	Thumb []byte
}

// SavePhoto sets a user's photo of a night, replacing any they had. Adding a
// photo says they saw it: a missing sighting is made, an existing one keeps
// its stars and note.
func (s *Store) SavePhoto(ctx context.Context, userID int64, date schedule.Date, p Photo, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	t := fmtTime(now)
	if _, err := tx.ExecContext(ctx, `insert into sightings (user_id, date, note, created_at, updated_at)
		values (?, ?, '', ?, ?) on conflict (user_id, date) do nothing`, userID, date, t, t); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into photos (user_id, date, sha256, width, height, created_at, full, thumb)
		values (?, ?, ?, ?, ?, ?, ?, ?)
		on conflict (user_id, date) do update set sha256 = excluded.sha256, width = excluded.width,
			height = excluded.height, created_at = excluded.created_at, full = excluded.full, thumb = excluded.thumb`,
		userID, date, p.SHA256, p.Width, p.Height, t, p.Full, p.Thumb); err != nil {
		return err
	}
	return tx.Commit()
}

// DeletePhoto removes a user's photo of a night (the sighting stays). It
// reports whether there was one.
func (s *Store) DeletePhoto(ctx context.Context, userID int64, date schedule.Date) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `delete from photos where user_id = ? and date = ?`, userID, date)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DeletePhotosBySHA removes a picture wherever it was posted (an admin
// taking one down). It reports how many there were.
func (s *Store) DeletePhotosBySHA(ctx context.Context, sha string) (int, error) {
	res, err := s.DB.ExecContext(ctx, `delete from photos where sha256 = ?`, sha)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// PhotoImage is a photo's jpeg by its hash: the thumbnail, or the full picture.
func (s *Store) PhotoImage(ctx context.Context, sha string, thumb bool) ([]byte, error) {
	col := "full"
	if thumb {
		col = "thumb"
	}
	var b []byte
	err := s.DB.QueryRowContext(ctx, `select `+col+` from photos where sha256 = ? limit 1`, sha).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}
