// Package store keeps everything in one sqlite file.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"

	_ "modernc.org/sqlite" // registers "sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Store is the sqlite database. Schema changes are new files in migrations/,
// never edits to old ones.
type Store struct {
	DB *sql.DB
}

// Open opens (creating if needed) the sqlite file and applies any migrations
// not yet applied. ":memory:" gives a private in-memory database.
func Open(ctx context.Context, file string) (*Store, error) {
	dsn := "file:" + file + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)"
	if file == ":memory:" {
		dsn = "file::memory:?_pragma=foreign_keys(1)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// sqlite takes one writer at a time, and an in-memory database lives in
	// one connection
	db.SetMaxOpenConns(1)
	s := &Store{DB: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.DB.Close() }

// Ping checks the database answers.
func (s *Store) Ping(ctx context.Context) error { return s.DB.PingContext(ctx) }

// migrate applies the migrations not yet applied, in order. Each is named for
// its version, like 001_meta.sql.
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, `create table if not exists migrations (version integer primary key)`); err != nil {
		return err
	}
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	for _, f := range files {
		version, err := strconv.Atoi(strings.SplitN(path.Base(f), "_", 2)[0])
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		var n int
		if err := s.DB.QueryRowContext(ctx, `select count(*) from migrations where version = ?`, version).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		b, err := migrations.ReadFile(f)
		if err != nil {
			return err
		}
		err = s.Tx(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, string(b)); err != nil {
				return fmt.Errorf("%s: %w", f, err)
			}
			_, err := tx.ExecContext(ctx, `insert into migrations (version) values (?)`, version)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Tx runs fn in a transaction, committing if it returns nil.
func (s *Store) Tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Meta reads a value from the meta table, "" when it's missing.
func (s *Store) Meta(ctx context.Context, key string) (string, error) {
	var v string
	err := s.DB.QueryRowContext(ctx, `select value from meta where key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// SetMeta writes a value to the meta table.
func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	_, err := s.DB.ExecContext(ctx, `insert into meta (key, value) values (?, ?)
		on conflict (key) do update set value = excluded.value`, key, value)
	return err
}
