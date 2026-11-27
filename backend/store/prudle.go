package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

// PrudleStatus is where a game stands.
type PrudleStatus string

const (
	PrudlePlaying PrudleStatus = "playing"
	PrudleWon     PrudleStatus = "won"
	PrudleLost    PrudleStatus = "lost"
	PrudlePassed  PrudleStatus = "passed" // "just show me"
)

// PrudleGame is one person's game of one night.
type PrudleGame struct {
	UserID     int64
	Name       string // the person's name, for the family board
	Date       schedule.Date
	Answer     []schedule.ColorKey
	Guesses    [][]schedule.ColorKey
	Status     PrudleStatus
	Misses     float64 // wrong guesses
	Turns      int
	CreatedAt  time.Time
	FinishedAt time.Time // zero while playing
}

// SavePrudleGame records a game as it now stands.
func (s *Store) SavePrudleGame(ctx context.Context, g PrudleGame) error {
	answer, err := json.Marshal(g.Answer)
	if err != nil {
		return err
	}
	if g.Guesses == nil {
		g.Guesses = [][]schedule.ColorKey{}
	}
	guesses, err := json.Marshal(g.Guesses)
	if err != nil {
		return err
	}
	var finished sql.NullString
	if !g.FinishedAt.IsZero() {
		finished = sql.NullString{String: fmtTime(g.FinishedAt), Valid: true}
	}
	_, err = s.DB.ExecContext(ctx, `insert into prudle_games
		(user_id, date, answer, guesses, status, misses, turns, created_at, finished_at)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict (user_id, date) do update set guesses = excluded.guesses, status = excluded.status,
			misses = excluded.misses, turns = excluded.turns, finished_at = excluded.finished_at`,
		g.UserID, g.Date, string(answer), string(guesses), g.Status, g.Misses, g.Turns, fmtTime(g.CreatedAt), finished)
	return err
}

const prudleCols = `g.user_id, u.name, g.date, g.answer, g.guesses, g.status, g.misses, g.turns,
	g.created_at, coalesce(g.finished_at, '') from prudle_games g join users u on u.id = g.user_id`

func (s *Store) prudleGames(ctx context.Context, query string, args ...any) ([]PrudleGame, error) {
	rows, err := s.DB.QueryContext(ctx, `select `+prudleCols+` `+query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PrudleGame{}
	for rows.Next() {
		var g PrudleGame
		var answer, guesses, created, finished string
		if err := rows.Scan(&g.UserID, &g.Name, &g.Date, &answer, &guesses, &g.Status, &g.Misses, &g.Turns, &created, &finished); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(answer), &g.Answer); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(guesses), &g.Guesses); err != nil {
			return nil, err
		}
		g.CreatedAt = parseTime(created)
		if finished != "" {
			g.FinishedAt = parseTime(finished)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// PrudleGame is a user's game of a night, or ErrNotFound before the first guess.
func (s *Store) PrudleGame(ctx context.Context, userID int64, date schedule.Date) (PrudleGame, error) {
	gs, err := s.prudleGames(ctx, `where g.user_id = ? and g.date = ?`, userID, date)
	if err != nil {
		return PrudleGame{}, err
	}
	if len(gs) == 0 {
		return PrudleGame{}, ErrNotFound
	}
	return gs[0], nil
}

// PrudleGames lists a user's games between two nights, oldest first.
func (s *Store) PrudleGames(ctx context.Context, userID int64, from, to schedule.Date) ([]PrudleGame, error) {
	return s.prudleGames(ctx, `where g.user_id = ? and g.date between ? and ? order by g.date`, userID, from, to)
}

// PrudleNight lists everyone's finished games of a night, first done first.
func (s *Store) PrudleNight(ctx context.Context, date schedule.Date) ([]PrudleGame, error) {
	return s.prudleGames(ctx, `where g.date = ? and g.status != 'playing' order by g.finished_at, g.user_id`, date)
}

// PrudleOff is whether a user turned Prudle off.
func (s *Store) PrudleOff(ctx context.Context, userID int64) (bool, error) {
	var off bool
	err := s.DB.QueryRowContext(ctx, `select prudle_off from users where id = ?`, userID).Scan(&off)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	return off, err
}

// SetPrudleOff turns Prudle off (or back on) for a user.
func (s *Store) SetPrudleOff(ctx context.Context, userID int64, off bool) error {
	_, err := s.DB.ExecContext(ctx, `update users set prudle_off = ? where id = ?`, off, userID)
	return err
}
