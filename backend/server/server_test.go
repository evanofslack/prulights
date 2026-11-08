package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evanofslack/prulights/config"
	"github.com/evanofslack/prulights/store"
)

// newTestServer builds a server on an in-memory store with a tiny static dir.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("index"), 0o644)
	os.WriteFile(filepath.Join(dir, "calendar.html"), []byte("calendar"), 0o644)
	cfg := config.Default()
	cfg.StaticDir = dir
	cfg.MetricsAddr = ""
	return New(cfg, st)
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestStatus(t *testing.T) {
	s := newTestServer(t)
	for _, p := range []string{"/healthz", "/readyz"} {
		if rec := get(t, s.Handler(), p); rec.Code != http.StatusOK {
			t.Errorf("%s = %d", p, rec.Code)
		}
	}
	s.ready.Store(false)
	if rec := get(t, s.Handler(), "/readyz"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz while shutting down = %d", rec.Code)
	}
}

func TestStatic(t *testing.T) {
	s := newTestServer(t)
	cases := map[string]string{
		"/":              "index",
		"/calendar":      "calendar",
		"/somewhere/new": "index",
	}
	for path, want := range cases {
		rec := get(t, s.Handler(), path)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s = %d %q, want %q", path, rec.Code, rec.Body.String(), want)
		}
	}
}

func TestAPINotFound(t *testing.T) {
	s := newTestServer(t)
	rec := get(t, s.Handler(), "/api/v1/nope")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"error"`) {
		t.Errorf("unknown api route = %d %q", rec.Code, rec.Body.String())
	}
}
