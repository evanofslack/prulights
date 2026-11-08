package store

import (
	"context"
	"path/filepath"
	"testing"
)

// newTest opens a fresh in-memory store for a test.
func newTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestMeta(t *testing.T) {
	ctx := context.Background()
	s := newTest(t)
	if v, err := s.Meta(ctx, "missing"); err != nil || v != "" {
		t.Fatalf("missing = %q, %v", v, err)
	}
	if err := s.SetMeta(ctx, "k", "a"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMeta(ctx, "k", "b"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Meta(ctx, "k"); v != "b" {
		t.Fatalf("k = %q, want b", v)
	}
}

func TestMigrateTwice(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "p.db")
	for range 2 {
		s, err := Open(ctx, file)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetMeta(ctx, "k", "v"); err != nil {
			t.Fatal(err)
		}
		s.Close()
	}
}
