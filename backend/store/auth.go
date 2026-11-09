package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNotFound is returned when a user, session or invite doesn't exist (or has
// expired, or was already used).
var ErrNotFound = errors.New("not found")

// ErrNameTaken is returned when another user already has a name (names are
// compared ignoring case).
var ErrNameTaken = errors.New("name taken")

// User is a person who was invited in.
type User struct {
	ID        int64
	Handle    []byte // the passkey user handle
	Name      string
	IsAdmin   bool
	CreatedAt time.Time
}

// NewToken makes a random url-safe token and its hash. Only the hash is
// stored; the token goes in a cookie or an invite link.
func NewToken() (token string, hash []byte) {
	b := make([]byte, 32)
	rand.Read(b)
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token)
}

// HashToken is how tokens are looked up.
func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

const userCols = `id, handle, name, is_admin, created_at`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	var created string
	err := row.Scan(&u.ID, &u.Handle, &u.Name, &u.IsAdmin, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	u.CreatedAt = parseTime(created)
	return u, err
}

// NewHandle makes a passkey user handle for someone about to register.
func NewHandle() []byte {
	b := make([]byte, 32)
	rand.Read(b)
	return b
}

func createUser(ctx context.Context, tx *sql.Tx, handle []byte, name string, admin bool, now time.Time) (User, error) {
	u := User{Handle: handle, Name: name, IsAdmin: admin, CreatedAt: now}
	res, err := tx.ExecContext(ctx, `INSERT INTO users (handle, name, is_admin, created_at) VALUES (?, ?, ?, ?)`,
		u.Handle, name, admin, fmtTime(now))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return u, ErrNameTaken
		}
		return u, err
	}
	u.ID, err = res.LastInsertId()
	return u, err
}

// UserByID finds a user.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.DB.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

// UserByHandle finds a user by passkey user handle.
func (s *Store) UserByHandle(ctx context.Context, handle []byte) (User, error) {
	return scanUser(s.DB.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE handle = ?`, handle))
}

// UserByName finds a user by name, ignoring case.
func (s *Store) UserByName(ctx context.Context, name string) (User, error) {
	return scanUser(s.DB.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE name = ? COLLATE NOCASE`, name))
}

// Users lists everyone, oldest first.
func (s *Store) Users(ctx context.Context) ([]User, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Credentials returns a user's passkeys, as the json the server stored.
func (s *Store) Credentials(ctx context.Context, userID int64) ([][]byte, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT data FROM credentials WHERE user_id = ? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := [][]byte{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func addCredential(ctx context.Context, tx *sql.Tx, userID int64, id, data []byte, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO credentials (id, user_id, data, created_at) VALUES (?, ?, ?, ?)`,
		id, userID, data, fmtTime(now))
	return err
}

// UpdateCredential saves a passkey after a sign-in (its sign count moves).
func (s *Store) UpdateCredential(ctx context.Context, id, data []byte, now time.Time) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE credentials SET data = ?, last_used_at = ? WHERE id = ?`, data, fmtTime(now), id)
	return err
}

// Invite is a link that lets someone in: as a new user, or (ForUser set) as a
// new passkey for someone already here.
type Invite struct {
	CreatedBy *int64
	ForUser   *int64
	IsAdmin   bool
	CreatedAt time.Time
	ExpiresAt time.Time
}

// CreateInvite stores an invite and returns its token.
func (s *Store) CreateInvite(ctx context.Context, inv Invite) (string, error) {
	token, hash := NewToken()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO invites (token_hash, created_by, for_user_id, is_admin, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`, hash, inv.CreatedBy, inv.ForUser, inv.IsAdmin, fmtTime(inv.CreatedAt), fmtTime(inv.ExpiresAt))
	return token, err
}

func scanInvite(row *sql.Row, now time.Time) (Invite, error) {
	var inv Invite
	var created, expires string
	var used sql.NullString
	err := row.Scan(&inv.CreatedBy, &inv.ForUser, &inv.IsAdmin, &created, &expires, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return inv, ErrNotFound
	}
	if err != nil {
		return inv, err
	}
	inv.CreatedAt, inv.ExpiresAt = parseTime(created), parseTime(expires)
	if used.Valid || !now.Before(inv.ExpiresAt) {
		return inv, ErrNotFound
	}
	return inv, nil
}

const inviteQuery = `SELECT created_by, for_user_id, is_admin, created_at, expires_at, used_at FROM invites WHERE token_hash = ?`

// InviteByToken finds an invite that's still good.
func (s *Store) InviteByToken(ctx context.Context, token string, now time.Time) (Invite, error) {
	return scanInvite(s.DB.QueryRowContext(ctx, inviteQuery, HashToken(token)), now)
}

// Redeem uses an invite to add a passkey: to a new user with this handle and
// name, or to the invite's ForUser. It checks the invite again inside the
// transaction, so an invite works once.
func (s *Store) Redeem(ctx context.Context, token string, handle []byte, name string, credID, credData []byte, now time.Time) (User, error) {
	var u User
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		inv, err := scanInvite(tx.QueryRowContext(ctx, inviteQuery, HashToken(token)), now)
		if err != nil {
			return err
		}
		if inv.ForUser != nil {
			u, err = scanUser(tx.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, *inv.ForUser))
		} else {
			u, err = createUser(ctx, tx, handle, name, inv.IsAdmin, now)
		}
		if err != nil {
			return err
		}
		if err := addCredential(ctx, tx, u.ID, credID, credData, now); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE invites SET used_at = ?, used_by = ? WHERE token_hash = ? AND used_at IS NULL`,
			fmtTime(now), u.ID, HashToken(token))
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return ErrNotFound
		}
		return nil
	})
	return u, err
}

// CreateSession signs a user in and returns the session token.
func (s *Store) CreateSession(ctx context.Context, userID int64, now time.Time, ttl time.Duration) (string, error) {
	token, hash := NewToken()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, created_at, last_seen_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		hash, userID, fmtTime(now), fmtTime(now), fmtTime(now.Add(ttl)))
	return token, err
}

// Session is a signed-in browser.
type Session struct {
	User       User
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

// SessionByToken finds the user behind a session that hasn't expired.
func (s *Store) SessionByToken(ctx context.Context, token string, now time.Time) (Session, error) {
	var sess Session
	var seen, expires, created string
	err := s.DB.QueryRowContext(ctx, `SELECT u.id, u.handle, u.name, u.is_admin, u.created_at, s.last_seen_at, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = ?`, HashToken(token)).
		Scan(&sess.User.ID, &sess.User.Handle, &sess.User.Name, &sess.User.IsAdmin, &created, &seen, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return sess, ErrNotFound
	}
	if err != nil {
		return sess, err
	}
	sess.User.CreatedAt, sess.LastSeenAt, sess.ExpiresAt = parseTime(created), parseTime(seen), parseTime(expires)
	if !now.Before(sess.ExpiresAt) {
		return sess, ErrNotFound
	}
	return sess, nil
}

// ExtendSession slides a session's expiry forward.
func (s *Store) ExtendSession(ctx context.Context, token string, now time.Time, ttl time.Duration) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE token_hash = ?`,
		fmtTime(now), fmtTime(now.Add(ttl)), HashToken(token))
	return err
}

// DeleteSession signs a browser out.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, HashToken(token))
	return err
}

// DeleteExpired removes sessions and invites past their expiry.
func (s *Store) DeleteExpired(ctx context.Context, now time.Time) error {
	t := fmtTime(now)
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, t); err != nil {
		return fmt.Errorf("sessions: %w", err)
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM invites WHERE used_at IS NULL AND expires_at <= ?`, t)
	return err
}
