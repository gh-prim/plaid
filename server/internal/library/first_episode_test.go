package library

import (
	"context"
	"errors"
	"testing"
	"time"
)

// The hero preview of a show comes from its first episode on disk:
// season 1 before specials, the lowest episode number, missing files
// skipped.
func TestFirstEpisode(t *testing.T) {
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
	item := func(id, kind, parent string, season, episode any) {
		exec(`INSERT INTO items (id, library_id, kind, parent_id, season_number, episode_number, created_at, updated_at)
			VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?)`, id, lib.ID, kind, parent, season, episode, now, now)
	}
	file := func(itemID string, missing bool) {
		var gone any
		if missing {
			gone = now
		}
		exec(`INSERT INTO media_files (id, library_id, folder_id, path, size, mtime, parse_status, missing_since, created_at, updated_at)
			VALUES (?, ?, ?, ?, 1, ?, 'ok', ?, ?, ?)`, "f-"+itemID, lib.ID, lib.Folders[0].ID, "/media/shows/"+itemID+".mkv", now, gone, now, now)
		exec(`INSERT INTO item_files (item_id, file_id) VALUES (?, ?)`, itemID, "f-"+itemID)
	}
	item("show", "show", "", nil, nil)
	item("specials", "season", "show", 0, nil)
	item("s2", "season", "show", 2, nil)
	item("s1", "season", "show", 1, nil)
	item("sp1", "episode", "specials", nil, 1)
	item("s2e1", "episode", "s2", nil, 1)
	item("s1e1", "episode", "s1", nil, 1)
	item("s1e2", "episode", "s1", nil, 2)
	for _, id := range []string{"sp1", "s2e1", "s1e2"} {
		file(id, false)
	}
	file("s1e1", true)

	if got, err := s.FirstEpisode(ctx, "show"); err != nil || got != "s1e2" {
		t.Errorf("first episode = %q, %v; want s1e2 (s1e1 is missing, specials last)", got, err)
	}
	item("empty", "show", "", nil, nil)
	if _, err := s.FirstEpisode(ctx, "empty"); !errors.Is(err, ErrNotFound) {
		t.Errorf("show without episodes: %v, want ErrNotFound", err)
	}
}
