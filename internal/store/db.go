package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database used for incident history and health samples.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) a SQLite database at the given path and
// applies the schema migrations.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: create dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	// WAL is the safe default for concurrent readers + one writer.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: set WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: set busy_timeout: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS incidents (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			status      TEXT NOT NULL,
			impact      TEXT NOT NULL,
			created_at  DATETIME NOT NULL,
			resolved_at DATETIME,
			shortlink   TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_incidents_created ON incidents(created_at)`,
		`CREATE TABLE IF NOT EXISTS pending_pushes (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			refspec    TEXT NOT NULL,
			status     TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME NOT NULL,
			pushed_at  DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS push_history (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			remote    TEXT NOT NULL,
			refspec   TEXT NOT NULL,
			status    TEXT NOT NULL,
			err_msg   TEXT NOT NULL DEFAULT '',
			pushed_at DATETIME NOT NULL,
			failover  INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS health_samples (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint    TEXT NOT NULL,
			status_code INT NOT NULL,
			latency_ms  INT NOT NULL,
			checked_at  DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_health_checked ON health_samples(checked_at)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}
	return nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}
