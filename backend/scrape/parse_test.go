package scrape

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// The saved pages, parsed, must match their golden JSON. After a real change
// to the parser, review the diff and run: go test ./scrape -update
func TestParseGolden(t *testing.T) {
	pages := []struct {
		file, url string
		year      int
		program   schedule.Program
		count     int
	}{
		{"2024-tower-lighting-schedule", "https://www.prudentialcenter.com/experiences/events/2024-tower-lighting-schedule/", 2024, schedule.ProgramRegular, 62},
		{"2026-tower-lightings", "https://www.prudentialcenter.com/experiences/events/2026-tower-lightings/", 2026, schedule.ProgramRegular, 77},
		{"31-nights-of-light", "https://www.prudentialcenter.com/experiences/events/31-nights-of-light/", 2026, schedule.ProgramNights, 31},
	}
	for _, pg := range pages {
		t.Run(pg.file, func(t *testing.T) {
			body, err := os.ReadFile("testdata/" + pg.file + ".html")
			if err != nil {
				t.Fatal(err)
			}
			p, err := ParsePage(pg.url, body, time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			if p.Year != pg.year || p.Program != pg.program || len(p.Lightings) != pg.count {
				t.Errorf("year %d program %s count %d; want %d %s %d", p.Year, p.Program, len(p.Lightings), pg.year, pg.program, pg.count)
			}
			if len(p.Warnings) > 0 {
				t.Errorf("warnings: %q", p.Warnings)
			}
			got, _ := json.MarshalIndent(p, "", "  ")
			golden := "testdata/" + pg.file + ".golden.json"
			if *update {
				os.WriteFile(golden, append(got, '\n'), 0o644)
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run go test ./scrape -update)", err)
			}
			if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
				t.Errorf("%s differs from the parse; review and run go test ./scrape -update", golden)
			}
		})
	}
}

func TestParseSpotChecks(t *testing.T) {
	body, _ := os.ReadFile("testdata/31-nights-of-light.html")
	p, err := ParsePage("https://www.prudentialcenter.com/experiences/events/31-nights-of-light/", body, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	byDate := map[schedule.Date]schedule.Lighting{}
	for _, l := range p.Lightings {
		byDate[l.Date] = l
	}
	check := func(d schedule.Date, colors []schedule.ColorKey, cause string) {
		t.Helper()
		l := byDate[d]
		if !slices.Equal(l.Colors, colors) || l.Cause != cause {
			t.Errorf("%s = %v %q; want %v %q", d, l.Colors, l.Cause, colors, cause)
		}
	}
	check("2026-12-10", []schedule.ColorKey{schedule.Red}, "Dress for Success")
	// the page leaves out the separator before the colors
	check("2026-12-16", []schedule.ColorKey{schedule.Blue, schedule.Gold}, "IMPACT Melanoma")
	// and swaps the colors and the cause
	check("2026-12-30", []schedule.ColorKey{schedule.Purple}, "Massachusetts Society for the Prevention of Cruelty to Children")
	check("2026-12-25", []schedule.ColorKey{schedule.Red, schedule.Green}, "Christmas")
	if byDate["2026-12-25"].Kind != schedule.KindHoliday {
		t.Errorf("christmas kind = %s", byDate["2026-12-25"].Kind)
	}
	if u := byDate["2026-12-10"].CauseURL; u != "http://boston.dressforsuccess.org" {
		t.Errorf("cause url = %q", u)
	}
}

func TestParseDates(t *testing.T) {
	cases := []struct {
		in   string
		want []schedule.Date
	}{
		{"October 1", []schedule.Date{"2026-10-01"}},
		{"April 18–20", []schedule.Date{"2026-04-18", "2026-04-19", "2026-04-20"}},
		{"April 13-15", []schedule.Date{"2026-04-13", "2026-04-14", "2026-04-15"}},
		{"April 30 – May 2", []schedule.Date{"2026-04-30", "2026-05-01", "2026-05-02"}},
		{"December 31 – January 1", []schedule.Date{"2026-12-31", "2027-01-01"}},
		{"Friday, October 3", []schedule.Date{"2026-10-03"}},
		{"January 2, 2027", []schedule.Date{"2027-01-02"}},
		{"Sept. 9", nil},
		{"February 30", nil},
	}
	for _, c := range cases {
		got, err := parseDates(c.in, 2026)
		if !slices.Equal(got, c.want) || (c.want == nil) != (err != nil) {
			t.Errorf("parseDates(%q) = %v, %v; want %v", c.in, got, err, c.want)
		}
	}
}

func TestFindColors(t *testing.T) {
	cases := []struct {
		parts         []string
		colors, cause string
	}{
		{[]string{"Pink for the Ellie Fund"}, "Pink", "for the Ellie Fund"},
		{[]string{"Red, White and Blue in Recognition in Honor of Memorial Day"}, "Red, White and Blue", "in Recognition in Honor of Memorial Day"},
		{[]string{"Green and GO C’s on the north side for the Celtics"}, "Green", "and GO C’s on the north side for the Celtics"},
		{[]string{"Blue- 9/11 Memorial"}, "Blue-", "9/11 Memorial"},
		{[]string{"Navy Blue & Gold for Angel Flight"}, "Navy Blue & Gold", "for Angel Flight"},
		{[]string{"Dress for Success", "Red"}, "Red", "Dress for Success"},
		{[]string{"PURPLE", "MSPCC"}, "PURPLE", "MSPCC"},
		{[]string{"IMPACT Melanoma Blue & Gold"}, "Blue & Gold", "IMPACT Melanoma"},
		{[]string{"Team Impact"}, "", "Team Impact"},
	}
	for _, c := range cases {
		colors, cause := findColors(c.parts)
		if colors != c.colors || cause != c.cause {
			t.Errorf("findColors(%q) = %q, %q; want %q, %q", c.parts, colors, cause, c.colors, c.cause)
		}
	}
}

func TestTrimCause(t *testing.T) {
	cases := map[string]string{
		"for the Ellie Fund":                          "Ellie Fund",
		"in Recognition in Honor of Memorial Day":     "Memorial Day",
		"in Partnership with The Fab Foundation":      "Fab Foundation",
		"to celebrate the NWSL BOS Nation FC launch.": "NWSL BOS Nation FC launch",
		"in Commemoration of September 11, 2001.":     "September 11, 2001",
		"Halloween": "Halloween",
	}
	for in, want := range cases {
		if got := trimCause(in); got != want {
			t.Errorf("trimCause(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"":                             "",
		"http://Bwhsteppingstrong.org": "http://bwhsteppingstrong.org",
		"www.zumix.org":                "https://www.zumix.org",
		"https://x.org/a?form=d&utm_source=g&gclid=1": "https://x.org/a?form=d",
	}
	for in, want := range cases {
		if got := normalizeURL(in); got != want {
			t.Errorf("normalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNotALightingPage(t *testing.T) {
	_, err := ParsePage("https://www.prudentialcenter.com/shop/x/", []byte("<html><head><title>Shop - Prudential Center</title></head></html>"), time.Time{})
	if err == nil || !strings.Contains(err.Error(), "not a lighting page") {
		t.Errorf("err = %v", err)
	}
}
