package db

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesWALDatabase(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	d, err := Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	if _, err := os.Stat(filepath.Join(dir, "plaid.db")); err != nil {
		t.Fatalf("plaid.db not created: %v", err)
	}
	var mode string
	if err := d.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
	if err := d.PingContext(ctx); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}

	for i := range 2 {
		d, err := Open(ctx, dir)
		if err != nil {
			t.Fatalf("Open #%d: %v", i+1, err)
		}
		var n int
		if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != len(files) {
			t.Errorf("open #%d: %d migrations recorded, want %d", i+1, n, len(files))
		}
		d.Close()
	}
}

// TestMigrationsApplyIncrementally opens a database that only has the first
// migration, as an existing install would, then upgrades it.
func TestMigrationsApplyIncrementally(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	d, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	// Roll back to "only 0001 applied".
	for _, stmt := range []string{
		"DROP TABLE media_streams", "DROP TABLE item_files", "DROP TABLE media_files",
		"DROP TABLE items", "DROP TABLE library_folders", "DROP TABLE libraries",
		"DELETE FROM schema_migrations WHERE name <> 'migrations/0001_init.sql'",
	} {
		if _, err := d.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	d.Close()

	d, err = Open(ctx, dir)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer d.Close()
	var n int
	if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'media_files'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Error("media_files missing after upgrade")
	}
}
