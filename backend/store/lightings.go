package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

// timeLayout is how times are stored: RFC 3339 in UTC, which sorts as text.
const timeLayout = time.RFC3339

func fmtTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) time.Time {
	t, _ := time.Parse(timeLayout, s)
	return t
}

// Applied counts what a page changed.
type Applied struct {
	Added   int `json:"added"`
	Changed int `json:"changed"`
	Removed int `json:"removed"`
}

type lightingKey struct {
	date  schedule.Date
	cause string
}

// ApplyPage makes the stored lightings for one source page match what was
// just parsed from it: new nights are added, changed ones updated, and ones
// no longer listed are marked removed. Only nights in the page's year are
// considered for removal, so a page rewritten for a new year doesn't erase
// the last one.
func (s *Store) ApplyPage(ctx context.Context, sourceURL string, year int, ls []schedule.Lighting, now time.Time) (Applied, error) {
	var out Applied
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `select id, date, cause, colors, colors_raw, cause_url, kind, program, removed_at
			from lightings where source_url = ?`, sourceURL)
		if err != nil {
			return err
		}
		type existing struct {
			id                                         int64
			colors, colorsRaw, causeURL, kind, program string
			removed                                    bool
		}
		have := map[lightingKey]existing{}
		for rows.Next() {
			var e existing
			var k lightingKey
			var removed sql.NullString
			if err := rows.Scan(&e.id, &k.date, &k.cause, &e.colors, &e.colorsRaw, &e.causeURL, &e.kind, &e.program, &removed); err != nil {
				rows.Close()
				return err
			}
			e.removed = removed.Valid
			have[k] = e
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		ts := fmtTime(now)
		seen := map[lightingKey]bool{}
		for _, l := range ls {
			k := lightingKey{l.Date, l.Cause}
			if seen[k] {
				continue
			}
			seen[k] = true
			colors, _ := json.Marshal(l.Colors)
			e, ok := have[k]
			switch {
			case !ok:
				_, err = tx.ExecContext(ctx, `insert into lightings
					(date, program, colors, colors_raw, cause, cause_url, kind, source_url, first_seen_at, last_seen_at)
					values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					l.Date, l.Program, string(colors), l.ColorsRaw, l.Cause, l.CauseURL, l.Kind, sourceURL, ts, ts)
				out.Added++
			case e.removed || e.colors != string(colors) || e.colorsRaw != l.ColorsRaw || e.causeURL != l.CauseURL ||
				e.kind != string(l.Kind) || e.program != string(l.Program):
				_, err = tx.ExecContext(ctx, `update lightings set colors = ?, colors_raw = ?, cause_url = ?, kind = ?,
					program = ?, last_seen_at = ?, removed_at = null where id = ?`,
					string(colors), l.ColorsRaw, l.CauseURL, l.Kind, l.Program, ts, e.id)
				out.Changed++
			default:
				_, err = tx.ExecContext(ctx, `update lightings set last_seen_at = ? where id = ?`, ts, e.id)
			}
			if err != nil {
				return err
			}
		}

		yearPrefix := fmt.Sprintf("%04d", year)
		for k, e := range have {
			if seen[k] || e.removed || !strings.HasPrefix(string(k.date), yearPrefix) {
				continue
			}
			if _, err := tx.ExecContext(ctx, `update lightings set removed_at = ? where id = ?`, ts, e.id); err != nil {
				return err
			}
			out.Removed++
		}
		return nil
	})
	return out, err
}

// CountForSource is how many nights a page currently lists for a year.
func (s *Store) CountForSource(ctx context.Context, sourceURL string, year int) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `select count(*) from lightings
		where source_url = ? and removed_at is null and substr(date, 1, 4) = ?`,
		sourceURL, fmt.Sprintf("%04d", year)).Scan(&n)
	return n, err
}

// Lightings returns the listed nights from from to to (inclusive), scraped
// and overridden, sorted by date. Nights with nothing listed aren't
// included; schedule.Fill adds them.
func (s *Store) Lightings(ctx context.Context, from, to schedule.Date) ([]schedule.Lighting, error) {
	rows, err := s.DB.QueryContext(ctx, `
		select date, program, colors, colors_raw, cause, cause_url, kind, source_url from lightings
			where removed_at is null and date between ? and ?
		union all
		select date, 'override', colors, '', cause, cause_url, kind, '' from overrides
			where date between ? and ?
		order by date, program`, from, to, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []schedule.Lighting
	for rows.Next() {
		var l schedule.Lighting
		var colors string
		if err := rows.Scan(&l.Date, &l.Program, &colors, &l.ColorsRaw, &l.Cause, &l.CauseURL, &l.Kind, &l.SourceURL); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(colors), &l.Colors); err != nil {
			return nil, err
		}
		l.Scheduled = true
		out = append(out, l)
	}
	// the 31 Nights page first on a night it shares with another listing:
	// it's the night's main program
	slices.SortStableFunc(out, func(a, b schedule.Lighting) int {
		if a.Date != b.Date {
			if a.Date < b.Date {
				return -1
			}
			return 1
		}
		return rank(a.Program) - rank(b.Program)
	})
	return out, rows.Err()
}

func rank(p schedule.Program) int {
	switch p {
	case schedule.ProgramOverride:
		return 0
	case schedule.ProgramNights:
		return 1
	default:
		return 2
	}
}

// PublishedThrough is the last night any source lists, "" when none do.
func (s *Store) PublishedThrough(ctx context.Context) (schedule.Date, error) {
	var d sql.NullString
	err := s.DB.QueryRowContext(ctx, `select max(date) from (
		select date from lightings where removed_at is null union all select date from overrides)`).Scan(&d)
	return schedule.Date(d.String), err
}

// Source is what we know about one source page.
type Source struct {
	URL          string           `json:"url"`
	Title        string           `json:"title"`
	Year         int              `json:"year"`
	Program      schedule.Program `json:"program"`
	LastMod      string           `json:"lastmod"`
	ETag         string           `json:"-"`
	LastModified string           `json:"-"`
	Items        int              `json:"items"`
	CheckedAt    time.Time        `json:"checked_at"`
	ChangedAt    time.Time        `json:"changed_at"`
}

// Source returns a source page's state; ok is false when it's new.
func (s *Store) Source(ctx context.Context, url string) (src Source, ok bool, err error) {
	var checked, changed string
	err = s.DB.QueryRowContext(ctx, `select url, title, year, program, lastmod, etag, last_modified, items, checked_at, changed_at
		from sources where url = ?`, url).
		Scan(&src.URL, &src.Title, &src.Year, &src.Program, &src.LastMod, &src.ETag, &src.LastModified, &src.Items, &checked, &changed)
	if errors.Is(err, sql.ErrNoRows) {
		return Source{URL: url}, false, nil
	}
	src.CheckedAt, src.ChangedAt = parseTime(checked), parseTime(changed)
	return src, err == nil, err
}

// SaveSource writes a source page's state.
func (s *Store) SaveSource(ctx context.Context, src Source) error {
	_, err := s.DB.ExecContext(ctx, `insert into sources
		(url, title, year, program, lastmod, etag, last_modified, items, checked_at, changed_at)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict (url) do update set title = excluded.title, year = excluded.year, program = excluded.program,
			lastmod = excluded.lastmod, etag = excluded.etag, last_modified = excluded.last_modified,
			items = excluded.items, checked_at = excluded.checked_at, changed_at = excluded.changed_at`,
		src.URL, src.Title, src.Year, src.Program, src.LastMod, src.ETag, src.LastModified, src.Items,
		fmtTime(src.CheckedAt), fmtTime(src.ChangedAt))
	return err
}

// Sources lists every known source page.
func (s *Store) Sources(ctx context.Context) ([]Source, error) {
	rows, err := s.DB.QueryContext(ctx, `select url from sources order by year desc, url`)
	if err != nil {
		return nil, err
	}
	var urls []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			rows.Close()
			return nil, err
		}
		urls = append(urls, u)
	}
	rows.Close()
	out := make([]Source, 0, len(urls))
	for _, u := range urls {
		src, _, err := s.Source(ctx, u)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, nil
}

// ScrapeRun is one run of the scraper.
type ScrapeRun struct {
	ID         int64     `json:"id"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Pages      int       `json:"pages"`
	Fetched    int       `json:"fetched"`
	Items      int       `json:"items"`
	Applied
	Warnings []string `json:"warnings"`
	Error    string   `json:"error,omitempty"`
}

// SaveRun records a finished run.
func (s *Store) SaveRun(ctx context.Context, r *ScrapeRun) error {
	warnings, _ := json.Marshal(r.Warnings)
	if r.Warnings == nil {
		warnings = []byte("[]")
	}
	res, err := s.DB.ExecContext(ctx, `insert into scrape_runs
		(started_at, finished_at, pages, fetched, items, added, changed, removed, warnings, error)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fmtTime(r.StartedAt), fmtTime(r.FinishedAt), r.Pages, r.Fetched, r.Items, r.Added, r.Changed, r.Removed,
		string(warnings), r.Error)
	if err != nil {
		return err
	}
	r.ID, err = res.LastInsertId()
	return err
}

// Runs returns the latest runs, newest first.
func (s *Store) Runs(ctx context.Context, limit int) ([]ScrapeRun, error) {
	rows, err := s.DB.QueryContext(ctx, `select id, started_at, coalesce(finished_at, ''), pages, fetched, items,
		added, changed, removed, warnings, error from scrape_runs order by id desc limit ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScrapeRun
	for rows.Next() {
		var r ScrapeRun
		var started, finished, warnings string
		if err := rows.Scan(&r.ID, &started, &finished, &r.Pages, &r.Fetched, &r.Items, &r.Added, &r.Changed,
			&r.Removed, &warnings, &r.Error); err != nil {
			return nil, err
		}
		r.StartedAt, r.FinishedAt = parseTime(started), parseTime(finished)
		json.Unmarshal([]byte(warnings), &r.Warnings)
		out = append(out, r)
	}
	return out, rows.Err()
}

// LastSuccess is when the scraper last finished without an error, zero if
// it never has.
func (s *Store) LastSuccess(ctx context.Context) (time.Time, error) {
	var t sql.NullString
	err := s.DB.QueryRowContext(ctx, `select max(finished_at) from scrape_runs where error = ''`).Scan(&t)
	return parseTime(t.String), err
}
