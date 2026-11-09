package render

import (
	_ "embed"
	"image"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Atkinson Hyperlegible, made by the Braille Institute for low-vision
// readers. SIL Open Font License: fonts/OFL.txt.
var (
	//go:embed fonts/AtkinsonHyperlegible-Regular.ttf
	regularTTF []byte
	//go:embed fonts/AtkinsonHyperlegible-Bold.ttf
	boldTTF []byte
)

type weight int

const (
	regular weight = iota
	bold
)

var (
	fontsOnce sync.Once
	fonts     [2]*opentype.Font
	facesMu   sync.Mutex
	faces     = map[faceKey]font.Face{}
)

type faceKey struct {
	w    weight
	size float64
}

// face returns a font face at a pixel size.
func face(w weight, size float64) font.Face {
	fontsOnce.Do(func() {
		var err error
		if fonts[regular], err = opentype.Parse(regularTTF); err != nil {
			panic(err)
		}
		if fonts[bold], err = opentype.Parse(boldTTF); err != nil {
			panic(err)
		}
	})
	facesMu.Lock()
	defer facesMu.Unlock()
	k := faceKey{w, size}
	if f, ok := faces[k]; ok {
		return f
	}
	f, err := opentype.NewFace(fonts[w], &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	faces[k] = f
	return f
}

// style is a font, size and ink, plus extra spacing between letters (for
// small capitals labels).
type style struct {
	w       weight
	size    float64
	ink     Ink
	spacing float64
}

func (s style) face() font.Face { return face(s.w, s.size) }

// width is how wide text is set in a style.
func (s style) width(text string) int {
	f := s.face()
	w := font.MeasureString(f, text)
	if s.spacing > 0 && len(text) > 1 {
		w += fixed.Int26_6(s.spacing*64) * fixed.Int26_6(len([]rune(text))-1)
	}
	return w.Ceil()
}

// ascent is how far the style's capitals rise above the baseline.
func (s style) ascent() int { return s.face().Metrics().CapHeight.Ceil() }

// lineHeight is the distance between baselines.
func (s style) lineHeight() int { return int(s.size * 1.22) }

// text draws one line with its baseline at y, starting at x. It returns the
// x where the text ends.
func (c *canvas) text(x, y int, s style, text string) int {
	f := s.face()
	if s.spacing == 0 {
		return c.drawRun(x, y, f, text, s.ink)
	}
	for _, r := range text {
		x = c.drawRun(x, y, f, string(r), s.ink) + int(s.spacing)
	}
	return x
}

// drawRun draws text into a coverage mask and thresholds it onto the canvas.
func (c *canvas) drawRun(x, y int, f font.Face, text string, ink Ink) int {
	adv := font.MeasureString(f, text)
	m := f.Metrics()
	r := image.Rect(x-2, y-m.Ascent.Ceil()-2, x+adv.Ceil()+4, y+m.Descent.Ceil()+2)
	mask := image.NewAlpha(r)
	d := font.Drawer{Dst: mask, Src: image.Opaque, Face: f, Dot: fixed.P(x, y)}
	d.DrawString(text)
	c.paintMask(mask, Solid(ink))
	return x + adv.Round()
}

// textRight draws text ending at x.
func (c *canvas) textRight(x, y int, s style, text string) {
	c.text(x-s.width(text), y, s, text)
}

// textCenter draws text centered on x.
func (c *canvas) textCenter(x, y int, s style, text string) {
	c.text(x-s.width(text)/2, y, s, text)
}

// wrap breaks text into at most maxLines lines no wider than width, ending
// the last with an ellipsis when it doesn't all fit.
func wrap(s style, text string, width, maxLines int) []string {
	words := strings.Fields(text)
	var lines []string
	cur := ""
	for i, w := range words {
		next := w
		if cur != "" {
			next = cur + " " + w
		}
		if s.width(next) <= width {
			cur = next
			continue
		}
		if cur == "" {
			// one word longer than the line: cut it
			cur = fit(s, w, width)
			continue
		}
		lines = append(lines, cur)
		if len(lines) == maxLines {
			lines[maxLines-1] = ellipsize(s, cur+" "+strings.Join(words[i:], " "), width)
			return lines
		}
		cur = w
		if s.width(cur) > width {
			cur = fit(s, cur, width)
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// ellipsize shortens text to fit width, ending with "…" when it was cut.
func ellipsize(s style, text string, width int) string {
	if s.width(text) <= width {
		return text
	}
	words := strings.Fields(text)
	for n := len(words) - 1; n > 0; n-- {
		t := strings.TrimRight(strings.Join(words[:n], " "), ",;:–—-·") + "…"
		if s.width(t) <= width {
			return t
		}
	}
	return fit(s, text, width)
}

// fit cuts text letter by letter to fit width, with an ellipsis.
func fit(s style, text string, width int) string {
	r := []rune(text)
	for n := len(r); n > 0; n-- {
		t := string(r[:n])
		if n < len(r) {
			t += "…"
		}
		if s.width(t) <= width {
			return t
		}
	}
	return ""
}
