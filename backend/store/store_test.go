package store

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"
	"time"
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
	files, _ := fs.Glob(migrations, "migrations/*.sql")
	for range 2 {
		s, err := Open(ctx, file)
		if err != nil {
			t.Fatal(err)
		}
		var n int
		if err := s.DB.QueryRowContext(ctx, `select count(*) from migrations`).Scan(&n); err != nil || n != len(files) {
			t.Fatalf("migrations = %d, %v", n, err)
		}
		s.Close()
	}
}

func TestBackup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, filepath.Join(dir, "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.SignUp(ctx, NewHandle(), "Mom", []byte("cred"), []byte("{}"), time.Now()); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "backup.db")
	if err := s.Backup(ctx, out); err != nil {
		t.Fatal(err)
	}
	if err := s.Backup(ctx, out); err == nil {
		t.Error("backed up over an existing file")
	}
	b, err := Open(ctx, out)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if u, err := b.UserByName(ctx, "Mom"); err != nil || u.Name != "Mom" {
		t.Errorf("backup has %+v, %v", u, err)
	}
}
