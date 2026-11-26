package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/evanofslack/prulights/config"
	"github.com/evanofslack/prulights/schedule"
	"github.com/evanofslack/prulights/store"
)

// seedPool is who the seed command can make up.
var seedPool = []string{"Logan", "Kendall", "Roman", "Shiv", "Connor", "Tom", "Greg", "Gerri"}

const (
	seedMax    = 5
	seedNights = 60 // how far back the seeded sightings go
	seedWindow = 7  // the nights before tonight that can still be logged
)

var seedNotes = []string{
	"Dad would hate this color.",
	"Saw it from the Mass Pike.",
	"Very on brand.",
	"Fine. It's fine. It's a building.",
	"Could see it from the hotel bar.",
	"Ludicrously tall. Respect.",
	"Greg says it's pretty. Greg.",
	"Not a real color. Ask me how I know.",
	"Took the long way down Boylston for this.",
	"Love that for the tower.",
	"The eldest boy approves.",
	"Wonderful. Truly. Moving on.",
}

// seedCmd fills a dev database with made-up people's sightings,
// to see the social parts of the site with company.
func seedCmd(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	namesFlag := fs.String("names", "Logan,Shiv,Roman,Greg", "comma-separated, from "+strings.Join(seedPool, ", ")+"; the first logs most nights")
	reset := fs.Bool("reset", false, "delete these people first, with everything they logged")
	seedN := fs.Uint64("seed", 0, "random seed for repeatable data (0: by the clock)")
	fs.Parse(args)

	cfg := config.Load()
	if !cfg.Dev {
		return errors.New("seed is dev only (set DEV=1)")
	}
	names, err := parseSeedNames(*namesFlag)
	if err != nil {
		return err
	}
	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	n := *seedN
	if n == 0 {
		n = uint64(time.Now().UnixNano())
	}
	out, err := seed(ctx, st, names, *reset, rand.New(rand.NewPCG(n, n)), time.Now())
	if err != nil {
		return err
	}
	for _, s := range out {
		fmt.Printf("%-8s %2d nights, %2d notes\n", s.Name, s.Nights, s.Notes)
	}
	return nil
}

// parseSeedNames reads "logan, Shiv" into pool names, in order.
func parseSeedNames(s string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name := ""
		for _, p := range seedPool {
			if strings.EqualFold(p, part) {
				name = p
			}
		}
		if name == "" {
			return nil, fmt.Errorf("%q isn't one of %s", part, strings.Join(seedPool, ", "))
		}
		if seen[name] {
			return nil, fmt.Errorf("%s is listed twice", name)
		}
		seen[name] = true
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil, errors.New("name at least one person")
	}
	if len(out) > seedMax {
		return nil, fmt.Errorf("at most %d people", seedMax)
	}
	return out, nil
}

type seedSummary struct {
	Name          string
	Nights, Notes int
}

// seed makes (or reuses) the named people and logs their nights: the first
// most nights, the second about half, the rest a handful.
func seed(ctx context.Context, st *store.Store, names []string, reset bool, rng *rand.Rand, now time.Time) ([]seedSummary, error) {
	tonight := schedule.Tonight(now)
	out := make([]seedSummary, len(names))
	for i, name := range names {
		u, err := st.UserByName(ctx, name)
		switch {
		case err == nil && reset:
			if err := st.DeleteUser(ctx, u.ID); err != nil {
				return nil, err
			}
			u, err = st.CreateUser(ctx, name, now)
			if err != nil {
				return nil, err
			}
		case errors.Is(err, store.ErrNotFound):
			if u, err = st.CreateUser(ctx, name, now); err != nil {
				return nil, err
			}
		case err != nil:
			return nil, err
		}
		out[i].Name = u.Name

		for _, d := range seedDates(i, rng, tonight) {
			rating := 0
			if rng.Float64() >= 0.2 {
				rating = []int{3, 4, 4, 5, 5, 5}[rng.IntN(6)]
			}
			note := ""
			if rng.Float64() < 0.3 {
				note = seedNotes[rng.IntN(len(seedNotes))]
				out[i].Notes++
			}
			if err := st.SaveSighting(ctx, u.ID, d, rating, note, seedTime(d, rng, now)); err != nil {
				return nil, err
			}
			out[i].Nights++
		}
	}

	return out, nil
}

// seedDates picks the nights the i-th person saw, newest first.
func seedDates(i int, rng *rand.Rand, tonight schedule.Date) []schedule.Date {
	var days []int // nights before tonight
	switch i {
	case 0, 1:
		p := map[int]float64{0: 0.85, 1: 0.45}[i]
		for back := range seedNights {
			// a couple of open nights each, so there's something in Lately
			if rng.Float64() < p || back == 1+i || back == 3+i {
				days = append(days, back)
			}
		}
	default:
		days = rng.Perm(seedNights)[:3+rng.IntN(4)]
	}
	out := make([]schedule.Date, len(days))
	for j, back := range days {
		out[j] = tonight.AddDays(-back)
	}
	return out
}

// seedTime is when someone said they saw a night: that evening, after
// lights on, but never in the future.
func seedTime(d schedule.Date, rng *rand.Rand, now time.Time) time.Time {
	t := d.Time().Add(19*time.Hour + time.Duration(rng.IntN(4*60))*time.Minute)
	if t.After(now) {
		return now
	}
	return t
}
