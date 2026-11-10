// Command prulights runs the server and its maintenance tasks.
//
//	prulights [serve]                 run the api, web app and scraper
//	prulights scrape [flags]          check the source pages once
//	prulights backfill                fetch and apply every source page
//	prulights frame new|config        prepare a frame for flashing
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/evanofslack/prulights/config"
	"github.com/evanofslack/prulights/scrape"
	"github.com/evanofslack/prulights/server"
	"github.com/evanofslack/prulights/store"
	"github.com/joho/godotenv"
)

// version is set at build time with -ldflags "-X main.version=…".
var version = "dev"

func main() {
	if err := godotenv.Load(); err == nil {
		slog.Info("loaded .env")
	}
	setupLogging()

	cmd, args := "serve", []string{}
	if len(os.Args) > 1 {
		cmd, args = os.Args[1], os.Args[2:]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch cmd {
	case "serve":
		err = serve(ctx)
	case "scrape":
		err = scrapeCmd(ctx, args, false)
	case "backfill":
		err = scrapeCmd(ctx, args, true)
	case "render":
		err = renderCmd(ctx, args)
	case "invite":
		err = inviteCmd(ctx, args)
	case "frame":
		err = frameCmd(ctx, args)
	case "version", "--version":
		fmt.Println(version)
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		slog.Error(cmd, "error", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: prulights <command>

commands:
  serve      run the api, web app and scraper (default)
  scrape     check the source pages once (-dry-run, -force, -json, -url U, -delay D)
  backfill   fetch and apply every source page, ignoring what's cached
  render     draw a frame view to a png (-view, -date, -palette, -out)
  invite     make an invite link and QR code (-admin, -for-user NAME)
  frame new  prepare a frame's identity for flashing (-invite, -out FILE)
  frame config  the files to flash: -from FILE -wifi "SSID:password" … -out DIR
  version    print the version`)
}

func setupLogging() {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

func openStore(ctx context.Context, cfg config.Config) (*store.Store, error) {
	st, err := store.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", cfg.DatabasePath, err)
	}
	return st, nil
}

func serve(ctx context.Context) error {
	cfg := config.Load()
	cfg.Version = version
	slog.Info("starting", "version", version, "base_url", cfg.BaseURL)
	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	s := server.New(cfg, st)
	if cfg.ScrapeEnabled {
		go scrape.New(st).Every(ctx, cfg.ScrapeInterval)
		s.SetScraper(func(ctx context.Context) error {
			_, err := scrape.New(st).Run(ctx, scrape.Options{})
			return err
		})
	}
	go cleanup(ctx, st)
	select {
	case err := <-s.Run():
		return err
	case <-ctx.Done():
	}
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.Shutdown(shutdownCtx)
}

type urlList []string

func (u *urlList) String() string     { return strings.Join(*u, ",") }
func (u *urlList) Set(s string) error { *u = append(*u, s); return nil }

// scrapeCmd runs the scraper once and prints what it found.
func scrapeCmd(ctx context.Context, args []string, backfill bool) error {
	fs := flag.NewFlagSet("scrape", flag.ExitOnError)
	dry := fs.Bool("dry-run", false, "parse but don't write to the database")
	force := fs.Bool("force", backfill, "fetch every page, ignoring the sitemap's lastmod and etags")
	asJSON := fs.Bool("json", false, "print the parsed pages as json")
	var urls urlList
	fs.Var(&urls, "url", "check only this page (repeatable)")
	delay := fs.Duration("delay", scrape.CrawlDelay, "wait between pages (the site's robots.txt asks for 10s)")
	fs.Parse(args)

	cfg := config.Load()
	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	sc := scrape.New(st)
	sc.Delay = *delay
	res, err := sc.Run(ctx, scrape.Options{Force: *force, DryRun: *dry, URLs: urls})
	if res == nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(res)
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	for _, p := range res.Pages {
		fmt.Fprintf(tw, "\n%s (%d, %s): %d nights\n", p.Title, p.Year, p.Program, len(p.Lightings))
		for _, l := range p.Lightings {
			colors := make([]string, len(l.Colors))
			for i, c := range l.Colors {
				colors[i] = string(c)
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", l.Date, strings.Join(colors, ", "), l.Kind, l.Cause)
		}
	}
	tw.Flush()
	r := res.Run
	fmt.Printf("\npages %d, fetched %d, nights %d, added %d, changed %d, removed %d, warnings %d\n",
		r.Pages, r.Fetched, r.Items, r.Added, r.Changed, r.Removed, len(r.Warnings))
	for _, w := range r.Warnings {
		fmt.Println("  warning:", w)
	}
	if *dry {
		fmt.Println("(dry run: nothing written)")
	}
	return err
}

// cleanup deletes expired sessions and invites once a day.
func cleanup(ctx context.Context, st *store.Store) {
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		if err := st.DeleteExpired(ctx, time.Now()); err != nil && ctx.Err() == nil {
			slog.Error("cleanup", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
