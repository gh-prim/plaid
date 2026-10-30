package catalog

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gh-prim/plaid/server/internal/access"
)

// seedShow: Dark with seasons 1 (3 episodes), 2 (2 episodes) and specials
// (1). Like the scanner, episodes carry no season number of their own.
func seedShow(f *fixture) {
	f.exec(`INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES ('dark', 'ls', 'show', 'Dark', ?, ?)`, now, now)
	for _, s := range []struct{ n, eps int }{{1, 3}, {2, 2}, {0, 1}} {
		sid := fmt.Sprintf("s%d", s.n)
		f.exec(`INSERT INTO items (id, library_id, kind, parent_id, season_number, created_at, updated_at) VALUES (?, 'ls', 'season', 'dark', ?, ?, ?)`, sid, s.n, now, now)
		for e := 1; e <= s.eps; e++ {
			f.exec(`INSERT INTO items (id, library_id, kind, parent_id, episode_number, created_at, updated_at) VALUES (?, 'ls', 'episode', ?, ?, ?, ?)`,
				fmt.Sprintf("s%de%d", s.n, e), sid, e, now, now)
		}
	}
	f.exec(`INSERT INTO item_metadata (item_id, state, age_rating, updated_at) SELECT id, 'matched', 16, ? FROM items WHERE library_id = 'ls'`, now)
}

func (f *fixture) upNext(id string) string {
	f.t.Helper()
	got, err := f.c.UpNext(context.Background(), id, "p1", access.Limit{})
	if err != nil {
		f.t.Fatalf("UpNext(%s): %v", id, err)
	}
	return got
}

func TestUpNext(t *testing.T) {
	f := newFixture(t)
	seedShow(f)

	// Nothing watched: the first episode, of the show and of a season.
	for id, want := range map[string]string{"dark": "s1e1", "s2": "s2e1", "s0": "s0e1"} {
		if got := f.upNext(id); got != want {
			t.Errorf("fresh %s: %q, want %q", id, got, want)
		}
	}
	// After an episode: the next one, across seasons, specials skipped; none after the last.
	for id, want := range map[string]string{"s1e1": "s1e2", "s1e3": "s2e1", "s2e2": "", "s0e1": ""} {
		if got := f.upNext(id); got != want {
			t.Errorf("after %s: %q, want %q", id, got, want)
		}
	}

	f.watch("p1", "s1e1", 0, true, "2026-10-01T00:00:00Z")
	f.watch("p1", "s1e2", 0, true, "2026-10-02T00:00:00Z")
	if got := f.upNext("dark"); got != "s1e3" {
		t.Errorf("after S1 E2: %q", got)
	}
	f.watch("p1", "s1e3", 0, true, "2026-10-03T00:00:00Z")
	if got := f.upNext("dark"); got != "s2e1" {
		t.Errorf("after S1 E3: %q", got)
	}
	// Rewatching S1 E1 later: the first unwatched after it is still S2 E1.
	f.exec(`UPDATE watch_state SET last_played_at = '2026-10-03T12:00:00Z' WHERE item_id = 's1e1'`)
	if got := f.upNext("dark"); got != "s2e1" {
		t.Errorf("after rewatching S1 E1: %q", got)
	}
	// An episode in progress wins.
	f.watch("p1", "s2e2", 120000, false, "2026-10-04T00:00:00Z")
	if got := f.upNext("dark"); got != "s2e2" {
		t.Errorf("in progress: %q", got)
	}
	if got := f.upNext("s1"); got != "" {
		t.Errorf("season 1 all watched: %q", got)
	}
	f.exec(`DELETE FROM watch_state`)

	// Every regular episode watched: nothing (specials don't count).
	for i, id := range []string{"s1e1", "s1e2", "s1e3", "s2e1", "s2e2"} {
		f.watch("p1", id, 0, true, fmt.Sprintf("2026-10-0%dT00:00:00Z", i+1))
	}
	if got := f.upNext("dark"); got != "" {
		t.Errorf("show done: %q", got)
	}

	// A hidden episode is skipped.
	f.exec(`DELETE FROM watch_state`)
	f.exec(`UPDATE items SET hidden = 1 WHERE id = 's1e2'`)
	if got := f.upNext("s1e1"); got != "s1e3" {
		t.Errorf("hidden S1 E2: %q", got)
	}
	// Above the age limit, or a movie: not found, or nothing.
	if _, err := f.c.UpNext(context.Background(), "dark", "p1", access.Limit{MaxAge: age(12)}); !errors.Is(err, ErrNotFound) {
		t.Errorf("kid on a 16 show: %v", err)
	}
	if _, err := f.c.UpNext(context.Background(), "nope", "p1", access.Limit{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
	seedMovies(f)
	if got := f.upNext("alien"); got != "" {
		t.Errorf("a movie: %q", got)
	}
}
