package server

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/evanofslack/prulights/schedule"
	"github.com/evanofslack/prulights/store"
	"github.com/go-chi/chi/v5"
)

const (
	// logWindow is how many nights back "I saw it" stays open, besides tonight.
	logWindow   = 7
	maxNoteLen  = 280
	maxCauseLen = 200
	// scrapeTimeout bounds a "scrape now" from the admin page.
	scrapeTimeout = 10 * time.Minute
)

// SetScraper gives the server a way to check the source pages on demand (the
// admin page's "scrape now").
func (s *Server) SetScraper(fn func(context.Context) error) { s.scrape = fn }

// mountSightings adds the signed-in sightings api and the admin pages' api,
// inside mountAccount's group.
func (s *Server) mountSightings(r chi.Router) {
	r.Get("/me/sightings", s.requireUser(s.getMySightings))
	r.Put("/me/sightings/{date}", s.requireUser(s.putSighting))
	r.Delete("/me/sightings/{date}", s.requireUser(s.deleteSighting))
	r.Get("/me/stats", s.requireUser(s.getMyStats))
	r.Get("/sightings/{date}", s.requireUser(s.getNightSightings))
}

func (s *Server) mountAdmin(r chi.Router) {
	r.Get("/users", s.getUsers)
	r.Get("/overrides", s.getOverrides)
	r.Put("/overrides/{date}", s.putOverride)
	r.Delete("/overrides/{date}", s.deleteOverride)
	r.Get("/scrape", s.getScrape)
	r.Post("/scrape", s.postScrape)
}

// canLog is whether a night is open for "I saw it": tonight and the week
// before it.
func canLog(d, tonight schedule.Date) bool {
	return d <= tonight && d >= tonight.AddDays(-logWindow)
}

func dateParam(w http.ResponseWriter, r *http.Request) (schedule.Date, bool) {
	d, err := schedule.ParseDate(chi.URLParam(r, "date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "date must be YYYY-MM-DD")
		return "", false
	}
	return d, true
}

func sightingOf(x store.Sighting, me int64) Sighting {
	return Sighting{Date: x.Date, Name: x.Name, Mine: x.UserID == me, Rating: x.Rating, Note: x.Note, At: x.CreatedAt}
}

func (s *Server) putSighting(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	var req SightingRequest
	if !decode(w, r, &req) {
		return
	}
	now := s.now()
	if !canLog(d, schedule.Tonight(now)) {
		writeError(w, http.StatusUnprocessableEntity, "you can log tonight and the week before it")
		return
	}
	if req.Rating < 0 || req.Rating > 5 {
		writeError(w, http.StatusBadRequest, "rating is 1 to 5, or 0 for none")
		return
	}
	req.Note = strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(req.Note) > maxNoteLen {
		writeError(w, http.StatusBadRequest, "keep the note to 280 characters")
		return
	}
	me := userFrom(r.Context())
	if err := s.store.SaveSighting(r.Context(), me.ID, d, req.Rating, req.Note, now); err != nil {
		s.internalError(w, err)
		return
	}
	slog.Info("sighting", "user", me.ID, "date", d, "rating", req.Rating)
	writeJSON(w, http.StatusOK, Sighting{Date: d, Name: me.Name, Mine: true, Rating: req.Rating, Note: req.Note, At: now})
}

func (s *Server) deleteSighting(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	if !canLog(d, schedule.Tonight(s.now())) {
		writeError(w, http.StatusUnprocessableEntity, "you can change tonight and the week before it")
		return
	}
	if err := s.store.DeleteSighting(r.Context(), userFrom(r.Context()).ID, d); err != nil {
		s.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getNightSightings(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	xs, err := s.store.NightSightings(r.Context(), d)
	if err != nil {
		s.internalError(w, err)
		return
	}
	me := userFrom(r.Context()).ID
	out := NightSightings{Date: d, Sightings: []Sighting{}, CanLog: canLog(d, schedule.Tonight(s.now()))}
	for _, x := range xs {
		out.Sightings = append(out.Sightings, sightingOf(x, me))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// myNights is everything a user has logged, newest first, with each night's
// lighting.
func (s *Server) myNights(r *http.Request, userID int64) ([]LoggedNight, error) {
	tonight := schedule.Tonight(s.now())
	xs, err := s.store.UserSightings(r.Context(), userID, "0001-01-01", tonight)
	if err != nil || len(xs) == 0 {
		return []LoggedNight{}, err
	}
	nights, err := s.lightings(r, xs[len(xs)-1].Date, xs[0].Date)
	if err != nil {
		return nil, err
	}
	byDate := map[schedule.Date]schedule.Lighting{}
	for _, l := range nights {
		byDate[l.Date] = l
	}
	out := make([]LoggedNight, len(xs))
	for i, x := range xs {
		out[i] = LoggedNight{Sighting: sightingOf(x, userID), Lighting: byDate[x.Date]}
	}
	return out, nil
}

func (s *Server) getMySightings(w http.ResponseWriter, r *http.Request) {
	nights, err := s.myNights(r, userFrom(r.Context()).ID)
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, MySightings{Nights: nights})
}

func (s *Server) getMyStats(w http.ResponseWriter, r *http.Request) {
	nights, err := s.myNights(r, userFrom(r.Context()).ID)
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, statsOf(nights, schedule.Tonight(s.now())))
}

// statsOf sums up logged nights (newest first).
func statsOf(nights []LoggedNight, tonight schedule.Date) MyStats {
	st := MyStats{Seen: len(nights), Favorites: []ColorCount{}, LogFrom: tonight.AddDays(-logWindow), Tonight: tonight}
	sum := 0
	counts := map[schedule.ColorKey]int{}
	seen := map[schedule.Date]bool{}
	for _, n := range nights {
		seen[n.Date] = true
		if n.Rating > 0 {
			st.Rated++
			sum += n.Rating
		}
		if n.Lighting.Scheduled {
			for _, c := range n.Lighting.Colors {
				counts[c]++
			}
		}
	}
	if st.Rated > 0 {
		st.Average = float64(sum) / float64(st.Rated)
	}
	d := tonight
	if !seen[d] {
		d = d.AddDays(-1) // tonight isn't over yet
	}
	for ; seen[d]; d = d.AddDays(-1) {
		st.Streak++
	}
	for c, n := range counts {
		st.Favorites = append(st.Favorites, ColorCount{Color: c, Count: n})
	}
	slices.SortFunc(st.Favorites, func(a, b ColorCount) int {
		return cmp.Or(b.Count-a.Count, strings.Compare(string(a.Color), string(b.Color)))
	})
	if len(st.Favorites) > 3 {
		st.Favorites = st.Favorites[:3]
	}
	return st
}

// getNightStats is public: a night's totals, with no names.
func (s *Server) getNightStats(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	n, err := s.store.NightStats(r.Context(), d)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeCached(w, r, NightStatsResponse{Date: d, Seen: n.Seen, Rated: n.Rated, Average: n.Average}, time.Minute)
}

// --- admin

func (s *Server) getUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.Users(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	counts, err := s.store.SightingCounts(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	out := make([]UserInfo, len(users))
	for i, u := range users {
		out[i] = UserInfo{ID: u.ID, Name: u.Name, IsAdmin: u.IsAdmin, CreatedAt: u.CreatedAt, Sightings: counts[u.ID]}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getOverrides(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Overrides(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	out := make([]OverrideInfo, len(list))
	for i, o := range list {
		out[i] = OverrideInfo{Date: o.Date, UpdatedAt: o.UpdatedAt,
			OverrideBody: OverrideBody{Colors: o.Colors, Cause: o.Cause, CauseURL: o.CauseURL, Kind: o.Kind, Note: o.Note}}
	}
	writeJSON(w, http.StatusOK, out)
}

var kinds = []schedule.Kind{schedule.KindNonprofit, schedule.KindHoliday, schedule.KindSports, schedule.KindCivic}

func (s *Server) putOverride(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	var b OverrideBody
	if !decode(w, r, &b) {
		return
	}
	if len(b.Colors) == 0 || len(b.Colors) > 6 {
		writeError(w, http.StatusBadRequest, "pick 1 to 6 colors")
		return
	}
	for _, c := range b.Colors {
		if _, ok := schedule.Lookup(c); !ok {
			writeError(w, http.StatusBadRequest, "unknown color "+string(c))
			return
		}
	}
	b.Cause, b.CauseURL, b.Note = strings.TrimSpace(b.Cause), strings.TrimSpace(b.CauseURL), strings.TrimSpace(b.Note)
	if utf8.RuneCountInString(b.Cause) > maxCauseLen || utf8.RuneCountInString(b.Note) > maxNoteLen {
		writeError(w, http.StatusBadRequest, "cause or note too long")
		return
	}
	if b.CauseURL != "" && !strings.HasPrefix(b.CauseURL, "https://") && !strings.HasPrefix(b.CauseURL, "http://") {
		writeError(w, http.StatusBadRequest, "the link must start with https://")
		return
	}
	if b.Kind == "" {
		b.Kind = schedule.Classify(b.Cause)
	} else if !slices.Contains(kinds, b.Kind) {
		writeError(w, http.StatusBadRequest, "unknown kind")
		return
	}
	o := store.Override{Date: d, Colors: b.Colors, Cause: b.Cause, CauseURL: b.CauseURL, Kind: b.Kind, Note: b.Note}
	if err := s.store.SaveOverride(r.Context(), o, s.now()); err != nil {
		s.internalError(w, err)
		return
	}
	slog.Info("override saved", "date", d, "by", userFrom(r.Context()).ID)
	writeJSON(w, http.StatusOK, OverrideInfo{Date: d, OverrideBody: b, UpdatedAt: s.now()})
}

func (s *Server) deleteOverride(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	found, err := s.store.DeleteOverride(r.Context(), d)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "no correction on that night")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getScrape(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.Runs(r.Context(), 20)
	if err != nil {
		s.internalError(w, err)
		return
	}
	out := ScrapeStatus{Running: s.scraping.Load(), Runs: make([]ScrapeRunInfo, len(runs))}
	for i, x := range runs {
		out.Runs[i] = ScrapeRunInfo{ID: x.ID, StartedAt: x.StartedAt, FinishedAt: x.FinishedAt, Pages: x.Pages,
			Fetched: x.Fetched, Items: x.Items, Added: x.Added, Changed: x.Changed, Removed: x.Removed,
			Warnings: x.Warnings, Error: x.Error}
		if out.Runs[i].Warnings == nil {
			out.Runs[i].Warnings = []string{}
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// postScrape checks the source pages now, in the background.
func (s *Server) postScrape(w http.ResponseWriter, r *http.Request) {
	if s.scrape == nil {
		writeError(w, http.StatusServiceUnavailable, "the scraper isn't running here")
		return
	}
	if !s.scraping.CompareAndSwap(false, true) {
		writeError(w, http.StatusConflict, "already checking")
		return
	}
	slog.Info("scrape requested", "by", userFrom(r.Context()).ID)
	go func() {
		defer s.scraping.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), scrapeTimeout)
		defer cancel()
		if err := s.scrape(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("scrape now", "error", err)
		}
	}()
	w.WriteHeader(http.StatusAccepted)
}
