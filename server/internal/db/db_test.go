package db

import (
	"bytes"
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
		"DROP TABLE problems", "DROP TABLE watch_state", "DROP TABLE media_keyframes",
		"DROP TABLE settings", "DROP TABLE sidecar_files", "DROP TABLE file_identifications",
		"DROP TABLE item_credits", "DROP TABLE people", "DROP TABLE match_candidates",
		"DROP TABLE item_metadata", "DROP TABLE artwork", "DROP TABLE collections",
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

// metadataTables are the tables migration 0004 creates, children first.
var metadataTables = []string{
	"settings", "sidecar_files", "file_identifications", "item_credits", "people",
	"match_candidates", "item_metadata", "artwork", "collections",
}

// TestMetadataMigrationUpgradesLibraries rolls a database back to "0003
// applied" with an existing library, then upgrades it: the library gets the
// default metadata language and region (F005 R10).
func TestMetadataMigrationUpgradesLibraries(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	d, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	stmts := make([]string, 0, len(metadataTables)+3)
	for _, name := range metadataTables {
		stmts = append(stmts, "DROP TABLE "+name)
	}
	stmts = append(stmts,
		"ALTER TABLE libraries DROP COLUMN metadata_language",
		"ALTER TABLE libraries DROP COLUMN metadata_region",
		"DELETE FROM schema_migrations WHERE name = 'migrations/0004_metadata.sql'",
	)
	for _, stmt := range stmts {
		if _, err := d.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	now := "2026-10-27T12:00:00Z"
	if _, err := d.ExecContext(ctx, `INSERT INTO libraries (id, name, type, created_at, updated_at) VALUES ('l1', 'Movies', 'movies', ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	d.Close()

	d, err = Open(ctx, dir)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer d.Close()
	var lang, region string
	if err := d.QueryRowContext(ctx, `SELECT metadata_language, metadata_region FROM libraries WHERE id = 'l1'`).Scan(&lang, &region); err != nil {
		t.Fatal(err)
	}
	if lang != "en" || region != "US" {
		t.Errorf("defaults = %s/%s, want en/US", lang, region)
	}
	for _, name := range metadataTables {
		var n int
		if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&n); err != nil || n != 1 {
			t.Errorf("table %s missing after upgrade (%v)", name, err)
		}
	}
}

// TestMetadataSchemaCascades checks the deletes F005 relies on: removing an
// item removes its metadata and credits, removing a library its collections.
func TestMetadataSchemaCascades(t *testing.T) {
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
	now := "2026-10-27T12:00:00Z"
	exec(`INSERT INTO libraries (id, name, type, created_at, updated_at) VALUES ('l1', 'Movies', 'movies', ?, ?)`, now, now)
	exec(`INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES ('i1', 'l1', 'movie', 'Alien', ?, ?)`, now, now)
	exec(`INSERT INTO collections (id, library_id, tmdb_id, name, updated_at) VALUES ('c1', 'l1', 8091, 'Alien Collection', ?)`, now)
	exec(`INSERT INTO item_metadata (item_id, state, tmdb_id, collection_id, updated_at) VALUES ('i1', 'matched', 348, 'c1', ?)`, now)
	exec(`INSERT INTO people (id, tmdb_id, name, updated_at) VALUES ('p1', 10205, 'Sigourney Weaver', ?)`, now)
	exec(`INSERT INTO item_credits (item_id, person_id, role, character, ord) VALUES ('i1', 'p1', 'cast', 'Ripley', 0)`)
	if _, err := d.ExecContext(ctx, `INSERT INTO item_metadata (item_id, state, updated_at) VALUES ('i1', 'bogus', ?)`, now); err == nil {
		t.Error("invalid state accepted")
	}

	exec(`DELETE FROM items WHERE id = 'i1'`)
	for _, table := range []string{"item_metadata", "item_credits"} {
		var n int
		if err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil || n != 0 {
			t.Errorf("deleting the item left %d rows in %s (%v)", n, table, err)
		}
	}
	exec(`DELETE FROM libraries WHERE id = 'l1'`)
	var n int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM collections`).Scan(&n); err != nil || n != 0 {
		t.Errorf("deleting the library left %d collections (%v)", n, err)
	}
}

func TestCheckPermissions(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	path := filepath.Join(dir, "plaid.db")
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("new database mode = %v (%v), want 0600", st.Mode().Perm(), err)
	}
	for _, c := range []struct {
		mode os.FileMode
		warn bool
	}{{0o600, false}, {0o644, true}, {0o660, true}} {
		if err := os.Chmod(path, c.mode); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		CheckPermissions(dir, slog.New(slog.NewTextHandler(&buf, nil)))
		if got := strings.Contains(buf.String(), "readable by other users"); got != c.warn {
			t.Errorf("mode %04o: warning = %v, want %v (log: %q)", c.mode, got, c.warn, buf.String())
		}
	}
}

// TestPlaybackMigrationUpgrades rolls a database back to "0004 applied"
// with a profile and an item, upgrades it, and checks the cascades F006
// relies on: deleting a profile or an item removes its watch state, and
// deleting a file only forgets it as the last file used (R33).
func TestPlaybackMigrationUpgrades(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	d, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		"DROP TABLE watch_state",
		"DROP TABLE media_keyframes",
		"DELETE FROM schema_migrations WHERE name = 'migrations/0005_playback.sql'",
	} {
		if _, err := d.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	now := "2026-10-27T12:00:00Z"
	exec(`INSERT INTO users (id, username, display_name, password_hash, role, created_at, updated_at) VALUES ('u1', 'bob', 'Bob', 'h', 'admin', ?, ?)`, now, now)
	exec(`INSERT INTO profiles (id, user_id, name, is_default, created_at, updated_at) VALUES ('p1', 'u1', 'Bob', 1, ?, ?)`, now, now)
	exec(`INSERT INTO profiles (id, user_id, name, created_at, updated_at) VALUES ('p2', 'u1', 'Kids', ?, ?)`, now, now)
	exec(`INSERT INTO libraries (id, name, type, created_at, updated_at) VALUES ('l1', 'Movies', 'movies', ?, ?)`, now, now)
	exec(`INSERT INTO library_folders (id, library_id, path, created_at) VALUES ('f1', 'l1', '/media/movies', ?)`, now)
	exec(`INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES ('i1', 'l1', 'movie', 'Alien', ?, ?)`, now, now)
	exec(`INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES ('i2', 'l1', 'movie', 'Aliens', ?, ?)`, now, now)
	exec(`INSERT INTO media_files (id, library_id, folder_id, path, size, mtime, parse_status, created_at, updated_at) VALUES ('m1', 'l1', 'f1', '/media/movies/Alien (1979).mkv', 1, ?, 'ok', ?, ?)`, now, now, now)
	d.Close()

	d, err = Open(ctx, dir)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer d.Close()
	exec(`INSERT INTO watch_state (profile_id, item_id, position_ms, last_file_id, updated_at) VALUES ('p1', 'i1', 60000, 'm1', ?)`, now)
	exec(`INSERT INTO watch_state (profile_id, item_id, position_ms, updated_at) VALUES ('p2', 'i2', 60000, ?)`, now)
	exec(`INSERT INTO watch_state (profile_id, item_id, position_ms, updated_at) VALUES ('p1', 'i2', 60000, ?)`, now)
	exec(`INSERT INTO media_keyframes (file_id, file_mtime, keyframes, created_at) VALUES ('m1', ?, x'00', ?)`, now, now)

	count := func(q string) int {
		t.Helper()
		var n int
		if err := d.QueryRowContext(ctx, q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	exec(`DELETE FROM media_files WHERE id = 'm1'`)
	if n := count(`SELECT COUNT(*) FROM watch_state WHERE item_id = 'i1' AND last_file_id IS NULL`); n != 1 {
		t.Errorf("deleting the file: %d watch rows kept without it, want 1", n)
	}
	if n := count(`SELECT COUNT(*) FROM media_keyframes`); n != 0 {
		t.Errorf("deleting the file left %d keyframe rows", n)
	}
	exec(`DELETE FROM profiles WHERE id = 'p2'`)
	if n := count(`SELECT COUNT(*) FROM watch_state WHERE profile_id = 'p2'`); n != 0 {
		t.Errorf("deleting the profile left %d watch rows", n)
	}
	exec(`DELETE FROM items WHERE id = 'i1'`)
	if n := count(`SELECT COUNT(*) FROM watch_state`); n != 1 {
		t.Errorf("after deleting the item, %d watch rows, want 1 (p1 on i2)", n)
	}
}
