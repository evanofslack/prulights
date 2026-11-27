// Package prudle is the December game: guess the night's colors, with its
// cause as the clue. A guess is a whole set of colors and wins only when it
// is the answer, no more and no fewer. Every color in it comes back a hit,
// close (another color of its family is on, see Families) or a miss, and its
// size comes back against the night's count: fewer, more or exact, so the
// count is narrowed, never shown. Colors found on ride along in the next
// guess, since leaving one out can never win. Each guess is a try; Tries
// wrong ones lose. Pure, no I/O.
//
// web/lib/prudle.ts plays the same rules (the lab tunes them there); the
// server's copy is the one that counts: testdata/games.json keeps the two in
// step.
package prudle

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

const (
	// Tries is how many guesses a game takes: the last wrong one loses.
	Tries = 6
	// MaxPicks is the most colors in one guess, found ones included. No
	// December night has had more than three.
	MaxPicks = 3
)

// InSeason is whether a night has a Prudle: December only.
func InSeason(d schedule.Date) bool {
	t, err := time.Parse("2006-01-02", string(d))
	return err == nil && t.Month() == time.December
}

// Mark is what a guessed color was.
type Mark string

const (
	Hit   Mark = "hit"
	Close Mark = "close"
	Miss  Mark = "miss"
)

// Count is a guess's size against the night's count.
type Count string

const (
	Fewer Count = "fewer" // the night has fewer colors than the guess
	More  Count = "more"
	Exact Count = "exact"
)

// Families are the colors a near guess counts in; each color is in at most
// one. Black and rainbow are close to nothing. The web's CLOSE is the same.
var Families = [][]schedule.ColorKey{
	{schedule.Red, schedule.Maroon, schedule.Pink},
	{schedule.Yellow, schedule.Gold, schedule.Orange},
	{schedule.Blue, schedule.Purple},
	{schedule.Green, schedule.Teal},
	{schedule.White, schedule.Silver},
}

// IsClose is whether two different colors share a family.
func IsClose(a, b schedule.ColorKey) bool {
	if a == b {
		return false
	}
	for _, f := range Families {
		if slices.Contains(f, a) && slices.Contains(f, b) {
			return true
		}
	}
	return false
}

// Pick is one color of a guess and what it was.
type Pick struct {
	Key     schedule.ColorKey `json:"key"`
	Mark    Mark              `json:"mark"`
	Carried bool              `json:"carried"` // found on before this guess
}

// Turn is one guess: the whole set (carried colors first, then the new
// ones), its size and the count.
type Turn struct {
	Picks []Pick `json:"picks"`
	Size  int    `json:"size"`
	Count Count  `json:"count"`
}

// Game is a game so far.
type Game struct {
	Answer []schedule.ColorKey
	Turns  []Turn
	Found  []schedule.ColorKey // in the order found
	Out    map[schedule.ColorKey]Mark
	Won    bool
}

// New starts a game on a night's colors.
func New(answer []schedule.ColorKey) Game {
	var a []schedule.ColorKey
	for _, k := range answer {
		if !slices.Contains(a, k) {
			a = append(a, k)
		}
	}
	return Game{Answer: a, Turns: []Turn{}, Found: []schedule.ColorKey{}, Out: map[schedule.ColorKey]Mark{}}
}

// Solved is whether a guess was the answer.
func (g Game) Solved() bool { return g.Won }

// Lost is whether the tries ran out.
func (g Game) Lost() bool { return !g.Won && len(g.Turns) >= Tries }

// Over is whether the game takes no more guesses.
func (g Game) Over() bool { return g.Solved() || g.Lost() }

// Wrong is how many guesses missed.
func (g Game) Wrong() int {
	if g.Won {
		return len(g.Turns) - 1
	}
	return len(g.Turns)
}

// Known is the count once a guess was its size, else 0.
func (g Game) Known() int {
	for _, t := range g.Turns {
		if t.Count == Exact {
			return t.Size
		}
	}
	return 0
}

var (
	ErrOver     = errors.New("the game is over")
	ErrEmpty    = errors.New("pick a color")
	ErrUnknown  = errors.New("not a color")
	ErrRepeat   = errors.New("already guessed")
	ErrTooMany  = errors.New("at most 3 colors")
	errNoAnswer = errors.New("no colors to guess")
)

func setKey(keys []schedule.ColorKey) string {
	s := make([]string, len(keys))
	for i, k := range keys {
		s[i] = string(k)
	}
	slices.Sort(s)
	return strings.Join(s, ",")
}

// Check says why picks (the new colors; found ones ride along) aren't a
// guess yet, or nil when they are.
func (g Game) Check(picks []schedule.ColorKey) error {
	if len(g.Answer) == 0 {
		return errNoAnswer
	}
	if g.Over() {
		return ErrOver
	}
	if len(g.Found)+len(picks) == 0 {
		return ErrEmpty
	}
	for i, k := range picks {
		if _, ok := schedule.Lookup(k); !ok {
			return ErrUnknown
		}
		if slices.Contains(picks[:i], k) || slices.Contains(g.Found, k) {
			return ErrRepeat
		}
		if _, out := g.Out[k]; out {
			return ErrRepeat
		}
	}
	if len(g.Found)+len(picks) > MaxPicks {
		return ErrTooMany
	}
	key := setKey(append(slices.Clone(g.Found), picks...))
	for _, t := range g.Turns {
		keys := make([]schedule.ColorKey, len(t.Picks))
		for i, p := range t.Picks {
			keys[i] = p.Key
		}
		if setKey(keys) == key {
			return ErrRepeat
		}
	}
	return nil
}

// Guess grades picks and returns the game after them.
func (g Game) Guess(picks []schedule.ColorKey) (Game, error) {
	if err := g.Check(picks); err != nil {
		return g, err
	}
	next := g
	next.Found = slices.Clone(g.Found)
	next.Out = make(map[schedule.ColorKey]Mark, len(g.Out)+len(picks))
	for k, m := range g.Out {
		next.Out[k] = m
	}
	turn := Turn{Picks: []Pick{}, Size: len(g.Found) + len(picks)}
	for _, k := range g.Found {
		turn.Picks = append(turn.Picks, Pick{Key: k, Mark: Hit, Carried: true})
	}
	hits := len(g.Found)
	for _, k := range picks {
		mark := Miss
		if slices.Contains(g.Answer, k) {
			mark = Hit
		} else if slices.ContainsFunc(g.Answer, func(a schedule.ColorKey) bool { return IsClose(a, k) }) {
			mark = Close
		}
		if mark == Hit {
			next.Found = append(next.Found, k)
			hits++
		} else {
			next.Out[k] = mark
		}
		turn.Picks = append(turn.Picks, Pick{Key: k, Mark: mark})
	}
	n := len(g.Answer)
	switch {
	case turn.Size == n:
		turn.Count = Exact
	case turn.Size > n:
		turn.Count = Fewer
	default:
		turn.Count = More
	}
	next.Won = turn.Size == n && hits == n
	next.Turns = append(slices.Clone(g.Turns), turn)
	return next, nil
}

// Replay plays saved guesses again on an answer.
func Replay(answer []schedule.ColorKey, guesses [][]schedule.ColorKey) (Game, error) {
	g := New(answer)
	for _, p := range guesses {
		var err error
		if g, err = g.Guess(p); err != nil {
			return g, err
		}
	}
	return g, nil
}
