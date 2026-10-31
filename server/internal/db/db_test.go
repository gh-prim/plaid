package db

import (
	"bytes"
	"context"
	"database/sql"
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
	for _, stmt := range append(catalogRollback[:len(catalogRollback):len(catalogRollback)],
		"DROP TABLE item_segments", "DROP TABLE problems", "DROP TABLE watch_state", "DROP TABLE media_keyframes",
		"DROP TABLE settings", "DROP TABLE sidecar_files", "DROP TABLE file_identifications",
		"DROP TABLE item_credits", "DROP TABLE people", "DROP TABLE match_candidates",
		"DROP TABLE item_metadata", "DROP TABLE artwork", "DROP TABLE collections",
		"DROP TABLE media_streams", "DROP TABLE item_files", "DROP TABLE media_files",
		"DROP TABLE items", "DROP TABLE library_folders", "DROP TABLE libraries",
		"DROP TABLE pairing_codes", "DROP TABLE sessions", "DROP TABLE profiles", "DROP TABLE users",
		"DELETE FROM schema_migrations WHERE name <> 'migrations/0001_init.sql'",
	) {
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
	// 0007's triggers read item_metadata: roll it back too, both are re-applied.
	stmts := append(make([]string, 0, len(catalogRollback)+len(metadataTables)+5), catalogRollback...)
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

// F008's search index needs FTS5 with diacritic folding and prefix indexes,
// on every architecture we ship (modernc.org/sqlite is pure Go).
func TestFTS5Search(t *testing.T) {
	ctx := context.Background()
	d, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	var n int
	if err := d.QueryRowContext(ctx,
		"SELECT count(*) FROM pragma_compile_options WHERE compile_options = 'ENABLE_FTS5'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("SQLite is built without ENABLE_FTS5")
	}

	for _, q := range []string{
		`CREATE VIRTUAL TABLE fts_probe USING fts5(title, tokenize = 'unicode61 remove_diacritics 2', prefix = '1 2 3')`,
		`INSERT INTO fts_probe(rowid, title) VALUES (1, 'Le Fabuleux Destin d''Amélie Poulain'), (2, 'The Lord of the Rings'), (3, 'Casablanca')`,
	} {
		if _, err := d.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for match, want := range map[string]int64{`"amel"*`: 1, `"AMÉLIE"`: 1, `"rings"*`: 2, `"lord"* "ri"*`: 2, `"casa"*`: 3} {
		var got int64
		if err := d.QueryRowContext(ctx, "SELECT rowid FROM fts_probe WHERE fts_probe MATCH ?", match).Scan(&got); err != nil {
			t.Errorf("MATCH %s: %v", match, err)
			continue
		}
		if got != want {
			t.Errorf("MATCH %s = row %d, want %d", match, got, want)
		}
	}
}

// catalogFixture opens a database and returns helpers to write and query it.
func catalogFixture(t *testing.T, dir string) (*sql.DB, func(string, ...any), func(string, ...any) int) {
	t.Helper()
	ctx := context.Background()
	d, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	count := func(q string, args ...any) int {
		t.Helper()
		var n int
		if err := d.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	return d, exec, count
}

const catalogNow = "2026-10-30T12:00:00Z"

func seedCatalog(exec func(string, ...any)) {
	exec(`INSERT INTO libraries (id, name, type, created_at, updated_at) VALUES ('lm', 'Movies', 'movies', ?, ?), ('ls', 'Shows', 'shows', ?, ?)`, catalogNow, catalogNow, catalogNow, catalogNow)
	exec(`INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES ('m1', 'lm', 'movie', 'Amelie', '2026-10-01T00:00:00Z', ?)`, catalogNow)
	exec(`INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES ('s1', 'ls', 'show', 'Dark', '2026-09-01T00:00:00Z', ?)`, catalogNow)
	exec(`INSERT INTO items (id, library_id, kind, parent_id, season_number, created_at, updated_at) VALUES ('s1x1', 'ls', 'season', 's1', 1, '2026-09-01T00:00:00Z', ?)`, catalogNow)
	exec(`INSERT INTO items (id, library_id, kind, parent_id, season_number, episode_number, title, created_at, updated_at) VALUES ('e1', 'ls', 'episode', 's1x1', 1, 1, 'Secrets', '2026-09-01T00:00:00Z', ?)`, catalogNow)
}

// search returns the ref_ids matching an FTS5 query, in rowid order.
func search(t *testing.T, d *sql.DB, match string) []string {
	t.Helper()
	rows, err := d.QueryContext(context.Background(), "SELECT ref_id FROM search_index WHERE search_index MATCH ? ORDER BY rowid", match)
	if err != nil {
		t.Fatalf("MATCH %s: %v", match, err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestCatalogTriggers(t *testing.T) {
	d, exec, count := catalogFixture(t, t.TempDir())
	seedCatalog(exec)

	// Items: movies, shows and episodes are indexed by their parsed title; seasons aren't.
	if got := search(t, d, `"amel"*`); len(got) != 1 || got[0] != "m1" {
		t.Errorf("parsed title: amel* = %v, want [m1]", got)
	}
	if got := search(t, d, `"secr"*`); len(got) != 1 || got[0] != "e1" {
		t.Errorf("episode: secr* = %v, want [e1]", got)
	}
	if n := count(`SELECT COUNT(*) FROM search_docs WHERE ref_id = 's1x1'`); n != 0 {
		t.Error("a season is indexed")
	}
	if n := count(`SELECT COUNT(*) FROM search_docs WHERE ref_id = 'e1' AND library_id = 'ls' AND kind = 'episode'`); n != 1 {
		t.Error("episode doc has the wrong kind or library")
	}

	// Metadata: the matched title wins, the original title is indexed, genres land in item_genres (id 0 skipped).
	exec(`INSERT INTO item_metadata (item_id, state, title, original_title, genres, updated_at) VALUES ('m1', 'matched', 'Le Fabuleux Destin d''Amélie Poulain', 'Amélie', '[{"id":35,"name":"Comédie"},{"id":10749,"name":"Romance"},{"id":0,"name":"Edited"}]', ?)`, catalogNow)
	if got := search(t, d, `"fabul"*`); len(got) != 1 || got[0] != "m1" {
		t.Errorf("matched title: fabul* = %v, want [m1]", got)
	}
	if got := search(t, d, `"AMELIE"`); len(got) != 1 {
		t.Errorf("original title, accents and case folded: %v", got)
	}
	if n := count(`SELECT COUNT(*) FROM search_index`); n != 3 {
		t.Errorf("%d index rows after a metadata write, want 3 (no duplicate)", n)
	}
	if n := count(`SELECT COUNT(*) FROM item_genres WHERE item_id = 'm1'`); n != 2 {
		t.Errorf("%d genres, want 2 (id 0 skipped)", n)
	}
	if n := count(`SELECT COUNT(*) FROM item_genres WHERE item_id = 'm1' AND genre_id = 35 AND name = 'Comédie'`); n != 1 {
		t.Error("genre id or name not kept")
	}

	exec(`UPDATE item_metadata SET title = 'Amélie from Montmartre', genres = '[{"id":18,"name":"Drama"}]' WHERE item_id = 'm1'`)
	if got := search(t, d, `"fabul"*`); len(got) != 0 {
		t.Errorf("old title still found: %v", got)
	}
	if got := search(t, d, `"montm"*`); len(got) != 1 {
		t.Errorf("new title not found: %v", got)
	}
	if n := count(`SELECT COUNT(*) FROM item_genres WHERE item_id = 'm1'`); n != 1 {
		t.Errorf("%d genres after update, want 1", n)
	}
	exec(`UPDATE item_metadata SET genres = 'not json' WHERE item_id = 'm1'`)
	if n := count(`SELECT COUNT(*) FROM item_genres WHERE item_id = 'm1'`); n != 0 {
		t.Errorf("invalid genres JSON left %d rows", n)
	}

	// Removing the metadata falls back to the parsed title.
	exec(`DELETE FROM item_metadata WHERE item_id = 'm1'`)
	if got := search(t, d, `"montm"*`); len(got) != 0 {
		t.Errorf("metadata title still found after delete: %v", got)
	}
	if got := search(t, d, `"amel"*`); len(got) != 1 {
		t.Errorf("parsed title lost after metadata delete: %v", got)
	}

	// A renamed item (rescan) is reindexed.
	exec(`UPDATE items SET title = 'Amelie Poulain' WHERE id = 'm1'`)
	if got := search(t, d, `"poul"*`); len(got) != 1 {
		t.Errorf("renamed item not found: %v", got)
	}

	// Shows: newest_child_added_at follows episode inserts, never goes back.
	if n := count(`SELECT COUNT(*) FROM items WHERE id = 's1' AND newest_child_added_at = '2026-09-01T00:00:00Z'`); n != 1 {
		t.Error("show's newest_child_added_at not set by its first episode")
	}
	exec(`INSERT INTO items (id, library_id, kind, parent_id, season_number, episode_number, title, created_at, updated_at) VALUES ('e2', 'ls', 'episode', 's1x1', 1, 2, 'Lies', '2026-10-20T00:00:00Z', ?)`, catalogNow)
	exec(`INSERT INTO items (id, library_id, kind, parent_id, season_number, episode_number, title, created_at, updated_at) VALUES ('e0', 'ls', 'episode', 's1x1', 1, 0, 'Old', '2026-08-01T00:00:00Z', ?)`, catalogNow)
	if n := count(`SELECT COUNT(*) FROM items WHERE id = 's1' AND newest_child_added_at = '2026-10-20T00:00:00Z'`); n != 1 {
		t.Error("show's newest_child_added_at not the newest episode's")
	}

	// People.
	exec(`INSERT INTO people (id, name, updated_at) VALUES ('p1', 'Audrey Tautou', ?)`, catalogNow)
	if got := search(t, d, `"taut"*`); len(got) != 1 || got[0] != "p1" {
		t.Errorf("person: %v", got)
	}
	exec(`UPDATE people SET name = 'Audrey Justine Tautou' WHERE id = 'p1'`)
	if got := search(t, d, `"justi"*`); len(got) != 1 {
		t.Errorf("renamed person: %v", got)
	}
	exec(`DELETE FROM people WHERE id = 'p1'`)
	if n := count(`SELECT COUNT(*) FROM search_docs WHERE kind = 'person'`); n != 0 {
		t.Error("deleted person still has a doc")
	}

	// Deleting a show cascades to its episodes' docs; deleting a library to everything.
	exec(`DELETE FROM items WHERE id = 's1'`)
	if n := count(`SELECT COUNT(*) FROM search_docs WHERE library_id = 'ls'`); n != 0 {
		t.Errorf("%d docs left after deleting the show", n)
	}
	exec(`DELETE FROM libraries WHERE id = 'lm'`)
	if n := count(`SELECT COUNT(*) FROM search_docs`); n != 0 {
		t.Errorf("%d docs left after deleting the library", n)
	}
	if n := count(`SELECT COUNT(*) FROM search_index`); n != 0 {
		t.Errorf("%d index rows left, want 0", n)
	}
}

// TestCatalogMigrationBackfills upgrades a database written before 0007.
func TestCatalogMigrationBackfills(t *testing.T) {
	dir := t.TempDir()
	d, exec, _ := catalogFixture(t, dir)
	for _, stmt := range catalogRollback {
		exec(stmt)
	}
	seedCatalog(exec)
	exec(`INSERT INTO item_metadata (item_id, state, title, genres, updated_at) VALUES ('m1', 'matched', 'Amélie', '[{"id":35,"name":"Comedy"}]', ?)`, catalogNow)
	exec(`INSERT INTO people (id, name, updated_at) VALUES ('p1', 'Audrey Tautou', ?)`, catalogNow)
	d.Close()

	d, _, count := catalogFixture(t, dir)
	if got := search(t, d, `"amel"*`); len(got) != 1 || got[0] != "m1" {
		t.Errorf("backfilled movie: %v", got)
	}
	if got := search(t, d, `"taut"*`); len(got) != 1 {
		t.Errorf("backfilled person: %v", got)
	}
	if n := count(`SELECT COUNT(*) FROM search_docs`); n != 4 {
		t.Errorf("%d docs, want 4 (movie, show, episode, person)", n)
	}
	if n := count(`SELECT COUNT(*) FROM item_genres WHERE item_id = 'm1' AND genre_id = 35`); n != 1 {
		t.Error("genres not backfilled")
	}
	if n := count(`SELECT COUNT(*) FROM items WHERE id = 's1' AND newest_child_added_at = '2026-09-01T00:00:00Z'`); n != 1 {
		t.Error("newest_child_added_at not backfilled")
	}
}

// catalogRollback undoes 0007 (its triggers go with the tables they're on, except items').
var catalogRollback = []string{
	"DROP TRIGGER items_search_insert", "DROP TRIGGER items_search_update", "DROP TRIGGER items_search_delete",
	"DROP TRIGGER items_newest_child_insert", "DROP TRIGGER items_newest_child_move",
	"DROP TRIGGER item_metadata_catalog_insert", "DROP TRIGGER item_metadata_catalog_update", "DROP TRIGGER item_metadata_catalog_delete",
	"DROP TRIGGER people_search_insert", "DROP TRIGGER people_search_update", "DROP TRIGGER people_search_delete",
	"DROP INDEX items_library_added", "DROP TABLE item_genres", "DROP TABLE search_index", "DROP TABLE search_docs",
	"ALTER TABLE items DROP COLUMN newest_child_added_at",
	"DELETE FROM schema_migrations WHERE name = 'migrations/0007_catalog.sql'",
	"DELETE FROM schema_migrations WHERE name = 'migrations/0008_catalog_triggers.sql'",
}

// An upsert that rewrites the same name or title (F005 writes a movie's
// whole cast each time) leaves the search index alone: rewriting it held
// the write lock for seconds on modest hardware (0008).
func TestCatalogTriggersSkipUnchanged(t *testing.T) {
	d, exec, count := catalogFixture(t, t.TempDir())
	seedCatalog(exec)
	exec(`INSERT INTO item_metadata (item_id, state, title, original_title, genres, updated_at) VALUES ('m1', 'matched', 'Amélie', 'Amélie', '[{"id":35,"name":"Comédie"}]', ?)`, catalogNow)
	changes := func() int { return count(`SELECT total_changes()`) }

	before := changes()
	exec(`UPDATE item_metadata SET title = 'Amélie', original_title = 'Amélie', genres = '[{"id":35,"name":"Comédie"}]' WHERE item_id = 'm1'`)
	if n := changes() - before; n != 1 {
		t.Errorf("an unchanged metadata update made %d changes, want 1 (the row only)", n)
	}
	before = changes()
	exec(`UPDATE items SET title = title WHERE id = 'm1'`)
	if n := changes() - before; n != 1 {
		t.Errorf("an unchanged title update made %d changes, want 1", n)
	}
	// A real change still reindexes.
	exec(`UPDATE item_metadata SET title = 'Le Fabuleux Destin' WHERE item_id = 'm1'`)
	if got := search(t, d, `"fabul"*`); len(got) != 1 {
		t.Errorf("changed title not indexed: %v", got)
	}
}
