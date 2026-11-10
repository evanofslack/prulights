package render

import (
	"image"
	"math"

	"github.com/evanofslack/prulights/schedule"
)

// The art: the Prudential Tower as pixel art, drawn in code so the night's
// colors can go where the real lights are. From the top: the lattice spire
// (lit in the night's first color, a red beacon on top), the rooftop
// machinery, then the crown: a thin ring of light at the roofline, a band of
// windows (where the PRUDENTIAL letters are, too small for the panel), a
// thicker ring under it, and the lit setback. Below that the shaft, its
// windows in a fine grid broken by dark mechanical floors.
//
// Around it, by choice: nothing, the Back Bay skyline (the Hancock's glass
// slab far left, 111 Huntington's dome beside the Pru, a few slabs to the
// right, brownstone roofs in front), or that skyline across the Charles with
// the crown's colors in the water.
//
// Everything is in panel pixels at a fixed size: the tower is drawn 1:1,
// so windows and lattice land on whole pixels. Views move it or clip it, but
// never scale it.

// Scene is what's drawn around the tower.
type Scene string

const (
	SceneTower   Scene = "tower"   // the tower alone
	SceneSkyline Scene = "skyline" // with Back Bay around it
	SceneRiver   Scene = "river"   // the skyline across the river, the colors in the water
)

// Words is how much the tonight view says under the picture.
type Words string

const (
	WordsNone  Words = "none"  // the picture only
	WordsColor Words = "color" // the date and the colors
	WordsCause Words = "cause" // and what they're for
)

// Look is how a frame's owner likes it drawn. The zero Look is the default:
// night, the skyline, colors and cause.
type Look struct {
	Light bool  // paper and ink instead of the night sky
	Scene Scene // "" is the skyline
	Words Words // "" is colors and cause
}

// Scenes and WordsList are the choices, for validating settings.
var (
	Scenes    = []Scene{SceneTower, SceneSkyline, SceneRiver}
	WordsList = []Words{WordsNone, WordsColor, WordsCause}
)

func (l Look) scene() Scene {
	if l.Scene == "" {
		return SceneSkyline
	}
	return l.Scene
}

func (l Look) words() Words {
	if l.Words == "" {
		return WordsCause
	}
	return l.Words
}

// theme is the inks a look draws with.
type theme struct {
	light bool
	bg    Ink   // the page
	fg    Ink   // text and lines
	rule  Paint // dotted dividers
	body  Paint // the small crowns' structure in lists
}

var (
	darkTheme  = theme{bg: Black, fg: White, rule: Mix{A: Black, B: White, Amount: 0.5}, body: Mix{A: Black, B: White, Amount: 0.25}}
	lightTheme = theme{light: true, bg: White, fg: Black, rule: Mix{A: White, B: Black, Amount: 0.5}, body: Solid(Black)}
)

func themeOf(l Look) theme {
	if l.Light {
		return lightTheme
	}
	return darkTheme
}

// Tower geometry, in pixels from the spire's tip (x from the center line).
const (
	towerHalf  = 84  // the shaft is 168 wide
	crownHalf  = 76  // the crown, a little narrower
	ringTop    = 112 // the roofline ring
	bandTop    = 120 // the window band (the letters' band)
	ringLow    = 138 // the thick ring
	setbackTop = 150 // the lit setback
	shaftTop   = 162
	floorPitch = 9
)

// scene is where a picture's parts go.
type scene struct {
	th      theme
	kind    Scene
	colors  []schedule.ColorKey
	cx, top int // the spire's tip
	horizon int // the foot of the buildings, or the river's far bank
	bottom  int // the picture's lower edge
}

// newScene lays out a full-width picture from the top of the canvas down to
// bottom.
func newScene(l Look, th theme, colors []schedule.ColorKey, bottom int) scene {
	s := scene{th: th, kind: l.scene(), colors: colors, cx: Width / 2, top: 30, horizon: bottom, bottom: bottom}
	if s.kind == SceneRiver {
		s.horizon = bottom - max(90, (bottom-300)/4)
	}
	return s
}

// drawScene draws the whole picture: sky, neighbors, the tower, the river.
func drawScene(c *canvas, s scene) {
	defer c.clipTo(image.Rect(0, 0, Width, s.bottom))()
	c.fill(image.Rect(0, 0, Width, s.bottom), Solid(s.th.bg))
	if !s.th.light {
		nightSky(c, s)
	}
	if s.kind != SceneTower {
		farSkyline(c, s)
	}
	drawTower(c, s.cx, s.top, s.horizon, s.colors, s.th)
	switch s.kind {
	case SceneTower:
		// the shaft fades into the night (or the paper) instead of ending
		c.fade(image.Rect(0, s.bottom-90, Width, s.bottom), s.th.bg, false)
	default:
		nearSkyline(c, s)
	}
	if s.kind == SceneRiver {
		river(c, s)
	}
}

// nightSky: black overhead, a little city glow toward the horizon, a few
// stars, and the crown's light on the sky around it.
func nightSky(c *canvas, s scene) {
	if s.kind != SceneTower { // the city's glow on the sky, behind the skyline
		from := s.horizon * 45 / 100
		for y := from; y < s.horizon; y++ {
			t := float64(y-from) / float64(s.horizon-from)
			c.fill(image.Rect(0, y, Width, y+1), Mix{A: Black, B: Blue, Amount: 0.02 + 0.2*t*t})
		}
	}
	for _, p := range [][2]int{{38, 52}, {92, 128}, {61, 214}, {140, 40}, {22, 300}, {118, 262},
		{350, 70}, {412, 34}, {438, 150}, {376, 196}, {452, 248}, {330, 118}, {406, 312}, {70, 380}} {
		x, y := p[0], p[1]
		if y > s.horizon-120 {
			continue
		}
		c.set(x, y, White)
		if (x+y)%3 == 0 {
			c.set(x+1, y, White)
			c.set(x, y+1, White)
			c.set(x+1, y+1, White)
		}
	}
	ink, amount := firstInk(expandBands(s.colors)[0]), 0.3
	if ink == White || ink == Yellow { // bright inks speckle: keep them faint
		amount = 0.14
	}
	glow(c, s.cx, s.top+ringLow, 170, 90, ink, amount)
}

// glow lights the sky around (cx, cy) with ink, strongest (amount) at the
// middle and fading to nothing at the ellipse rx by ry.
func glow(c *canvas, cx, cy, rx, ry int, ink Ink, amount float64) {
	r := image.Rect(cx-rx, cy-ry, cx+rx, cy+ry).Intersect(c.clip)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dx, dy := float64(x-cx)/float64(rx), float64(y-cy)/float64(ry)
			d := dx*dx + dy*dy
			if d >= 1 || c.pix[y*c.w+x] != Black {
				continue
			}
			f := 1 - math.Sqrt(d)
			if (float64(bayer8[y&7][x&7])+0.5)/64 < amount*f*f {
				c.pix[y*c.w+x] = ink
			}
		}
	}
}

// drawTower draws the tower with its spire's tip at (cx, top), its shaft
// running down to bottom.
func drawTower(c *canvas, cx, top, bottom int, colors []schedule.ColorKey, th theme) {
	bands := expandBands(colors)
	R := func(x0, y0, x1, y1 int) image.Rectangle {
		return image.Rect(cx+x0, top+y0, cx+x1, top+y1)
	}
	lit := Paint(Solid(firstInk(bands[0])))
	edge := Paint(Mix{A: Black, B: White, Amount: 0.4})
	if th.light {
		lit, edge = Solid(Black), Solid(Black)
	}

	// the spire: a whip on top, then a lattice mast widening to its legs
	c.fill(R(-1, 2, 1, 30), lit)
	c.fill(R(-1, -1, 2, 2), Solid(Red)) // the beacon
	for y := 30; y < 98; y++ {
		hw := 3 + (y-30)*5/68
		c.set(cx-hw, top+y, lit.At(cx-hw, top+y))
		c.set(cx+hw-1, top+y, lit.At(cx+hw-1, top+y))
	}
	for y := 30; y+8 <= 98; y += 8 { // cross bracing
		hw0, hw1 := 3+(y-30)*5/68, 3+(y+8-30)*5/68
		c.pline(cx-hw0, top+y, cx+hw1-1, top+y+8, lit)
		c.pline(cx+hw0-1, top+y, cx-hw1, top+y+8, lit)
	}
	c.fill(R(-8, 58, 8, 74), lit) // the antenna panels
	if !th.light {
		c.fill(R(-6, 60, 6, 72), Veil{P: lit, Ink: Black, Amount: 0.5})
	}
	c.set(cx, top+64, Red)
	c.pline(cx-8, top+98, cx-16, top+106, lit) // legs
	c.pline(cx+7, top+98, cx+15, top+106, lit)

	// the rooftop: machinery, antennas, beacons
	c.fill(R(-58, 104, 58, ringTop), Solid(Black))
	c.fill(R(-58, 104, 58, 105), edge)
	for _, a := range [][2]int{{-48, 84}, {-36, 92}, {-20, 96}, {30, 90}, {46, 80}, {54, 94}} {
		c.fill(R(a[0], a[1], a[0]+1, 104), edge)
	}
	c.fill(R(-60, 102, -58, 104), Solid(Red))
	c.fill(R(58, 102, 60, 104), Solid(Red))

	// the crown
	if th.light { // a black edge, so a white ring still reads on paper
		c.fill(R(-crownHalf-3, ringTop-1, crownHalf+3, setbackTop+1), Solid(Black))
	}
	ring(c, R(-crownHalf, ringTop, crownHalf, bandTop), bands, nil)
	c.fill(R(-crownHalf+2, bandTop, crownHalf-2, ringLow), Solid(Black))
	for x := -crownHalf + 4; x < crownHalf-4; x += 4 {
		r := R(x, bandTop+4, x+2, ringLow-4)
		switch {
		case th.light:
			c.fill(r, Solid(White))
		case noise(x, 1, 7) < 0.4:
			c.fill(r, Mix{A: White, B: Yellow, Amount: 0.3})
		default:
			c.fill(r, Mix{A: Black, B: Blue, Amount: 0.35})
		}
	}
	ring(c, R(-crownHalf-2, ringLow, crownHalf+2, setbackTop), bands, nil)
	c.fill(R(-68, setbackTop, 68, shaftTop), Solid(Black))
	ring(c, R(-66, setbackTop, 66, shaftTop), bands, &Veil{Ink: Black, Amount: 0.45})

	// the shaft and its windows
	shaft := image.Rect(cx-towerHalf, top+shaftTop, cx+towerHalf, bottom)
	c.fill(shaft, Solid(Black))
	c.fill(image.Rect(shaft.Min.X, shaft.Min.Y, shaft.Max.X, shaft.Min.Y+1), edge)
	if !th.light {
		c.fill(image.Rect(shaft.Min.X, shaft.Min.Y, shaft.Min.X+1, shaft.Max.Y), edge)
		c.fill(image.Rect(shaft.Max.X-1, shaft.Min.Y, shaft.Max.X, shaft.Max.Y), edge)
	}
	for f, y := 0, top+shaftTop+5; y < bottom; f, y = f+1, y+floorPitch {
		if f%14 == 13 { // a mechanical floor: no windows
			continue
		}
		for x := -towerHalf + 4; x+2 <= towerHalf-3; x += 4 {
			r := image.Rect(cx+x, y, cx+x+2, y+5)
			switch n := noise(x, f, 3); {
			case th.light:
				c.fill(r, Solid(White))
			case n < 0.07:
				c.fill(r, Solid(White))
			case n < 0.1:
				c.fill(r, Mix{A: White, B: Yellow, Amount: 0.5})
			default:
				c.fill(r, Mix{A: Black, B: Blue, Amount: 0.2})
			}
		}
	}
	// the crown's light spills down the top floors
	if !th.light {
		c.fade(image.Rect(cx-towerHalf+1, top+shaftTop+1, cx+towerHalf-1, top+shaftTop+16), Black, false)
		c.fill(image.Rect(cx-towerHalf+1, top+shaftTop+1, cx+towerHalf-1, top+shaftTop+3), Veil{P: ColorPaint(bands[0]), Ink: Black, Amount: 0.6})
	}
}

// ring fills r with the night's colors, one segment each from left to
// right. veil, when set, darkens them (Veil.P is filled in per color).
func ring(c *canvas, r image.Rectangle, bands []schedule.ColorKey, veil *Veil) {
	n := len(bands)
	for i, k := range bands {
		x0 := r.Min.X + r.Dx()*i/n
		x1 := r.Min.X + r.Dx()*(i+1)/n
		var p Paint = ColorPaint(k)
		if veil != nil {
			v := *veil
			v.P = p
			p = v
		}
		c.fill(image.Rect(x0, r.Min.Y, x1, r.Max.Y), p)
	}
}

// building is a neighbor: x from the tower's center line, its height as a
// share of the height from the horizon to the Pru's crown.
type building struct {
	x0, x1 int
	h      float64
}

// farSkyline: what stands behind the Pru: the Hancock's glass slab and a
// darker tower to the right.
func farSkyline(c *canvas, s scene) {
	H := float64(s.horizon - (s.top + ringTop))
	top := func(h float64) int { return s.horizon - int(h*H) }
	glass := Paint(Mix{A: Black, B: Blue, Amount: 0.16})
	dark := Paint(Solid(Black))
	if s.th.light {
		glass, dark = Mix{A: White, B: Blue, Amount: 0.22}, Mix{A: White, B: Black, Amount: 0.18}
	}
	// the Hancock: a slab of glass with its notch
	hx0, hx1 := s.cx-232, s.cx-156
	c.fill(image.Rect(hx0, top(0.74), hx1, s.horizon), glass)
	c.fill(image.Rect(hx0+22, top(0.74), hx0+25, s.horizon), Solid(s.th.bg))
	// a dark tower behind the right side of the Pru
	c.fill(image.Rect(s.cx+70, top(0.6), s.cx+142, s.horizon), dark)
	windows(c, image.Rect(s.cx+86, top(0.6)+6, s.cx+142, s.horizon), 0.05, 11, s.th)
}

// nearSkyline: 111 Huntington's dome to the left, slabs to the right, and
// brownstone roofs across the front hiding the tower's base.
func nearSkyline(c *canvas, s scene) {
	H := float64(s.horizon - (s.top + ringTop))
	top := func(h float64) int { return s.horizon - int(h*H) }
	body := Paint(Solid(Black))
	trim := Paint(Mix{A: Black, B: White, Amount: 0.7})
	if s.th.light {
		body, trim = Mix{A: White, B: Black, Amount: 0.3}, Solid(White)
	}
	for i, b := range []building{{-246, -176, 0.3}, {120, 182, 0.38}, {176, 246, 0.27}, {86, 132, 0.2}} {
		r := image.Rect(s.cx+b.x0, top(b.h), s.cx+b.x1, s.horizon)
		c.fill(r, body)
		windows(c, r.Inset(4), 0.09, 20+i, s.th)
	}

	// 111 Huntington: a round-shouldered tower capped with a ribbed dome
	bx0, bx1 := s.cx-152, s.cx-98
	bt := top(0.46)
	mid := float64(bx0+bx1) / 2
	rx, ry := float64(bx1-bx0)/2-4, 24.0
	c.fill(image.Rect(bx0, bt, bx1, s.horizon), body)
	windows(c, image.Rect(bx0+4, bt+8, bx1-4, s.horizon), 0.14, 31, s.th)
	c.fill(image.Rect(bx0+2, bt-4, bx1-2, bt), body)
	var dome []pt
	for i := 0; i <= 24; i++ {
		a := math.Pi + math.Pi*float64(i)/24
		dome = append(dome, pt{mid + rx*math.Cos(a), float64(bt-4) + ry*math.Sin(a)})
	}
	c.polygon(dome, body)
	for _, f := range []float64{-0.8, -0.45, 0, 0.45, 0.8} { // ribs
		x := int(mid + f*rx)
		y := int(float64(bt-4) - ry*math.Sqrt(1-f*f))
		c.pline(x, y+2, int(mid+f*rx*0.9), bt-4, trim)
	}
	c.pline(int(mid-rx), bt-5, int(mid+rx), bt-5, trim)
	c.fill(image.Rect(int(mid), bt-4-int(ry)-12, int(mid)+1, bt-4-int(ry)), trim)

	// brownstones: a ragged run of roofs and chimneys, with a few lamps lit
	roof := Paint(Solid(Black))
	if s.th.light {
		roof = Mix{A: White, B: Black, Amount: 0.5}
	}
	for x, i := 0, 0; x < Width; i++ {
		w := 24 + int(noise(i, 0, 41)*30)
		h := 20 + int(noise(i, 1, 41)*34)
		r := image.Rect(x, s.horizon-h, x+w, s.horizon)
		c.fill(r, roof)
		if noise(i, 2, 41) < 0.5 {
			cx := x + 4 + int(noise(i, 3, 41)*float64(w-10))
			c.fill(image.Rect(cx, r.Min.Y-7, cx+4, r.Min.Y), roof)
		}
		windows(c, image.Rect(x+3, r.Min.Y+6, x+w-3, s.horizon-2), 0.16, 50+i, s.th)
		x += w
	}
}

// windows lights a few windows in r (the dark only).
func windows(c *canvas, r image.Rectangle, chance float64, seed int, th theme) {
	if th.light {
		return
	}
	warm := Mix{A: Yellow, B: White, Amount: 0.4}
	for y := r.Min.Y; y+3 <= r.Max.Y; y += 7 {
		for x := r.Min.X; x+2 <= r.Max.X; x += 5 {
			if noise(x, y, seed) < chance {
				c.fill(image.Rect(x, y, x+2, y+3), warm)
			}
		}
	}
}

// river: the far bank's lamps, then the water with the crown's colors
// broken into ripples under the tower and the city's lights scattered.
func river(c *canvas, s scene) {
	c.fill(image.Rect(0, s.horizon, Width, s.bottom), Solid(s.th.bg))
	lamp := Ink(Yellow)
	for x := 6; x < Width; x += 14 {
		c.fill(image.Rect(x, s.horizon+1, x+2, s.horizon+3), Solid(lamp))
	}
	bands := expandBands(s.colors)
	n := len(bands)
	depth := float64(s.bottom - s.horizon)
	for y := s.horizon + 6; y < s.bottom-2; y += 3 {
		t := float64(y-s.horizon) / depth
		half := 70 + int(30*t)
		keep := 0.85 - 0.45*t
		for x := s.cx - half; x < s.cx+half; {
			l := 3 + int(noise(x, y, 61)*12)
			if noise(x, y, 62) < keep {
				i := min(n-1, max(0, (x-(s.cx-half))*n/(2*half)))
				p := ColorPaint(bands[i])
				for xx := x; xx < min(x+l, s.cx+half); xx++ {
					c.set(xx, y, p.At(xx, y))
				}
			}
			x += l + 2 + int(noise(x, y, 63)*5)
		}
		// the city's lights, and on paper a few strokes of water
		for x := 0; x < Width; x += 9 {
			if x > s.cx-half-6 && x < s.cx+half+6 {
				continue
			}
			v := noise(x, y, 64)
			switch {
			case v < 0.05:
				c.fill(image.Rect(x, y, x+4, y+1), Solid(Yellow))
			case v < 0.08 && !s.th.light:
				c.fill(image.Rect(x, y, x+3, y+1), Solid(White))
			case v < 0.09 && s.th.light:
				c.fill(image.Rect(x, y, x+6, y+1), Solid(Blue))
			}
		}
	}
}

// crownIcon is a night's colors as a small crown and spire, for lists and
// calendars: the rings carry the colors.
func crownIcon(c *canvas, r image.Rectangle, colors []schedule.ColorKey, th theme) {
	bands := expandBands(colors)
	W, H := r.Dx(), r.Dy()
	cx := r.Min.X + W/2
	y := func(f float64) int { return r.Min.Y + int(math.Round(f*float64(H))) }
	half := func(f float64) (int, int) { w := int(math.Round(f * float64(W) / 2)); return cx - w, cx + w }

	spire := Paint(Solid(firstInk(bands[0])))
	if th.light {
		spire = Solid(Black)
	}
	c.fill(image.Rect(cx-1, y(0), cx+1, y(0.3)), spire)
	x0, x1 := half(0.62)
	c.fill(image.Rect(x0, y(0.27), x1, y(0.34)), th.body)
	x0, x1 = half(0.9)
	if th.light {
		c.fill(image.Rect(x0-1, y(0.34)-1, x1+1, y(0.78)+1), Solid(Black))
	}
	ring(c, image.Rect(x0, y(0.34), x1, y(0.45)), bands, nil)
	c.fill(image.Rect(x0+1, y(0.45), x1-1, y(0.57)), th.body)
	ring(c, image.Rect(x0-1, y(0.57), x1+1, y(0.72)), bands, nil)
	sx0, sx1 := half(0.78)
	ring(c, image.Rect(sx0, y(0.72), sx1, y(0.78)), bands, &Veil{Ink: Black, Amount: 0.45})
	x0, x1 = half(1)
	c.fill(image.Rect(x0, y(0.78), x1, y(1)), th.body)
}

// colorBar is a night's colors as one flat ring, for compact lists.
func colorBar(c *canvas, r image.Rectangle, colors []schedule.ColorKey, th theme) {
	if th.light {
		c.fill(r.Inset(-1), Solid(Black))
	}
	ring(c, r, expandBands(colors), nil)
}

// firstInk is the main ink of a color, for its glow and thin lines.
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
