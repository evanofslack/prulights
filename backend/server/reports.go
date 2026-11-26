package server

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/evanofslack/prulights/schedule"
	"github.com/evanofslack/prulights/store"
	"github.com/go-chi/chi/v5"
)

// Reports: members say a night's colors were listed wrong. They change
// nothing by themselves; an admin accepting them writes an override.

func (s *Server) mountReports(r chi.Router) {
	r.Put("/me/reports/{date}", s.requireUser(s.putReport))
	r.Delete("/me/reports/{date}", s.requireUser(s.deleteReport))
	r.Get("/reports/{date}", s.requireUser(s.getNightReports))
}

func (s *Server) mountAdminReports(r chi.Router) {
	r.Get("/reports", s.getAdminReports)
	r.Post("/reports/{date}/accept", s.acceptReports)
	r.Post("/reports/{date}/dismiss", s.dismissReports)
}

// canReport is whether a night takes reports: the week before tonight (like
// "I saw it") and every night after, since the schedule can be wrong ahead.
func canReport(d, tonight schedule.Date) bool {
	return d >= tonight.AddDays(-logWindow)
}

// validColors checks a picked set of colors: 1 to 6 palette keys.
func validColors(w http.ResponseWriter, colors []schedule.ColorKey) bool {
	if len(colors) == 0 || len(colors) > 6 {
		writeError(w, http.StatusBadRequest, "pick 1 to 6 colors")
		return false
	}
	for _, c := range colors {
		if _, ok := schedule.Lookup(c); !ok {
			writeError(w, http.StatusBadRequest, "unknown color "+string(c))
			return false
		}
	}
	return true
}

func reportOf(x store.Report, me int64) Report {
	return Report{Date: x.Date, Name: x.Name, Mine: x.UserID == me, Colors: x.Colors, Note: x.Note, Status: x.Status, At: x.UpdatedAt}
}

func (s *Server) putReport(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	var req ReportRequest
	if !decode(w, r, &req) {
		return
	}
	now := s.now()
	if !canReport(d, schedule.Tonight(now)) {
		writeError(w, http.StatusUnprocessableEntity, "you can report the week before tonight and any night after")
		return
	}
	if !validColors(w, req.Colors) {
		return
	}
	req.Note = strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(req.Note) > maxNoteLen {
		writeError(w, http.StatusBadRequest, "keep the note to 280 characters")
		return
	}
	me := userFrom(r.Context())
	if err := s.store.SaveReport(r.Context(), me.ID, d, req.Colors, req.Note, now); err != nil {
		s.internalError(w, err)
		return
	}
	slog.Info("report", "user", me.ID, "date", d, "colors", req.Colors)
	writeJSON(w, http.StatusOK, Report{Date: d, Name: me.Name, Mine: true, Colors: req.Colors, Note: req.Note,
		Status: store.ReportOpen, At: now})
}

func (s *Server) deleteReport(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteReport(r.Context(), userFrom(r.Context()).ID, d); err != nil {
		s.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getNightReports(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	xs, err := s.store.NightReports(r.Context(), d)
	if err != nil {
		s.internalError(w, err)
		return
	}
	me := userFrom(r.Context()).ID
	out := NightReports{Date: d, Reports: []Report{}, CanReport: canReport(d, schedule.Tonight(s.now()))}
	for _, x := range xs {
		out.Reports = append(out.Reports, reportOf(x, me))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// suggested is the set of colors the most reports name (the earliest such
// report breaks a tie).
func suggested(xs []store.Report) []schedule.ColorKey {
	counts := map[string]int{}
	var best []schedule.ColorKey
	top := 0
	for _, x := range xs {
		var k strings.Builder
		for _, c := range x.Colors {
			k.WriteString(string(c) + ",")
		}
		counts[k.String()]++
		if n := counts[k.String()]; n > top {
			top, best = n, x.Colors
		}
	}
	return best
}

func (s *Server) getAdminReports(w http.ResponseWriter, r *http.Request) {
	xs, err := s.store.OpenReports(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	out := AdminReports{Nights: []ReportedNight{}}
	byDate := map[schedule.Date][]store.Report{}
	var dates []schedule.Date
	for _, x := range xs {
		if _, ok := byDate[x.Date]; !ok {
			dates = append(dates, x.Date)
		}
		byDate[x.Date] = append(byDate[x.Date], x)
	}
	if len(dates) > 0 {
		// dates come newest first
		nights, err := s.lightings(r, dates[len(dates)-1], dates[0])
		if err != nil {
			s.internalError(w, err)
			return
		}
		lit := map[schedule.Date]schedule.Lighting{}
		for _, l := range nights {
			lit[l.Date] = l
		}
		me := userFrom(r.Context()).ID
		for _, d := range dates {
			n := ReportedNight{Lighting: lit[d], Reports: []Report{}, Suggested: suggested(byDate[d])}
			for _, x := range byDate[d] {
				n.Reports = append(n.Reports, reportOf(x, me))
			}
			out.Nights = append(out.Nights, n)
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// acceptReports corrects a night to the reported colors, keeping its cause,
// and closes its open reports.
func (s *Server) acceptReports(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	var b AcceptReportBody
	if r.ContentLength != 0 && !decode(w, r, &b) {
		return
	}
	xs, err := s.store.NightReports(r.Context(), d)
	if err != nil {
		s.internalError(w, err)
		return
	}
	xs = slices.DeleteFunc(xs, func(x store.Report) bool { return x.Status != store.ReportOpen })
	if len(xs) == 0 {
		writeError(w, http.StatusNotFound, "no open reports on that night")
		return
	}
	if len(b.Colors) == 0 {
		b.Colors = suggested(xs)
	}
	if !validColors(w, b.Colors) {
		return
	}
	nights, err := s.lightings(r, d, d) // one night, filled if unlisted
	if err != nil {
		s.internalError(w, err)
		return
	}
	l := nights[0]
	o := store.Override{Date: d, Colors: b.Colors, Cause: l.Cause, CauseURL: l.CauseURL, Kind: l.Kind, Note: "from reports"}
	now := s.now()
	if err := s.store.SaveOverride(r.Context(), o, now); err != nil {
		s.internalError(w, err)
		return
	}
	if _, err := s.store.ResolveReports(r.Context(), d, store.ReportAccepted, now); err != nil {
		s.internalError(w, err)
		return
	}
	slog.Info("reports accepted", "date", d, "colors", b.Colors, "by", userFrom(r.Context()).ID)
	writeJSON(w, http.StatusOK, OverrideInfo{Date: d, UpdatedAt: now,
		OverrideBody: OverrideBody{Colors: o.Colors, Cause: o.Cause, CauseURL: o.CauseURL, Kind: o.Kind, Note: o.Note}})
}

func (s *Server) dismissReports(w http.ResponseWriter, r *http.Request) {
	d, ok := dateParam(w, r)
	if !ok {
		return
	}
	n, err := s.store.ResolveReports(r.Context(), d, store.ReportDismissed, s.now())
	if err != nil {
		s.internalError(w, err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "no open reports on that night")
		return
	}
	slog.Info("reports dismissed", "date", d, "by", userFrom(r.Context()).ID)
	w.WriteHeader(http.StatusNoContent)
}
