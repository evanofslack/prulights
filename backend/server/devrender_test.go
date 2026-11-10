package server

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"testing"
)

func TestDevRender(t *testing.T) {
	s := seeded(t)
	if rec := get(t, s.Handler(), "/dev/render/tonight"); rec.Code == http.StatusOK && rec.Header().Get("Content-Type") == "image/png" {
		t.Fatal("dev routes should be off without DEV")
	}
	s.cfg.Dev = true
	s = New(s.cfg, s.store)
	for _, path := range []string{
		"/dev/render/tonight?date=2026-12-12",
		"/dev/render/month?date=2026-12-12&palette=ideal",
		"/dev/render/log?sample=1&stale=1&low_battery=1",
	} {
		rec := get(t, s.Handler(), path)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
			t.Errorf("%s = %d %s", path, rec.Code, rec.Body.String())
			continue
		}
		img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
		if err != nil || img.Bounds().Dx() != 480 || img.Bounds().Dy() != 800 {
			t.Errorf("%s: %v %v", path, err, img.Bounds())
		}
	}
	for path, want := range map[string]int{"/dev/render/nope": 404, "/dev/render/week?date=soon": 400} {
		if rec := get(t, s.Handler(), path); rec.Code != want {
			t.Errorf("%s = %d, want %d", path, rec.Code, want)
		}
	}
}

// The emulator reads the device api's packed frames with /dev/inks. Doing the
// same here must give back the picture the compat png shows.
func TestDevInksDecodeFrames(t *testing.T) {
	s := seeded(t)
	s.cfg.Dev = true
	s = New(s.cfg, s.store)
	inks := decodeAs[DevInks](t, get(t, s.Handler(), "/dev/inks").Body.Bytes())
	if len(inks.Inks) != 6 || inks.PanelWidth != 800 {
		t.Fatalf("inks = %+v", inks)
	}
	byCode := map[byte]string{}
	for _, ink := range inks.Inks {
		byCode[ink.Code] = ink.Ideal
	}

	reg := register(t, s)
	item := bundleOf(t, s, reg.Token).Items[0]
	zr, err := gzip.NewReader(dev(t, s, "GET", item.URL, "", reg.Token).Body)
	if err != nil {
		t.Fatal(err)
	}
	packed, _ := io.ReadAll(zr)
	want, err := png.Decode(bytes.NewReader(dev(t, s, "GET", "/api/device/compat/current.png", "", reg.Token).Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 800; y++ {
		for x := 0; x < 480; x++ {
			nx, ny := y, inks.PanelHeight-1-x
			if !inks.RotateClockwise {
				nx, ny = inks.PanelWidth-1-y, x
			}
			i := ny*inks.PanelWidth + nx
			code := packed[i/2] >> 4
			if i%2 == 1 {
				code = packed[i/2] & 0xf
			}
			r, g, b, _ := want.At(x, y).RGBA()
			if got, exp := byCode[code], fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8); got != exp {
				t.Fatalf("(%d, %d) decodes to %s, the png has %s", x, y, got, exp)
			}
		}
	}
}
