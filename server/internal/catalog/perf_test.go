//go:build perf

package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/db"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/metadata"
)

// TestPerfCatalog checks F008's NFRs on generated libraries:
//   - GET /search p95 < 100 ms with 10,000 items and 30,000 people
//   - GET /home < 150 ms with 2,000 movies and 200 shows
//
// and reports a filtered grid page with its total and the facets. Run it
// with `go test -tags perf -run Perf -v ./internal/catalog/`, in the
// throttled container (see CLAUDE.md) to approximate a Raspberry Pi.
func TestPerfCatalog(t *testing.T) {
	t.Logf("CPUs: %d (GOMAXPROCS %d)", runtime.NumCPU(), runtime.GOMAXPROCS(0))
	t.Run("search", perfSearch)
	t.Run("home", perfHome)
}

var words = strings.Fields(`the a dark night love last star black red city dead blood house man woman
	girl boy king queen war world time life day sun moon fire ice water sea river road home secret lost
	return rise fall shadow light ghost wild lady lord game story song dream heart storm silent golden
	little big new old great final first ultimate empire legend summer winter autumn spring`)

var names = strings.Fields(`anna ben carla david emma felix grace hugo iris jack karin leo maria noah
	olga paul quinn rosa sam tara umar vera will xena yuri zoe amélie björn chloé dmitri éloïse`)

func title(r *rand.Rand) string {
	n := 1 + r.IntN(4)
	parts := make([]string, n)
	for i := range parts {
		parts[i] = words[r.IntN(len(words))]
	}
	return strings.ToUpper(parts[0][:1]) + strings.Join(parts, " ")[1:]
}

// seedPerf fills a database: movies and shows (episodes per show) in two
// libraries, each with metadata, genres and an age rating, and people
// credited on movies.
func seedPerf(t *testing.T, movies, shows, episodes, people int) (*sql.DB, *Catalog) {
	t.Helper()
	d, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	start := time.Now()
	r := rand.New(rand.NewPCG(1, 2))
	tx, err := d.Begin()
	if err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		if _, err := tx.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO users (id, username, display_name, password_hash, role, created_at, updated_at) VALUES ('u1', 'ann', 'Ann', 'h', 'admin', ?, ?)`, now, now)
	exec(`INSERT INTO profiles (id, user_id, name, is_default, created_at, updated_at) VALUES ('p1', 'u1', 'Ann', 1, ?, ?)`, now, now)
	exec(`INSERT INTO libraries (id, name, type, created_at, updated_at) VALUES ('lm', 'Movies', 'movies', ?, ?), ('ls', 'Shows', 'shows', ?, ?)`, now, now, now, now)
	exec(`INSERT INTO library_folders (id, library_id, path, created_at) VALUES ('fm', 'lm', '/m', ?)`, now)
	ages := []any{0, 7, 10, 12, 13, 16, 18, nil}
	meta := func(id, t string, i int) {
		g := fmt.Sprintf(`[{"id":%d,"name":"Genre %d"},{"id":%d,"name":"Genre %d"}]`, i%15, i%15, 100+i%7, 100+i%7)
		exec(`INSERT INTO item_metadata (item_id, state, title, sort_title, release_date, rating, rating_count, age_rating, genres, updated_at)
			VALUES (?, 'matched', ?, ?, ?, ?, ?, ?, ?, ?)`, id, t, strings.ToLower(t), fmt.Sprintf("%d-06-01", 1950+i%75),
			float64(r.IntN(100))/10, r.IntN(5000), ages[i%len(ages)], g, now)
	}
	added := func(i int) string {
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute).Format(time.RFC3339)
	}
	for i := range movies {
		id, tt := fmt.Sprintf("m%05d", i), title(r)
		exec(`INSERT INTO items (id, library_id, kind, title, sort_title, year, created_at, updated_at) VALUES (?, 'lm', 'movie', ?, ?, ?, ?, ?)`,
			id, tt, strings.ToLower(tt), 1950+i%75, added(i), now)
		meta(id, tt, i)
		exec(`INSERT INTO media_files (id, library_id, folder_id, path, size, mtime, parse_status, width, height, created_at, updated_at)
			VALUES (?, 'lm', 'fm', ?, 1, ?, 'ok', ?, ?, ?, ?)`, "f"+id, "/m/"+id, now, []int{3840, 1920, 1280, 720}[i%4], []int{2160, 1080, 720, 480}[i%4], now, now)
		exec(`INSERT INTO item_files (item_id, file_id) VALUES (?, ?)`, id, "f"+id)
		if i%3 == 0 {
			exec(`INSERT INTO artwork (id, owner_type, owner_id, kind, source, selected, created_at) VALUES (?, 'item', ?, 'backdrop', 'tmdb', 1, ?)`, "b"+id, id, now)
		}
	}
	for s := range shows {
		id, tt := fmt.Sprintf("s%04d", s), title(r)
		exec(`INSERT INTO items (id, library_id, kind, title, sort_title, created_at, updated_at) VALUES (?, 'ls', 'show', ?, ?, ?, ?)`, id, tt, strings.ToLower(tt), added(s), now)
		meta(id, tt, s)
		for season := 1; season <= 2; season++ {
			sid := fmt.Sprintf("%s-%d", id, season)
			exec(`INSERT INTO items (id, library_id, kind, parent_id, season_number, created_at, updated_at) VALUES (?, 'ls', 'season', ?, ?, ?, ?)`, sid, id, season, added(s), now)
			for e := range episodes / 2 {
				eid := fmt.Sprintf("%s-%d", sid, e)
				exec(`INSERT INTO items (id, library_id, kind, parent_id, episode_number, title, created_at, updated_at) VALUES (?, 'ls', 'episode', ?, ?, ?, ?, ?)`,
					eid, sid, e+1, title(r), added(s*episodes+e), now)
			}
		}
	}
	for p := range people {
		pid := fmt.Sprintf("p%05d", p)
		exec(`INSERT INTO people (id, name, updated_at) VALUES (?, ?, ?)`, pid, names[r.IntN(len(names))]+" "+names[r.IntN(len(names))]+fmt.Sprint(p), now)
		if movies > 0 {
			exec(`INSERT OR IGNORE INTO item_credits (item_id, person_id, role, ord) VALUES (?, ?, 'cast', 0)`, fmt.Sprintf("m%05d", r.IntN(movies)), pid)
		}
	}
	for i := range 40 {
		exec(`INSERT INTO watch_state (profile_id, item_id, position_ms, watched, last_played_at, updated_at) VALUES ('p1', ?, ?, ?, ?, ?)`,
			fmt.Sprintf("m%05d", i*7%movies), 60000*(i%2), i%2 == 0, added(i), now)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`ANALYZE`); err != nil {
		t.Fatal(err)
	}
	t.Logf("seeded %d movies, %d shows × %d episodes, %d people in %s", movies, shows, episodes, people, time.Since(start).Round(time.Millisecond))
	return d, New(Options{DB: d})
}

// prefix is the first n letters of w (or all of it).
func prefix(w string, n int) string {
	rs := []rune(w)
	return string(rs[:min(n, len(rs))])
}

func p95(ds []time.Duration) (time.Duration, time.Duration) {
	slices.Sort(ds)
	return ds[len(ds)/2], ds[len(ds)*95/100]
}

func perfSearch(t *testing.T) {
	_, c := seedPerf(t, 6000, 400, 10, 30000) // 6,000 movies + 400 shows + 4,000 episodes
	r := rand.New(rand.NewPCG(3, 4))
	ctx := context.Background()
	var ds []time.Duration
	for i := range 300 {
		var q string
		switch i % 4 {
		case 0: // the first letters of a word, as typed
			q = prefix(words[r.IntN(len(words))], 1+r.IntN(3))
		case 1: // two words
			q = words[r.IntN(len(words))] + " " + prefix(words[r.IntN(len(words))], 2)
		case 2: // a name
			q = prefix(names[r.IntN(len(names))], 3)
		default: // nothing matches
			q = "zzq" + fmt.Sprint(i)
		}
		lim := access.Limit{}
		if i%5 == 0 {
			lim = access.Limit{MaxAge: new(12)}
		}
		start := time.Now()
		if _, err := c.Search(ctx, q, lim, 8); err != nil {
			t.Fatal(err)
		}
		ds = append(ds, time.Since(start))
	}
	med, p := p95(ds)
	t.Logf("search, 10,000 items and 30,000 people: median %s, p95 %s (target p95 < 100 ms)", med.Round(time.Microsecond), p.Round(time.Microsecond))
	if p > 100*time.Millisecond {
		t.Errorf("search p95 %s over 100 ms", p)
	}
}

func perfHome(t *testing.T) {
	d, c := seedPerf(t, 2000, 200, 10, 2000)
	ctx := context.Background()
	lib, meta := library.NewStore(d), metadata.NewStore(d)
	cw := []string{"m00000", "m00007", "m00014"}
	var ds []time.Duration
	for range 30 {
		start := time.Now()
		ambient, rows, err := c.Home(ctx, HomeQuery{ProfileID: "p1", ContinueWatching: cw})
		if err != nil {
			t.Fatal(err)
		}
		// What the handler loads next: items and summaries of every row, in one batch.
		ids := []string{ambient}
		for _, r := range rows {
			ids = append(ids, r.IDs...)
		}
		items, err := lib.Items(ctx, ids)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := meta.Summaries(ctx, ids); err != nil {
			t.Fatal(err)
		}
		ds = append(ds, time.Since(start))
		if len(rows) < 5 || len(items) < 80 {
			t.Fatalf("home has %d rows, %d items", len(rows), len(items))
		}
	}
	med, p := p95(ds)
	t.Logf("home, 2,000 movies and 200 shows: median %s, p95 %s (target < 150 ms)", med.Round(time.Microsecond), p.Round(time.Microsecond))
	if med > 150*time.Millisecond {
		t.Errorf("home median %s over 150 ms", med)
	}

	q := Query{LibraryID: "lm", Kind: "movie", ProfileID: "p1", Sort: SortAdded, Desc: true, Watch: WatchUnwatched,
		Genres: []int{3, 5}, Resolutions: []string{Res1080, Res2160}, Limit: 200, WithTotal: true}
	ds = ds[:0]
	for range 30 {
		start := time.Now()
		if _, err := c.List(ctx, q); err != nil {
			t.Fatal(err)
		}
		ds = append(ds, time.Since(start))
	}
	med, p = p95(ds)
	t.Logf("grid page of 200 with total, filtered by watch, genres and resolution: median %s, p95 %s", med.Round(time.Microsecond), p.Round(time.Microsecond))

	ds = ds[:0]
	for range 30 {
		start := time.Now()
		if _, err := c.Facets(ctx, "lm", "movie", access.Limit{}); err != nil {
			t.Fatal(err)
		}
		ds = append(ds, time.Since(start))
	}
	med, p = p95(ds)
	t.Logf("facets of 2,000 movies: median %s, p95 %s", med.Round(time.Microsecond), p.Round(time.Microsecond))
}
