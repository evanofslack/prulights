package prudle

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/evanofslack/prulights/schedule"
)

var glyph = map[Mark]string{Hit: "●", Close: "◐", Miss: "○"}

// rowOf is a turn the way web/lib/prudle.ts's row and the share text draw
// it: carried colors on, then the new picks good news first, then the count.
func rowOf(t Turn) string {
	var b strings.Builder
	b.WriteString(strings.Repeat(glyph[Hit], t.Size-len(t.Picks)))
	for _, m := range []Mark{Hit, Close, Miss} {
		for _, p := range t.Picks {
			if p.Mark == m {
				b.WriteString(glyph[m])
			}
		}
	}
	return fmt.Sprintf("%s %s %d", b.String(), t.Count, t.Size)
}

// TestGames plays the shared cases in testdata/games.json.
func TestGames(t *testing.T) {
	raw, err := os.ReadFile("testdata/games.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name    string
		Answer  []schedule.ColorKey
		Guesses [][]schedule.ColorKey
		Turns   []string
		Misses  float64
		Solved  bool
		Lost    bool
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			g, err := Replay(c.Answer, c.Guesses)
			if err != nil {
				t.Fatal(err)
			}
			var rows []string
			for _, turn := range g.Turns {
				rows = append(rows, rowOf(turn))
			}
			if !slices.Equal(rows, c.Turns) {
				t.Errorf("turns = %q, want %q", rows, c.Turns)
			}
			if g.Misses != c.Misses || g.Solved() != c.Solved || g.Lost() != c.Lost {
				t.Errorf("misses %v solved %v lost %v", g.Misses, g.Solved(), g.Lost())
			}
		})
	}
}

func TestCheck(t *testing.T) {
	g, _ := Replay([]schedule.ColorKey{"red", "blue", "gold"}, [][]schedule.ColorKey{{"red", "pink"}})
	for _, c := range []struct {
		picks []schedule.ColorKey
		want  error
	}{
		{nil, ErrEmpty},
		{[]schedule.ColorKey{"red"}, ErrRepeat},  // found
		{[]schedule.ColorKey{"pink"}, ErrRepeat}, // out
		{[]schedule.ColorKey{"blue", "blue"}, ErrRepeat},
		{[]schedule.ColorKey{"mauve"}, ErrUnknown},
		{[]schedule.ColorKey{"blue", "gold", "green", "teal"}, ErrTooMany}, // red carries: 5
		{[]schedule.ColorKey{"blue", "gold", "green"}, nil},
	} {
		if err := g.Check(c.picks); !errors.Is(err, c.want) {
			t.Errorf("%v: %v, want %v", c.picks, err, c.want)
		}
	}
	won, _ := g.Guess([]schedule.ColorKey{"blue", "gold"})
	if !won.Solved() || !won.Bands {
		t.Fatalf("not solved: %+v", won)
	}
	if _, err := won.Guess([]schedule.ColorKey{"green"}); !errors.Is(err, ErrOver) {
		t.Errorf("after solved: %v", err)
	}
	if g.Turns[0].Count != More || g.Lo != 3 || len(g.Out) != 1 {
		t.Errorf("first turn: %+v lo %d", g.Turns[0], g.Lo)
	}
}

func TestCountBounds(t *testing.T) {
	g, _ := Replay([]schedule.ColorKey{"red"}, [][]schedule.ColorKey{{"blue", "green", "white"}, {"black", "silver"}})
	if g.Hi != 1 || g.Lo != 1 || g.Bands {
		t.Errorf("lo %d hi %d bands %v", g.Lo, g.Hi, g.Bands)
	}
}

func TestInSeason(t *testing.T) {
	for d, want := range map[schedule.Date]bool{"2026-12-01": true, "2026-12-31": true, "2026-11-30": false, "2027-01-01": false, "x": false} {
		if InSeason(d) != want {
			t.Errorf("%s: %v", d, !want)
		}
	}
}
