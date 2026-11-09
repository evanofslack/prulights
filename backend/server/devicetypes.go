package server

import (
	"time"

	"github.com/evanofslack/prulights/schedule"
)

// Bodies for frames (the device api under /api/device/v1, which the emulator
// uses too) and for managing them when signed in. Not in the public spec.

// DeviceSettings are what a frame's owner chooses. Views are render view
// names: tonight, week, month, log.
type DeviceSettings struct {
	// DefaultView is what the frame shows after it syncs or redraws.
	DefaultView string `json:"default_view"`
	// Views are the ones the button cycles through, in order.
	Views []string `json:"views"`
	// QuietStart and QuietEnd ("22:00", "07:00", Boston time) bound when the
	// frame shows "good night" and doesn't redraw. Both empty for none.
	QuietStart string `json:"quiet_start"`
	QuietEnd   string `json:"quiet_end"`
	// Paused frames keep their picture and only check in once a day.
	Paused bool `json:"paused"`
}

// RegisterRequest is POST /api/device/v1/register.
type RegisterRequest struct {
	Firmware string `json:"firmware"`
}

// RegisterResponse is a new frame's identity. The token is shown once; the
// frame sends it as "Authorization: Bearer <token>" from then on.
type RegisterResponse struct {
	DeviceID  int64  `json:"device_id"`
	Token     string `json:"token"`
	ClaimCode string `json:"claim_code"`
	ClaimURL  string `json:"claim_url"`
}

// DeviceState is GET /api/device/v1/state, and the answer to POST /events:
// what the frame should know each time it wakes up.
type DeviceState struct {
	Claimed bool   `json:"claimed"`
	Name    string `json:"name"`
	// ClaimCode and ClaimURL are set while nobody has claimed the frame.
	ClaimCode string `json:"claim_code,omitempty"`
	ClaimURL  string `json:"claim_url,omitempty"`
	// BundleVersion changes whenever any of the frame's pictures would.
	BundleVersion string `json:"bundle_version"`
	// NextCheckinS is how many seconds to sleep before checking in again.
	NextCheckinS int            `json:"next_checkin_s"`
	ServerTime   time.Time      `json:"server_time"`
	TZ           string         `json:"tz"`
	Tonight      schedule.Date  `json:"tonight"`
	Settings     DeviceSettings `json:"settings"`
}

// BundleItem is one picture: a view of a night, as a gzipped framebuffer.
type BundleItem struct {
	Date schedule.Date `json:"date"`
	View string        `json:"view"`
	// Stale marks the picture to show once the bundle has run out of nights:
	// the night after the last one, with a banner saying the schedule is old.
	Stale  bool   `json:"stale,omitempty"`
	SHA256 string `json:"sha256"`
	Size   int    `json:"size"`
	URL    string `json:"url"`
}

// Bundle is GET /api/device/v1/bundle: every picture for the next week. A
// frame downloads only the hashes it doesn't have.
type Bundle struct {
	Version string        `json:"version"`
	Tonight schedule.Date `json:"tonight"`
	Items   []BundleItem  `json:"items"`
}

// DeviceEventIn is something the frame noticed: short, long or double (button
// presses), seen (logged with a long press), boot, sync, redraw or error.
type DeviceEventIn struct {
	Type string    `json:"type"`
	At   time.Time `json:"at"`
	// Date is the night a "seen" is for; tonight at At when empty.
	Date   schedule.Date `json:"date,omitempty"`
	Detail string        `json:"detail,omitempty"`
}

// EventsRequest is POST /api/device/v1/events: the frame's check-in, with
// whatever it queued while asleep or offline.
type EventsRequest struct {
	Events    []DeviceEventIn `json:"events"`
	BatteryMV int             `json:"battery_mv"`
	Power     string          `json:"power"` // usb | battery
	Firmware  string          `json:"firmware"`
}

// ClaimRequest is POST /api/v1/devices/claim: the code on a frame's setup
// screen, and what to call the frame.
type ClaimRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// DeviceInfo is a frame, as its owner sees it.
type DeviceInfo struct {
	ID        int64          `json:"id"`
	Name      string         `json:"name"`
	ClaimedAt time.Time      `json:"claimed_at"`
	CheckinAt *time.Time     `json:"last_checkin_at,omitempty"`
	BatteryMV int            `json:"battery_mv"`
	Power     string         `json:"power"`
	Firmware  string         `json:"firmware"`
	Settings  DeviceSettings `json:"settings"`
}

// DeviceUpdate is PATCH /api/v1/me/devices/{id}. Fields left out stay.
type DeviceUpdate struct {
	Name     *string         `json:"name,omitempty"`
	Settings *DeviceSettings `json:"settings,omitempty"`
}

// AdminDevice is a frame, as the admin page lists them.
type AdminDevice struct {
	DeviceInfo `tstype:",extends"`
	Owner      string    `json:"owner"`
	ClaimCode  string    `json:"claim_code,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
