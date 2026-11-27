package store

import (
	"context"
	"time"
)

// Feedback is a member's idea for the site.
type Feedback struct {
	ID        int64
	UserID    int64
	Name      string // who sent it
	Body      string
	CreatedAt time.Time
	DoneAt    time.Time // zero until an admin marks it done
}

// AddFeedback keeps someone's idea.
func (s *Store) AddFeedback(ctx context.Context, userID int64, body string, now time.Time) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `insert into feedback (user_id, body, created_at) values (?, ?, ?)`,
		userID, body, fmtTime(now))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// AllFeedback is every idea, the ones not yet done first, newest first.
func (s *Store) AllFeedback(ctx context.Context) ([]Feedback, error) {
	rows, err := s.DB.QueryContext(ctx, `select f.id, f.user_id, u.name, f.body, f.created_at, coalesce(f.done_at, '')
		from feedback f join users u on u.id = f.user_id
		order by f.done_at is not null, f.created_at desc, f.id desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Feedback{}
	for rows.Next() {
		var f Feedback
		var created, done string
		if err := rows.Scan(&f.ID, &f.UserID, &f.Name, &f.Body, &created, &done); err != nil {
			return nil, err
		}
		f.CreatedAt = parseTime(created)
		if done != "" {
			f.DoneAt = parseTime(done)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// SetFeedbackDone marks an idea done, or not done again.
func (s *Store) SetFeedbackDone(ctx context.Context, id int64, done bool, now time.Time) error {
	var at any
	if done {
		at = fmtTime(now)
	}
	res, err := s.DB.ExecContext(ctx, `update feedback set done_at = ? where id = ?`, at, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteFeedback forgets an idea.
func (s *Store) DeleteFeedback(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `delete from feedback where id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
