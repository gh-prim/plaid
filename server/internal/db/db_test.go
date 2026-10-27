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
		"DROP TABLE pairing_codes", "DROP TABLE sessions", "DROP TABLE profiles", "DROP TABLE users",
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

func TestAccountsSchema(t *testing.T) {
	ctx := context.Background()
	d, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	now := "2026-10-27T12:00:00.000Z"
	exec(`INSERT INTO users (id, username, display_name, password_hash, role, created_at, updated_at) VALUES ('u1', 'Bob', 'Bob', 'h', 'admin', ?, ?)`, now, now)
	if _, err := d.ExecContext(ctx, `INSERT INTO users (id, username, display_name, password_hash, role, created_at, updated_at) VALUES ('u2', 'bob', 'Bob 2', 'h', 'member', ?, ?)`, now, now); err == nil {
		t.Error("usernames differing only by case were both accepted")
	}
	exec(`INSERT INTO profiles (id, user_id, name, is_default, created_at, updated_at) VALUES ('p1', 'u1', 'Bob', 1, ?, ?)`, now, now)
	if _, err := d.ExecContext(ctx, `INSERT INTO profiles (id, user_id, name, is_default, created_at, updated_at) VALUES ('p2', 'u1', 'Kids', 1, ?, ?)`, now, now); err == nil {
		t.Error("two default profiles accepted for one user")
	}
	exec(`INSERT INTO sessions (id, user_id, profile_id, token_hash, device_name, device_type, created_at, last_seen_at, expires_at) VALUES ('s1', 'u1', 'p1', x'00', 'Firefox', 'web', ?, ?, ?)`, now, now, now)
	exec(`DELETE FROM profiles WHERE id = 'p1'`)
	var profileID *string
	if err := d.QueryRowContext(ctx, `SELECT profile_id FROM sessions WHERE id = 's1'`).Scan(&profileID); err != nil || profileID != nil {
		t.Errorf("deleting a profile: session profile_id = %v (%v), want NULL", profileID, err)
	}
	exec(`DELETE FROM users WHERE id = 'u1'`)
	var n int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil || n != 0 {
		t.Errorf("deleting a user left %d sessions (%v)", n, err)
	}
}
