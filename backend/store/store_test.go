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

func TestMigrateTwice(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "p.db")
	for range 2 {
		s, err := Open(ctx, file)
		if err != nil {
			t.Fatal(err)
		}
		var n int
		if err := s.DB.QueryRowContext(ctx, `select count(*) from migrations`).Scan(&n); err != nil || n != 1 {
			t.Fatalf("migrations = %d, %v", n, err)
		}
		s.Close()
	}
}
