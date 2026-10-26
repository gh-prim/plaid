// Package db opens the SQLite database and applies embedded migrations.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite" // pure Go driver, keeps the binary CGO-free
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens (or creates) dataDir/plaid.db in WAL mode and runs pending migrations.
func Open(ctx context.Context, dataDir string) (*sql.DB, error) {
	path := filepath.Join(dataDir, "plaid.db")
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"

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
