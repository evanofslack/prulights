package server

import (
	"fmt"
	"image/color"
	"net/http"
	"slices"

	"github.com/evanofslack/prulights/render"
	"github.com/evanofslack/prulights/schedule"
	"github.com/go-chi/chi/v5"
)

// mountDev adds routes for working on the frame, only when DEV is set.
//
//	GET /dev/render/{view}?date=2026-12-12&palette=measured|ideal&sample=1&stale=1&low_battery=1
//
// draws a frame view as a png from the real schedule (or, with sample=1, the
// renderer's sample data, which has sightings and stats).
//
//	GET /dev/inks
//
// says how to read a packed frame: each ink's panel code and colors, and which
// way the picture is turned onto the panel. The emulator decodes the device
// api's frames with it.
func (s *Server) mountDev() {
	s.router.Get("/dev/render/{view}", s.getDevRender)
	s.router.Get("/dev/inks", s.getDevInks)
}

// DevInk is one of the panel's inks, for the emulator.
type DevInk struct {
	Name     string `json:"name"`
	Code     byte   `json:"code"`
	Measured string `json:"measured"` // #rrggbb
	Ideal    string `json:"ideal"`
}

// DevInks is GET /dev/inks.
type DevInks struct {
	Inks []DevInk `json:"inks"`
	// RotateClockwise: portrait (x, y) is panel (y, 479-x); otherwise (799-y, x).
	RotateClockwise bool `json:"rotate_clockwise"`
	PanelWidth      int  `json:"panel_width"`
	PanelHeight     int  `json:"panel_height"`
}

func (s *Server) getDevInks(w http.ResponseWriter, r *http.Request) {
	out := DevInks{RotateClockwise: render.RotateClockwise, PanelWidth: render.PanelWidth, PanelHeight: render.PanelHeight}
	hex := func(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }
	for _, ink := range render.Inks {
		out.Inks = append(out.Inks, DevInk{Name: ink.String(), Code: ink.PanelCode(),
			Measured: hex(render.Measured.Colors[ink]), Ideal: hex(render.Ideal.Colors[ink])})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getDevRender(w http.ResponseWriter, r *http.Request) {
	view := render.View(chi.URLParam(r, "view"))
	if !slices.Contains(render.Views, view) {
		writeError(w, http.StatusNotFound, "no such view")
		return
	}
	q := r.URL.Query()
	date := schedule.Tonight(s.now())
	if v := q.Get("date"); v != "" {
		d, err := schedule.ParseDate(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "date must be YYYY-MM-DD")
			return
		}
		date = d
	}

	var data render.Data
	if q.Get("sample") == "1" {
		data = render.SampleData(date)
	} else {
		// enough nights for the month view and the week after tonight
		first := schedule.Date(string(date)[:8] + "01")
		nights, err := s.lightings(r, first.AddDays(-7), first.AddDays(45))
		if err != nil {
			s.internalError(w, err)
			return
		}
		data = render.Data{Tonight: date, Nights: nights, UpdatedAt: s.now()}
	}
	data.BaseURL = s.cfg.BaseURL
	if q.Get("stale") == "1" {
		data.StaleSince = date.AddDays(-3)
	}
	data.LowBattery = q.Get("low_battery") == "1"

	f, err := render.Render(view, data)
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(f.PNG(render.PaletteNamed(q.Get("palette"))))
}
