package render

import (
	"image"
	"math"

	"golang.org/x/image/vector"
)

// Width and Height are the portrait canvas. The panel is 800×480; Packed
// rotates.
const (
	Width  = 480
	Height = 800
)

// canvas is a grid of inks being drawn on. Every pixel is always exactly one
// ink: shapes and text are rasterized to a coverage mask and thresholded,
// never blended.
type canvas struct {
	w, h int
	pix  []Ink
	clip image.Rectangle // drawing stays inside it
}

func newCanvas(w, h int, bg Ink) *canvas {
	c := &canvas{w: w, h: h, pix: make([]Ink, w*h), clip: image.Rect(0, 0, w, h)}
	for i := range c.pix {
		c.pix[i] = bg
	}
	return c
}

func (c *canvas) set(x, y int, ink Ink) {
	if image.Pt(x, y).In(c.clip) {
		c.pix[y*c.w+x] = ink
	}
}

// clipTo limits drawing to r until the returned func puts it back.
func (c *canvas) clipTo(r image.Rectangle) func() {
	old := c.clip
	c.clip = r.Intersect(old)
	return func() { c.clip = old }
}

func (c *canvas) at(x, y int) Ink { return c.pix[y*c.w+x] }

// fill paints a rectangle.
func (c *canvas) fill(r image.Rectangle, p Paint) {
	r = r.Intersect(c.clip)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c.pix[y*c.w+x] = p.At(x, y)
		}
	}
}

// outline draws a rectangle's border, t pixels thick, inside r.
func (c *canvas) outline(r image.Rectangle, t int, ink Ink) {
	c.fill(image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+t), Solid(ink))
	c.fill(image.Rect(r.Min.X, r.Max.Y-t, r.Max.X, r.Max.Y), Solid(ink))
	c.fill(image.Rect(r.Min.X, r.Min.Y, r.Min.X+t, r.Max.Y), Solid(ink))
	c.fill(image.Rect(r.Max.X-t, r.Min.Y, r.Max.X, r.Max.Y), Solid(ink))
}

// paintMask paints p wherever the mask (positioned at its own bounds) is at
// least half covered.
func (c *canvas) paintMask(m *image.Alpha, p Paint) {
	b := m.Bounds().Intersect(c.clip)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if m.AlphaAt(x, y).A >= 128 {
				c.pix[y*c.w+x] = p.At(x, y)
			}
		}
	}
}

type pt struct{ x, y float64 }

// polygon fills a closed polygon.
func (c *canvas) polygon(pts []pt, p Paint) {
	if len(pts) < 3 {
		return
	}
	minX, minY, maxX, maxY := pts[0].x, pts[0].y, pts[0].x, pts[0].y
	for _, q := range pts {
		minX, minY = math.Min(minX, q.x), math.Min(minY, q.y)
		maxX, maxY = math.Max(maxX, q.x), math.Max(maxY, q.y)
	}
	r := image.Rect(int(math.Floor(minX)), int(math.Floor(minY)), int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1)
	z := vector.NewRasterizer(r.Dx(), r.Dy())
	z.MoveTo(float32(pts[0].x-float64(r.Min.X)), float32(pts[0].y-float64(r.Min.Y)))
	for _, q := range pts[1:] {
		z.LineTo(float32(q.x-float64(r.Min.X)), float32(q.y-float64(r.Min.Y)))
	}
	z.ClosePath()
	m := image.NewAlpha(r)
	z.Draw(m, r, image.Opaque, image.Point{})
	c.paintMask(m, p)
}

// line draws a straight stroke w pixels wide.
func (c *canvas) line(a, b pt, w float64, p Paint) {
	dx, dy := b.x-a.x, b.y-a.y
	l := math.Hypot(dx, dy)
	if l == 0 {
		return
	}
	nx, ny := -dy/l*w/2, dx/l*w/2
	c.polygon([]pt{{a.x + nx, a.y + ny}, {b.x + nx, b.y + ny}, {b.x - nx, b.y - ny}, {a.x - nx, a.y - ny}}, p)
}

// polyline draws connected strokes.
func (c *canvas) polyline(pts []pt, w float64, p Paint) {
	for i := 1; i < len(pts); i++ {
		c.line(pts[i-1], pts[i], w, p)
		c.disc(pts[i], w/2, p)
	}
}

// disc fills a circle.
func (c *canvas) disc(o pt, r float64, p Paint) { c.ellipse(o, r, r, p) }

// ellipse fills an ellipse with radii rx, ry.
func (c *canvas) ellipse(o pt, rx, ry float64, p Paint) {
	n := 48
	pts := make([]pt, n)
	for i := range pts {
		a := 2 * math.Pi * float64(i) / float64(n)
		pts[i] = pt{o.x + rx*math.Cos(a), o.y + ry*math.Sin(a)}
	}
	c.polygon(pts, p)
}

// star fills (or, hollow, outlines) a five-pointed star of outer radius r.
func (c *canvas) star(o pt, r float64, ink Ink, hollow bool) {
	pts := starPoints(o, r)
	if !hollow {
		c.polygon(pts, Solid(ink))
		return
	}
	c.polyline(append(pts, pts[0]), 2, Solid(ink))
}

func starPoints(o pt, r float64) []pt {
	pts := make([]pt, 10)
	for i := range pts {
		rr := r
		if i%2 == 1 {
			rr = r * 0.45
		}
		a := -math.Pi/2 + math.Pi*float64(i)/5
		pts[i] = pt{o.x + rr*math.Cos(a), o.y + rr*math.Sin(a)}
	}
	return pts
}

// check draws a check mark fitting in a size×size box at its top left.
func (c *canvas) check(at image.Point, size float64, ink Ink) {
	x, y := float64(at.X), float64(at.Y)
	c.polyline([]pt{{x + size*0.1, y + size*0.55}, {x + size*0.4, y + size*0.85}, {x + size*0.92, y + size*0.18}}, size*0.16, Solid(ink))
}

// pline draws a one-pixel line, pixel by pixel (Bresenham), for the art's
// thin lattice and ribs.
func (c *canvas) pline(x0, y0, x1, y1 int, p Paint) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	e := dx + dy
	for {
		c.set(x0, y0, p.At(x0, y0))
		if x0 == x1 && y0 == y1 {
			return
		}
		if e2 := 2 * e; e2 >= dy {
			e += dy
			x0 += sx
		} else {
			e += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// fade covers r with ink, from none of the pixels at its top to all of them
// at its bottom (or the other way when up), as an ordered dither.
func (c *canvas) fade(r image.Rectangle, ink Ink, up bool) {
	r = r.Intersect(c.clip)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		t := (float64(y-r.Min.Y) + 0.5) / float64(r.Dy())
		if up {
			t = 1 - t
		}
		for x := r.Min.X; x < r.Max.X; x++ {
			if (float64(bayer8[y&7][x&7])+0.5)/64 < t {
				c.pix[y*c.w+x] = ink
			}
		}
	}
}
