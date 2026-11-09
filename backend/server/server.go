// Package server serves the API and the web app.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/evanofslack/prulights/config"
	"github.com/evanofslack/prulights/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-webauthn/webauthn/webauthn"
)

// Server is the http server: the API under /api and the static web app
// everywhere else.
type Server struct {
	cfg     config.Config
	store   *store.Store
	router  chi.Router
	http    *http.Server
	metrics *http.Server
	ready   atomic.Bool

	// now is the clock, swapped in tests.
	now func() time.Time

	wa         *webauthn.WebAuthn
	ceremonies ceremonies

	// scrape checks the source pages now; scraping is set while it runs.
	scrape   func(context.Context) error
	scraping atomic.Bool
}

// New builds the server and its routes.
func New(cfg config.Config, st *store.Store) *Server {
	registerExtensions()
	cfg.Fill()
	wa, err := newWebAuthn(cfg.RPID, cfg.RPOrigins)
	if err != nil {
		panic(fmt.Sprintf("passkeys: bad RP_ID or RP_ORIGINS: %v", err))
	}
	s := &Server{cfg: cfg, store: st, router: chi.NewRouter(), now: time.Now, wa: wa}
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.Recoverer)
	s.router.Use(requestLog)
	s.mountStatus()
	if cfg.Dev {
		s.mountDev()
	}
	s.router.Route("/api", func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			s.mountPublic(r)
			s.mountAccount(r)
		})
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "not found")
		})
		r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		})
	})
	s.mountStatic()
	s.http = &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	if cfg.MetricsAddr != "" {
		s.metrics = &http.Server{Addr: cfg.MetricsAddr, Handler: metricsHandler(), ReadHeaderTimeout: 10 * time.Second}
	}
	s.ready.Store(true)
	return s
}

// Handler is the root handler, for tests.
func (s *Server) Handler() http.Handler { return s.router }

// Run starts listening. The channel gets the first error from either listener.
func (s *Server) Run() <-chan error {
	errc := make(chan error, 2)
	go func() {
		slog.Info("listening", "addr", s.http.Addr)
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()
	if s.metrics != nil {
		go func() {
			slog.Info("metrics listening", "addr", s.metrics.Addr)
			if err := s.metrics.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errc <- err
			}
		}()
	}
	return errc
}

// Shutdown fails readiness, then stops accepting connections and waits for
// in-flight requests.
func (s *Server) Shutdown(ctx context.Context) error {
	s.ready.Store(false)
	if s.metrics != nil {
		s.metrics.Shutdown(ctx)
	}
	return s.http.Shutdown(ctx)
}

func (s *Server) mountStatus() {
	s.router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if !s.ready.Load() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "shutting down"})
			return
		}
		if err := s.store.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

// mountStatic serves the web app's static export. A path that isn't a file
// tries path.html (next's export writes /calendar as calendar.html), then
// falls back to index.html.
func (s *Server) mountStatic() {
	dir := s.cfg.StaticDir
	s.router.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean("/" + r.URL.Path)
		for _, p := range []string{clean, clean + ".html", filepath.Join(clean, "index.html")} {
			full := filepath.Join(dir, p)
			if fi, err := os.Stat(full); err == nil && !fi.IsDir() {
				if strings.HasPrefix(clean, "/_next/static/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				http.ServeFile(w, r, full)
				return
			}
		}
		index := filepath.Join(dir, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, r, index)
	}))
}

func registerExtensions() {
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".css", "text/css")
	mime.AddExtensionType(".svg", "image/svg+xml")
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

// requestLog logs each request through slog.
func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			return
		}
		slog.Debug("request", "method", r.Method, "path", r.URL.Path, "status", ww.Status(),
			"bytes", ww.BytesWritten(), "dur", time.Since(start))
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, apiError{Error: msg})
}
