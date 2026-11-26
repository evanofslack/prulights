package server

import (
	"github.com/evanofslack/prulights/schedule"
)

// Bodies for Prudle, December's game (backend/prudle). Signed in only; not in
// the public spec.

// PrudleResult is how one of your nights went.
type PrudleResult struct {
	Date schedule.Date `json:"date"`
	// Status is "playing", "won", "lost" or "passed" ("just show me").
	Status string  `json:"status"`
	Turns  int     `json:"turns"`
	Misses float64 `json:"misses"`
}

// PrudleSummary is GET /api/v1/me/prudle: whether you play tonight, and
// your December so far.
type PrudleSummary struct {
	Tonight schedule.Date `json:"tonight"`
	// Season is whether tonight is in December, when Prudle runs.
	Season bool `json:"season"`
	// Playable is whether tonight has a Prudle: in season and listed with a
	// cause and colors.
	Playable bool `json:"playable"`
	// Off is whether you turned Prudle off and see every color.
	Off bool `json:"off"`
	// Done is whether tonight's game is over (won, lost or passed).
	Done bool `json:"done"`
	// Games are this December's, oldest first.
	Games  []PrudleResult `json:"games"`
	Won    int            `json:"won"`
	Streak int            `json:"streak"` // nights won in a row, through tonight or last night
}

// PrudlePick is one new color of a guess and what it was: "hit", "close" or "miss".
type PrudlePick struct {
	Key  schedule.ColorKey `json:"key"`
	Mark string            `json:"mark"`
}

// PrudleTurn is one guess: its new colors, its whole size (colors already
// found ride along) and the count: "fewer", "more" or "exact".
type PrudleTurn struct {
	Picks []PrudlePick `json:"picks"`
	Size  int          `json:"size"`
	Count string       `json:"count"`
}

// PrudleState is GET /api/v1/me/prudle/{date}: your game of a night. The
// answer only comes once the game is over or the night is past.
type PrudleState struct {
	Date     schedule.Date       `json:"date"`
	Cause    string              `json:"cause"`
	Budget   int                 `json:"budget"`    // misses allowed; one more loses
	MaxPicks int                 `json:"max_picks"` // the most colors in one guess
	Status   string              `json:"status"`
	CanPlay  bool                `json:"can_play"` // tonight, and not over
	Turns    []PrudleTurn        `json:"turns"`
	Found    []schedule.ColorKey `json:"found"`
	// Out are the colors guessed and not on, with their mark.
	Out    []PrudlePick `json:"out"`
	Misses float64      `json:"misses"`
	// Lo and Hi are what's known of how many colors; Hi is 0 until a "fewer".
	Lo int `json:"lo"`
	Hi int `json:"hi"`
	// Bands is whether the count is known (then it's Lo).
	Bands  bool                `json:"bands"`
	Answer []schedule.ColorKey `json:"answer,omitempty"`
}

// PrudleGuess is POST /api/v1/me/prudle/{date}/guesses: the new colors of a
// guess (the ones already found ride along on their own).
type PrudleGuess struct {
	Colors []schedule.ColorKey `json:"colors"`
}

// PrudleSettings is PUT /api/v1/me/prudle/settings.
type PrudleSettings struct {
	Off bool `json:"off"`
}

// PrudleRow is one guess as marks only, for sharing: the colors already
// found, then the new ones good news first, and the count.
type PrudleRow struct {
	Marks []string `json:"marks"`
	Size  int      `json:"size"`
	Count string   `json:"count"`
}

// PrudlePlayer is someone's finished game of a night, without its colors.
type PrudlePlayer struct {
	Name   string      `json:"name"`
	Mine   bool        `json:"mine"`
	Status string      `json:"status"`
	Misses float64     `json:"misses"`
	Rows   []PrudleRow `json:"rows"`
}

// PrudleNight is GET /api/v1/prudle/{date}: how everyone did. Locked (and
// empty) until your own game of the night is over, or the night is past.
type PrudleNight struct {
	Date    schedule.Date  `json:"date"`
	Locked  bool           `json:"locked"`
	Players []PrudlePlayer `json:"players"`
}
