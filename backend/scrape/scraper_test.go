package scrape

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/evanofslack/prulights/store"
)

// fakeSite serves the saved pages and a sitemap pointing at them, with ETags.
type fakeSite struct {
	*httptest.Server
	mu       sync.Mutex
	pages    map[string][]byte // path → body
	lastmods map[string]string
	hits     map[string]int
	notMod   int
}

func newFakeSite(t *testing.T) *fakeSite {
	t.Helper()
	f := &fakeSite{pages: map[string][]byte{}, lastmods: map[string]string{}, hits: map[string]int{}}
	for path, file := range map[string]string{
		"/experiences/events/2026-tower-lightings/": "2026-tower-lightings",
		"/experiences/events/31-nights-of-light/":   "31-nights-of-light",
	} {
		b, err := os.ReadFile("testdata/" + file + ".html")
		if err != nil {
			t.Fatal(err)
		}
		f.pages[path] = b
		f.lastmods[path] = "2026-11-06T13:58:06+00:00"
	}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeSite) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hits[r.URL.Path]++
	if r.Header.Get("User-Agent") != UserAgent {
		http.Error(w, "who are you", http.StatusForbidden)
		return
	}
	switch {
	case r.URL.Path == SitemapPath:
		var b strings.Builder
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
		b.WriteString(`<url><loc>` + f.URL + `/experiences/events/holiday-market/</loc><lastmod>2026-10-01T00:00:00+00:00</lastmod></url>`)
		for path, lm := range f.lastmods {
			fmt.Fprintf(&b, "<url><loc>%s%s</loc><lastmod>%s</lastmod></url>", f.URL, path, lm)
		}
		b.WriteString(`</urlset>`)
		w.Write([]byte(b.String()))
	case r.URL.Path == "/experiences/events/2025-tower-lightings/":
		http.Redirect(w, r, "/experiences/events/2026-tower-lightings/", http.StatusMovedPermanently)
	default:
		body, ok := f.pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		etag := fmt.Sprintf(`"%d"`, len(body))
		if r.Header.Get("If-None-Match") == etag {
			f.notMod++
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Write(body)
	}
}

func newTestScraper(t *testing.T, site string) (*Scraper, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s := New(st)
	s.Site, s.Delay = site, 0
	s.Now = func() time.Time { return time.Date(2026, 11, 8, 17, 0, 0, 0, time.UTC) }
	return s, st
}

func TestRun(t *testing.T) {
	ctx := context.Background()
	site := newFakeSite(t)
	s, st := newTestScraper(t, site.URL)

	res, err := s.Run(ctx, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r := res.Run
	if r.Pages != 2 || r.Fetched != 2 || r.Items != 77+31 || r.Added != 77+31 || len(r.Warnings) != 0 {
		t.Fatalf("first run = %+v", r)
	}
	ls, _ := st.Lightings(ctx, "2026-12-01", "2026-12-31")
	if len(ls) != 31 {
		t.Fatalf("december has %d listings", len(ls))
	}
	if d, _ := st.PublishedThrough(ctx); d != "2026-12-31" {
		t.Errorf("published through %s", d)
	}
	if runs, _ := st.Runs(ctx, 5); len(runs) != 1 || runs[0].ID == 0 {
		t.Errorf("runs = %+v", runs)
	}

	// nothing changed: the sitemap's lastmod lets it skip both pages
	res, _ = s.Run(ctx, Options{})
	if res.Run.Fetched != 0 || site.hits["/experiences/events/31-nights-of-light/"] != 1 {
		t.Errorf("second run fetched %d, hits %v", res.Run.Fetched, site.hits)
	}

	// lastmod moved but the page didn't: a conditional request gets a 304
	site.mu.Lock()
	site.lastmods["/experiences/events/31-nights-of-light/"] = "2026-11-08T10:00:00+00:00"
	site.mu.Unlock()
	res, _ = s.Run(ctx, Options{})
	if site.notMod != 1 || res.Run.Added+res.Run.Changed != 0 {
		t.Errorf("expected one 304, got %d; run %+v", site.notMod, res.Run)
	}
	src, _, _ := st.Source(ctx, site.URL+"/experiences/events/31-nights-of-light/")
	if src.LastMod != "2026-11-08T10:00:00+00:00" || src.Year != 2026 || src.Items != 31 {
		t.Errorf("source = %+v", src)
	}
}

func TestRunRejectsBrokenPage(t *testing.T) {
	ctx := context.Background()
	site := newFakeSite(t)
	s, st := newTestScraper(t, site.URL)
	if _, err := s.Run(ctx, Options{}); err != nil {
		t.Fatal(err)
	}
	// the page is redesigned and the list is gone
	site.mu.Lock()
	site.pages["/experiences/events/31-nights-of-light/"] = []byte(`<html><head><title>31 Nights of Light - Prudential Center</title></head>
		<body><p>2026 Lighting Schedule</p><ul><li>December 1 | Something | Gold</li></ul></body></html>`)
	site.mu.Unlock()
	res, err := s.Run(ctx, Options{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.Removed != 0 || len(res.Run.Warnings) == 0 || !strings.Contains(res.Run.Warnings[0], "keeping the last good data") {
		t.Errorf("run = %+v", res.Run)
	}
	if ls, _ := st.Lightings(ctx, "2026-12-01", "2026-12-31"); len(ls) != 31 {
		t.Errorf("december has %d listings after a broken page", len(ls))
	}
}

func TestRunFollowsRenamedPage(t *testing.T) {
	ctx := context.Background()
	site := newFakeSite(t)
	s, st := newTestScraper(t, site.URL)
	res, err := s.Run(ctx, Options{URLs: []string{site.URL + "/experiences/events/2025-tower-lightings/"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Pages) != 1 || !strings.HasSuffix(res.Pages[0].URL, "/2026-tower-lightings/") {
		t.Fatalf("pages = %+v", res.Pages)
	}
	if _, ok, _ := st.Source(ctx, site.URL+"/experiences/events/2026-tower-lightings/"); !ok {
		t.Error("nights should be filed under the page's new address")
	}
}

func TestRunDryRun(t *testing.T) {
	ctx := context.Background()
	site := newFakeSite(t)
	s, st := newTestScraper(t, site.URL)
	res, err := s.Run(ctx, Options{DryRun: true})
	if err != nil || len(res.Pages) != 2 {
		t.Fatalf("dry run = %+v, %v", res, err)
	}
	if ls, _ := st.Lightings(ctx, "2026-01-01", "2026-12-31"); len(ls) != 0 {
		t.Errorf("dry run wrote %d nights", len(ls))
	}
	if runs, _ := st.Runs(ctx, 5); len(runs) != 0 {
		t.Errorf("dry run saved a run")
	}
}

func TestRunSiteDown(t *testing.T) {
	s, _ := newTestScraper(t, "http://127.0.0.1:1")
	res, err := s.Run(context.Background(), Options{})
	if err == nil || res.Run.Error == "" {
		t.Errorf("expected an error when the site is down, got %+v", res.Run)
	}
}

func TestReject(t *testing.T) {
	cases := []struct {
		got, prev int
		reject    bool
	}{
		{0, 0, true}, {0, 31, true}, {1, 31, true}, {15, 31, true}, {16, 31, false},
		{31, 31, false}, {3, 4, false}, {40, 31, false},
	}
	for _, c := range cases {
		if got := reject(c.got, c.prev) != ""; got != c.reject {
			t.Errorf("reject(%d, %d) = %v", c.got, c.prev, got)
		}
	}
}
