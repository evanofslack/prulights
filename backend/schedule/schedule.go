// Package schedule is the domain: a night's lighting, the palette, what
// "tonight" means in Boston, and when the lights come on.
package schedule

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Boston is the tower's time zone. Everything about "tonight" uses it.
var Boston = mustLoad("America/New_York")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("load %s: %v (is tzdata installed?)", name, err))
	}
	return loc
}

// RolloverHour is when "tonight" moves to the next day: until 4am the
// lighting you see is still last night's.
const RolloverHour = 4

// DefaultLightsOnHour is the lights-on hour (5pm) when nothing better is known.
const DefaultLightsOnHour = 17

const defaultLightsOn = DefaultLightsOnHour * time.Hour

// Date is a calendar date in Boston, formatted 2026-12-12.
type Date string

const dateLayout = "2006-01-02"

// ParseDate checks and normalizes a YYYY-MM-DD date.
func ParseDate(s string) (Date, error) {
	t, err := time.ParseInLocation(dateLayout, s, Boston)
	if err != nil {
		return "", fmt.Errorf("date %q: want YYYY-MM-DD", s)
	}
	return Date(t.Format(dateLayout)), nil
}

// DateOf is the calendar date of t in Boston.
func DateOf(t time.Time) Date { return Date(t.In(Boston).Format(dateLayout)) }

// Time is midnight at the start of the date, in Boston.
func (d Date) Time() time.Time {
	t, _ := time.ParseInLocation(dateLayout, string(d), Boston)
	return t
}

// AddDays moves the date by n days.
func (d Date) AddDays(n int) Date {
	t := d.Time()
	return Date(time.Date(t.Year(), t.Month(), t.Day()+n, 0, 0, 0, 0, Boston).Format(dateLayout))
}

// DaysUntil counts days from d to e (negative when e is earlier).
func (d Date) DaysUntil(e Date) int {
	a, b := d.Time(), e.Time()
	return int(time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC).
		Sub(time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)).Hours() / 24)
}

// Tonight is the date of the lighting showing (or about to show) at now:
// today's date, or yesterday's before RolloverHour.
func Tonight(now time.Time) Date {
	local := now.In(Boston)
	if local.Hour() < RolloverHour {
		local = local.AddDate(0, 0, -1)
	}
	return DateOf(local)
}

// Kind says what a lighting is for.
type Kind string

const (
	KindNonprofit Kind = "nonprofit"
	KindHoliday   Kind = "holiday"
	KindSports    Kind = "sports"
	KindCivic     Kind = "civic" // observances: heritage months, memorials, awareness
	KindDefault   Kind = "default"
)

// Program is which source a lighting came from.
type Program string

const (
	ProgramRegular  Program = "regular"  // the yearly tower lightings page
	ProgramNights   Program = "31nights" // 31 Nights of Light, every night in December
	ProgramOverride Program = "override" // an admin correction
	ProgramUnlisted Program = "default"  // nothing listed: the crown is white
)

// Where a lighting's LightsOn came from.
const (
	LightsOnSource   = "source"  // the source states the time (31 Nights: 5pm)
	LightsOnSunset   = "sunset"  // computed sunset at the tower
	LightsOnFallback = "default" // 5pm
)

// Lighting is one night of the crown.
type Lighting struct {
	Date           Date       `json:"date"`
	Colors         []ColorKey `json:"colors"`
	ColorsRaw      string     `json:"colors_raw,omitempty"`
	Cause          string     `json:"cause"`
	CauseURL       string     `json:"cause_url,omitempty"`
	Kind           Kind       `json:"kind"`
	Program        Program    `json:"program"`
	Scheduled      bool       `json:"scheduled"`
	SourceURL      string     `json:"source_url,omitempty"`
	LightsOn       time.Time  `json:"lights_on"`
	LightsOnSource string     `json:"lights_on_source"`
}

// Unlisted is the lighting for a night with nothing listed: plain white.
func Unlisted(d Date) Lighting {
	l := Lighting{Date: d, Colors: []ColorKey{White}, Kind: KindDefault, Program: ProgramUnlisted}
	l.LightsOn, l.LightsOnSource = LightsOn(d, l.Program)
	return l
}

// LightsOn is when the crown lights on a date: the time the source states
// (31 Nights of Light says 5pm), else sunset at the tower, else 5pm.
func LightsOn(d Date, p Program) (time.Time, string) {
	day := d.Time()
	if p == ProgramNights {
		return day.Add(defaultLightsOn), LightsOnSource
	}
	if t, ok := Sunset(day.Year(), day.Month(), day.Day(), TowerLat, TowerLon); ok {
		return t.In(Boston), LightsOnSunset
	}
	return day.Add(defaultLightsOn), LightsOnFallback
}

// Merge folds several listings for one date into one lighting: the colors
// of all of them in order, causes joined. The first one's program and kind
// win, except an override, which replaces everything.
func Merge(ls []Lighting) Lighting {
	if len(ls) == 0 {
		return Lighting{}
	}
	for _, l := range ls {
		if l.Program == ProgramOverride {
			return l
		}
	}
	out := ls[0]
	out.Colors = slices.Clone(out.Colors)
	for _, l := range ls[1:] {
		for _, c := range l.Colors {
			if !slices.Contains(out.Colors, c) {
				out.Colors = append(out.Colors, c)
			}
		}
		if l.Cause != "" && !strings.Contains(out.Cause, l.Cause) {
			out.Cause = strings.TrimSpace(out.Cause + " · " + l.Cause)
		}
		if out.CauseURL == "" {
			out.CauseURL = l.CauseURL
		}
	}
	return out
}

// Fill returns one lighting per night from from to to (inclusive), using
// the listed ones and white for the rest. Several listings on one night are
// merged. listed needn't be sorted.
func Fill(from, to Date, listed []Lighting) []Lighting {
	byDate := map[Date][]Lighting{}
	for _, l := range listed {
		byDate[l.Date] = append(byDate[l.Date], l)
	}
	var out []Lighting
	for d := from; d <= to; d = d.AddDays(1) {
		if ls, ok := byDate[d]; ok {
			out = append(out, Merge(ls))
		} else {
			out = append(out, Unlisted(d))
		}
	}
	return out
}

var kindPatterns = []struct {
	kind Kind
	re   *regexp.Regexp
}{
	{KindSports, regexp.MustCompile(`(?i)\b(red sox|celtics|bruins|patriots|revolution|legacy fc|fifa|world cup|olympi|paralympi|marathon|regatta|head of the charles|super ?bowl|stanley cup|world series|nba finals|home opener|opening home game|jersey retirement|championship)\b`)},
	{KindHoliday, regexp.MustCompile(`(?i)\b(christmas|hanukkah|chanukah|kwanzaa|thanksgiving|new year|halloween|valentine|st\.? patrick|independence day|fourth of july|easter|passover|diwali|lunar new year|eid|rosh hashanah)\b`)},
	{KindCivic, regexp.MustCompile(`(?i)\b(memorial day|veterans day|labor day|president'?s'? day|juneteenth|martin luther king|mlk|indigenous peoples|columbus day|9/11|september 11|patriots'? day|history month|heritage month|pride|awareness (day|week|month)|world [a-z]+ day|national [a-z ]+ day)\b`)},
}

// Classify guesses a lighting's kind from its cause. Holidays and sports are
// named; observances are civic; everything else is a nonprofit partner.
func Classify(cause string) Kind {
	for _, p := range kindPatterns {
		if p.re.MatchString(cause) {
			return p.kind
		}
	}
	return KindNonprofit
}
