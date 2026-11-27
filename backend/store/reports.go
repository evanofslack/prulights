package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

// A report's status: open until an admin accepts or dismisses it.
const (
	ReportOpen      = "open"
	ReportAccepted  = "accepted"
	ReportDismissed = "dismissed"
)

// Report is someone saying a night's colors were really something else.
type Report struct {
	ID         int64
	UserID     int64
	Name       string // the person's name, for shared lists
	Date       schedule.Date
	Colors     []schedule.ColorKey
	Note       string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ResolvedAt time.Time // zero while open
}

// SaveReport records (or changes) a user's report on a night. Changing a
// resolved report opens it again.
func (s *Store) SaveReport(ctx context.Context, userID int64, date schedule.Date, colors []schedule.ColorKey, note string, now time.Time) error {
	b, err := json.Marshal(colors)
	if err != nil {
		return err
	}
	t := fmtTime(now)
	_, err = s.DB.ExecContext(ctx, `insert into reports (user_id, date, colors, note, status, created_at, updated_at)
		values (?, ?, ?, ?, 'open', ?, ?)
		on conflict (user_id, date) do update set colors = excluded.colors, note = excluded.note,
			status = 'open', resolved_at = null, updated_at = excluded.updated_at`,
		userID, date, string(b), note, t, t)
	return err
}

// DeleteReport takes back a user's report on a night.
func (s *Store) DeleteReport(ctx context.Context, userID int64, date schedule.Date) error {
	_, err := s.DB.ExecContext(ctx, `delete from reports where user_id = ? and date = ?`, userID, date)
	return err
}

const reportCols = `r.id, r.user_id, u.name, r.date, r.colors, r.note, r.status, r.created_at, r.updated_at, coalesce(r.resolved_at, '')
	from reports r join users u on u.id = r.user_id`

func (s *Store) reports(ctx context.Context, query string, args ...any) ([]Report, error) {
	rows, err := s.DB.QueryContext(ctx, `select `+reportCols+` `+query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Report{}
	for rows.Next() {
		var x Report
		var colors, created, updated, resolved string
		if err := rows.Scan(&x.ID, &x.UserID, &x.Name, &x.Date, &colors, &x.Note, &x.Status, &created, &updated, &resolved); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(colors), &x.Colors); err != nil {
			return nil, errors.Join(errors.New("report "+string(x.Date)), err)
		}
		x.CreatedAt, x.UpdatedAt = parseTime(created), parseTime(updated)
		if resolved != "" {
			x.ResolvedAt = parseTime(resolved)
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// NightReports is everyone's reports on a night, oldest first.
func (s *Store) NightReports(ctx context.Context, date schedule.Date) ([]Report, error) {
	return s.reports(ctx, `where r.date = ? order by r.created_at, r.id`, date)
}

// OpenReports is every report waiting on an admin, by night (newest first).
func (s *Store) OpenReports(ctx context.Context) ([]Report, error) {
	return s.reports(ctx, `where r.status = 'open' order by r.date desc, r.created_at, r.id`)
}

// ResolveReports accepts or dismisses every open report on a night. It
// reports how many there were.
func (s *Store) ResolveReports(ctx context.Context, date schedule.Date, status string, now time.Time) (int, error) {
	if status != ReportAccepted && status != ReportDismissed {
		return 0, errors.New("unknown report status " + status)
	}
	t := fmtTime(now)
	res, err := s.DB.ExecContext(ctx, `update reports set status = ?, resolved_at = ?, updated_at = ?
		where date = ? and status = 'open'`, status, t, t, date)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
