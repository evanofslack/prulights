package server

import (
	"time"

	"github.com/evanofslack/prulights/schedule"
)

// Bodies for sightings and the admin page. Signed in only; not in the public
// spec (except NightStatsResponse, in apitypes.go).

// SightingRequest is PUT /api/v1/me/sightings/{date}: "I saw it", with an
// optional 1–5 rating (0 for none) and a note of up to 280 characters.
type SightingRequest struct {
	Rating int    `json:"rating"`
	Note   string `json:"note"`
}

// Sighting is one person's sighting of a night.
type Sighting struct {
	Date   schedule.Date `json:"date"`
	Name   string        `json:"name"`
	Mine   bool          `json:"mine"`
	Rating int           `json:"rating"`
	Note   string        `json:"note"`
	At     time.Time     `json:"at"`
}

// NightSightings is GET /api/v1/sightings/{date}: everyone who saw a night.
type NightSightings struct {
	Date      schedule.Date `json:"date"`
	Sightings []Sighting    `json:"sightings"`
	// CanLog is whether the night is still open for "I saw it": tonight and
	// the week before.
	CanLog bool `json:"can_log"`
}

// LoggedNight is one of your sightings with the night it was of.
type LoggedNight struct {
	Sighting `tstype:",extends"`
	Lighting schedule.Lighting `json:"lighting"`
}

// MySightings is GET /api/v1/me/sightings: everything you've logged, newest first.
type MySightings struct {
	Nights []LoggedNight `json:"nights"`
}

// ColorCount is how many of your nights had a color.
type ColorCount struct {
	Color schedule.ColorKey `json:"color"`
	Count int               `json:"count"`
}

// MyStats is GET /api/v1/me/stats.
type MyStats struct {
	Seen    int     `json:"seen"`
	Rated   int     `json:"rated"`
	Average float64 `json:"average"`
	// Streak is how many nights in a row you've seen, up to tonight (or last
	// night, if tonight isn't logged yet).
	Streak int `json:"streak"`
	// Favorites are the colors on the most of your nights, most first.
	Favorites []ColorCount `json:"favorites"`
	// LogFrom is the earliest night you can still log.
	LogFrom schedule.Date `json:"log_from"`
	Tonight schedule.Date `json:"tonight"`
}

// OverrideBody is PUT /api/v1/admin/overrides/{date}: a correction to a night.
type OverrideBody struct {
	Colors   []schedule.ColorKey `json:"colors"`
	Cause    string              `json:"cause"`
	CauseURL string              `json:"cause_url"`
	// Kind is worked out from the cause when empty.
	Kind schedule.Kind `json:"kind"`
	Note string        `json:"note"`
}

// OverrideInfo is a correction, as the admin page lists them.
type OverrideInfo struct {
	Date         schedule.Date `json:"date"`
	OverrideBody `tstype:",extends"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserInfo is a person, as the admin page lists them.
type UserInfo struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
	Sightings int       `json:"sightings"`
}

// ScrapeRunInfo is one check of the source pages.
type ScrapeRunInfo struct {
	ID         int64     `json:"id"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Pages      int       `json:"pages"`
	Fetched    int       `json:"fetched"`
	Items      int       `json:"items"`
	Added      int       `json:"added"`
	Changed    int       `json:"changed"`
	Removed    int       `json:"removed"`
	Warnings   []string  `json:"warnings"`
	Error      string    `json:"error,omitempty"`
}

// ScrapeStatus is GET /api/v1/admin/scrape: recent runs, and whether one is
// going now.
type ScrapeStatus struct {
	Running bool            `json:"running"`
	Runs    []ScrapeRunInfo `json:"runs"`
}
