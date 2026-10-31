package library

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func count(t *testing.T, d *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := d.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestStoreCreateGetDelete(t *testing.T) {
	d := openTestDB(t)
	s := NewStore(d)
	ctx := context.Background()

	lib, err := s.CreateLibrary(ctx, "Movies", TypeMovies, []string{"/media/a", "/media/b"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetLibrary(ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Movies" || got.Type != TypeMovies || len(got.Folders) != 2 || got.Folders[0].Status != FolderOK {
		t.Errorf("got %+v", got)
	}
	if got.MetadataLanguage != "en" || got.MetadataRegion != "US" {
		t.Errorf("metadata locale = %s/%s, want the en/US defaults", got.MetadataLanguage, got.MetadataRegion)
	}
	fr, err := s.CreateLibrary(ctx, "Films", TypeMovies, []string{"/media/fr"}, CreateOptions{MetadataLanguage: "fr", MetadataRegion: "FR"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.GetLibrary(ctx, fr.ID); err != nil || got.MetadataLanguage != "fr" || got.MetadataRegion != "FR" {
		t.Errorf("fr library = %+v (%v)", got, err)
	}
	if _, err := s.GetLibrary(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing library: %v", err)
	}
	// UNIQUE(path) on folders.
	if _, err := s.CreateLibrary(ctx, "Other", TypeMovies, []string{"/media/a"}); err == nil {
		t.Error("two libraries share a folder path")
	}
	if n := count(t, d, "SELECT COUNT(*) FROM libraries"); n != 2 {
		t.Errorf("failed create left %d libraries, want 2", n)
	}
}

func TestStoreDeleteCascades(t *testing.T) {
	d := openTestDB(t)
	s := NewStore(d)
	ctx := context.Background()
	lib, err := s.CreateLibrary(ctx, "Shows", TypeShows, []string{"/media/shows"})
	if err != nil {
		t.Fatal(err)
	}
	now := ts(time.Now())
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES ('show', ?, 'show', 'S', ?, ?)`, lib.ID, now, now)
	exec(`INSERT INTO items (id, library_id, kind, parent_id, season_number, created_at, updated_at) VALUES ('season', ?, 'season', 'show', 1, ?, ?)`, lib.ID, now, now)
	exec(`INSERT INTO items (id, library_id, kind, parent_id, episode_number, created_at, updated_at) VALUES ('ep', ?, 'episode', 'season', 1, ?, ?)`, lib.ID, now, now)
	exec(`INSERT INTO media_files (id, library_id, folder_id, path, size, mtime, parse_status, created_at, updated_at) VALUES ('f', ?, ?, '/media/shows/S/a.mkv', 1, ?, 'ok', ?, ?)`, lib.ID, lib.Folders[0].ID, now, now, now)
	exec(`INSERT INTO item_files (item_id, file_id) VALUES ('ep', 'f')`)
	exec(`INSERT INTO media_streams (file_id, idx, type) VALUES ('f', 0, 'video')`)

	// UNIQUE(path) on files.
	if _, err := d.ExecContext(ctx, `INSERT INTO media_files (id, library_id, folder_id, path, size, mtime, parse_status, created_at, updated_at) VALUES ('g', ?, ?, '/media/shows/S/a.mkv', 1, ?, 'ok', ?, ?)`, lib.ID, lib.Folders[0].ID, now, now, now); err == nil {
		t.Error("two files share a path")
	}

	if err := s.DeleteLibrary(ctx, lib.ID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"libraries", "library_folders", "items", "media_files", "item_files", "media_streams"} {
		if n := count(t, d, "SELECT COUNT(*) FROM "+table); n != 0 {
			t.Errorf("%s: %d rows left after delete", table, n)
		}
	}
	if err := s.DeleteLibrary(ctx, lib.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
}

func TestBatchCommitsEvery500Rows(t *testing.T) {
	d := openTestDB(t)
	s := NewStore(d)
	ctx := context.Background()
	lib, err := s.CreateLibrary(ctx, "Movies", TypeMovies, []string{"/m"})
	if err != nil {
		t.Fatal(err)
	}
	commits := 0
	s.afterCommit = func() error { commits++; return nil }
	s.batchHold = time.Hour // rows only; -race is slow enough to hit 250 ms
	b := s.NewBatch(ctx)
	now := ts(time.Now())
	for i := range 1200 {
		if err := b.Exec(`INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES (?, ?, 'movie', 't', ?, ?)`,
			newID(), lib.ID, now, now); err != nil {
			t.Fatal(err)
		}
		if err := b.EndUnit(); err != nil {
			t.Fatal(err)
		}
		if i == 499 && commits != 1 {
			t.Errorf("after 500 rows: %d commits", commits)
		}
	}
	if err := b.Commit(); err != nil {
		t.Fatal(err)
	}
	if commits != 3 {
		t.Errorf("commits = %d, want 3 (500 + 500 + 200)", commits)
	}
	if n := count(t, d, "SELECT COUNT(*) FROM items"); n != 1200 {
		t.Errorf("items = %d", n)
	}
}
