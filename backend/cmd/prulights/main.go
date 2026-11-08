// Command prulights runs the server and its maintenance tasks.
//
//	prulights [serve]   run the api, web app and scraper
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/evanofslack/prulights/config"
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
	case "version", "--version":
		fmt.Println(version)
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		err = fmt.Errorf("unknown command %q", cmd)
	}
	_ = args
	if err != nil {
		slog.Error(cmd, "error", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: prulights <command>

commands:
  serve      run the api, web app and scraper (default)
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
	slog.Info("starting", "version", version, "base_url", cfg.BaseURL)
	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	s := server.New(cfg, st)
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
