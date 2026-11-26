// Package prudle is the December game: guess the night's colors, with its
// cause as the clue. Each turn is a whole guess, a set of colors; the ones
// already found stay in it for free. Every new color comes back a hit, close
// (another color of its family is on, see Close) or a miss, and the guess's
// size comes back against the night's count: fewer, more or exact. Misses
// cost 1, close ½, hits nothing; more than Budget misses loses. Pure, no I/O.
//
// It is web/lib/prudle.ts's guess at the rules the lab settled on
// (maxPicks 4, count "dir", close "half", bands "exact"), and the server's
// copy is the one that counts: testdata/games.json keeps the two in step.
package prudle

import (
	"errors"
	"slices"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

const (
	// Budget is how many misses a game can take: one more loses.
	Budget = 5
	// MaxPicks is the most colors in one guess, found ones included.
	MaxPicks = 4
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

// Pick is one new color of a guess and what it was.
type Pick struct {
	Key  schedule.ColorKey `json:"key"`
	Mark Mark              `json:"mark"`
}

// Turn is one guess: its new colors, its whole size (found colors
// included) and the count.
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
	Misses float64
	Lo, Hi int  // what the guesser knows of the count; Hi 0 until a "fewer"
	Bands  bool // the crown shows its empty bands (the count is known)
}

// New starts a game on a night's colors.
func New(answer []schedule.ColorKey) Game {
	var a []schedule.ColorKey
	for _, k := range answer {
		if !slices.Contains(a, k) {
			a = append(a, k)
		}
	}
	return Game{Answer: a, Turns: []Turn{}, Found: []schedule.ColorKey{}, Out: map[schedule.ColorKey]Mark{}, Lo: 1}
}

// Solved is whether every color is found.
func (g Game) Solved() bool { return len(g.Answer) > 0 && len(g.Found) == len(g.Answer) }

// Lost is whether the misses went past the budget.
func (g Game) Lost() bool { return !g.Solved() && g.Misses > Budget }

// Over is whether the game takes no more guesses.
func (g Game) Over() bool { return g.Solved() || g.Lost() }

var (
	ErrOver     = errors.New("the game is over")
	ErrEmpty    = errors.New("pick a color")
	ErrUnknown  = errors.New("not a color")
	ErrRepeat   = errors.New("already guessed")
	ErrTooMany  = errors.New("at most 4 colors")
	errNoAnswer = errors.New("no colors to guess")
)

// Check says why picks aren't a guess yet, or nil when they are.
func (g Game) Check(picks []schedule.ColorKey) error {
	if len(g.Answer) == 0 {
		return errNoAnswer
	}
	if g.Over() {
		return ErrOver
	}
	if len(picks) == 0 {
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
	turn := Turn{Picks: make([]Pick, len(picks)), Size: len(g.Found) + len(picks)}
	for i, k := range picks {
		mark := Miss
		if slices.Contains(g.Answer, k) {
			mark = Hit
		} else if slices.ContainsFunc(g.Answer, func(a schedule.ColorKey) bool { return IsClose(a, k) }) {
			mark = Close
		}
		switch mark {
		case Hit:
			next.Found = append(next.Found, k)
		case Close:
			next.Out[k] = mark
			next.Misses += 0.5
		default:
			next.Out[k] = mark
			next.Misses++
		}
		turn.Picks[i] = Pick{Key: k, Mark: mark}
	}
	n := len(g.Answer)
	switch {
	case turn.Size == n:
		turn.Count = Exact
		next.Lo, next.Hi = n, n
	case turn.Size > n:
		turn.Count = Fewer
		if next.Hi == 0 || turn.Size-1 < next.Hi {
			next.Hi = turn.Size - 1
		}
	default:
		turn.Count = More
		next.Lo = max(next.Lo, turn.Size+1)
	}
	next.Lo = max(next.Lo, len(next.Found))
	next.Turns = append(slices.Clone(g.Turns), turn)
	next.Bands = g.Bands || turn.Count == Exact || next.Solved()
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
