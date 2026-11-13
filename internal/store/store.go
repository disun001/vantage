// Package store owns the SQLite database: connection, migrations and the
// generated queries. It is the only importer of the driver and imports
// nothing from this repo.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/disun001/vantage/internal/store/gen"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Chat is a stored chat row.
type Chat = gen.Chat

// Event is a stored event row.
type Event = gen.Event

// Store is an open database. Safe for use from one process; other processes
// open their own Store (WAL mode plus busy_timeout make that work).
type Store struct {
	db *sql.DB
	q  *gen.Queries
}

// StateDir returns the directory holding the database:
// $VANTAGE_STATE_DIR > $XDG_STATE_HOME/vantage > ~/.local/state/vantage.
// It is the single lookup, like workspace.Dir (not os.UserConfigDir, which is
// wrong on macOS).
func StateDir() (string, error) {
	if d := os.Getenv("VANTAGE_STATE_DIR"); d != "" {
		return d, nil
	}
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "vantage"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("state dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "vantage"), nil
}

// Exists reports whether the database file is already there, so a reader such
// as the sidebar can avoid creating one for people who never use chat.
func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "vantage.db"))
	return err == nil
}

// Open opens (creating if needed) vantage.db in dir and applies migrations.
// The directory is 0700 and the file 0600: transcripts may hold private code.
func Open(ctx context.Context, dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	path := filepath.Join(dir, "vantage.db")
	if err := touch0600(path); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db, q: gen.New(db)}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

func touch0600(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create database file: %w", err)
	}
	return f.Close()
}

func migrate(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
