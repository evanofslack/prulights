package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/evanofslack/prulights/schedule"
	"github.com/evanofslack/prulights/store"
	"github.com/go-chi/chi/v5"
	"golang.org/x/image/draw"
)

// Photos: one picture per person per night they saw, members only. The
// browser sends a jpeg; the server decodes it and keeps two re-encoded
// copies, so nothing from the phone (location, camera) survives.

const (
	maxPhotoBytes = 10 << 20
	maxPhotoSide  = 8000 // pixels, checked before decoding
	photoFull     = 1600 // long side of the kept picture
	photoThumb    = 400
)

func (s *Server) mountPhotos(r chi.Router) {
	r.Put("/me/sightings/{date}/photo", s.requireUser(s.putPhoto))
	r.Delete("/me/sightings/{date}/photo", s.requireUser(s.deleteMyPhoto))
	r.Get("/photos/{sha}/full.jpg", s.requireUser(s.servePhoto(false)))
	r.Get("/photos/{sha}/thumb.jpg", s.requireUser(s.servePhoto(true)))
}

func (s *Server) mountAdminPhotos(r chi.Router) {
	r.Delete("/photos/{sha}", s.adminDeletePhoto)
}

var errNotPhoto = errors.New("that isn't a jpeg we can read")

// processPhoto turns an uploaded jpeg into the kept picture and its thumbnail.
func processPhoto(body []byte) (store.Photo, error) {
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return store.Photo{}, errNotPhoto
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxPhotoSide || cfg.Height > maxPhotoSide {
		return store.Photo{}, errors.New("that picture is too big, at most 8000 pixels a side")
	}
	src, err := jpeg.Decode(bytes.NewReader(body))
	if err != nil {
		return store.Photo{}, errNotPhoto
	}
	full, fw, fh, err := encodeScaled(src, photoFull, 82)
	if err != nil {
		return store.Photo{}, err
	}
	thumb, _, _, err := encodeScaled(src, photoThumb, 75)
	if err != nil {
		return store.Photo{}, err
	}
	sum := sha256.Sum256(full)
	return store.Photo{PhotoInfo: store.PhotoInfo{SHA256: hex.EncodeToString(sum[:]), Width: fw, Height: fh},
		Full: full, Thumb: thumb}, nil
}

// encodeScaled shrinks an image to fit a long side (never enlarging it) and
// encodes it as a jpeg.
func encodeScaled(src image.Image, long, quality int) ([]byte, int, int, error) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if m := max(w, h); m > long {
		w, h = max(1, w*long/m), max(1, h*long/m)
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: quality}); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), w, h, nil
}

func (s *Server) putPhoto(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	if r.Header.Get("Content-Type") != "image/jpeg" {
		writeError(w, http.StatusUnsupportedMediaType, "send the photo as a jpeg")
		return
	}
	now := s.now()
	if !canLog(d, schedule.Tonight(now)) {
		writeError(w, http.StatusUnprocessableEntity, "you can add photos for tonight and the week before it")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPhotoBytes))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeError(w, http.StatusRequestEntityTooLarge, "that photo is too big, at most 10 MB")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "couldn't read the photo")
		return
	}
	p, err := processPhoto(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	me := userFrom(r.Context())
	if err := s.store.SavePhoto(r.Context(), me.ID, d, p, now); err != nil {
		s.internalError(w, err)
		return
	}
	slog.Info("photo", "user", me.ID, "date", d, "bytes", len(p.Full))
	s.writeMySighting(w, r, me.ID, d)
}

// writeMySighting answers with a user's sighting of a night, as it is now.
func (s *Server) writeMySighting(w http.ResponseWriter, r *http.Request, me int64, d schedule.Date) {
	xs, err := s.store.UserSightings(r.Context(), me, d, d)
	if err != nil || len(xs) != 1 {
		s.internalError(w, errors.Join(errors.New("sighting missing after photo"), err))
		return
	}
	writeJSON(w, http.StatusOK, sightingOf(xs[0], me))
}

// deleteMyPhoto takes a photo down. Unlike the sighting itself, that's open
// any time: it's your picture.
func (s *Server) deleteMyPhoto(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	if _, err := s.store.DeletePhoto(r.Context(), userFrom(r.Context()).ID, d); err != nil {
		s.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) servePhoto(thumb bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sha := chi.URLParam(r, "sha")
		etag := `"` + sha + `"`
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		b, err := s.store.PhotoImage(r.Context(), sha, thumb)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no such photo")
			return
		}
		if err != nil {
			s.internalError(w, err)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", strconv.Itoa(len(b)))
		// the address is the content's hash, so it never changes; private,
		// since it's for members
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		w.Header().Set("ETag", etag)
		w.Write(b)
	}
}

func (s *Server) adminDeletePhoto(w http.ResponseWriter, r *http.Request) {
	sha := chi.URLParam(r, "sha")
	n, err := s.store.DeletePhotosBySHA(r.Context(), sha)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "no such photo")
		return
	}
	slog.Info("photo removed", "sha", sha, "by", userFrom(r.Context()).ID)
	w.WriteHeader(http.StatusNoContent)
}
