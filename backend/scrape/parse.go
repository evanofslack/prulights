// Package scrape reads the Prudential Center's tower lighting pages.
package scrape

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/evanofslack/prulights/schedule"
)

// Page is one parsed lighting page.
type Page struct {
	URL       string              `json:"url"`
	Title     string              `json:"title"`
	Year      int                 `json:"year"`
	Program   schedule.Program    `json:"program"`
	Lightings []schedule.Lighting `json:"lightings"`
	Warnings  []string            `json:"warnings,omitempty"`
}

// entry is one lighting item's text, before it's expanded to nights.
type entry struct {
	text     string
	causeURL string
}

var (
	months   = `January|February|March|April|May|June|July|August|September|October|November|December`
	dateRe   = regexp.MustCompile(`^(?i)(?:(?:mon|tues|wednes|thurs|fri|satur|sun)day,?\s+)?(` + months + `)\.?\s+(\d{1,2})(?:st|nd|rd|th)?(?:,\s*(\d{4}))?(?:\s*[-–—]\s*(?:(` + months + `)\.?\s+)?(\d{1,2})(?:st|nd|rd|th)?)?(?:,\s*(\d{4}))?$`)
	itemRe   = regexp.MustCompile(`^(?i)(?:(?:mon|tues|wednes|thurs|fri|satur|sun)day,?\s+)?(?:` + months + `)\b`)
	yearRe   = regexp.MustCompile(`\b(20\d\d)\b`)
	schedRe  = regexp.MustCompile(`(?i)\b(20\d\d)\s+(?:lighting schedule|lighting partners|tower lighting|upcoming lightings)`)
	spacesRe = regexp.MustCompile(`\s+`)
)

// connectors are the phrases between a lighting's colors and its cause
// ("Purple for March of Dimes", "Blue in Support of …"). They're removed from
// the start of the cause, longest first.
var connectors = []string{
	"in recognition in honor of", "in partnership with", "in commemoration of", "in celebration of",
	"in recognition of", "in support of", "in honor of", "in memory of", "for celebration of",
	"to celebrate", "to honor", "to support", "to recognize", "celebrating", "honoring", "supporting",
	"recognizing", "in support", "and", "for", "of",
}

// ParsePage reads a tower lighting page: the yearly schedule
// (<li><strong>October 1</strong> | Pink for the Ellie Fund</li>) or 31 Nights
// of Light (<li>December 10 | <a href>Dress for Success</a> | Red</li>).
// Items it can't read become warnings, never errors; an error means the page
// isn't a lighting page at all.
func ParsePage(url string, body []byte, lastMod time.Time) (Page, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return Page{}, err
	}
	p := Page{URL: url}
	// the head's <title> ("2026 Tower Lightings - Prudential Center"); the
	// svg icons later in the page have titles too, so take the first
	p.Title = clean(doc.Find("head title").First().Text())
	p.Title = strings.TrimSpace(strings.TrimSuffix(p.Title, "- Prudential Center"))
	lowerTitle := strings.ToLower(p.Title + " " + url)
	switch {
	case strings.Contains(lowerTitle, "nights of light") || strings.Contains(lowerTitle, "nights-of-light"):
		p.Program = schedule.ProgramNights
	case strings.Contains(lowerTitle, "tower light"):
		p.Program = schedule.ProgramRegular
	default:
		return p, fmt.Errorf("%s: not a lighting page (title %q)", url, p.Title)
	}
	p.Year = pageYear(doc, p.Title, lastMod)
	if p.Year == 0 {
		return p, fmt.Errorf("%s: can't tell the page's year", url)
	}

	var items []entry
	doc.Find("li").Each(func(_ int, li *goquery.Selection) {
		if li.Find("li").Length() > 0 {
			return // a wrapper around the real items
		}
		text := clean(li.Text())
		if !strings.Contains(text, "|") || !itemRe.MatchString(text) {
			return
		}
		e := entry{text: text}
		if href, ok := li.Find("a[href]").First().Attr("href"); ok {
			e.causeURL = strings.TrimSpace(href)
		}
		items = append(items, e)
	})

	for _, e := range items {
		ls, warn := p.parseEntry(e)
		if warn != "" {
			p.Warnings = append(p.Warnings, warn)
		}
		p.Lightings = append(p.Lightings, ls...)
	}
	return p, nil
}

// pageYear finds the year the page's dates are in: the title ("2026 Tower
// Lightings"), else a "2026 Lighting Schedule" line in the text, else the year
// the page was last modified.
func pageYear(doc *goquery.Document, title string, lastMod time.Time) int {
	if m := yearRe.FindStringSubmatch(title); m != nil {
		y, _ := strconv.Atoi(m[1])
		return y
	}
	body := clean(doc.Find("main, article, .entry-content, body").First().Text())
	if m := schedRe.FindStringSubmatch(body); m != nil {
		y, _ := strconv.Atoi(m[1])
		return y
	}
	if !lastMod.IsZero() {
		return lastMod.In(schedule.Boston).Year()
	}
	return 0
}

// parseEntry turns one item into its nights (several for a range like
// "April 18–20").
func (p *Page) parseEntry(e entry) ([]schedule.Lighting, string) {
	parts := strings.Split(e.text, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	dates, err := parseDates(parts[0], p.Year)
	if err != nil {
		return nil, fmt.Sprintf("%q: %v", e.text, err)
	}

	colorsRaw, cause := findColors(parts[1:])
	colors, unknown := schedule.ParseColors(colorsRaw)
	if len(colors) == 0 {
		return nil, fmt.Sprintf("%q: no colors", e.text)
	}
	var warn string
	if len(unknown) > 0 {
		warn = fmt.Sprintf("%q: unknown colors %q", e.text, unknown)
	}
	cause = trimCause(cause)

	out := make([]schedule.Lighting, 0, len(dates))
	for _, d := range dates {
		l := schedule.Lighting{
			Date:      d,
			Colors:    colors,
			ColorsRaw: colorsRaw,
			Cause:     cause,
			CauseURL:  normalizeURL(e.causeURL),
			Kind:      schedule.Classify(cause),
			Program:   p.Program,
			Scheduled: true,
			SourceURL: p.URL,
		}
		out = append(out, l)
	}
	return out, warn
}

// findColors finds the colors in the parts of an item after its date, which
// the source writes several ways: "colors for cause" (yearly page),
// "cause | colors" (31 Nights), "colors | cause", and "cause colors" when a
// separator is missing. A part that is only colors wins; otherwise the
// colors are the leading words, else the trailing words, of the text.
func findColors(parts []string) (colorsRaw, cause string) {
	if len(parts) > 1 {
		for i, p := range parts {
			if keys, unknown := schedule.ParseColors(p); len(keys) > 0 && len(unknown) == 0 {
				rest := append(append([]string{}, parts[:i]...), parts[i+1:]...)
				return p, strings.Join(rest, " | ")
			}
		}
	}
	text := strings.Join(parts, " ")
	if colors, rest := splitColors(text); colors != "" {
		return colors, rest
	}
	return trailingColors(text)
}

// trailingColors splits "IMPACT Melanoma Blue & Gold" into the cause and its
// trailing run of colors.
func trailingColors(s string) (colors, rest string) {
	words := strings.Fields(s)
	for start := 0; start < len(words); start++ {
		if c, r := splitColors(strings.Join(words[start:], " ")); c != "" && r == "" {
			return c, strings.Join(words[:start], " ")
		}
	}
	return "", s
}

// splitColors splits "Blue, Pink & Gold for the Ellie Fund" into its leading
// run of colors and the rest. Colors are single or two-word palette words
// joined by commas, "&", "and" or "/".
func splitColors(s string) (colors, rest string) {
	words := strings.Fields(s)
	end := 0 // words[:end] are the colors
	i := 0
	for i < len(words) {
		w := strings.Trim(words[i], ",;:-–—")
		lw := strings.ToLower(w)
		switch {
		case lw == "&" || lw == "and" || lw == "/" || lw == "+" || lw == "":
			i++
			continue
		case i+1 < len(words) && schedule.IsColorWord(w+" "+strings.Trim(words[i+1], ",;:-–—")):
			i += 2
		case schedule.IsColorWord(w):
			i++
		default:
			return strings.Join(words[:end], " "), strings.Join(words[end:], " ")
		}
		end = i
	}
	return strings.Join(words[:end], " "), strings.Join(words[end:], " ")
}

// trimCause drops the connecting phrase ("in Support of the") and trailing
// punctuation from a cause.
func trimCause(s string) string {
	s = strings.TrimSpace(s)
	for {
		lower := strings.ToLower(s)
		trimmed := false
		for _, c := range connectors {
			if strings.HasPrefix(lower, c+" ") {
				s = strings.TrimSpace(s[len(c):])
				trimmed = true
				break
			}
		}
		if !trimmed {
			break
		}
	}
	if strings.HasPrefix(strings.ToLower(s), "the ") {
		s = s[4:]
	}
	return strings.Trim(s, " .,;:-–—")
}

// parseDates reads "October 1", "April 13-15", "April 30 – May 2" or
// "December 31, 2026" into the nights it covers.
func parseDates(s string, year int) ([]schedule.Date, error) {
	m := dateRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return nil, fmt.Errorf("can't read date %q", s)
	}
	y := year
	if m[3] != "" {
		y, _ = strconv.Atoi(m[3])
	} else if m[6] != "" {
		y, _ = strconv.Atoi(m[6])
	}
	start, err := mkDate(y, m[1], m[2])
	if err != nil {
		return nil, err
	}
	if m[5] == "" {
		return []schedule.Date{start}, nil
	}
	endMonth := m[4]
	if endMonth == "" {
		endMonth = m[1]
	}
	end, err := mkDate(y, endMonth, m[5])
	if err != nil {
		return nil, err
	}
	if end < start {
		// a range into the next year ("December 30 – January 2")
		t := end.Time()
		end = schedule.DateOf(time.Date(t.Year()+1, t.Month(), t.Day(), 12, 0, 0, 0, schedule.Boston))
	}
	n := start.DaysUntil(end)
	if n > 31 {
		return nil, fmt.Errorf("range %q is %d days long", s, n+1)
	}
	out := make([]schedule.Date, 0, n+1)
	for d := start; d <= end; d = d.AddDays(1) {
		out = append(out, d)
	}
	return out, nil
}

func mkDate(year int, month, day string) (schedule.Date, error) {
	t, err := time.Parse("January 2 2006", fmt.Sprintf("%s %s %d", titleCase(month), day, year))
	if err != nil {
		return "", fmt.Errorf("bad date %s %s %d", month, day, year)
	}
	return schedule.Date(t.Format("2006-01-02")), nil
}

func titleCase(s string) string {
	s = strings.ToLower(s)
	return strings.ToUpper(s[:1]) + s[1:]
}

var (
	openParenRe  = regexp.MustCompile(`\(\s+`)
	closeParenRe = regexp.MustCompile(`\s+\)`)
)

// clean collapses whitespace (and non-breaking spaces) in a text node.
func clean(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = spacesRe.ReplaceAllString(s, " ")
	s = closeParenRe.ReplaceAllString(openParenRe.ReplaceAllString(s, "("), ")")
	return strings.TrimSpace(s)
}

// trackingParams are query parameters left on links by ad campaigns.
var trackingParams = []string{"gclid", "gclsrc", "gad_source", "gbraid", "wbraid", "fbclid", "msclkid", "srccode"}

// normalizeURL tidies a cause link: lowercases the scheme and host, adds
// https to bare hosts, and drops campaign tracking parameters. Empty or
// unparseable stays as it was.
func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	q := u.Query()
	for k := range q {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "utm_") || strings.HasPrefix(lk, "donationtrackingparam") || slices.Contains(trackingParams, lk) {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
