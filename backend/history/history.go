// Package history is the schedule the site no longer shows: 2025 and
// December 2024, from Wayback Machine copies of the pages it has since
// rewritten. The copies are embedded, and Apply loads them into a database
// once, at startup, so every database has them without a step to remember.
package history

import (
	"bytes"
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/evanofslack/prulights/scrape"
	"github.com/evanofslack/prulights/store"
)

// Name marks the import in the database. Change it when the manifest
// changes, so databases that have the old one load the new one.
const Name = "wayback-2025-v1"

//go:embed manifest.json pages/*.html.gz
var files embed.FS

// Snapshot is one archived copy of a page (manifest.json). Year is the year
// its nights are in, checked against the parse.
type Snapshot struct {
	URL      string `json:"url"`
	Year     int    `json:"year"`
	Snapshot string `json:"snapshot"` // the Wayback timestamp, UTC
}

// File is where the copy is kept, in pages/.
func (s Snapshot) File() string {
	return "pages/" + path.Base(s.URL) + "-" + s.Snapshot + ".html.gz"
}

// Time is when the copy was taken.
func (s Snapshot) Time() (time.Time, error) {
	return time.Parse("20060102150405", s.Snapshot)
}

// Manifest lists the copies in the order they're applied.
func Manifest() ([]Snapshot, error) {
	b, err := files.ReadFile("manifest.json")
	if err != nil {
		return nil, err
	}
	var out []Snapshot
	return out, json.Unmarshal(b, &out)
}

// Pages parses every copy in the manifest.
func Pages() ([]scrape.Page, []store.ImportPage, error) {
	snaps, err := Manifest()
	if err != nil {
		return nil, nil, err
	}
	var parsed []scrape.Page
	var pages []store.ImportPage
	for _, s := range snaps {
		at, err := s.Time()
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", s.File(), err)
		}
		gz, err := files.ReadFile(s.File())
		if err != nil {
			return nil, nil, err
		}
		zr, err := gzip.NewReader(bytes.NewReader(gz))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", s.File(), err)
		}
		body, err := io.ReadAll(zr)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", s.File(), err)
		}
		p, err := scrape.ParsePage(s.URL, body, at)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", s.File(), err)
		}
		if p.Year != s.Year {
			return nil, nil, fmt.Errorf("%s: the page is for %d, the manifest says %d", s.File(), p.Year, s.Year)
		}
		if len(p.Lightings) == 0 {
			return nil, nil, fmt.Errorf("%s: no nights parsed", s.File())
		}
		parsed = append(parsed, p)
		pages = append(pages, store.ImportPage{URL: s.URL, Year: s.Year, Lightings: p.Lightings, At: at})
	}
	return parsed, pages, nil
}

// Apply loads the archived pages into st unless it has them already. Each
// copy is applied as the scraper would have when it was live, oldest first,
// under the page's own address, so later corrections on the site win and
// the live scraper (which only removes nights in a page's current year)
// never touches them.
func Apply(ctx context.Context, st *store.Store, now time.Time) (done bool, out store.Applied, err error) {
	_, pages, err := Pages()
	if err != nil {
		return false, out, err
	}
	return st.Import(ctx, Name, pages, now)
}
