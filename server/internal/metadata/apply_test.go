package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/db"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/metadata/tmdbtest"
)

// applyEnv scans a real folder with F004's scanner, wired to the service
// the way main.go does, against the recorded TMDB responses.
type applyEnv struct {
	t    *testing.T
	db   *sql.DB
	srv  *tmdbtest.Server
	svc  *Service
	lib  library.Library
	root string
	now  time.Time
	errs []error
}

// withArtwork turns on the image store (slower: images get downloaded,
// decoded and resized).
func withArtwork(e *applyEnv, o *Options) {
	o.DataDir, o.ImageHTTP, o.ImageHosts = e.t.TempDir(), e.srv.HTTP(), e.srv.ImageHosts()
}

func newApplyEnv(t *testing.T, typ, lang, region string, withProvider bool, opts ...func(*applyEnv, *Options)) *applyEnv {
	t.Helper()
	d, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	e := &applyEnv{t: t, db: d, root: t.TempDir(), now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
	o := Options{DB: d, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return e.now }}
	if withProvider {
		e.srv = tmdbtest.New(t)
		o.Provider, o.Fanart = e.srv.Client(), e.srv.Fanart()
	}
	for _, f := range opts {
		f(e, &o)
	}
	e.svc = New(o)
	if err := e.svc.ReloadKeys(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.lib, err = library.NewStore(d).CreateLibrary(context.Background(), "Lib", typ, []string{e.root},
		library.CreateOptions{MetadataLanguage: lang, MetadataRegion: region})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *applyEnv) write(rel, content string) {
	e.t.Helper()
	p := filepath.Join(e.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

// scan runs F004's scanner with the hooks main.go installs, then every
// due job, the way the queue would.
func (e *applyEnv) scan() {
	e.t.Helper()
	sc := &library.Scanner{Store: library.NewStore(e.db), Hooks: library.Hooks{ItemsCreated: e.svc.ItemsCreated, Sidecars: e.svc.SidecarsSeen}}
	if _, err := sc.ScanLibrary(context.Background(), e.lib.ID, library.ScanOptions{}); err != nil {
		e.t.Fatal(err)
	}
	e.drain()
}

func (e *applyEnv) drain() {
	e.t.Helper()
	e.errs = nil
	ctx := context.Background()
	for range 100 {
		j, ok, err := e.svc.store.Claim(ctx, time.Minute)
		if err != nil {
			e.t.Fatal(err)
		}
		if !ok {
			return
		}
		if err := e.svc.Process(ctx, j); err != nil {
			e.errs = append(e.errs, err)
			if !errors.Is(err, ErrPaused) {
				_ = e.svc.store.Fail(ctx, j, err)
			} else {
				_ = e.svc.store.Release(ctx, j, time.Hour)
			}
		}
	}
	e.t.Fatal("drain: the queue never emptied")
}

func (e *applyEnv) logErrs() {
	for _, err := range e.errs {
		e.t.Log("process error:", err)
	}
}

// item returns the id of the item with a parsed title (movies, shows) or
// the episode/season number under a show.
func (e *applyEnv) item(where string, args ...any) string {
	e.t.Helper()
	var id string
	if err := e.db.QueryRow(`SELECT id FROM items WHERE `+where, args...).Scan(&id); err != nil {
		e.t.Fatalf("item %s %v: %v", where, args, err)
	}
	return id
}

type meta struct {
	State, Source, Title, Overview, Tagline, Cert, Region, Langs, Local, Locked, Dup, Collection string
	TMDBID                                                                                       int
	Age                                                                                          sql.NullInt64
	Confidence                                                                                   float64
	Refresh                                                                                      sql.NullString
}

func (e *applyEnv) meta(itemID string) meta {
	e.t.Helper()
	var m meta
	err := e.db.QueryRow(`SELECT state, COALESCE(match_source, ''), COALESCE(title, ''), COALESCE(overview, ''),
		COALESCE(tagline, ''), COALESCE(certification, ''), COALESCE(certification_region, ''), COALESCE(field_languages, ''),
		COALESCE(local_fields, ''), COALESCE(locked_fields, ''), COALESCE(duplicate_of, ''), COALESCE(collection_id, ''),
		COALESCE(tmdb_id, 0), age_rating, COALESCE(confidence, 0), refresh_at FROM item_metadata WHERE item_id = ?`, itemID).
		Scan(&m.State, &m.Source, &m.Title, &m.Overview, &m.Tagline, &m.Cert, &m.Region, &m.Langs, &m.Local, &m.Locked,
			&m.Dup, &m.Collection, &m.TMDBID, &m.Age, &m.Confidence, &m.Refresh)
	if err != nil {
		e.t.Fatalf("metadata of %s: %v", itemID, err)
	}
	return m
}

func (e *applyEnv) count(q string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.db.QueryRow(q, args...).Scan(&n); err != nil {
		e.t.Fatalf("%s: %v", q, err)
	}
	return n
}

func (e *applyEnv) searched() bool {
	for _, p := range e.srv.Paths() {
		if strings.HasPrefix(p, "/search/") {
			return true
		}
	}
	return false
}

func TestHintMatchesWithoutSearch(t *testing.T) {
	e := newApplyEnv(t, library.TypeMovies, "en", "US", true)
	e.write("Alien (1979) {tmdb-348}/Alien (1979) {tmdb-348}.mkv", "x")
	e.scan()
	m := e.meta(e.item(`kind = 'movie'`))
	if m.State != "matched" || m.Source != "hint" || m.TMDBID != 348 || m.Confidence != 1 || m.Title != "Alien" {
		t.Errorf("Alien = %+v", m)
	}
	if e.searched() {
		t.Errorf("a hinted movie was searched: %v", e.srv.Paths())
	}
	if m.Cert != "R" || m.Region != "US" || !m.Age.Valid || m.Age.Int64 != 18 {
		t.Errorf("certification %s/%s age %v", m.Cert, m.Region, m.Age)
	}
	if m.Collection == "" || e.count(`SELECT COUNT(*) FROM collections WHERE tmdb_id = 8091`) != 1 {
		t.Errorf("collection not stored (%q)", m.Collection)
	}
	if n := e.count(`SELECT COUNT(*) FROM item_credits WHERE item_id = (SELECT id FROM items WHERE kind = 'movie') AND role = 'cast'`); n == 0 || n > 20 {
		t.Errorf("%d cast credits", n)
	}
	var trailers string
	_ = e.db.QueryRow(`SELECT COALESCE(trailers, '') FROM item_metadata`).Scan(&trailers)
	if !strings.Contains(trailers, `"site":"YouTube"`) {
		t.Errorf("trailers = %s", trailers)
	}
	if !m.Refresh.Valid || !strings.HasPrefix(m.Refresh.String, "2027-01-25") {
		t.Errorf("an old movie refreshes in 90 days, got %v", m.Refresh)
	}
}

func TestNFOWithIMDbID(t *testing.T) {
	e := newApplyEnv(t, library.TypeMovies, "en", "US", true)
	e.write("Matrix/The Matrix (1999).mkv", "x")
	e.write("Matrix/movie.nfo", `<movie><plot>Local plot</plot><uniqueid type="imdb">tt0133093</uniqueid><mpaa>FR:12</mpaa></movie>`)
	e.scan()
	e.logErrs()
	m := e.meta(e.item(`kind = 'movie'`))
	if m.State != "matched" || m.Source != "nfo" || m.TMDBID != 603 || m.Overview != "Local plot" || m.Title != "The Matrix" {
		t.Errorf("Matrix = %+v", m)
	}
	if !strings.Contains(m.Local, "overview") || !strings.Contains(m.Local, "certification") {
		t.Errorf("local fields = %s", m.Local)
	}
	if m.Cert != "12" || m.Region != "FR" || m.Age.Int64 != 12 {
		t.Errorf("the .nfo rating wins: %s/%s %v", m.Cert, m.Region, m.Age)
	}
	if !slices.ContainsFunc(e.srv.Paths(), func(p string) bool { return strings.HasPrefix(p, "/find/tt0133093") }) || e.searched() {
		t.Errorf("requests = %v, want /find and no search", e.srv.Paths())
	}
}

func TestSearchDecisions(t *testing.T) {
	e := newApplyEnv(t, library.TypeMovies, "en", "US", true)
	e.write("Dune/Dune.mkv", "x")
	e.write("Asdfgh Qwerty (2020)/Asdfgh Qwerty (2020).mkv", "x")
	e.write("The Matrix (1999)/The Matrix (1999).mkv", "x")
	e.scan()
	e.logErrs()
	dune := e.meta(e.item(`title = 'Dune'`))
	if dune.State != "needs_review" || dune.TMDBID != 0 || dune.Title != "" {
		t.Errorf("Dune = %+v", dune)
	}
	if n := e.count(`SELECT COUNT(*) FROM match_candidates WHERE item_id = ?`, e.item(`title = 'Dune'`)); n != 5 {
		t.Errorf("Dune keeps %d candidates, want 5", n)
	}
	none := e.meta(e.item(`title = 'Asdfgh Qwerty'`))
	if none.State != "unmatched" || !none.Refresh.Valid || !strings.HasPrefix(none.Refresh.String, "2026-11-03") {
		t.Errorf("unmatched = %+v, want a retry in 7 days", none)
	}
	matrix := e.meta(e.item(`title = 'The Matrix'`))
	if matrix.State != "matched" || matrix.Source != "auto" || matrix.TMDBID != 603 || matrix.Confidence != 1 {
		t.Errorf("The Matrix = %+v", matrix)
	}
	if e.count(`SELECT COUNT(*) FROM item_metadata WHERE state = 'pending'`) != 0 {
		t.Error("items left pending")
	}
}

func TestShowSeasonsEpisodesAndAge(t *testing.T) {
	e := newApplyEnv(t, library.TypeShows, "en", "US", true)
	e.write("Breaking Bad/Season 01/Breaking Bad S01E01.mkv", "x")
	e.write("Breaking Bad/Season 01/Breaking Bad S01E02-E03.mkv", "x")
	e.write("Breaking Bad/Specials/Breaking Bad S00E02.mkv", "x")
	e.scan()
	show := e.item(`kind = 'show'`)
	m := e.meta(show)
	if m.State != "matched" || m.TMDBID != 1396 || m.Cert != "TV-MA" || m.Age.Int64 != 18 {
		t.Errorf("show = %+v", m)
	}
	var seasonCalls []string
	for _, p := range e.srv.Paths() {
		if strings.Contains(p, "/season/") {
			seasonCalls = append(seasonCalls, p[:strings.IndexByte(p, '?')])
		}
	}
	slices.Sort(seasonCalls)
	if !slices.Equal(seasonCalls, []string{"/tv/1396/season/0", "/tv/1396/season/1"}) {
		t.Errorf("season calls = %v, want exactly seasons 0 and 1", seasonCalls)
	}
	for n, want := range map[int]string{1: "Pilot", 2: "Cat's in the Bag...", 3: "...And the Bag's in the River"} {
		ep := e.meta(e.item(`kind = 'episode' AND episode_number = ? AND parent_id IN (SELECT id FROM items WHERE season_number = 1)`, n))
		if ep.State != "matched" || ep.Title != want || ep.Source != "parent" || ep.TMDBID != 1396 || ep.Age.Int64 != 18 {
			t.Errorf("S01E0%d = %+v", n, ep)
		}
	}
	special := e.meta(e.item(`kind = 'episode' AND episode_number = 2 AND parent_id IN (SELECT id FROM items WHERE season_number = 0)`))
	if special.Title != "Wedding Day" {
		t.Errorf("S00E02 = %+v", special)
	}
	season := e.meta(e.item(`kind = 'season' AND season_number = 1`))
	if season.State != "matched" || season.Title != "Season 1" || season.Age.Int64 != 18 {
		t.Errorf("season = %+v", season)
	}
}

func TestDateBasedEpisode(t *testing.T) {
	e := newApplyEnv(t, library.TypeShows, "en", "US", true)
	e.write("The Daily Show/2024-02-12.mkv", "x")
	e.scan()
	ep := e.meta(e.item(`kind = 'episode'`))
	if ep.State != "matched" || !strings.HasPrefix(ep.Title, "February 12, 2024") {
		t.Errorf("date-based episode = %+v", ep)
	}
	if show := e.meta(e.item(`kind = 'show'`)); show.TMDBID != 2224 || show.Age.Int64 != 16 {
		t.Errorf("The Daily Show = %+v (TV-14 → 16)", show)
	}
}

func TestFrenchLibraryFallsBackToEnglish(t *testing.T) {
	e := newApplyEnv(t, library.TypeShows, "fr", "FR", true)
	e.write("The Office (2005)/Season 01/The Office S01E01.mkv", "x")
	e.scan()
	m := e.meta(e.item(`kind = 'show'`))
	var langs map[string]string
	_ = json.Unmarshal([]byte(m.Langs), &langs)
	if m.TMDBID != 2316 || m.Tagline == "" || langs["tagline"] != "en" || langs["overview"] != "" {
		t.Errorf("The Office in French = %+v", m)
	}
	ep := e.meta(e.item(`kind = 'episode'`))
	if ep.Title != "La rumeur" {
		t.Errorf("episode title = %q, want the French one", ep.Title)
	}
}

func TestLockedFieldsAndRefreshKeepTheMatch(t *testing.T) {
	e := newApplyEnv(t, library.TypeMovies, "en", "US", true)
	e.write("The Matrix (1999)/The Matrix (1999).mkv", "x")
	e.scan()
	id := e.item(`kind = 'movie'`)
	if _, err := e.db.Exec(`UPDATE item_metadata SET title = 'My Matrix', locked_fields = '["title"]', refresh_at = '2000-01-01T00:00:00Z' WHERE item_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	before := len(e.srv.Paths())
	e.drain()
	m := e.meta(id)
	if m.Title != "My Matrix" || m.TMDBID != 603 || m.State != "matched" {
		t.Errorf("after refresh = %+v", m)
	}
	for _, p := range e.srv.Paths()[before:] {
		if strings.HasPrefix(p, "/search/") {
			t.Errorf("a refresh searched again: %s", p)
		}
	}
}

func TestFailureLeavesDataUntouched(t *testing.T) {
	e := newApplyEnv(t, library.TypeShows, "en", "US", true)
	e.write("Breaking Bad/Season 01/Breaking Bad S01E01.mkv", "x")
	e.scan()
	show := e.item(`kind = 'show'`)
	ep := e.item(`kind = 'episode'`)
	before, beforeEp := e.meta(show), e.meta(ep)

	// The show call works, a season call fails: nothing is written.
	if _, err := e.db.Exec(`UPDATE item_metadata SET refresh_at = '2000-01-01T00:00:00Z' WHERE item_id = ?`, show); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE item_metadata SET title = 'stale' WHERE item_id = ?`, ep); err != nil {
		t.Fatal(err)
	}
	e.srv.FailPaths("/season/", 503)
	e.drain()
	if len(e.errs) != 1 {
		t.Fatalf("errors = %v", e.errs)
	}
	after := e.meta(show)
	if after.Title != before.Title || after.TMDBID != before.TMDBID || after.State != "matched" {
		t.Errorf("show changed after a failed refresh: %+v", after)
	}
	if got := e.meta(ep).Title; got != "stale" {
		t.Errorf("episode written by a failed refresh: %q (was %q)", got, beforeEp.Title)
	}
	var attempts int
	_ = e.db.QueryRow(`SELECT attempts FROM item_metadata WHERE item_id = ?`, show).Scan(&attempts)
	if attempts != 1 {
		t.Errorf("attempts = %d", attempts)
	}
}

func TestDuplicates(t *testing.T) {
	e := newApplyEnv(t, library.TypeMovies, "en", "US", true)
	e.write("A/The Matrix (1999) {tmdb-603}.mkv", "x")
	e.scan()
	e.now = e.now.Add(time.Hour)
	e.write("B/Matrix 1999 copy {tmdb-603}.mkv", "y")
	e.scan()
	older := e.item(`title = 'The Matrix'`)
	newer := e.item(`title <> 'The Matrix'`)
	if d := e.meta(newer).Dup; d != older {
		t.Errorf("newer duplicate_of = %q, want %q", d, older)
	}
	if d := e.meta(older).Dup; d != "" {
		t.Errorf("older duplicate_of = %q", d)
	}
}

func TestNoKeyKeepsLocalData(t *testing.T) {
	e := newApplyEnv(t, library.TypeMovies, "en", "US", false)
	e.write("Matrix/The Matrix (1999).mkv", "x")
	e.write("Matrix/movie.nfo", `<movie><title>The Matrix</title><plot>Local plot</plot><mpaa>Rated R</mpaa></movie>`)
	e.scan()
	m := e.meta(e.item(`kind = 'movie'`))
	if m.State != "pending" || m.Overview != "Local plot" || m.Age.Int64 != 18 {
		t.Errorf("no key = %+v", m)
	}
	if len(e.errs) != 1 || !errors.Is(e.errs[0], ErrPaused) {
		t.Errorf("errors = %v, want ErrPaused", e.errs)
	}
}

func TestNewEpisodeRefreshesItsShow(t *testing.T) {
	e := newApplyEnv(t, library.TypeShows, "en", "US", true)
	e.write("Breaking Bad/Season 01/Breaking Bad S01E01.mkv", "x")
	e.scan()
	e.write("Breaking Bad/Season 01/Breaking Bad S01E04.mkv", "x")
	e.scan()
	ep := e.meta(e.item(`kind = 'episode' AND episode_number = 4`))
	if ep.State != "matched" || ep.Title == "" {
		t.Errorf("new episode = %+v", ep)
	}
}
