package render

import (
	"image"

	qrcode "github.com/skip2/go-qrcode"
)

// qr draws a QR code for text with its top left at at, sized to fit in
// size×size with whole-pixel modules, and returns the box it used. The code
// includes a white quiet zone of two modules.
func qr(c *canvas, at image.Point, size int, text string) image.Rectangle {
	q, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		return image.Rectangle{}
	}
	q.DisableBorder = true
	bits := q.Bitmap()
	n := len(bits) + 4 // two modules of quiet zone each side
	mod := size / n
	if mod < 1 {
		mod = 1
	}
	box := image.Rect(at.X, at.Y, at.X+n*mod, at.Y+n*mod)
	c.fill(box, Solid(White))
	for y, row := range bits {
		for x, on := range row {
			if on {
				px, py := at.X+(x+2)*mod, at.Y+(y+2)*mod
				c.fill(image.Rect(px, py, px+mod, py+mod), Solid(Black))
			}
		}
	}
	return box
}
