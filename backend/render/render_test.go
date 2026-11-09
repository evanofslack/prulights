package render

import (
	"bytes"
	"compress/gzip"
	"flag"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evanofslack/prulights/schedule"
)

var update = flag.Bool("update", false, "rewrite the golden PNGs in testdata/golden")

type goldenCase struct {
	name string
	view View
	data func() Data
}

func goldenCases() []goldenCase {
	dec12 := func() Data { return SampleData("2026-12-12") }
	return []goldenCase{
		{"tonight", ViewTonight, dec12},
		{"tonight-seen", ViewTonight, func() Data {
			d := dec12()
			d.Sightings["2026-12-12"] = Sighting{Seen: true, Rating: 4}
			return d
		}},
		// nothing listed: the crown is white
		{"tonight-white", ViewTonight, func() Data { return SampleData("2026-11-20") }},
		{"tonight-rainbow-stale", ViewTonight, func() Data {
			d := SampleData("2026-06-01")
			d.StaleSince = "2026-05-28"
			d.LowBattery = true
			return d
		}},
		{"week", ViewWeek, dec12},
		{"month", ViewMonth, dec12},
		{"log", ViewLog, dec12},
		{"log-empty", ViewLog, func() Data {
			d := dec12()
			d.Stats = nil
			return d
		}},
		{"setup", ViewSetup, dec12},
		{"goodnight", ViewGoodnight, dec12},
	}
}

// Each view, drawn from fixed data, must match its golden PNG (measured
// colors). After a deliberate change, look at the new images and run
// go test ./render -update.
func TestGolden(t *testing.T) {
	for _, gc := range goldenCases() {
		t.Run(gc.name, func(t *testing.T) {
			f, err := Render(gc.view, gc.data())
			if err != nil {
				t.Fatal(err)
			}
			for i, ink := range f.Pix {
				if ink >= numInks {
					t.Fatalf("pixel %d is not an ink: %d", i, ink)
				}
			}
			got := f.PNG(Measured)
			file := filepath.Join("testdata", "golden", gc.name+".png")
			if *update {
				if err := os.WriteFile(file, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("%v (run go test ./render -update)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s changed; look at it and run go test ./render -update", file)
			}
		})
	}
}

// Frames don't depend on anything but their data.
func TestRenderDeterministic(t *testing.T) {
	a, _ := Render(ViewTonight, SampleData("2026-12-12"))
	b, _ := Render(ViewTonight, SampleData("2026-12-12"))
	if a.SHA256() != b.SHA256() {
		t.Error("the same data rendered twice differs")
	}
	c, _ := Render(ViewTonight, SampleData("2026-12-13"))
	if a.SHA256() == c.SHA256() {
		t.Error("different nights rendered the same")
	}
}

func TestRenderErrors(t *testing.T) {
	if _, err := Render("nope", SampleData("2026-12-12")); err == nil {
		t.Error("unknown view rendered")
	}
	if _, err := Render(ViewTonight, Data{}); err == nil {
		t.Error("rendered without a date")
	}
}

// A color's dither puts its second ink on about Mix of the pixels.
func TestDitherRatio(t *testing.T) {
	for _, col := range schedule.Palette() {
		if col.Ink.Mix == 0 {
			continue
		}
		p := ColorPaint(col.Key)
		b := pigmentInk[col.Ink.B]
		n := 0
		for y := range 64 {
			for x := range 64 {
				if p.At(x, y) == b {
					n++
				}
			}
		}
		got := float64(n) / (64 * 64)
		if math.Abs(got-col.Ink.Mix) > 1.0/64 {
			t.Errorf("%s: %.3f of pixels are %v, want %.3f", col.Key, got, col.Ink.B, col.Ink.Mix)
		}
	}
	if ColorPaint("chartreuse") != Solid(White) {
		t.Error("an unknown color should paint white")
	}
}

// Text is never dithered: on white paper, black text leaves only black and
// white pixels, and colored text only its own ink.
func TestTextIsSolid(t *testing.T) {
	for _, ink := range []Ink{Black, Red, Blue} {
		c := newCanvas(300, 60, White)
		c.text(4, 44, style{w: regular, size: 28, ink: ink}, "Blue, Pink & Gold")
		seen := map[Ink]int{}
		for _, p := range c.pix {
			seen[p]++
		}
		if len(seen) != 2 || seen[ink] == 0 {
			t.Errorf("text in %v left inks %v", ink, seen)
		}
	}
}

func TestPacked(t *testing.T) {
	defer func(v bool) { RotateClockwise = v }(RotateClockwise)
	f := &Frame{Pix: make([]Ink, Width*Height)}
	for i := range f.Pix {
		f.Pix[i] = White
	}
	f.Pix[0] = Red          // portrait top left
	f.Pix[Width*1+0] = Blue // one row down
	f.Pix[Width-1] = Green  // portrait top right

	RotateClockwise = true
	b := f.Packed()
	if len(b) != PanelBytes || PanelBytes != 192000 {
		t.Fatalf("packed %d bytes", len(b))
	}
	// portrait (x, y) → panel (y, 479-x); left pixel in the high nibble
	nibble := func(nx, ny int) byte {
		i := ny*PanelWidth + nx
		if i%2 == 0 {
			return b[i/2] >> 4
		}
		return b[i/2] & 0xf
	}
	if got := nibble(0, 479); got != panelCode[Red] {
		t.Errorf("top left → panel (0,479) = %x", got)
	}
	if got := nibble(1, 479); got != panelCode[Blue] {
		t.Errorf("second row → panel (1,479) = %x", got)
	}
	if got := nibble(0, 0); got != panelCode[Green] {
		t.Errorf("top right → panel (0,0) = %x", got)
	}
	if got := nibble(400, 200); got != panelCode[White] {
		t.Errorf("white pixel = %x", got)
	}

	RotateClockwise = false
	b = f.Packed()
	if got := nibble(799, 0); got != panelCode[Red] {
		t.Errorf("counterclockwise: top left → panel (799,0) = %x", got)
	}

	// the download is the packed frame, gzipped
	r, err := gzip.NewReader(bytes.NewReader(f.PanelBin()))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(r)
	if !bytes.Equal(raw, f.Packed()) {
		t.Error("PanelBin doesn't unzip to Packed")
	}
	if len(f.SHA256()) != 64 {
		t.Errorf("sha = %q", f.SHA256())
	}
}

func TestPanelCodes(t *testing.T) {
	want := map[Ink]byte{Black: 0, White: 1, Yellow: 2, Red: 3, Blue: 5, Green: 6}
	for ink, code := range want {
		if panelCode[ink] != code {
			t.Errorf("%v = %x, want %x", ink, panelCode[ink], code)
		}
	}
}

func TestWrap(t *testing.T) {
	s := causeStyle
	text := "The Gillian Remy Stepping Strong Center for Trauma Innovation"
	lines := wrap(s, text, 432, 2)
	if len(lines) != 2 || strings.Join(lines, " ") != text {
		t.Errorf("wrap = %q", lines)
	}
	lines = wrap(s, text, 200, 2)
	if len(lines) != 2 || !strings.HasSuffix(lines[1], "…") {
		t.Errorf("wrap to two short lines = %q", lines)
	}
	for _, l := range lines {
		if s.width(l) > 200 {
			t.Errorf("%q is %d wide", l, s.width(l))
		}
	}
	if got := ellipsize(s, "Supercalifragilisticexpialidocious", 120); !strings.HasSuffix(got, "…") || s.width(got) > 120 {
		t.Errorf("ellipsize one long word = %q", got)
	}
	if got := ellipsize(s, "Short", 400); got != "Short" {
		t.Errorf("ellipsize short = %q", got)
	}
}

func TestColorNames(t *testing.T) {
	cases := map[string][]schedule.ColorKey{
		"White":             nil,
		"Red":               {schedule.Red},
		"Blue & Gold":       {schedule.Blue, schedule.Gold},
		"Blue, Pink & Gold": {schedule.Blue, schedule.Pink, schedule.Gold},
		"Rainbow":           {schedule.Rainbow},
	}
	for want, colors := range cases {
		if got := colorNames(colors); got != want {
			t.Errorf("colorNames(%v) = %q, want %q", colors, got, want)
		}
	}
}

func TestLightsOnText(t *testing.T) {
	d := SampleData("2026-12-12")
	if got := lightsOnText(d.night("2026-12-12")); got != "Lights on at 5 pm" {
		t.Errorf("31 nights = %q", got)
	}
	if got := lightsOnText(d.night("2026-11-20")); !strings.HasPrefix(got, "Lights on around 4:") {
		t.Errorf("unlisted november night = %q", got)
	}
}
