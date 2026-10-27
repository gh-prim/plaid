// Package db opens the SQLite database and applies embedded migrations.
package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite" // pure Go driver, keeps the binary CGO-free
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens (or creates) dataDir/plaid.db in WAL mode and runs pending migrations.
func Open(ctx context.Context, dataDir string) (*sql.DB, error) {
	path := filepath.Join(dataDir, "plaid.db")
	// Transactions take the write lock up front (_txlock=immediate): a
	// deferred transaction that reads then writes can fail with SQLITE_BUSY
	// on the lock upgrade, which busy_timeout doesn't retry. It also makes
	// check-then-write transactions (first-run setup, last admin) atomic.
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_txlock=immediate"

	// A new database is only readable by its owner (CheckPermissions).
	// SQLite gives its -wal and -shm files the same mode.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // the configured data directory
	switch {
	case err == nil:
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("create %s: %w", path, err)
		}
	case !errors.Is(err, fs.ErrExist):
		return nil, fmt.Errorf("create %s: %w", path, err)
	}

	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := d.PingContext(ctx); err != nil {
		d.Close()
		return nil, fmt.Errorf("ping %s: %w", path, err)
	}
	if err := migrate(ctx, d); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// migrate applies every migrations/*.sql file not yet recorded in
// schema_migrations, in filename order, each in its own transaction.
func migrate(ctx context.Context, d *sql.DB) error {
	if _, err := d.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name       TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	for _, name := range names {
		var applied int
		if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE name = ?", name).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied > 0 {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (name) VALUES (?)", name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

// CheckPermissions warns when dataDir/plaid.db can be read or written by
// anyone but its owner: it holds session hashes and, since F005, the API
// keys an admin saves (F005 R34, ADR 0007). The expected mode is 0600.
func CheckPermissions(dataDir string, log *slog.Logger) {
	path := filepath.Join(dataDir, "plaid.db")
	st, err := os.Stat(path)
	if err != nil {
		log.Warn("can't check database permissions", "path", path, "err", err)
		return
	}
	if mode := st.Mode().Perm(); mode&0o077 != 0 {
		log.Warn("the database is readable by other users: run chmod 600 on it", "path", path, "mode", fmt.Sprintf("%04o", mode))
	}
}
