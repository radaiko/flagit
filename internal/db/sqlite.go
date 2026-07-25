// Package db owns the SQLite schema, migrations and all query access.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, no CGO
)

// timeLayout is a fixed-width UTC layout so timestamps sort lexicographically
// in SQLite. RFC3339Nano is not usable here: it trims trailing zeros, which
// breaks string ordering (and therefore the polling endpoint).
const timeLayout = "2006-01-02T15:04:05.000000000Z"

// FormatTime renders t as a sortable UTC timestamp string.
func FormatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

// ParseTime reads a timestamp written by FormatTime.
func ParseTime(s string) (time.Time, error) {
	return time.Parse(timeLayout, s)
}

// ParseFlexibleTime accepts anything a client may reasonably send as an
// ISO 8601 / RFC 3339 timestamp, plus the internal storage layout.
func ParseFlexibleTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	layouts := []string{timeLayout, time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp %q", s)
}

// DB wraps a SQLite connection pool plus the clock used for timestamps.
// Tests swap Now to get deterministic ordering.
type DB struct {
	sql *sql.DB
	Now func() time.Time
}

// InitDB opens (creating if needed) the SQLite database at path and runs
// migrations. Pass ":memory:" for an ephemeral database.
func InitDB(path string) (*DB, error) {
	if path != ":memory:" {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("create db directory: %w", err)
			}
		}
	}

	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	if path == ":memory:" {
		dsn = path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	}

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// A single connection keeps ":memory:" coherent (each new connection would
	// otherwise get its own empty database) and removes writer contention.
	sqlDB.SetMaxOpenConns(1)

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	d := &DB{sql: sqlDB, Now: time.Now}
	if err := d.RunMigrations(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return d, nil
}

// Close releases the underlying connection pool.
func (d *DB) Close() error { return d.sql.Close() }

// SQL exposes the raw handle, for tests and health checks.
func (d *DB) SQL() *sql.DB { return d.sql }

// migrations are applied in order; never edit one that has shipped, append instead.
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS tickets (
		id                TEXT PRIMARY KEY,
		type              TEXT NOT NULL,
		title             TEXT NOT NULL,
		body              TEXT NOT NULL DEFAULT '',
		status            TEXT NOT NULL DEFAULT 'open',
		app_name          TEXT NOT NULL DEFAULT '',
		app_version       TEXT NOT NULL DEFAULT '',
		os                TEXT NOT NULL DEFAULT '',
		platform          TEXT NOT NULL DEFAULT '',
		device_model      TEXT NOT NULL DEFAULT '',
		device_token_hash TEXT NOT NULL DEFAULT '',
		log_ring_buffer   TEXT NOT NULL DEFAULT '',
		logs_duration_min INTEGER NOT NULL DEFAULT 0,
		shipped_version   TEXT NOT NULL DEFAULT '',
		created_at        TEXT NOT NULL,
		updated_at        TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_tickets_updated_at ON tickets(updated_at);
	CREATE INDEX IF NOT EXISTS idx_tickets_app_name ON tickets(app_name);`,

	`CREATE TABLE IF NOT EXISTS messages (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		ticket_id  TEXT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
		body       TEXT NOT NULL,
		role       TEXT NOT NULL,
		created_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_messages_ticket ON messages(ticket_id, id);`,

	`CREATE TABLE IF NOT EXISTS commits (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		ticket_id   TEXT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
		commit_hash TEXT NOT NULL,
		branch      TEXT NOT NULL DEFAULT '',
		message     TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_commits_ticket ON commits(ticket_id, id);`,

	`CREATE TABLE IF NOT EXISTS apps (
		name                 TEXT PRIMARY KEY,
		auto_process_enabled INTEGER NOT NULL DEFAULT 0,
		created_at           TEXT NOT NULL
	);`,

	`CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	);`,
}

// RunMigrations brings the schema up to date. It is safe to call repeatedly.
func (d *DB) RunMigrations() error {
	if _, err := d.sql.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var applied int
	if err := d.sql.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&applied); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	for i := applied; i < len(migrations); i++ {
		if _, err := d.sql.Exec(migrations[i]); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := d.sql.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, i+1); err != nil {
			return fmt.Errorf("record migration %d: %w", i+1, err)
		}
	}
	return nil
}

// SchemaVersion reports the highest applied migration.
func (d *DB) SchemaVersion() (int, error) {
	var v int
	err := d.sql.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v)
	return v, err
}
