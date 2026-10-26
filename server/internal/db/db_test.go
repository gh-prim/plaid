package db

import (
	"context"
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

	for i := range 2 {
		d, err := Open(ctx, dir)
		if err != nil {
			t.Fatalf("Open #%d: %v", i+1, err)
		}
		var n int
		if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("open #%d: %d migrations recorded, want 1", i+1, n)
		}
		d.Close()
	}
}
