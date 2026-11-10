package schedule

import (
	"slices"
	"testing"
	"time"
)

func TestParseColors(t *testing.T) {
	cases := []struct {
		raw     string
		want    []ColorKey
		unknown []string
	}{
		{"Red", []ColorKey{Red}, nil},
		{"Blue & Gold", []ColorKey{Blue, Gold}, nil},
		{"Blue, Pink, & Gold", []ColorKey{Blue, Pink, Gold}, nil},
		{"Red, White and Blue", []ColorKey{Red, White, Blue}, nil},
		{"Orange and Gold", []ColorKey{Orange, Gold}, nil},
		{"Green, Gold, White & Black", []ColorKey{Green, Gold, White, Black}, nil},
		{"Red, Silver & Blue", []ColorKey{Red, Silver, Blue}, nil},
		{"Patriotic", []ColorKey{Red, White, Blue}, nil},
		{"Rainbow", []ColorKey{Rainbow}, nil},
		{"Teal, Orange & Green", []ColorKey{Teal, Orange, Green}, nil},
		{"Navy Blue / White", []ColorKey{Blue, White}, nil},
		{"Blue & Blue", []ColorKey{Blue}, nil},
		{"Chartreuse & Blue", []ColorKey{Blue}, []string{"chartreuse"}},
		{"", nil, nil},
	}
	for _, c := range cases {
		got, unknown := ParseColors(c.raw)
		if !slices.Equal(got, c.want) || !slices.Equal(unknown, c.unknown) {
			t.Errorf("ParseColors(%q) = %v, %v; want %v, %v", c.raw, got, unknown, c.want, c.unknown)
		}
	}
}

func TestPaletteComplete(t *testing.T) {
	for _, c := range Palette() {
		if c.Hex == "" || c.Label == "" || c.Ink.A == "" {
			t.Errorf("%s is missing a field: %+v", c.Key, c)
		}
		for _, b := range c.Bands {
			if _, ok := Lookup(b); !ok {
				t.Errorf("%s band %s is not in the palette", c.Key, b)
			}
		}
		if c.Ink.Mix < 0 || c.Ink.Mix > 1 || (c.Ink.Mix > 0) != (c.Ink.B != "") {
			t.Errorf("%s has a bad ink recipe %+v", c.Key, c.Ink)
		}
	}
	for word, keys := range aliases {
		for _, k := range keys {
			if _, ok := Lookup(k); !ok {
				t.Errorf("alias %q points at unknown %s", word, k)
			}
		}
	}
}

func boston(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, Boston)
}

func TestTonight(t *testing.T) {
	cases := []struct {
		now  time.Time
		want Date
	}{
		{boston(2026, 12, 12, 19, 0), "2026-12-12"},
		{boston(2026, 12, 13, 1, 30), "2026-12-12"}, // still last night
		{boston(2026, 12, 13, 3, 59), "2026-12-12"},
		{boston(2026, 12, 13, 4, 0), "2026-12-13"},
		{boston(2027, 1, 1, 0, 30), "2026-12-31"},                     // new year's eve, after midnight
		{time.Date(2026, 12, 13, 5, 0, 0, 0, time.UTC), "2026-12-12"}, // midnight in Boston
	}
	for _, c := range cases {
		if got := Tonight(c.now); got != c.want {
			t.Errorf("Tonight(%v) = %s, want %s", c.now, got, c.want)
		}
	}
}

func TestDates(t *testing.T) {
	if _, err := ParseDate("2026-13-01"); err == nil {
		t.Error("month 13 parsed")
	}
	d, err := ParseDate("2026-12-31")
	if err != nil || d.AddDays(1) != "2027-01-01" || d.AddDays(-31) != "2026-11-30" {
		t.Errorf("AddDays: %s %v", d, err)
	}
	// across the daylight saving change
	if n := Date("2026-11-01").DaysUntil("2026-11-02"); n != 1 {
		t.Errorf("DaysUntil across DST = %d", n)
	}
	if n := Date("2026-03-07").DaysUntil("2026-03-09"); n != 2 {
		t.Errorf("DaysUntil across DST = %d", n)
	}
	if n := Date("2026-12-12").DaysUntil("2026-12-01"); n != -11 {
		t.Errorf("DaysUntil backwards = %d", n)
	}
}

// Boston sunsets from the US Naval Observatory tables, local time.
func TestSunset(t *testing.T) {
	cases := []struct {
		date       Date
		hour, mins int
	}{
		{"2026-12-12", 16, 12},
		{"2026-06-21", 20, 25},
		{"2026-03-20", 19, 0},
		{"2026-09-22", 18, 43},
		{"2026-11-08", 16, 29},
	}
	for _, c := range cases {
		day := c.date.Time()
		got, ok := Sunset(day.Year(), day.Month(), day.Day(), TowerLat, TowerLon)
		if !ok {
			t.Fatalf("%s: no sunset", c.date)
		}
		want := boston(day.Year(), day.Month(), day.Day(), c.hour, c.mins)
		if diff := got.Sub(want); diff < -3*time.Minute || diff > 3*time.Minute {
			t.Errorf("%s: sunset %s, want about %s", c.date, got.In(Boston).Format("15:04"), want.Format("15:04"))
		}
	}
	// no sunset at the pole in midsummer
	if _, ok := Sunset(2026, 6, 21, 89, 0); ok {
		t.Error("expected no sunset at the north pole in June")
	}
}

// Boston sunrises, from the same tables.
func TestSunrise(t *testing.T) {
	cases := []struct {
		date       Date
		hour, mins int
	}{
		{"2026-12-12", 7, 4},
		{"2026-06-21", 5, 7},
		{"2026-03-20", 6, 51},
	}
	for _, c := range cases {
		day := c.date.Time()
		got, ok := Sunrise(day.Year(), day.Month(), day.Day(), TowerLat, TowerLon)
		want := boston(day.Year(), day.Month(), day.Day(), c.hour, c.mins)
		if diff := got.Sub(want); !ok || diff < -3*time.Minute || diff > 3*time.Minute {
			t.Errorf("%s: sunrise %s, want about %s", c.date, got.In(Boston).Format("15:04"), want.Format("15:04"))
		}
	}
}

func TestLightsOn(t *testing.T) {
	at, src := LightsOn("2026-12-10", ProgramNights)
	if src != LightsOnSource || !at.Equal(boston(2026, 12, 10, 17, 0)) {
		t.Errorf("31 nights = %v %s, want 17:00 source", at, src)
	}
	at, src = LightsOn("2026-11-11", ProgramRegular)
	if src != LightsOnSunset || at.Hour() != 16 {
		t.Errorf("veterans day = %v %s, want a sunset around 16:2x", at, src)
	}
	if at.Location() != Boston {
		t.Errorf("lights on should be in Boston time, got %v", at.Location())
	}
}

func TestFill(t *testing.T) {
	listed := []Lighting{
		{Date: "2026-12-03", Colors: []ColorKey{White, Green}, Cause: "Friends of the Public Garden", Program: ProgramNights, Scheduled: true},
		{Date: "2026-12-01", Colors: []ColorKey{Gold, Maroon}, Cause: "Life is Good", Program: ProgramNights, Scheduled: true},
		{Date: "2026-12-01", Colors: []ColorKey{Red, Gold}, Cause: "Second listing", Program: ProgramRegular, Scheduled: true},
	}
	got := Fill("2026-11-30", "2026-12-03", listed)
	if len(got) != 4 {
		t.Fatalf("got %d nights, want 4", len(got))
	}
	if got[0].Scheduled || got[0].Kind != KindDefault || !slices.Equal(got[0].Colors, []ColorKey{White}) {
		t.Errorf("unlisted night = %+v", got[0])
	}
	if got[0].LightsOnSource != LightsOnSunset {
		t.Errorf("unlisted night lights on from %s", got[0].LightsOnSource)
	}
	if !slices.Equal(got[1].Colors, []ColorKey{Gold, Maroon, Red}) || got[1].Cause != "Life is Good · Second listing" {
		t.Errorf("merged night = %+v", got[1])
	}
	if got[2].Program != ProgramUnlisted || got[3].Cause != "Friends of the Public Garden" {
		t.Errorf("nights 2-3 = %+v %+v", got[2], got[3])
	}
}

func TestMergeOverride(t *testing.T) {
	got := Merge([]Lighting{
		{Colors: []ColorKey{Red}, Cause: "scraped", Program: ProgramRegular},
		{Colors: []ColorKey{Blue}, Cause: "fixed", Program: ProgramOverride},
	})
	if got.Cause != "fixed" || !slices.Equal(got.Colors, []ColorKey{Blue}) {
		t.Errorf("override should win, got %+v", got)
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]Kind{
		"Veterans Day":                         KindCivic,
		"Thanksgiving":                         KindHoliday,
		"Christmas":                            KindHoliday,
		"New Year's Eve":                       KindHoliday,
		"Celtics Opening Home Game 2026":       KindSports,
		"130th Boston Marathon":                KindSports,
		"61st Head of the Charles Regatta":     KindSports,
		"LGBTQ+ Pride Month":                   KindCivic,
		"Hispanic/Latino Heritage Month":       KindCivic,
		"Dress for Success":                    KindNonprofit,
		"Alzheimer's Foundation of America":    KindNonprofit,
		"Massachusetts Society for Prevention": KindNonprofit,
		"Women's History Month":                KindCivic,
		"Martin Luther King Jr. Day":           KindCivic,
		"Patriots, Super Bowl":                 KindSports,
		"President’s Day":                      KindCivic,
		"Flag Day":                             KindCivic,
		"President’ Day":                       KindCivic,
		"Our Returning Team USA Olympians":     KindSports,
	}
	for cause, want := range cases {
		if got := Classify(cause); got != want {
			t.Errorf("Classify(%q) = %s, want %s", cause, got, want)
		}
	}
}
