package server

import (
	"time"

	"github.com/evanofslack/prulights/schedule"
)

// The api's response bodies. tygo turns these into web/interfaces/api.ts.

// TonightResponse is GET /api/v1/tonight.
type TonightResponse struct {
	Date     schedule.Date       `json:"date"`
	Lighting schedule.Lighting   `json:"lighting"`
	Upcoming []schedule.Lighting `json:"upcoming"`
	// PublishedThrough is the last night the source lists. White nights after
	// it may just not be announced yet.
	PublishedThrough schedule.Date `json:"published_through"`
}

// LightingsResponse is GET /api/v1/lightings.
type LightingsResponse struct {
	From             schedule.Date       `json:"from"`
	To               schedule.Date       `json:"to"`
	Lightings        []schedule.Lighting `json:"lightings"`
	PublishedThrough schedule.Date       `json:"published_through"`
}

// PaletteResponse is GET /api/v1/palette.
type PaletteResponse struct {
	Colors []schedule.Color `json:"colors"`
}

// MetaSource is one page the schedule is read from.
type MetaSource struct {
	URL       string           `json:"url"`
	Title     string           `json:"title"`
	Year      int              `json:"year"`
	Program   schedule.Program `json:"program"`
	Nights    int              `json:"nights"`
	CheckedAt time.Time        `json:"checked_at"`
	ChangedAt time.Time        `json:"changed_at"`
}

// MetaResponse is GET /api/v1/meta: where the data comes from and how fresh it is.
type MetaResponse struct {
	Tonight          schedule.Date `json:"tonight"`
	Timezone         string        `json:"timezone"`
	RolloverHour     int           `json:"rollover_hour"`
	PublishedThrough schedule.Date `json:"published_through"`
	LastScrape       *time.Time    `json:"last_scrape"`
	Sources          []MetaSource  `json:"sources"`
	Attribution      string        `json:"attribution"`
	Version          string        `json:"version"`
	// Dev is true when the server runs with DEV=1, so dev tools like the
	// frame emulator's previews are available.
	Dev bool `json:"dev"`
}

// apiError is the body of every error response.
type apiError struct {
	Error string `json:"error"`
}

// NightStatsResponse is GET /api/v1/lightings/{date}/stats: how many people
// said they saw a night, and how they rated it. No names.
type NightStatsResponse struct {
	Date    schedule.Date `json:"date"`
	Seen    int           `json:"seen"`
	Rated   int           `json:"rated"`
	Average float64       `json:"average"`
}
