package server

import (
	"bytes"
	"image/png"
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
