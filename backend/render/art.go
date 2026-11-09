package render

import (
	"image"
	"math"

	"github.com/evanofslack/prulights/schedule"
)

// The tower art: the Prudential Tower at night, its crown in the night's
// colors, over a low Back Bay skyline. It's drawn in code, scaled to a box,
// so a hand-drawn version can replace this file without touching the views.

// Shades of the night, as dithered black and white.
var (
	towerBody  = Mix{A: Black, B: White, Amount: 0.30}
	towerEdge  = Mix{A: Black, B: White, Amount: 0.55}
	skylineFar = Mix{A: Black, B: White, Amount: 0.12}
	skylineNar = Mix{A: Black, B: White, Amount: 0.20}
)

// drawTower draws the scene into r (black sky included). The tower's
// proportions are fixed; it's scaled to r's height and centered.
func drawTower(c *canvas, r image.Rectangle, colors []schedule.ColorKey, stars bool) {
	c.fill(r, Solid(Black))
	h := float64(r.Dy())
	s := h / 440 // the art is designed 440 tall
	cx := float64(r.Min.X+r.Max.X) / 2
	top := float64(r.Min.Y)
	X := func(v float64) int { return int(math.Round(cx + v*s)) }
	Y := func(v float64) int { return int(math.Round(top + v*s)) }
	rect := func(x0, y0, x1, y1 float64) image.Rectangle { return image.Rect(X(x0), Y(y0), X(x1), Y(y1)) }

	if stars {
		// a few fixed stars, away from the tower
		for _, p := range [][2]float64{{-200, 40}, {-150, 110}, {-185, 190}, {-120, 30}, {150, 60}, {205, 130}, {130, 150}, {180, 25}, {-95, 75}, {110, 95}} {
			x, y := X(p[0]), Y(p[1])
			if x > r.Min.X+2 && x < r.Max.X-2 {
				c.fill(image.Rect(x, y, x+2, y+2), Solid(White))
			}
		}
	}

	// the glow around the crown: the night's first color, faint, on the sky
	bands := expandBands(colors)
	glowInk := firstInk(bands[0])
	for i, amt := range []float64{0.05, 0.1, 0.17, 0.26} {
		grow := float64(3-i) * 16
		c.ellipse(pt{cx, top + 80*s}, (78+grow)*s, (44+grow*0.75)*s, Mix{A: Black, B: glowInk, Amount: amt})
	}

	// antenna mast and its crossbars
	c.fill(rect(-2, 2, 2, 52), Solid(White))
	c.fill(rect(-9, 20, 9, 22), Solid(White))
	c.fill(rect(-6, 34, 6, 36), Solid(White))
	// the roof cap above the crown
	c.fill(rect(-36, 52, 36, 58), towerEdge)

	// the crown: the night's colors as vertical bands
	crown := rect(-44, 58, 44, 102)
	drawBands(c, crown, colors)

	// the setback and the shaft
	c.fill(rect(-48, 102, 48, 112), towerEdge)
	shaft := rect(-58, 112, 58, 440)
	c.fill(shaft, towerBody)
	// lit edges
	c.fill(rect(-58, 112, -55, 440), towerEdge)
	c.fill(rect(55, 112, 58, 440), towerEdge)
	// floors and mullions
	for y := 120.0; y < 430; y += 9 {
		c.fill(rect(-55, y, 55, y+2), Solid(Black))
	}
	for x := -46.0; x <= 46; x += 13 {
		c.fill(rect(x, 112, x+2, 440), Solid(Black))
	}
	// a few lit windows
	for _, w := range [][2]float64{{-40, 160}, {-1, 205}, {25, 268}, {-27, 322}, {12, 367}, {38, 401}, {-14, 133}} {
		c.fill(rect(w[0], w[1], w[0]+9, w[1]+5), Solid(Yellow))
	}

	// the skyline: far buildings, then nearer ones in front of the tower's base
	for _, b := range [][4]float64{{-240, 330, -170, 440}, {-175, 300, -125, 440}, {-125, 350, -70, 440}, {70, 320, 120, 440}, {118, 280, 160, 440}, {160, 340, 240, 440}} {
		c.fill(rect(b[0], b[1], b[2], b[3]), skylineFar)
	}
	// the Hancock tower's slim slab, to the right
	c.polygon([]pt{{float64(X(150)), float64(Y(205))}, {float64(X(186)), float64(Y(215))}, {float64(X(186)), float64(Y(440))}, {float64(X(150)), float64(Y(440))}}, skylineNar)
	for _, b := range [][4]float64{{-240, 390, -150, 440}, {-150, 375, -80, 440}, {-80, 400, 90, 440}, {90, 385, 150, 440}, {150, 400, 240, 440}} {
		c.fill(rect(b[0], b[1], b[2], b[3]), Solid(Black))
	}
	// a few lit windows in the near buildings
	for _, w := range [][2]float64{{-220, 400}, {-190, 415}, {-130, 390}, {-110, 410}, {100, 400}, {125, 418}, {175, 412}, {205, 420}} {
		c.fill(rect(w[0], w[1], w[0]+6, w[1]+4), Solid(Yellow))
	}
}

// drawBands fills r with the colors as equal vertical bands (rainbow as its
// six). One color fills it.
func drawBands(c *canvas, r image.Rectangle, colors []schedule.ColorKey) {
	bands := expandBands(colors)
	n := len(bands)
	for i, k := range bands {
		x0 := r.Min.X + r.Dx()*i/n
		x1 := r.Min.X + r.Dx()*(i+1)/n
		c.fill(image.Rect(x0, r.Min.Y, x1, r.Max.Y), ColorPaint(k))
	}
}

// swatch is a night's colors as bands with a black border, for the paper
// background.
func swatch(c *canvas, r image.Rectangle, colors []schedule.ColorKey) {
	drawBands(c, r.Inset(2), colors)
	c.outline(r, 2, Black)
}

// firstInk is the main ink of a color, for its glow.
func firstInk(k schedule.ColorKey) Ink {
	c, ok := schedule.Lookup(k)
	if !ok {
		return White
	}
	ink := pigmentInk[c.Ink.A]
	if ink == Black {
		return White
	}
	return ink
}
