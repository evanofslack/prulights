// Package render draws the e-ink frame's views: a 480×800 portrait image in
// the six inks of an E Ink Spectra 6 panel (Waveshare PhotoPainter 7.3"),
// packed for the panel or as a PNG preview.
package render

import (
	"image/color"

	"github.com/evanofslack/prulights/schedule"
)

// Ink is one of the panel's six pigments.
type Ink uint8

const (
	Black Ink = iota
	White
	Yellow
	Red
	Blue
	Green
	numInks
)

// Inks lists every ink.
var Inks = []Ink{Black, White, Yellow, Red, Blue, Green}

// panelCode is each ink's 4-bit value in the panel's framebuffer. From the
// Spectra 6 mapping in aitjcize/esp32-photoframe (components/epaper_src/
// GUI_ColorMap.h), which matches Waveshare's 7.3" E6 driver: 4 is unused.
var panelCode = [numInks]byte{
	Black:  0x0,
	White:  0x1,
	Yellow: 0x2,
	Red:    0x3,
	Blue:   0x5,
	Green:  0x6,
}

// PanelCode is the ink's 4-bit value in the panel's framebuffer.
func (i Ink) PanelCode() byte { return panelCode[i] }

// String is the ink's name.
func (i Ink) String() string {
	return [numInks]string{"black", "white", "yellow", "red", "blue", "green"}[i]
}

// Palette is how each ink is shown in a PNG.
type Palette struct {
	Name   string
	Colors [numInks]color.RGBA
}

// Ideal shows each ink as the pure color it stands for, which is also what
// stock firmware expects in an uploaded image.
var Ideal = Palette{Name: "ideal", Colors: [numInks]color.RGBA{
	Black:  {0, 0, 0, 255},
	White:  {255, 255, 255, 255},
	Yellow: {255, 255, 0, 255},
	Red:    {255, 0, 0, 255},
	Blue:   {0, 0, 255, 255},
	Green:  {0, 255, 0, 255},
}}

// Measured shows each ink as a real panel does: much darker and duller
// than its ideal color. These are the default calibration in
// aitjcize/esp32-photoframe (main/color_palette.c, measured on the
// PhotoPainter; see its docs/MEASURED_PALETTE.md). Replace them with our own
// panel's measurements once it arrives.
var Measured = Palette{Name: "measured", Colors: [numInks]color.RGBA{
	Black:  {2, 2, 2, 255},
	White:  {190, 200, 200, 255},
	Yellow: {205, 202, 0, 255},
	Red:    {135, 19, 0, 255},
	Blue:   {5, 64, 158, 255},
	Green:  {39, 102, 60, 255},
}}

// PaletteNamed returns Ideal or Measured by name; anything else is Measured.
func PaletteNamed(name string) Palette {
	if name == Ideal.Name {
		return Ideal
	}
	return Measured
}

var pigmentInk = map[schedule.Pigment]Ink{
	schedule.InkBlack:  Black,
	schedule.InkWhite:  White,
	schedule.InkYellow: Yellow,
	schedule.InkRed:    Red,
	schedule.InkBlue:   Blue,
	schedule.InkGreen:  Green,
}

// bayer8 is the 8×8 ordered dither matrix.
var bayer8 = [8][8]uint8{
	{0, 32, 8, 40, 2, 34, 10, 42},
	{48, 16, 56, 24, 50, 18, 58, 26},
	{12, 44, 4, 36, 14, 46, 6, 38},
	{60, 28, 52, 20, 62, 30, 54, 22},
	{3, 35, 11, 43, 1, 33, 9, 41},
	{51, 19, 59, 27, 49, 17, 57, 25},
	{15, 47, 7, 39, 13, 45, 5, 37},
	{63, 31, 55, 23, 61, 29, 53, 21},
}

// Paint says which ink a pixel gets.
type Paint interface {
	At(x, y int) Ink
}

// Solid paints one ink.
type Solid Ink

func (s Solid) At(int, int) Ink { return Ink(s) }

// Mix is an ordered dither: ink B on about Amount of the pixels, A on the
// rest. The pattern is fixed to the canvas, so neighboring fills line up.
type Mix struct {
	A, B   Ink
	Amount float64
}

func (m Mix) At(x, y int) Ink {
	if m.Amount <= 0 {
		return m.A
	}
	t := (float64(bayer8[y&7][x&7]) + 0.5) / 64
	if t < m.Amount {
		return m.B
	}
	return m.A
}

// ColorPaint is how a palette color is drawn: its ink recipe as a Mix.
// Unknown colors paint white.
func ColorPaint(k schedule.ColorKey) Paint {
	c, ok := schedule.Lookup(k)
	if !ok {
		return Solid(White)
	}
	a := pigmentInk[c.Ink.A]
	if c.Ink.B == "" || c.Ink.Mix <= 0 {
		return Solid(a)
	}
	return Mix{A: a, B: pigmentInk[c.Ink.B], Amount: c.Ink.Mix}
}

// expandBands turns a night's colors into the bands drawn for it: rainbow
// becomes its six bands.
func expandBands(colors []schedule.ColorKey) []schedule.ColorKey {
	var out []schedule.ColorKey
	for _, k := range colors {
		if c, ok := schedule.Lookup(k); ok && len(c.Bands) > 0 {
			out = append(out, c.Bands...)
			continue
		}
		out = append(out, k)
	}
	if len(out) == 0 {
		out = []schedule.ColorKey{schedule.White}
	}
	return out
}

// Veil lays Ink over Amount of another paint's pixels, as an ordered dither:
// a color seen through the dark, or in shadow.
type Veil struct {
	P      Paint
	Ink    Ink
	Amount float64
}

func (v Veil) At(x, y int) Ink {
	if (float64(bayer8[y&7][x&7])+0.5)/64 < v.Amount {
		return v.Ink
	}
	return v.P.At(x, y)
}

// noise is a fixed pseudo-random number in [0, 1) for a spot, so lit
// windows and the river's ripples land the same way every time.
func noise(x, y, seed int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263 + uint32(seed)*2246822519
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h%10007) / 10007
}
