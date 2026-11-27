package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/evanofslack/prulights/store"
	"github.com/go-chi/chi/v5"
)

// Your name and your ideas: a member can change the name everyone sees, and
// send ideas for the site, which the admins read in /admin.

const maxFeedbackLen = 2000

func (s *Server) mountProfile(r chi.Router) {
	r.Put("/me/name", s.requireUser(s.putName))
	r.Post("/me/feedback", s.requireUser(s.postFeedback))
}

func (s *Server) mountAdminFeedback(r chi.Router) {
	r.Get("/feedback", s.getFeedback)
	r.Put("/feedback/{id}", s.putFeedbackDone)
	r.Delete("/feedback/{id}", s.deleteFeedback)
}

func (s *Server) putName(w http.ResponseWriter, r *http.Request) {
	var req RenameRequest
	if !decode(w, r, &req) {
		return
	}
	name, ok := cleanName(req.Name)
	if !ok {
		writeError(w, http.StatusBadRequest, "a name is 1 to 40 characters")
		return
	}
	me := userFrom(r.Context())
	err := s.store.RenameUser(r.Context(), me.ID, name)
	if errors.Is(err, store.ErrNameTaken) {
		writeError(w, http.StatusConflict, "someone already has that name")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	slog.Info("renamed", "user", me.ID, "from", me.Name, "to", name)
	u := *me
	u.Name = name
	writeJSON(w, http.StatusOK, meOf(u))
}

func (s *Server) postFeedback(w http.ResponseWriter, r *http.Request) {
	var req FeedbackRequest
	if !decode(w, r, &req) {
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeError(w, http.StatusBadRequest, "write your idea first")
		return
	}
	if utf8.RuneCountInString(text) > maxFeedbackLen {
		writeError(w, http.StatusBadRequest, "keep it to 2000 characters")
		return
	}
	me := userFrom(r.Context())
	if _, err := s.store.AddFeedback(r.Context(), me.ID, text, s.now()); err != nil {
		s.internalError(w, err)
		return
	}
	slog.Info("feedback", "user", me.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getFeedback(w http.ResponseWriter, r *http.Request) {
	xs, err := s.store.AllFeedback(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	out := make([]FeedbackInfo, 0, len(xs))
	for _, f := range xs {
		out = append(out, FeedbackInfo{ID: f.ID, Name: f.Name, Text: f.Body, CreatedAt: f.CreatedAt, Done: !f.DoneAt.IsZero()})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

func feedbackID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad id")
		return 0, false
	}
	return id, true
}

func (s *Server) putFeedbackDone(w http.ResponseWriter, r *http.Request) {
	id, ok := feedbackID(w, r)
	if !ok {
		return
	}
	var req FeedbackDone
	if !decode(w, r, &req) {
		return
	}
	s.feedbackResult(w, s.store.SetFeedbackDone(r.Context(), id, req.Done, s.now()))
}

func (s *Server) deleteFeedback(w http.ResponseWriter, r *http.Request) {
	id, ok := feedbackID(w, r)
	if !ok {
		return
	}
	s.feedbackResult(w, s.store.DeleteFeedback(r.Context(), id))
}

func (s *Server) feedbackResult(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "no such idea")
	case err != nil:
		s.internalError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
