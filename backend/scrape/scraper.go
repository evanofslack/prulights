package scrape

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/evanofslack/prulights/metrics"
	"github.com/evanofslack/prulights/schedule"
	"github.com/evanofslack/prulights/store"
)

const (
	// Site is where the schedule is published.
	Site = "https://www.prudentialcenter.com"
	// SitemapPath lists the site's event pages with their last-modified times.
	SitemapPath = "/event-sitemap.xml"
	// UserAgent says who we are, with a link, no email.
	UserAgent = "prulights/1.0 (+https://prulights.evanslack.dev)"
	// CrawlDelay is what the site's robots.txt asks for between requests.
	CrawlDelay = 10 * time.Second
	// maxBody caps a page download.
	maxBody = 8 << 20
)

// lightingPageRe picks the tower lighting pages out of the sitemap.
var lightingPageRe = regexp.MustCompile(`/experiences/events/[^/]*(tower-light|nights-of-light)[^/]*/?$`)

// Seeds are always checked, in case the sitemap is down or misses a page.
var Seeds = []string{
	Site + "/experiences/events/31-nights-of-light/",
}

// Scraper keeps the stored schedule in step with the site.
type Scraper struct {
	Store  *store.Store
	Client *http.Client
	Site   string        // Site, or a test server
	Delay  time.Duration // between requests; CrawlDelay
	Now    func() time.Time
}

// New makes a scraper for the real site.
func New(st *store.Store) *Scraper {
	return &Scraper{
		Store:  st,
		Client: &http.Client{Timeout: 30 * time.Second},
		Site:   Site,
		Delay:  CrawlDelay,
		Now:    time.Now,
	}
}

// Options change a run.
type Options struct {
	Force  bool     // fetch every page, ignoring lastmod and etags
	DryRun bool     // parse but don't write anything
	URLs   []string // check only these pages instead of the sitemap
}

// Result is a run and the pages it parsed.
type Result struct {
	Run   store.ScrapeRun
	Pages []Page
}

// Run checks the sitemap and fetches the lighting pages that changed since
// the last run, applying what they list. A page that fails, or parses to far
// fewer nights than it had, is skipped with a warning and its last good data
// kept. The returned error is only for a run that couldn't do anything.
func (s *Scraper) Run(ctx context.Context, opt Options) (*Result, error) {
	res := &Result{Run: store.ScrapeRun{StartedAt: s.Now()}}
	run := &res.Run
	warn := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		slog.Warn("scrape", "warning", msg)
		run.Warnings = append(run.Warnings, msg)
	}

	type candidate struct{ url, lastmod string }
	var pages []candidate
	if len(opt.URLs) > 0 {
		for _, u := range opt.URLs {
			pages = append(pages, candidate{url: u})
		}
	} else {
		entries, err := s.sitemap(ctx)
		if err != nil {
			warn("sitemap: %v", err)
		}
		for _, e := range entries {
			if lightingPageRe.MatchString(e.Loc) {
				pages = append(pages, candidate{url: e.Loc, lastmod: e.LastMod})
			}
		}
		for _, seed := range s.seeds() {
			if !slices.ContainsFunc(pages, func(c candidate) bool { return c.url == seed }) {
				pages = append(pages, candidate{url: seed})
			}
		}
	}
	run.Pages = len(pages)

	var fetchErrs int
	for i, c := range pages {
		if i > 0 && run.Fetched > 0 {
			if err := sleep(ctx, s.Delay); err != nil {
				return s.finish(ctx, res, opt, err)
			}
		}
		page, applied, err := s.page(ctx, c.url, c.lastmod, opt, warn)
		if err != nil {
			fetchErrs++
			warn("%s: %v", c.url, err)
			continue
		}
		if page == nil {
			continue
		}
		run.Fetched++
		run.Items += len(page.Lightings)
		run.Added += applied.Added
		run.Changed += applied.Changed
		run.Removed += applied.Removed
		res.Pages = append(res.Pages, *page)
	}
	var err error
	if len(pages) == 0 {
		err = errors.New("no lighting pages found")
	} else if fetchErrs == len(pages) {
		err = errors.New("every page failed")
	}
	return s.finish(ctx, res, opt, err)
}

// seeds are the built-in pages, pointed at the configured site.
func (s *Scraper) seeds() []string {
	out := []string{}
	for _, u := range Seeds {
		out = append(out, strings.Replace(u, Site, s.Site, 1))
	}
	return out
}

func (s *Scraper) finish(ctx context.Context, res *Result, opt Options, err error) (*Result, error) {
	run := &res.Run
	run.FinishedAt = s.Now()
	if err != nil {
		run.Error = err.Error()
		metrics.ScrapeRuns.WithLabelValues("error").Inc()
	} else {
		metrics.ScrapeRuns.WithLabelValues("ok").Inc()
		metrics.ScrapeLastSuccess.Set(float64(run.FinishedAt.Unix()))
	}
	metrics.ScrapeWarnings.Add(float64(len(run.Warnings)))
	if !opt.DryRun {
		if serr := s.Store.SaveRun(context.WithoutCancel(ctx), run); serr != nil && err == nil {
			err = serr
		}
		if d, derr := s.Store.PublishedThrough(ctx); derr == nil && d != "" {
			metrics.ScheduleNights.Set(float64(schedule.Tonight(s.Now()).DaysUntil(d)))
		}
	}
	slog.Info("scrape finished", "pages", run.Pages, "fetched", run.Fetched, "items", run.Items,
		"added", run.Added, "changed", run.Changed, "removed", run.Removed, "warnings", len(run.Warnings), "error", run.Error)
	return res, err
}

// page checks one page. It returns a nil page when the page hasn't changed.
func (s *Scraper) page(ctx context.Context, url, lastmod string, opt Options, warn func(string, ...any)) (*Page, store.Applied, error) {
	src, known, err := s.Store.Source(ctx, url)
	if err != nil {
		return nil, store.Applied{}, err
	}
	if !opt.Force && known && lastmod != "" && src.LastMod == lastmod && src.Items > 0 {
		metrics.ScrapeFetches.WithLabelValues("skipped").Inc()
		return nil, store.Applied{}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, store.Applied{}, err
	}
	req.Header.Set("User-Agent", UserAgent)
	if !opt.Force && known && src.Items > 0 {
		if src.ETag != "" {
			req.Header.Set("If-None-Match", src.ETag)
		}
		if src.LastModified != "" {
			req.Header.Set("If-Modified-Since", src.LastModified)
		}
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		metrics.ScrapeFetches.WithLabelValues("error").Inc()
		return nil, store.Applied{}, err
	}
	defer resp.Body.Close()
	now := s.Now()

	if resp.StatusCode == http.StatusNotModified {
		metrics.ScrapeFetches.WithLabelValues("unchanged").Inc()
		src.CheckedAt = now
		if lastmod != "" {
			src.LastMod = lastmod
		}
		if !opt.DryRun {
			return nil, store.Applied{}, s.Store.SaveSource(ctx, src)
		}
		return nil, store.Applied{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		metrics.ScrapeFetches.WithLabelValues("error").Inc()
		return nil, store.Applied{}, fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		metrics.ScrapeFetches.WithLabelValues("error").Inc()
		return nil, store.Applied{}, err
	}

	// a renamed page redirects; file its nights under where it lives now
	final := resp.Request.URL.String()
	lm, _ := time.Parse(time.RFC3339, lastmod)
	p, err := ParsePage(final, body, lm)
	if err != nil {
		metrics.ScrapeFetches.WithLabelValues("error").Inc()
		return nil, store.Applied{}, err
	}
	for _, w := range p.Warnings {
		warn("%s: %s", final, w)
	}

	prev, err := s.Store.CountForSource(ctx, final, p.Year)
	if err != nil {
		return nil, store.Applied{}, err
	}
	if reason := reject(len(p.Lightings), prev); reason != "" {
		metrics.ScrapeFetches.WithLabelValues("rejected").Inc()
		warn("%s: %s; keeping the last good data", final, reason)
		return &p, store.Applied{}, nil
	}
	metrics.ScrapeFetches.WithLabelValues("changed").Inc()
	if opt.DryRun {
		return &p, store.Applied{}, nil
	}

	applied, err := s.Store.ApplyPage(ctx, final, p.Year, p.Lightings, now)
	if err != nil {
		return nil, store.Applied{}, err
	}
	if final != url {
		src = store.Source{URL: final}
		if old, ok, _ := s.Store.Source(ctx, final); ok {
			src = old
		}
	}
	src.Title, src.Year, src.Program = p.Title, p.Year, p.Program
	if lastmod != "" {
		src.LastMod = lastmod
	}
	src.ETag, src.LastModified = resp.Header.Get("ETag"), resp.Header.Get("Last-Modified")
	src.Items, src.CheckedAt = len(p.Lightings), now
	if applied != (store.Applied{}) || src.ChangedAt.IsZero() {
		src.ChangedAt = now
	}
	return &p, applied, s.Store.SaveSource(ctx, src)
}

// reject says why a parse shouldn't replace what's stored: nothing parsed, or
// less than half of what the page had before (a layout change, not a real
// edit). Empty means apply it.
func reject(got, prev int) string {
	switch {
	case got == 0:
		return "no nights parsed"
	case prev >= 6 && got*2 < prev:
		return fmt.Sprintf("only %d nights parsed, down from %d", got, prev)
	}
	return ""
}

// sitemapURL is one <url> in a sitemap.
type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

func (s *Scraper) sitemap(ctx context.Context) ([]sitemapURL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Site+SitemapPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return parseSitemap(io.LimitReader(resp.Body, maxBody))
}

func parseSitemap(r io.Reader) ([]sitemapURL, error) {
	var doc struct {
		URLs []sitemapURL `xml:"url"`
	}
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("sitemap: %w", err)
	}
	for i := range doc.URLs {
		doc.URLs[i].Loc = strings.TrimSpace(doc.URLs[i].Loc)
		doc.URLs[i].LastMod = strings.TrimSpace(doc.URLs[i].LastMod)
	}
	return doc.URLs, nil
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Every runs the scraper at startup (when the last success is older than
// interval) and then every interval, with up to a tenth of jitter, until ctx
// is done.
func (s *Scraper) Every(ctx context.Context, interval time.Duration) {
	last, _ := s.Store.LastSuccess(ctx)
	wait := 5 * time.Second
	if !last.IsZero() {
		metrics.ScrapeLastSuccess.Set(float64(last.Unix()))
		if since := s.Now().Sub(last); since < interval {
			wait = interval - since
		}
	}
	for {
		if err := sleep(ctx, wait); err != nil {
			return
		}
		if _, err := s.Run(ctx, Options{}); err != nil && ctx.Err() == nil {
			slog.Error("scrape", "error", err)
		}
		jitter := time.Duration(float64(interval) * 0.1 * (2*randFloat() - 1))
		wait = interval + jitter
	}
}

func randFloat() float64 { return rand.Float64() }
