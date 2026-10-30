package catalog

import (
	"context"
	"database/sql"
	"slices"
	"testing"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/db"
)

const now = "2026-10-30T12:00:00Z"

type fixture struct {
	t  *testing.T
	db *sql.DB
	c  *Catalog
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	d, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	f := &fixture{t: t, db: d, c: New(Options{DB: d})}
	f.exec(`INSERT INTO users (id, username, display_name, password_hash, role, created_at, updated_at) VALUES ('u1', 'ann', 'Ann', 'h', 'admin', ?, ?)`, now, now)
	f.exec(`INSERT INTO profiles (id, user_id, name, created_at, updated_at) VALUES ('p1', 'u1', 'Ann', ?, ?), ('p2', 'u1', 'Kid', ?, ?)`, now, now, now, now)
	f.exec(`INSERT INTO libraries (id, name, type, created_at, updated_at) VALUES ('lm', 'Movies', 'movies', ?, ?), ('ls', 'Shows', 'shows', ?, ?)`, now, now, now, now)
	f.exec(`INSERT INTO library_folders (id, library_id, path, created_at) VALUES ('fm', 'lm', '/m', ?), ('fs', 'ls', '/s', ?)`, now, now)
	return f
}

func (f *fixture) exec(q string, args ...any) {
	f.t.Helper()
	if _, err := f.db.Exec(q, args...); err != nil {
		f.t.Fatalf("%s: %v", q, err)
	}
}

type movie struct {
	id, title, added, released string
	rating                     float64
	votes                      int
	age                        *int
	genres                     string // JSON
	width, height              int
	hdr                        string
}

func (f *fixture) movie(m movie) {
	f.t.Helper()
	f.exec(`INSERT INTO items (id, library_id, kind, title, sort_title, created_at, updated_at) VALUES (?, 'lm', 'movie', ?, ?, ?, ?)`, m.id, m.title, m.title, m.added, now)
	var rating, release any
	if m.rating > 0 {
		rating = m.rating
	}
	if m.released != "" {
		release = m.released
	}
	f.exec(`INSERT INTO item_metadata (item_id, state, title, sort_title, release_date, rating, rating_count, age_rating, genres, updated_at)
		VALUES (?, 'matched', ?, ?, ?, ?, ?, ?, ?, ?)`, m.id, m.title, m.title, release, rating, m.votes, m.age, m.genres, now)
	if m.width > 0 {
		f.file(m.id, "lm", "fm", m.width, m.height, m.hdr)
	}
}

func (f *fixture) file(itemID, lib, folder string, w, h int, hdr string) {
	f.t.Helper()
	var hdrv any
	if hdr != "" {
		hdrv = hdr
	}
	f.exec(`INSERT INTO media_files (id, library_id, folder_id, path, size, mtime, parse_status, width, height, hdr_format, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, 'ok', ?, ?, ?, ?, ?)`, "f-"+itemID, lib, folder, "/x/"+itemID, now, w, h, hdrv, now, now)
	f.exec(`INSERT INTO item_files (item_id, file_id) VALUES (?, ?)`, itemID, "f-"+itemID)
}

func (f *fixture) watch(profile, item string, position int, watched bool, last string) {
	f.t.Helper()
	f.exec(`INSERT INTO watch_state (profile_id, item_id, position_ms, watched, last_played_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		profile, item, position, watched, last, now)
}

func (f *fixture) list(q Query) Page {
	f.t.Helper()
	if q.LibraryID == "" {
		q.LibraryID, q.Kind = "lm", "movie"
	}
	if q.Limit == 0 {
		q.Limit = 50
	}
	p, err := f.c.List(context.Background(), q)
	if err != nil {
		f.t.Fatalf("List(%+v): %v", q, err)
	}
	return p
}

func age(n int) *int { return &n }

// seedMovies: five movies covering every sort key and filter.
func seedMovies(f *fixture) {
	f.movie(movie{id: "alien", title: "Alien", added: "2026-01-01T00:00:00Z", released: "1979-05-25", rating: 8.2, votes: 900, age: age(16),
		genres: `[{"id":878,"name":"Science Fiction"},{"id":27,"name":"Horror"}]`, width: 1920, height: 800})
	f.movie(movie{id: "amelie", title: "Amélie", added: "2026-03-01T00:00:00Z", released: "2001-04-25", rating: 7.9, votes: 500, age: age(12),
		genres: `[{"id":35,"name":"Comedy"}]`, width: 1280, height: 544})
	f.movie(movie{id: "dune", title: "Dune: Part Two", added: "2026-10-01T00:00:00Z", released: "2024-02-27", rating: 8.2, votes: 2000, age: age(13),
		genres: `[{"id":878,"name":"Science Fiction"}]`, width: 3840, height: 1600, hdr: "dolby_vision"})
	f.movie(movie{id: "home", title: "Home Video", added: "2026-05-01T00:00:00Z", genres: `[]`, width: 640, height: 480})
	f.movie(movie{id: "toy", title: "Toy Story", added: "2026-02-01T00:00:00Z", released: "1995-11-22", rating: 8.0, votes: 800, age: age(0),
		genres: `[{"id":16,"name":"Animation"},{"id":35,"name":"Comedy"}]`, width: 1920, height: 1080})
}

func TestListSorts(t *testing.T) {
	f := newFixture(t)
	seedMovies(f)
	f.watch("p1", "toy", 0, true, "2026-10-20T00:00:00Z")
	f.watch("p1", "alien", 60000, false, "2026-10-25T00:00:00Z")

	for _, c := range []struct {
		name string
		q    Query
		want []string
	}{
		{"title", Query{}, []string{"alien", "amelie", "dune", "home", "toy"}},
		{"title desc", Query{Desc: true}, []string{"toy", "home", "dune", "amelie", "alien"}},
		{"added desc", Query{Sort: SortAdded, Desc: true}, []string{"dune", "home", "amelie", "toy", "alien"}},
		{"added asc", Query{Sort: SortAdded}, []string{"alien", "toy", "amelie", "home", "dune"}},
		{"released desc, unknown last", Query{Sort: SortReleased, Desc: true}, []string{"dune", "amelie", "toy", "alien", "home"}},
		{"released asc, unknown last", Query{Sort: SortReleased}, []string{"alien", "toy", "amelie", "dune", "home"}},
		// Alien and Dune tie at 8.2: more votes first.
		{"rating desc", Query{Sort: SortRating, Desc: true}, []string{"dune", "alien", "toy", "amelie", "home"}},
		{"played desc, never played last", Query{Sort: SortPlayed, Desc: true, ProfileID: "p1"}, []string{"alien", "toy", "amelie", "dune", "home"}},
	} {
		if got := f.list(c.q).IDs; !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestListRandom(t *testing.T) {
	f := newFixture(t)
	seedMovies(f)
	a := f.list(Query{Sort: SortRandom, Seed: 42}).IDs
	b := f.list(Query{Sort: SortRandom, Seed: 42}).IDs
	if !slices.Equal(a, b) {
		t.Errorf("same seed, different orders: %v, %v", a, b)
	}
	sorted := slices.Sorted(slices.Values(a))
	if !slices.Equal(sorted, []string{"alien", "amelie", "dune", "home", "toy"}) {
		t.Errorf("random lost or duplicated items: %v", a)
	}
	differs := false
	for seed := range int64(20) {
		if !slices.Equal(f.list(Query{Sort: SortRandom, Seed: seed}).IDs, a) {
			differs = true
		}
	}
	if !differs {
		t.Error("20 seeds gave the same order")
	}
	// Pages of one seed tile the whole list.
	p1 := f.list(Query{Sort: SortRandom, Seed: 42, Limit: 3}).IDs
	p2 := f.list(Query{Sort: SortRandom, Seed: 42, Limit: 3, Offset: 3}).IDs
	if !slices.Equal(append(p1, p2...), a) {
		t.Errorf("pages %v + %v, want %v", p1, p2, a)
	}
	if got := f.list(Query{Sort: SortRandom, Seed: 42, Offset: 10}).IDs; len(got) != 0 {
		t.Errorf("offset past the end: %v", got)
	}
}

func TestListFilters(t *testing.T) {
	f := newFixture(t)
	seedMovies(f)
	f.watch("p1", "toy", 0, true, "2026-10-20T00:00:00Z")
	f.watch("p1", "alien", 60000, false, "2026-10-25T00:00:00Z")
	f.watch("p2", "dune", 0, true, "2026-10-25T00:00:00Z") // another profile's state doesn't count
	yes, no := true, false

	for _, c := range []struct {
		name string
		q    Query
		want []string
	}{
		{"watched", Query{ProfileID: "p1", Watch: WatchWatched}, []string{"toy"}},
		{"in progress", Query{ProfileID: "p1", Watch: WatchInProgress}, []string{"alien"}},
		{"unwatched", Query{ProfileID: "p1", Watch: WatchUnwatched}, []string{"amelie", "dune", "home"}},
		{"genre", Query{Genres: []int{878}}, []string{"alien", "dune"}},
		{"genres, any of", Query{Genres: []int{27, 16}}, []string{"alien", "toy"}},
		{"years", Query{YearFrom: 1990, YearTo: 2009}, []string{"amelie", "toy"}},
		{"year from", Query{YearFrom: 2020}, []string{"dune"}},
		{"4K", Query{Resolutions: []string{Res2160}}, []string{"dune"}},
		{"1080p counts scope", Query{Resolutions: []string{Res1080}}, []string{"alien", "toy"}},
		{"720p and sd", Query{Resolutions: []string{Res720, ResSD}}, []string{"amelie", "home"}},
		{"hdr", Query{HDR: &yes}, []string{"dune"}},
		{"no hdr", Query{HDR: &no}, []string{"alien", "amelie", "home", "toy"}},
		{"combined", Query{ProfileID: "p1", Watch: WatchUnwatched, Genres: []int{878, 35}, Resolutions: []string{Res2160, Res720}}, []string{"amelie", "dune"}},
	} {
		if got := f.list(c.q).IDs; !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestListTotalAndAccess(t *testing.T) {
	f := newFixture(t)
	seedMovies(f)
	f.exec(`UPDATE items SET hidden = 1 WHERE id = 'home'`)

	p := f.list(Query{WithTotal: true, Limit: 2, Offset: 1})
	if p.Total != 4 || !slices.Equal(p.IDs, []string{"amelie", "dune"}) {
		t.Errorf("total %d, page %v; want 4, [amelie dune]", p.Total, p.IDs)
	}
	if p := f.list(Query{}); p.Total != -1 {
		t.Errorf("total without asking: %d", p.Total)
	}

	kid := access.Limit{MaxAge: age(12)}
	p = f.list(Query{Access: kid, WithTotal: true})
	if p.Total != 2 || !slices.Equal(p.IDs, []string{"amelie", "toy"}) {
		t.Errorf("max age 12: total %d, %v; want 2, [amelie toy]", p.Total, p.IDs)
	}
	p = f.list(Query{Access: kid, Genres: []int{878}, WithTotal: true})
	if p.Total != 0 || len(p.IDs) != 0 {
		t.Errorf("a filter reveals hidden items: total %d, %v", p.Total, p.IDs)
	}
}

func TestListShows(t *testing.T) {
	f := newFixture(t)
	show := func(id, title, created string) {
		f.exec(`INSERT INTO items (id, library_id, kind, title, sort_title, created_at, updated_at) VALUES (?, 'ls', 'show', ?, ?, ?, ?)`, id, title, title, created, now)
		f.exec(`INSERT INTO items (id, library_id, kind, parent_id, season_number, created_at, updated_at) VALUES (?, 'ls', 'season', ?, 1, ?, ?)`, id+"-s1", id, created, now)
	}
	ep := func(show string, n int, created string) string {
		id := show + "-e" + string(rune('0'+n))
		f.exec(`INSERT INTO items (id, library_id, kind, parent_id, season_number, episode_number, created_at, updated_at) VALUES (?, 'ls', 'episode', ?, 1, ?, ?, ?)`, id, show+"-s1", n, created, now)
		return id
	}
	show("bear", "The Bear", "2026-01-01T00:00:00Z")
	b1, b2 := ep("bear", 1, "2026-01-01T00:00:00Z"), ep("bear", 2, "2026-10-28T00:00:00Z")
	show("dark", "Dark", "2026-05-01T00:00:00Z")
	d1, d2 := ep("dark", 1, "2026-05-01T00:00:00Z"), ep("dark", 2, "2026-05-01T00:00:00Z")
	show("lupin", "Lupin", "2026-07-01T00:00:00Z")
	l1 := ep("lupin", 1, "2026-07-01T00:00:00Z")
	f.file(b2, "ls", "fs", 3840, 2160, "hdr10")
	f.file(d1, "ls", "fs", 1920, 1080, "")

	f.watch("p1", d1, 0, true, "2026-10-01T00:00:00Z")
	f.watch("p1", d2, 0, true, "2026-10-02T00:00:00Z")
	f.watch("p1", b1, 30000, false, "2026-10-29T00:00:00Z")
	_ = l1

	q := func(q Query) []string {
		q.LibraryID, q.Kind = "ls", "show"
		return f.list(q).IDs
	}
	// The Bear's newest episode is the most recent addition.
	if got := q(Query{Sort: SortAdded, Desc: true}); !slices.Equal(got, []string{"bear", "lupin", "dark"}) {
		t.Errorf("added: %v", got)
	}
	if got := q(Query{ProfileID: "p1", Watch: WatchWatched}); !slices.Equal(got, []string{"dark"}) {
		t.Errorf("watched: %v", got)
	}
	if got := q(Query{ProfileID: "p1", Watch: WatchInProgress}); !slices.Equal(got, []string{"bear"}) {
		t.Errorf("in progress: %v", got)
	}
	if got := q(Query{ProfileID: "p1", Watch: WatchUnwatched}); !slices.Equal(got, []string{"lupin"}) {
		t.Errorf("unwatched: %v", got)
	}
	if got := q(Query{ProfileID: "p1", Sort: SortPlayed, Desc: true}); !slices.Equal(got, []string{"bear", "dark", "lupin"}) {
		t.Errorf("played: %v", got)
	}
	if got := q(Query{Resolutions: []string{Res2160}}); !slices.Equal(got, []string{"bear"}) {
		t.Errorf("4K by an episode's file: %v", got)
	}
	if got := q(Query{Resolutions: []string{Res1080}}); !slices.Equal(got, []string{"dark"}) {
		t.Errorf("1080p: %v", got)
	}
}
