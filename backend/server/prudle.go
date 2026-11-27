package server

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/evanofslack/prulights/prudle"
	"github.com/evanofslack/prulights/schedule"
	"github.com/evanofslack/prulights/store"
	"github.com/go-chi/chi/v5"
)

// Prudle is December's game: each night, guess the crown's colors with the
// cause as the clue, before the site shows them. Only tonight is playable; the
// server keeps the answer, grades every guess (backend/prudle) and keeps the
// results. The public api still shows every color: hiding them is the web's.

func (s *Server) mountPrudle(r chi.Router) {
	r.Get("/me/prudle", s.requireUser(s.getMyPrudle))
	r.Put("/me/prudle/settings", s.requireUser(s.putPrudleSettings))
	r.Get("/me/prudle/{date}", s.requireUser(s.getPrudleGame))
	r.Post("/me/prudle/{date}/guesses", s.requireUser(s.postPrudleGuess))
	r.Post("/me/prudle/{date}/pass", s.requireUser(s.postPrudlePass))
	r.Get("/prudle/{date}", s.requireUser(s.getPrudleNight))
}

// prudleNight is a night's lighting when it has a Prudle: in December, listed,
// with a cause and colors.
func (s *Server) prudleNight(r *http.Request, d schedule.Date) (schedule.Lighting, bool, error) {
	if !prudle.InSeason(d) {
		return schedule.Lighting{}, false, nil
	}
	ls, err := s.lightings(r, d, d)
	if err != nil || len(ls) == 0 {
		return schedule.Lighting{}, false, err
	}
	l := ls[0]
	return l, l.Scheduled && l.Cause != "" && len(l.Colors) > 0, nil
}

// prudleGame is a user's game of a night, a new one (not yet saved) before
// the first guess.
func (s *Server) prudleGame(r *http.Request, userID int64, l schedule.Lighting) (store.PrudleGame, error) {
	g, err := s.store.PrudleGame(r.Context(), userID, l.Date)
	if errors.Is(err, store.ErrNotFound) {
		return store.PrudleGame{UserID: userID, Date: l.Date, Answer: slices.Clone(l.Colors),
			Guesses: [][]schedule.ColorKey{}, Status: store.PrudlePlaying, CreatedAt: s.now()}, nil
	}
	return g, err
}

func (s *Server) getMyPrudle(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	tonight := schedule.Tonight(s.now())
	out := PrudleSummary{Tonight: tonight, Season: prudle.InSeason(tonight), Games: []PrudleResult{}}
	var err error
	if out.Off, err = s.store.PrudleOff(r.Context(), me.ID); err != nil {
		s.internalError(w, err)
		return
	}
	if out.Season {
		if _, out.Playable, err = s.prudleNight(r, tonight); err != nil {
			s.internalError(w, err)
			return
		}
		games, err := s.store.PrudleGames(r.Context(), me.ID, schedule.Date(string(tonight)[:8]+"01"), tonight)
		if err != nil {
			s.internalError(w, err)
			return
		}
		won := map[schedule.Date]bool{}
		for _, g := range games {
			out.Games = append(out.Games, PrudleResult{Date: g.Date, Status: string(g.Status), Turns: g.Turns})
			if g.Status == store.PrudleWon {
				out.Won++
				won[g.Date] = true
			}
			if g.Date == tonight && g.Status != store.PrudlePlaying {
				out.Done = true
			}
		}
		d := tonight
		if !won[d] {
			d = d.AddDays(-1) // tonight isn't played yet
		}
		for ; won[d]; d = d.AddDays(-1) {
			out.Streak++
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) putPrudleSettings(w http.ResponseWriter, r *http.Request) {
	var req PrudleSettings
	if !decode(w, r, &req) {
		return
	}
	if err := s.store.SetPrudleOff(r.Context(), userFrom(r.Context()).ID, req.Off); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, req)
}

// prudleDate reads the date and its night, answering for itself when there's
// no Prudle that night or it's still to come.
func (s *Server) prudleDate(w http.ResponseWriter, r *http.Request) (schedule.Lighting, schedule.Date, bool) {
	d, ok := dateParam(w, r)
	if !ok {
		return schedule.Lighting{}, "", false
	}
	tonight := schedule.Tonight(s.now())
	if d > tonight {
		writeError(w, http.StatusUnprocessableEntity, "that night's Prudle isn't open yet")
		return schedule.Lighting{}, "", false
	}
	l, ok, err := s.prudleNight(r, d)
	if err != nil {
		s.internalError(w, err)
		return schedule.Lighting{}, "", false
	}
	if !ok {
		writeError(w, http.StatusNotFound, "no Prudle that night")
		return schedule.Lighting{}, "", false
	}
	return l, tonight, true
}

func (s *Server) getPrudleGame(w http.ResponseWriter, r *http.Request) {
	l, tonight, ok := s.prudleDate(w, r)
	if !ok {
		return
	}
	g, err := s.prudleGame(r, userFrom(r.Context()).ID, l)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.writePrudle(w, l, g, tonight)
}

func (s *Server) postPrudleGuess(w http.ResponseWriter, r *http.Request) {
	l, tonight, ok := s.prudleDate(w, r)
	if !ok {
		return
	}
	var req PrudleGuess
	if !decode(w, r, &req) {
		return
	}
	if l.Date != tonight {
		writeError(w, http.StatusUnprocessableEntity, "only tonight's Prudle is open")
		return
	}
	me := userFrom(r.Context())
	g, err := s.prudleGame(r, me.ID, l)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if g.Status != store.PrudlePlaying {
		writeError(w, http.StatusUnprocessableEntity, "this game is over")
		return
	}
	game, err := prudle.Replay(g.Answer, g.Guesses)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if game, err = game.Guess(req.Colors); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	g.Guesses = append(g.Guesses, req.Colors)
	g.Turns, g.Misses = len(game.Turns), float64(game.Wrong())
	switch {
	case game.Solved():
		g.Status = store.PrudleWon
	case game.Lost():
		g.Status = store.PrudleLost
	}
	if g.Status != store.PrudlePlaying {
		g.FinishedAt = s.now()
	}
	if err := s.store.SavePrudleGame(r.Context(), g); err != nil {
		s.internalError(w, err)
		return
	}
	slog.Info("prudle guess", "user", me.ID, "date", l.Date, "turn", g.Turns, "status", g.Status)
	s.writePrudle(w, l, g, tonight)
}

// postPrudlePass is "just show me": tonight's game ends, colors shown.
func (s *Server) postPrudlePass(w http.ResponseWriter, r *http.Request) {
	l, tonight, ok := s.prudleDate(w, r)
	if !ok {
		return
	}
	if l.Date != tonight {
		writeError(w, http.StatusUnprocessableEntity, "only tonight's Prudle is open")
		return
	}
	me := userFrom(r.Context())
	g, err := s.prudleGame(r, me.ID, l)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if g.Status == store.PrudlePlaying {
		g.Status, g.FinishedAt = store.PrudlePassed, s.now()
		if err := s.store.SavePrudleGame(r.Context(), g); err != nil {
			s.internalError(w, err)
			return
		}
		slog.Info("prudle pass", "user", me.ID, "date", l.Date)
	}
	s.writePrudle(w, l, g, tonight)
}

func (s *Server) writePrudle(w http.ResponseWriter, l schedule.Lighting, g store.PrudleGame, tonight schedule.Date) {
	game, err := prudle.Replay(g.Answer, g.Guesses)
	if err != nil {
		s.internalError(w, err)
		return
	}
	out := PrudleState{
		Date: l.Date, Cause: l.Cause, Tries: prudle.Tries, MaxPicks: prudle.MaxPicks,
		Status: string(g.Status), Turns: turnsOf(game.Turns), Found: game.Found, Out: []PrudlePick{},
	}
	over := g.Status != store.PrudlePlaying || l.Date < tonight
	out.CanPlay = !over
	if over {
		out.Answer = game.Answer
	}
	for k, m := range game.Out {
		out.Out = append(out.Out, PrudlePick{Key: k, Mark: string(m)})
	}
	slices.SortFunc(out.Out, func(a, b PrudlePick) int { return strings.Compare(string(a.Key), string(b.Key)) })
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

func turnsOf(ts []prudle.Turn) []PrudleTurn {
	out := make([]PrudleTurn, len(ts))
	for i, t := range ts {
		picks := make([]PrudlePick, len(t.Picks))
		for j, p := range t.Picks {
			picks[j] = PrudlePick{Key: p.Key, Mark: string(p.Mark), Carried: p.Carried}
		}
		out[i] = PrudleTurn{Picks: picks, Size: t.Size, Count: string(t.Count)}
	}
	return out
}

// rowsOf is a game's guesses as marks only, good news first
// (web/lib/prudle.ts's glyphs).
func rowsOf(ts []prudle.Turn) []PrudleRow {
	out := make([]PrudleRow, len(ts))
	for i, t := range ts {
		marks := []string{}
		for _, m := range []prudle.Mark{prudle.Hit, prudle.Close, prudle.Miss} {
			for _, p := range t.Picks {
				if p.Mark == m {
					marks = append(marks, string(m))
				}
			}
		}
		out[i] = PrudleRow{Marks: marks, Size: t.Size, Count: string(t.Count)}
	}
	return out
}

// getPrudleNight is how the family did on a night, without the colors: it
// opens once your own game of it is over, or the night is past.
func (s *Server) getPrudleNight(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	me := userFrom(r.Context())
	tonight := schedule.Tonight(s.now())
	out := PrudleNight{Date: d, Players: []PrudlePlayer{}}
	if d >= tonight {
		g, err := s.store.PrudleGame(r.Context(), me.ID, d)
		out.Locked = d > tonight || err != nil || g.Status == store.PrudlePlaying
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			s.internalError(w, err)
			return
		}
	}
	if !out.Locked {
		games, err := s.store.PrudleNight(r.Context(), d)
		if err != nil {
			s.internalError(w, err)
			return
		}
		for _, g := range games {
			game, err := prudle.Replay(g.Answer, g.Guesses)
			if err != nil {
				s.internalError(w, err)
				return
			}
			out.Players = append(out.Players, PrudlePlayer{Name: g.Name, Mine: g.UserID == me.ID,
				Status: string(g.Status), Rows: rowsOf(game.Turns)})
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}
