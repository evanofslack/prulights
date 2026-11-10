package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

// Device is a frame (or the emulator).
type Device struct {
	ID        int64
	UserID    int64  // who claimed it, 0 while unclaimed
	Owner     string // their name
	Name      string
	ClaimCode string // set while unclaimed
	Settings  []byte // json, the server's to read
	CreatedAt time.Time
	ClaimedAt time.Time // zero while unclaimed
	CheckinAt time.Time // last time it synced, zero if never
	BatteryMV int
	Power     string // usb | battery
	Firmware  string
	// InviteUntil is set while the frame is marked for a new person: until
	// then, its code makes an account as well as claiming it.
	InviteUntil time.Time
	// APSSID and APPassword are the frame's own setup hotspot.
	APSSID     string
	APPassword string
}

// Invites is whether the frame's code makes an account right now.
func (d Device) Invites(now time.Time) bool {
	return !d.Claimed() && !d.InviteUntil.IsZero() && now.Before(d.InviteUntil)
}

// Claimed is whether someone has claimed the frame.
func (d Device) Claimed() bool { return d.UserID != 0 }

// DeviceEvent is something a frame reports: a button press, a boot, an error.
type DeviceEvent struct {
	Type       string
	At         time.Time
	Date       schedule.Date // the night it's about, for "seen"
	Detail     string
	ReceivedAt time.Time
}

// claimAlphabet leaves out letters and digits that read alike (0/O, 1/I/L,
// 2/Z, 5/S, 8/B), since someone may type the code off the frame.
const claimAlphabet = "ACDEFGHJKMNPQRTUVWXY34679"

func randomFrom(alphabet string, n int) string {
	b := make([]byte, n)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

func newClaimCode() string {
	s := randomFrom(claimAlphabet, 6)
	return s[:3] + "-" + s[3:]
}

// NewHotspot makes a frame's setup hotspot: a name to tell frames apart and a
// password someone can type off the screen.
func NewHotspot() (ssid, password string) {
	return "prulights-" + randomFrom(claimAlphabet, 4), strings.ToLower(randomFrom(claimAlphabet, 10))
}

// NormalizeClaimCode reads a code as typed: any case, with or without the
// dash or spaces.
func NormalizeClaimCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(code) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if len(s) != 6 {
		return s
	}
	return s[:3] + "-" + s[3:]
}

// CreateDevice registers a new, unclaimed frame. The token is returned once;
// only its hash is kept. Without hotspot details, it makes some.
func (s *Store) CreateDevice(ctx context.Context, firmware, apSSID, apPassword string, now time.Time) (Device, string, error) {
	token, hash := NewToken()
	if apSSID == "" || apPassword == "" {
		apSSID, apPassword = NewHotspot()
	}
	for range 5 {
		d := Device{ClaimCode: newClaimCode(), Settings: []byte("{}"), CreatedAt: now, Firmware: firmware,
			APSSID: apSSID, APPassword: apPassword}
		res, err := s.DB.ExecContext(ctx, `insert into devices (token_hash, claim_code, created_at, fw_version, ap_ssid, ap_password)
			values (?, ?, ?, ?, ?, ?)`, hash, d.ClaimCode, fmtTime(now), firmware, apSSID, apPassword)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				continue // the claim code was taken; draw another
			}
			return d, "", err
		}
		d.ID, err = res.LastInsertId()
		return d, token, err
	}
	return Device{}, "", errors.New("no free claim code")
}

const deviceCols = `d.id, coalesce(d.user_id, 0), coalesce(u.name, ''), d.name, coalesce(d.claim_code, ''), d.settings,
	d.created_at, coalesce(d.claimed_at, ''), coalesce(d.last_checkin_at, ''), d.battery_mv, d.power, d.fw_version,
	coalesce(d.invite_until, ''), d.ap_ssid, d.ap_password
	from devices d left join users u on u.id = d.user_id`

func scanDevice(row interface{ Scan(...any) error }) (Device, error) {
	var d Device
	var settings, created, claimed, checkin, invite string
	err := row.Scan(&d.ID, &d.UserID, &d.Owner, &d.Name, &d.ClaimCode, &settings, &created, &claimed, &checkin,
		&d.BatteryMV, &d.Power, &d.Firmware, &invite, &d.APSSID, &d.APPassword)
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	d.Settings = []byte(settings)
	d.CreatedAt = parseTime(created)
	if claimed != "" {
		d.ClaimedAt = parseTime(claimed)
	}
	if checkin != "" {
		d.CheckinAt = parseTime(checkin)
	}
	if invite != "" {
		d.InviteUntil = parseTime(invite)
	}
	return d, err
}

func (s *Store) devices(ctx context.Context, query string, args ...any) ([]Device, error) {
	rows, err := s.DB.QueryContext(ctx, `select `+deviceCols+` `+query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeviceByToken finds the frame a bearer token belongs to.
func (s *Store) DeviceByToken(ctx context.Context, token string) (Device, error) {
	return scanDevice(s.DB.QueryRowContext(ctx, `select `+deviceCols+` where d.token_hash = ?`, HashToken(token)))
}

// DeviceByID finds a frame.
func (s *Store) DeviceByID(ctx context.Context, id int64) (Device, error) {
	return scanDevice(s.DB.QueryRowContext(ctx, `select `+deviceCols+` where d.id = ?`, id))
}

// DeviceByClaimCode finds a frame waiting to be claimed by its code, typed
// loosely.
func (s *Store) DeviceByClaimCode(ctx context.Context, code string) (Device, error) {
	return scanDevice(s.DB.QueryRowContext(ctx, `select `+deviceCols+` where d.claim_code = ? and d.user_id is null`,
		NormalizeClaimCode(code)))
}

// UserDevices lists the frames someone has claimed, oldest first.
func (s *Store) UserDevices(ctx context.Context, userID int64) ([]Device, error) {
	return s.devices(ctx, `where d.user_id = ? order by d.id`, userID)
}

// Devices lists every frame, newest first, for the admin page.
func (s *Store) Devices(ctx context.Context) ([]Device, error) {
	return s.devices(ctx, `order by d.id desc`)
}

// Claim gives an unclaimed frame to a user. A code that isn't waiting to be
// claimed is ErrNotFound.
func (s *Store) Claim(ctx context.Context, code string, userID int64, name string, now time.Time) (Device, error) {
	var id int64
	err := s.DB.QueryRowContext(ctx, `update devices set user_id = ?, name = ?, claimed_at = ?, claim_code = null,
		invite_until = null where claim_code = ? and user_id is null returning id`,
		userID, name, fmtTime(now), NormalizeClaimCode(code)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, ErrNotFound
	}
	if err != nil {
		return Device{}, err
	}
	return s.DeviceByID(ctx, id)
}

// MarkForInvite lets a frame's code make an account until a time, for
// someone new to set it up. Only unclaimed frames can be marked. byUserID is
// the admin who marked it, 0 from the command line.
func (s *Store) MarkForInvite(ctx context.Context, id, byUserID int64, until time.Time) error {
	by := sql.NullInt64{Int64: byUserID, Valid: byUserID != 0} // 0: the command line
	res, err := s.DB.ExecContext(ctx, `update devices set invite_until = ?, invited_by = ? where id = ? and user_id is null`,
		fmtTime(until), by, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UnmarkInvite stops a frame's code from making accounts.
func (s *Store) UnmarkInvite(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `update devices set invite_until = null, invited_by = null where id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// RedeemFrame uses a marked frame's code as an invite: it makes a new user
// with this passkey and gives them the frame, all at once. A frame that isn't
// marked (or whose mark ran out, or that was claimed meanwhile) is
// ErrNotFound; a taken name is ErrNameTaken, and nothing changes.
func (s *Store) RedeemFrame(ctx context.Context, code string, handle []byte, name string, credID, credData []byte, now time.Time) (User, Device, error) {
	var u User
	var id int64
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `select id from devices where claim_code = ? and user_id is null and invite_until > ?`,
			NormalizeClaimCode(code), fmtTime(now)).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if u, err = createUser(ctx, tx, handle, name, false, now); err != nil {
			return err
		}
		if err := addCredential(ctx, tx, u.ID, credID, credData, now); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `update devices set user_id = ?, name = 'Frame', claimed_at = ?, claim_code = null,
			invite_until = null where id = ?`, u.ID, fmtTime(now), id)
		return err
	})
	if err != nil {
		return User{}, Device{}, err
	}
	d, err := s.DeviceByID(ctx, id)
	return u, d, err
}

// SetHotspot records a frame's setup hotspot, when it says.
func (s *Store) SetHotspot(ctx context.Context, id int64, ssid, password string) error {
	_, err := s.DB.ExecContext(ctx, `update devices set ap_ssid = ?, ap_password = ? where id = ?`, ssid, password, id)
	return err
}

// UpdateDevice renames a frame and replaces its settings. Only its owner can.
func (s *Store) UpdateDevice(ctx context.Context, id, userID int64, name string, settings []byte) error {
	res, err := s.DB.ExecContext(ctx, `update devices set name = ?, settings = ? where id = ? and user_id = ?`,
		name, string(settings), id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Unclaim lets go of a frame: it forgets its owner and settings and gets a
// new claim code, so it goes back to its setup screen on its next sync.
func (s *Store) Unclaim(ctx context.Context, id, userID int64) error {
	for range 5 {
		res, err := s.DB.ExecContext(ctx, `update devices set user_id = null, claimed_at = null, name = '', settings = '{}',
			invite_until = null, claim_code = ? where id = ? and user_id = ?`, newClaimCode(), id, userID)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				continue
			}
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	}
	return errors.New("no free claim code")
}

// DeleteDevice forgets a frame entirely; its token stops working.
func (s *Store) DeleteDevice(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `delete from devices where id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CheckIn records that a frame synced, with what it said about its power.
// Zero values leave what it said before.
func (s *Store) CheckIn(ctx context.Context, id int64, now time.Time, batteryMV int, power, firmware string) error {
	_, err := s.DB.ExecContext(ctx, `update devices set last_checkin_at = ?,
		battery_mv = case when ? > 0 then ? else battery_mv end,
		power = case when ? != '' then ? else power end,
		fw_version = case when ? != '' then ? else fw_version end
		where id = ?`, fmtTime(now), batteryMV, batteryMV, power, power, firmware, firmware, id)
	return err
}

// AddEvents saves what a frame reported.
func (s *Store) AddEvents(ctx context.Context, deviceID int64, events []DeviceEvent, now time.Time) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		for _, e := range events {
			if _, err := tx.ExecContext(ctx, `insert into device_events (device_id, type, at, date, detail, received_at)
				values (?, ?, ?, ?, ?, ?)`, deviceID, e.Type, fmtTime(e.At), e.Date, e.Detail, fmtTime(now)); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeviceEvents lists a frame's latest events, newest first.
func (s *Store) DeviceEvents(ctx context.Context, deviceID int64, limit int) ([]DeviceEvent, error) {
	rows, err := s.DB.QueryContext(ctx, `select type, at, date, detail, received_at from device_events
		where device_id = ? order by at desc, id desc limit ?`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DeviceEvent{}
	for rows.Next() {
		var e DeviceEvent
		var at, received string
		if err := rows.Scan(&e.Type, &at, &e.Date, &e.Detail, &received); err != nil {
			return nil, err
		}
		e.At, e.ReceivedAt = parseTime(at), parseTime(received)
		out = append(out, e)
	}
	return out, rows.Err()
}

// SaveImage keeps a rendered frame by its hash, or marks it used again.
func (s *Store) SaveImage(ctx context.Context, sha string, bin []byte, now time.Time) error {
	t := fmtTime(now)
	_, err := s.DB.ExecContext(ctx, `insert into device_images (sha256, bin, created_at, last_used_at) values (?, ?, ?, ?)
		on conflict (sha256) do update set last_used_at = excluded.last_used_at`, sha, bin, t, t)
	return err
}

// Image is a rendered frame's gzipped framebuffer.
func (s *Store) Image(ctx context.Context, sha string) ([]byte, error) {
	var bin []byte
	err := s.DB.QueryRowContext(ctx, `select bin from device_images where sha256 = ?`, sha).Scan(&bin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return bin, err
}

const (
	// imageTTL is how long a rendered frame is kept after a bundle last
	// listed it. Bundles are rebuilt at least daily.
	imageTTL = 30 * 24 * time.Hour
	// eventTTL is how long device events are kept.
	eventTTL = 180 * 24 * time.Hour
)

func (s *Store) deleteOldDeviceData(ctx context.Context, now time.Time) error {
	if _, err := s.DB.ExecContext(ctx, `delete from device_images where last_used_at <= ?`, fmtTime(now.Add(-imageTTL))); err != nil {
		return fmt.Errorf("device images: %w", err)
	}
	if _, err := s.DB.ExecContext(ctx, `delete from device_events where received_at <= ?`, fmtTime(now.Add(-eventTTL))); err != nil {
		return fmt.Errorf("device events: %w", err)
	}
	return nil
}
