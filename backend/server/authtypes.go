package server

import "time"

// Request and response bodies for sign-in and the signed-in api. These routes
// aren't in the public spec.

// MeResponse is GET /api/v1/me: who's signed in (204 when nobody is).
type MeResponse struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
}

// InviteInfo is GET /api/v1/auth/invites/{token}: what an invite link is for.
type InviteInfo struct {
	// ForUser names who a re-invite adds a passkey for; empty for a new person.
	ForUser string `json:"for_user,omitempty"`
	IsAdmin bool   `json:"is_admin"`
	// ExpiresAt is when the link stops working.
	ExpiresAt time.Time `json:"expires_at"`
}

// RegisterBegin is POST /api/v1/auth/register/begin.
type RegisterBegin struct {
	Invite string `json:"invite"`
	// Name is the new person's display name; ignored for a re-invite.
	Name string `json:"name"`
}

// Ceremony is the reply to a register or login begin: the options to pass to
// the browser's passkey prompt, and an id to send back with the result
// (POST .../finish?ceremony=id).
type Ceremony struct {
	Ceremony string `json:"ceremony"`
	Options  any    `json:"options"`
}

// CreateInvite is POST /api/v1/admin/invites.
type CreateInvite struct {
	IsAdmin bool `json:"is_admin"`
	// ForUserID makes a re-invite: a link that adds a passkey to that person.
	ForUserID *int64 `json:"for_user_id,omitempty"`
}

// InviteLink is a new invite.
type InviteLink struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}
