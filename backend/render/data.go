package render

import (
	"strings"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

// View names one of the frame's screens.
type View string

const (
	ViewTonight   View = "tonight"
	ViewWeek      View = "week"
	ViewMonth     View = "month"
	ViewLog       View = "log"
	ViewSetup     View = "setup"
	ViewGoodnight View = "goodnight"
	ViewWifi      View = "wifi"
)

// Views lists every view, the ones the button cycles through first.
var Views = []View{ViewTonight, ViewWeek, ViewMonth, ViewLog, ViewSetup, ViewGoodnight, ViewWifi}

// Sighting is what the frame's owner logged for a night.
type Sighting struct {
	Seen   bool
	Rating int // 1–5, 0 for none
}

// ColorCount is how often a color appeared in the nights someone saw.
type ColorCount struct {
	Color schedule.ColorKey
	Count int
}

// RatedNight is one of the owner's recent ratings.
type RatedNight struct {
	Date   schedule.Date
	Colors []schedule.ColorKey
	Cause  string
	Rating int
}

// Stats is the owner's log, for the log view.
type Stats struct {
	Seen      int
	Rated     int
	AvgRating float64
	Favorites []ColorCount // most seen first
	Recent    []RatedNight // rated nights, newest first
	Nights    []RatedNight // seen nights (rated or not), newest first
}

// Data is everything a view draws from. Nights should cover what the views
// need (tonight through six nights on, and tonight's whole month), with
// unlisted nights already filled in; a night that's missing is drawn white.
type Data struct {
	Tonight   schedule.Date
	Nights    []schedule.Lighting
	Sightings map[schedule.Date]Sighting
	Stats     *Stats

	BaseURL   string // for the QR code to a night's page
	ClaimURL  string // setup view
	ClaimCode string

	// The frame's setup hotspot and its page, for the Wi-Fi view.
	APSSID     string
	APPassword string
	PortalURL  string
	UpdatedAt  time.Time // shown small on the tonight view

	StaleSince schedule.Date // when set, a banner says the schedule is old
	LowBattery bool

	Look Look // how the owner likes it drawn
}

// night returns the lighting for d, white when it's not in Nights.
func (d *Data) night(day schedule.Date) schedule.Lighting {
	for _, l := range d.Nights {
		if l.Date == day {
			return l
		}
	}
	return schedule.Unlisted(day)
}

func (d *Data) sighting(day schedule.Date) Sighting {
	if d.Sightings == nil {
		return Sighting{}
	}
	return d.Sightings[day]
}

// nightURL is the web page for a night, which the QR code opens.
func (d *Data) nightURL(day schedule.Date) string {
	base := strings.TrimRight(d.BaseURL, "/")
	if base == "" {
		base = "https://prulights.evanslack.dev"
	}
	return base + "/night?d=" + string(day)
}

// colorNames reads a night's colors: "Blue, Pink & Gold".
func colorNames(colors []schedule.ColorKey) string {
	names := make([]string, 0, len(colors))
	for _, k := range colors {
		if c, ok := schedule.Lookup(k); ok {
			names = append(names, c.Label)
		} else {
			names = append(names, string(k))
		}
	}
	switch len(names) {
	case 0:
		return "White"
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " & " + names[len(names)-1]
}

// causeText is what a night is for, with a line for plain white nights.
func causeText(l schedule.Lighting) string {
	if !l.Scheduled || l.Cause == "" {
		return "No special lighting tonight"
	}
	return "For " + l.Cause
}

// SampleData is a fixed week of real 31 Nights of Light colors around date,
// for previews and tests when there's no database to draw from.
func SampleData(tonight schedule.Date) Data {
	listed := []schedule.Lighting{
		sample("2026-12-01", "Life is Good Playmaker Project", schedule.Gold, schedule.Maroon),
		sample("2026-12-02", "Food Rescue", schedule.Orange),
		sample("2026-12-03", "Friends of the Public Garden", schedule.White, schedule.Green),
		sample("2026-12-04", "Hanukkah", schedule.Blue),
		sample("2026-12-05", "New England Donor Services", schedule.Blue, schedule.Green),
		sample("2026-12-06", "Associates of the Boston Public Library", schedule.Orange, schedule.Green),
		sample("2026-12-07", "Angel Flight", schedule.Blue, schedule.Gold),
		sample("2026-12-08", "Save the Bay / Save the Harbor", schedule.Blue, schedule.Teal, schedule.White),
		sample("2026-12-09", "Broken Tail rescue", schedule.Blue, schedule.White),
		sample("2026-12-10", "Dress for Success", schedule.Red),
		sample("2026-12-11", "Pawsitively 4Pink", schedule.White, schedule.Pink, schedule.Maroon),
		sample("2026-12-12", "The Gillian Remy Stepping Strong Center for Trauma Innovation", schedule.Blue, schedule.Pink, schedule.Gold),
		sample("2026-12-13", "Read to a Child Inc.", schedule.Blue, schedule.Orange, schedule.Green),
		sample("2026-12-14", "DEAF Inc.", schedule.Blue, schedule.White),
		sample("2026-12-15", "Cradles to Crayons", schedule.Purple),
		sample("2026-12-16", "IMPACT Melanoma", schedule.Blue, schedule.Gold),
		sample("2026-12-17", "Boston HEAT", schedule.Orange, schedule.Gold),
		sample("2026-12-18", "Smile Mass", schedule.Teal),
		sample("2026-12-19", "Association for Autism and Neurodiversity", schedule.Teal, schedule.Orange, schedule.Green),
		sample("2026-12-20", "Hamilton-Garrett Center for Music & Arts", schedule.Blue, schedule.Orange, schedule.Red),
		sample("2026-12-21", "Victory Programs", schedule.Red),
		sample("2026-12-22", "Cancer Education and Research Institute", schedule.Blue),
		sample("2026-12-23", "Three Squares New England", schedule.Teal, schedule.Gold),
		sample("2026-12-24", "NEADS Service Dogs", schedule.Blue, schedule.Red),
		sample("2026-12-25", "Christmas", schedule.Red, schedule.Green),
		sample("2026-12-26", "REACH Beyond Domestic Violence", schedule.Purple),
		sample("2026-12-27", "Behavioral Health Network", schedule.White, schedule.Green, schedule.Purple),
		sample("2026-12-28", "Epilepsy Foundation New England", schedule.Purple),
		sample("2026-12-29", "Zumix", schedule.Blue, schedule.Pink, schedule.Gold),
		sample("2026-12-30", "Massachusetts Society for the Prevention of Cruelty to Children", schedule.Purple),
		sample("2026-12-31", "New Year's Eve", schedule.Gold),
		sample("2026-11-11", "Veterans Day", schedule.Red, schedule.White, schedule.Blue),
		sample("2026-11-18", "National Injury Prevention", schedule.Green),
		sample("2026-11-26", "Thanksgiving", schedule.Orange, schedule.Gold),
		sample("2026-06-01", "LGBTQ+ Pride Month", schedule.Rainbow),
		sample("2026-06-06", "Boston Pride for the People Parade", schedule.Rainbow),
	}
	t := tonight.Time()
	from := schedule.Date(time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, schedule.Boston).Format("2006-01-02"))
	if back := tonight.AddDays(-7); back < from {
		from = back
	}
	to := tonight.AddDays(40)
	return Data{
		Tonight: tonight,
		Nights:  schedule.Fill(from, to, listed),
		Sightings: map[schedule.Date]Sighting{
			tonight.AddDays(-1): {Seen: true, Rating: 5},
			tonight.AddDays(-2): {Seen: true, Rating: 4},
			tonight.AddDays(-4): {Seen: true},
		},
		Stats: &Stats{
			Seen: 47, Rated: 38, AvgRating: 4.3,
			Favorites: []ColorCount{{schedule.Blue, 21}, {schedule.Gold, 12}, {schedule.Teal, 9}, {schedule.Purple, 7}},
			Recent: []RatedNight{
				{Date: "2026-12-11", Colors: []schedule.ColorKey{schedule.White, schedule.Pink, schedule.Maroon}, Cause: "Pawsitively 4Pink", Rating: 5},
				{Date: "2026-12-10", Colors: []schedule.ColorKey{schedule.Red}, Cause: "Dress for Success", Rating: 4},
				{Date: "2026-12-07", Colors: []schedule.ColorKey{schedule.Blue, schedule.Gold}, Cause: "Angel Flight", Rating: 5},
				{Date: "2026-12-04", Colors: []schedule.ColorKey{schedule.Blue}, Cause: "Hanukkah", Rating: 3},
			},
			Nights: []RatedNight{
				{Date: "2026-12-11", Colors: []schedule.ColorKey{schedule.White, schedule.Pink, schedule.Maroon}, Rating: 5},
				{Date: "2026-12-10", Colors: []schedule.ColorKey{schedule.Red}, Rating: 4},
				{Date: "2026-12-08", Colors: []schedule.ColorKey{schedule.Blue, schedule.Teal, schedule.White}},
				{Date: "2026-12-07", Colors: []schedule.ColorKey{schedule.Blue, schedule.Gold}, Rating: 5},
				{Date: "2026-12-05", Colors: []schedule.ColorKey{schedule.Blue, schedule.Green}},
				{Date: "2026-12-04", Colors: []schedule.ColorKey{schedule.Blue}, Rating: 3},
				{Date: "2026-12-01", Colors: []schedule.ColorKey{schedule.Gold, schedule.Maroon}, Rating: 4},
				{Date: "2026-11-26", Colors: []schedule.ColorKey{schedule.Orange, schedule.Gold}, Rating: 5},
			},
		},
		BaseURL:    "https://prulights.evanslack.dev",
		ClaimURL:   "https://prulights.evanslack.dev/claim?code=K7Q-9MX",
		ClaimCode:  "K7Q-9MX",
		APSSID:     "prulights-K7Q9",
		APPassword: "mxq4ad7tph",
		PortalURL:  "http://192.168.4.1",
		UpdatedAt:  time.Date(t.Year(), t.Month(), t.Day(), 12, 2, 0, 0, schedule.Boston),
	}
}

func sample(d schedule.Date, cause string, colors ...schedule.ColorKey) schedule.Lighting {
	l := schedule.Lighting{Date: d, Colors: colors, Cause: cause, Kind: schedule.Classify(cause),
		Program: schedule.ProgramNights, Scheduled: true}
	l.LightsOn, l.LightsOnSource = schedule.LightsOn(d, l.Program)
	return l
}

// dayLabel is "Thu Dec 10".
func dayLabel(d schedule.Date) string { return d.Time().Format("Mon Jan 2") }
