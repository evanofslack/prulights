package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

// Sighting is one person seeing the crown on one night.
type Sighting struct {
	UserID    int64
	Name      string // the person's name, for shared lists
	Date      schedule.Date
	Rating    int // 1–5, 0 for none
	Note      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SaveSighting records (or updates) that a user saw a night.
func (s *Store) SaveSighting(ctx context.Context, userID int64, date schedule.Date, rating int, note string, now time.Time) error {
	var r sql.NullInt64
	if rating > 0 {
		r = sql.NullInt64{Int64: int64(rating), Valid: true}
	}
	t := fmtTime(now)
	_, err := s.DB.ExecContext(ctx, `insert into sightings (user_id, date, rating, note, created_at, updated_at)
		values (?, ?, ?, ?, ?, ?)
		on conflict (user_id, date) do update set rating = excluded.rating, note = excluded.note, updated_at = excluded.updated_at`,
		userID, date, r, note, t, t)
	return err
}

// MarkSeen records that a user saw a night, keeping any rating or note
// they already gave it. It reports whether the sighting is new.
func (s *Store) MarkSeen(ctx context.Context, userID int64, date schedule.Date, now time.Time) (bool, error) {
	t := fmtTime(now)
	res, err := s.DB.ExecContext(ctx, `insert into sightings (user_id, date, note, created_at, updated_at)
		values (?, ?, '', ?, ?) on conflict (user_id, date) do nothing`, userID, date, t, t)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DeleteSighting takes back a sighting.
func (s *Store) DeleteSighting(ctx context.Context, userID int64, date schedule.Date) error {
	_, err := s.DB.ExecContext(ctx, `delete from sightings where user_id = ? and date = ?`, userID, date)
	return err
}

const sightingCols = `s.user_id, u.name, s.date, coalesce(s.rating, 0), s.note, s.created_at, s.updated_at
	from sightings s join users u on u.id = s.user_id`

func (s *Store) sightings(ctx context.Context, query string, args ...any) ([]Sighting, error) {
	rows, err := s.DB.QueryContext(ctx, `select `+sightingCols+` `+query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Sighting{}
	for rows.Next() {
		var x Sighting
		var created, updated string
		if err := rows.Scan(&x.UserID, &x.Name, &x.Date, &x.Rating, &x.Note, &created, &updated); err != nil {
			return nil, err
		}
		x.CreatedAt, x.UpdatedAt = parseTime(created), parseTime(updated)
		out = append(out, x)
	}
	return out, rows.Err()
}

// UserSightings lists a user's sightings between two nights, newest first.
func (s *Store) UserSightings(ctx context.Context, userID int64, from, to schedule.Date) ([]Sighting, error) {
	return s.sightings(ctx, `where s.user_id = ? and s.date between ? and ? order by s.date desc`, userID, from, to)
}

// NightSightings lists everyone who saw a night, first to say so first.
func (s *Store) NightSightings(ctx context.Context, date schedule.Date) ([]Sighting, error) {
	return s.sightings(ctx, `where s.date = ? order by s.created_at, s.user_id`, date)
}

// NightStats are a night's anonymous totals.
type NightStats struct {
	Seen    int
	Rated   int
	Average float64 // of the ratings, 0 when none
}

// NightStats counts a night's sightings and averages their ratings.
func (s *Store) NightStats(ctx context.Context, date schedule.Date) (NightStats, error) {
	var n NightStats
	var avg sql.NullFloat64
	err := s.DB.QueryRowContext(ctx, `select count(*), count(rating), avg(rating) from sightings where date = ?`, date).
		Scan(&n.Seen, &n.Rated, &avg)
	n.Average = avg.Float64
	return n, err
}

// SightingCounts is how many sightings each user has logged.
func (s *Store) SightingCounts(ctx context.Context) (map[int64]int, error) {
	rows, err := s.DB.QueryContext(ctx, `select user_id, count(*) from sightings group by user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// Override is a correction to a night: it's listed first, ahead of what the
// source pages say.
type Override struct {
	Date      schedule.Date
	Colors    []schedule.ColorKey
	Cause     string
	CauseURL  string
	Kind      schedule.Kind
	Note      string // why, for the admin page
	UpdatedAt time.Time
}

// SaveOverride sets the correction for a night.
func (s *Store) SaveOverride(ctx context.Context, o Override, now time.Time) error {
	colors, err := json.Marshal(o.Colors)
	if err != nil {
		return err
	}
	t := fmtTime(now)
	_, err = s.DB.ExecContext(ctx, `insert into overrides (date, colors, cause, cause_url, kind, note, created_at, updated_at)
		values (?, ?, ?, ?, ?, ?, ?, ?)
		on conflict (date) do update set colors = excluded.colors, cause = excluded.cause, cause_url = excluded.cause_url,
			kind = excluded.kind, note = excluded.note, updated_at = excluded.updated_at`,
		o.Date, string(colors), o.Cause, o.CauseURL, o.Kind, o.Note, t, t)
	return err
}

// DeleteOverride removes a night's correction. It reports whether there was one.
func (s *Store) DeleteOverride(ctx context.Context, date schedule.Date) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `delete from overrides where date = ?`, date)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Overrides lists the corrections, newest night first.
func (s *Store) Overrides(ctx context.Context) ([]Override, error) {
	rows, err := s.DB.QueryContext(ctx, `select date, colors, cause, cause_url, kind, note, updated_at from overrides order by date desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Override{}
	for rows.Next() {
		var o Override
		var colors, updated string
		if err := rows.Scan(&o.Date, &colors, &o.Cause, &o.CauseURL, &o.Kind, &o.Note, &updated); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(colors), &o.Colors); err != nil {
			return nil, errors.Join(errors.New("override "+string(o.Date)), err)
		}
		o.UpdatedAt = parseTime(updated)
		out = append(out, o)
	}
	return out, rows.Err()
}
