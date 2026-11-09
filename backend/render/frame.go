package render

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
)

// Panel geometry: the PhotoPainter's 7.3" Spectra 6 panel is 800×480 in its
// own orientation, two pixels to a byte.
const (
	PanelWidth  = 800
	PanelHeight = 480
	// PanelBytes is the size of a packed frame.
	PanelBytes = PanelWidth * PanelHeight / 2
)

// Frame is a rendered view: Width×Height portrait pixels, each one ink.
type Frame struct {
	View View
	Pix  []Ink // row-major, Width×Height
}

// At is the ink at a portrait pixel.
func (f *Frame) At(x, y int) Ink { return f.Pix[y*Width+x] }

// Image is the frame as a paletted image in a palette's colors.
func (f *Frame) Image(p Palette) *image.Paletted {
	pal := make(color.Palette, numInks)
	for i := range pal {
		pal[i] = p.Colors[i]
	}
	img := image.NewPaletted(image.Rect(0, 0, Width, Height), pal)
	for i, ink := range f.Pix {
		img.Pix[i] = uint8(ink)
	}
	return img
}

// PNG encodes the frame in a palette's colors: Measured to preview how the
// panel will look, Ideal for the design.
func (f *Frame) PNG(p Palette) []byte {
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&b, f.Image(p)); err != nil {
		panic(err) // encoding to memory doesn't fail
	}
	return b.Bytes()
}

// Packed is the frame as the panel's framebuffer: rotated to 800×480,
// row-major, two pixels a byte with the left pixel in the high nibble, each
// nibble the ink's panel code.
//
// Portrait pixel (x, y) lands at panel pixel (y, PanelHeight-1-x): the panel
// turned a quarter clockwise to stand upright. If the frame stands the
// other way up, flip RotateClockwise.
func (f *Frame) Packed() []byte {
	out := make([]byte, PanelBytes)
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			nx, ny := panelXY(x, y)
			code := panelCode[f.Pix[y*Width+x]]
			i := ny*PanelWidth + nx
			if i%2 == 0 {
				out[i/2] |= code << 4
			} else {
				out[i/2] |= code
			}
		}
	}
	return out
}

// RotateClockwise picks which way the portrait image is turned onto the
// panel. Check against the real frame.
var RotateClockwise = true

func panelXY(x, y int) (nx, ny int) {
	if RotateClockwise {
		return y, PanelHeight - 1 - x
	}
	return PanelWidth - 1 - y, x
}

// PanelBin is Packed, gzipped: what the frame downloads.
func (f *Frame) PanelBin() []byte {
	var b bytes.Buffer
	w, _ := gzip.NewWriterLevel(&b, gzip.BestCompression)
	w.Write(f.Packed())
	w.Close()
	return b.Bytes()
}

// SHA256 identifies the frame's content: the hex hash of Packed.
func (f *Frame) SHA256() string {
	sum := sha256.Sum256(f.Packed())
	return hex.EncodeToString(sum[:])
}
