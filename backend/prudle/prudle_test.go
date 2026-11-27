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

// rowOf is a turn the way web/lib/prudle.ts's glyphs and the share text draw
// it: the whole guess good news first, then the count.
func rowOf(t Turn) string {
	var b strings.Builder
	for _, m := range []Mark{Hit, Close, Miss} {
		for _, p := range t.Picks {
			if p.Mark == m {
				b.WriteString(glyph[m])
			}
		}
	}
	return fmt.Sprintf("%s %s %d", b.String(), t.Count, t.Size)
}

// TestGames plays the cases in testdata/games.json.
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
		Wrong   int
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
			if g.Wrong() != c.Wrong || g.Solved() != c.Solved || g.Lost() != c.Lost {
				t.Errorf("wrong %v solved %v lost %v", g.Wrong(), g.Solved(), g.Lost())
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
		{[]schedule.ColorKey{"red"}, ErrRepeat},  // found
		{[]schedule.ColorKey{"pink"}, ErrRepeat}, // out
		{[]schedule.ColorKey{"blue", "blue"}, ErrRepeat},
		{[]schedule.ColorKey{"mauve"}, ErrUnknown},
		{[]schedule.ColorKey{"blue", "gold", "green"}, ErrTooMany}, // red carries: 4
		{nil, nil}, // red alone is a new guess
		{[]schedule.ColorKey{"blue", "green"}, nil},
	} {
		if err := g.Check(c.picks); !errors.Is(err, c.want) {
			t.Errorf("%v: %v, want %v", c.picks, err, c.want)
		}
	}
	if err := New([]schedule.ColorKey{"red"}).Check(nil); !errors.Is(err, ErrEmpty) {
		t.Errorf("empty first guess: %v", err)
	}
	alone, _ := g.Guess(nil)
	if alone.Won || alone.Turns[1].Count != More {
		t.Fatalf("red alone: %+v", alone)
	}
	if err := alone.Check(nil); !errors.Is(err, ErrRepeat) {
		t.Errorf("red alone again: %v", err)
	}
	won, _ := alone.Guess([]schedule.ColorKey{"blue", "gold"})
	if !won.Solved() || won.Known() != 3 || won.Wrong() != 2 {
		t.Fatalf("not solved: %+v", won)
	}
	if _, err := won.Guess([]schedule.ColorKey{"green"}); !errors.Is(err, ErrOver) {
		t.Errorf("after solved: %v", err)
	}
	if p := won.Turns[2].Picks[0]; !p.Carried || p.Key != "red" {
		t.Errorf("carried: %+v", won.Turns[2].Picks)
	}
}

func TestInSeason(t *testing.T) {
	for d, want := range map[schedule.Date]bool{"2026-12-01": true, "2026-12-31": true, "2026-11-30": false, "2027-01-01": false, "x": false} {
		if InSeason(d) != want {
			t.Errorf("%s: %v", d, !want)
		}
	}
}
