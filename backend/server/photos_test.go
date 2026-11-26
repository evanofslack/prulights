package server

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testJPEG is a w×h jpeg with an EXIF segment carrying a marker, standing in
// for a phone's location.
func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	payload := append([]byte("Exif\x00\x00"), []byte("GPS 42.3471N 71.0825W")...)
	n := len(payload) + 2
	app1 := append([]byte{0xFF, 0xE1, byte(n >> 8), byte(n)}, payload...)
	return append(append(append([]byte{}, b[:2]...), app1...), b[2:]...)
}

func upload(t *testing.T, s *Server, date, contentType string, body []byte, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("PUT", "/api/v1/me/sightings/"+date+"/photo", bytes.NewReader(body))
	req.Header.Set("Origin", origin)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestPhotoUpload(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)
	_, dad := person(t, s, "Dad", false)
	pic := testJPEG(t, 2000, 1000)
	if !bytes.Contains(pic, []byte("GPS 42.3471N")) {
		t.Fatal("test jpeg lost its exif")
	}

	// a photo logs the night
	rec := upload(t, s, "2026-12-12", "image/jpeg", pic, mom)
	if rec.Code != 200 {
		t.Fatalf("upload = %d %s", rec.Code, rec.Body.String())
	}
	got := decodeAs[Sighting](t, rec.Body.Bytes())
	if got.Photo == nil || got.Photo.Width != 1600 || got.Photo.Height != 800 || !got.Mine {
		t.Fatalf("sighting = %+v", got)
	}

	// others see it; it's re-encoded with no exif, and the thumb is small
	night := decodeAs[NightSightings](t, call(t, s, "GET", "/api/v1/sightings/2026-12-12", "", dad).Body.Bytes())
	if len(night.Sightings) != 1 || night.Sightings[0].Photo == nil || night.Sightings[0].Photo.SHA256 != got.Photo.SHA256 {
		t.Fatalf("night = %+v", night)
	}
	full := call(t, s, "GET", "/api/v1/photos/"+got.Photo.SHA256+"/full.jpg", "", dad)
	if full.Code != 200 || full.Header().Get("Content-Type") != "image/jpeg" || bytes.Contains(full.Body.Bytes(), []byte("GPS")) {
		t.Errorf("full = %d %s", full.Code, full.Header())
	}
	thumb := call(t, s, "GET", "/api/v1/photos/"+got.Photo.SHA256+"/thumb.jpg", "", dad)
	if cfg, err := jpeg.DecodeConfig(thumb.Body); err != nil || cfg.Width != 400 || cfg.Height != 200 {
		t.Errorf("thumb = %+v %v", cfg, err)
	}
	if rec := call(t, s, "GET", "/api/v1/photos/"+got.Photo.SHA256+"/thumb.jpg", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out = %d", rec.Code)
	}

	// one per night: a new one replaces it
	small := testJPEG(t, 300, 200)
	again := decodeAs[Sighting](t, upload(t, s, "2026-12-12", "image/jpeg", small, mom).Body.Bytes())
	if again.Photo == nil || again.Photo.Width != 300 || again.Photo.SHA256 == got.Photo.SHA256 {
		t.Errorf("replaced = %+v", again.Photo)
	}
	if rec := call(t, s, "GET", "/api/v1/photos/"+got.Photo.SHA256+"/full.jpg", "", dad); rec.Code != http.StatusNotFound {
		t.Errorf("old photo = %d", rec.Code)
	}

	// taking it down keeps the sighting; un-logging the night takes the photo
	if rec := call(t, s, "DELETE", "/api/v1/me/sightings/2026-12-12/photo", "", mom); rec.Code != http.StatusNoContent {
		t.Errorf("delete = %d", rec.Code)
	}
	night = decodeAs[NightSightings](t, call(t, s, "GET", "/api/v1/sightings/2026-12-12", "", dad).Body.Bytes())
	if len(night.Sightings) != 1 || night.Sightings[0].Photo != nil {
		t.Errorf("after delete = %+v", night)
	}
	again = decodeAs[Sighting](t, upload(t, s, "2026-12-12", "image/jpeg", small, mom).Body.Bytes())
	call(t, s, "DELETE", "/api/v1/me/sightings/2026-12-12", "", mom)
	if rec := call(t, s, "GET", "/api/v1/photos/"+again.Photo.SHA256+"/full.jpg", "", dad); rec.Code != http.StatusNotFound {
		t.Errorf("photo outlived the sighting: %d", rec.Code)
	}
}

func TestPhotoRejects(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)
	pic := testJPEG(t, 40, 30)
	for name, c := range map[string]struct {
		date, ct string
		body     []byte
		want     int
	}{
		"png":       {"2026-12-12", "image/png", pic, http.StatusUnsupportedMediaType},
		"json":      {"2026-12-12", "application/json", []byte(`{}`), http.StatusUnsupportedMediaType},
		"not jpeg":  {"2026-12-12", "image/jpeg", []byte("hello"), http.StatusBadRequest},
		"too wide":  {"2026-12-12", "image/jpeg", testJPEG(t, 8001, 2), http.StatusBadRequest},
		"too big":   {"2026-12-12", "image/jpeg", append(append([]byte{}, pic...), make([]byte, maxPhotoBytes)...), http.StatusRequestEntityTooLarge},
		"too early": {"2026-12-04", "image/jpeg", pic, http.StatusUnprocessableEntity},
		"ahead":     {"2026-12-13", "image/jpeg", pic, http.StatusUnprocessableEntity},
	} {
		if rec := upload(t, s, c.date, c.ct, c.body, mom); rec.Code != c.want {
			t.Errorf("%s = %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}
	if rec := upload(t, s, "2026-12-12", "image/jpeg", pic, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out = %d", rec.Code)
	}
}

func TestAdminRemovesPhoto(t *testing.T) {
	s := seeded(t)
	_, mom := person(t, s, "Mom", false)
	_, evan := person(t, s, "Evan", true)
	got := decodeAs[Sighting](t, upload(t, s, "2026-12-12", "image/jpeg", testJPEG(t, 40, 30), mom).Body.Bytes())
	path := "/api/v1/admin/photos/" + got.Photo.SHA256
	if rec := call(t, s, "DELETE", path, "", mom); rec.Code != http.StatusForbidden {
		t.Errorf("non-admin = %d", rec.Code)
	}
	if rec := call(t, s, "DELETE", path, "", evan); rec.Code != http.StatusNoContent {
		t.Errorf("admin = %d", rec.Code)
	}
	if rec := call(t, s, "DELETE", path, "", evan); rec.Code != http.StatusNotFound {
		t.Errorf("twice = %d", rec.Code)
	}
}
