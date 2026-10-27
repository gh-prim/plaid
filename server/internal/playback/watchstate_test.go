package playback

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/db"
)

// watchEnv is a database with two profiles, a movie, and a show with two
// seasons of three episodes and a special.
type watchEnv struct {
	t     *testing.T
	db    *sql.DB
	store *WatchStore
	clock *fakeClock
}

func newWatchEnv(t *testing.T) *watchEnv {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	clock := &fakeClock{now: time.Date(2026, 10, 27, 20, 0, 0, 0, time.UTC)}
	e := &watchEnv{t: t, db: d, store: &WatchStore{DB: d, Now: clock.Now}, clock: clock}
	now := "2026-10-27T12:00:00Z"
	e.exec(`INSERT INTO users (id, username, display_name, password_hash, role, created_at, updated_at) VALUES ('u1', 'ada', 'Ada', 'h', 'admin', ?, ?)`, now, now)
	e.exec(`INSERT INTO profiles (id, user_id, name, is_default, created_at, updated_at) VALUES ('ada', 'u1', 'Ada', 1, ?, ?)`, now, now)
	e.exec(`INSERT INTO profiles (id, user_id, name, created_at, updated_at) VALUES ('kid', 'u1', 'Kid', ?, ?)`, now, now)
	e.exec(`INSERT INTO libraries (id, name, type, created_at, updated_at) VALUES ('l1', 'All', 'shows', ?, ?)`, now, now)
	item := func(id, kind, parent string, season, episode any) {
		e.exec(`INSERT INTO items (id, library_id, kind, parent_id, title, season_number, episode_number, created_at, updated_at)
			VALUES (?, 'l1', ?, NULLIF(?, ''), ?, ?, ?, ?, ?)`, id, kind, parent, id, season, episode, now, now)
	}
	item("movie", "movie", "", nil, nil)
	item("show", "show", "", nil, nil)
	item("s0", "season", "show", 0, nil)
	item("s0e1", "episode", "s0", 0, 1)
	for _, s := range []string{"1", "2"} {
		item("s"+s, "season", "show", s, nil)
		for _, ep := range []string{"1", "2", "3"} {
			item("s"+s+"e"+ep, "episode", "s"+s, s, ep)
		}
	}
	return e
}

func (e *watchEnv) exec(q string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Exec(q, args...); err != nil {
		e.t.Fatalf("%s: %v", q, err)
	}
}

func (e *watchEnv) state(profile, item string) UserState {
	e.t.Helper()
	st, err := e.store.States(context.Background(), profile, []string{item})
	if err != nil {
		e.t.Fatal(err)
	}
	return st[item]
}

func TestReachedEnd(t *testing.T) {
	minute := int64(60_000)
	for _, c := range []struct {
		pos, dur int64
		want     bool
	}{
		{90 * minute, 100 * minute, true},
		{89 * minute, 100 * minute, false},
		{119 * minute, 121 * minute, true},
		{60 * minute, 121 * minute, false},
		{int64(2.5 * 60_000), 4 * minute, true}, // 62%, but 90 s left past the middle
		{0, 3 * minute, false},                  // a 3-minute short isn't watched at 0
		{2 * minute, 3 * minute, true},
		{minute, 0, false},
	} {
		if got := reachedEnd(c.pos, c.dur); got != c.want {
			t.Errorf("reachedEnd(%d, %d) = %v", c.pos, c.dur, got)
		}
	}
}

func TestSaveKeepsProfilesApart(t *testing.T) {
	e := newWatchEnv(t)
	ctx := context.Background()
	if err := e.store.Save(ctx, "ada", "movie", "", 3_600_000, 7_200_000, false, false); err != nil {
		t.Fatal(err)
	}
	if st := e.state("ada", "movie"); st.PositionMs != 3_600_000 || st.Watched || st.LastPlayedAt == nil {
		t.Errorf("ada = %+v", st)
	}
	if st := e.state("kid", "movie"); st.PositionMs != 0 || st.LastPlayedAt != nil {
		t.Errorf("kid = %+v, want nothing", st)
	}
	// Under a minute: no resume point.
	if err := e.store.Save(ctx, "kid", "movie", "", 30_000, 7_200_000, false, false); err != nil {
		t.Fatal(err)
	}
	if st := e.state("kid", "movie"); st.PositionMs != 0 {
		t.Errorf("30 s saved as a resume point: %+v", st)
	}
	// Watched: position reset, counted once.
	if err := e.store.Save(ctx, "ada", "movie", "", 6_600_000, 7_200_000, true, true); err != nil {
		t.Fatal(err)
	}
	if err := e.store.Save(ctx, "ada", "movie", "", 6_700_000, 7_200_000, true, false); err != nil {
		t.Fatal(err)
	}
	if st := e.state("ada", "movie"); !st.Watched || st.PositionMs != 0 || st.PlayCount != 1 {
		t.Errorf("after the end = %+v", st)
	}
}

func TestMarkShowAndSeason(t *testing.T) {
	e := newWatchEnv(t)
	ctx := context.Background()
	if err := e.store.Mark(ctx, "ada", "s1", true); err != nil {
		t.Fatal(err)
	}
	if st := e.state("ada", "s1"); !st.Aggregate || !st.Watched || st.WatchedEpisodes != 3 || st.TotalEpisodes != 3 {
		t.Errorf("season 1 = %+v", st)
	}
	if st := e.state("ada", "show"); st.Watched || st.WatchedEpisodes != 3 || st.TotalEpisodes != 7 {
		t.Errorf("show = %+v", st)
	}
	if err := e.store.Mark(ctx, "ada", "show", true); err != nil {
		t.Fatal(err)
	}
	if st := e.state("ada", "show"); !st.Watched || st.WatchedEpisodes != 7 {
		t.Errorf("show marked = %+v", st)
	}
	if err := e.store.Mark(ctx, "ada", "show", false); err != nil {
		t.Fatal(err)
	}
	if st := e.state("ada", "s2e2"); st.Watched || st.PlayCount != 0 {
		t.Errorf("episode after unmarking = %+v", st)
	}
	if st := e.state("kid", "show"); st.WatchedEpisodes != 0 {
		t.Errorf("kid's show = %+v", st)
	}
}

func TestContinueWatching(t *testing.T) {
	e := newWatchEnv(t)
	ctx := context.Background()
	save := func(item string, pos int64, watched bool) {
		t.Helper()
		e.clock.Add(time.Minute)
		if err := e.store.Save(ctx, "ada", item, "", pos, 3_000_000, watched, watched); err != nil {
			t.Fatal(err)
		}
	}
	save("movie", 1_200_000, false)
	save("s1e1", 2_900_000, true)
	save("s1e2", 2_900_000, true)
	save("s1e3", 2_900_000, true)

	all := func(lim access.Limit) []string {
		t.Helper()
		ids, next, err := e.store.ContinueWatching(ctx, "ada", lim, "", 0)
		if err != nil || next != "" {
			t.Fatalf("next %q, err %v", next, err)
		}
		return ids
	}
	if got := all(access.Limit{}); !slices.Equal(got, []string{"s2e1", "movie"}) {
		t.Errorf("continue watching = %v, want the next episode then the movie", got)
	}
	save("s2e1", 600_000, false) // started the next one
	if got := all(access.Limit{}); !slices.Equal(got, []string{"s2e1", "movie"}) {
		t.Errorf("with s2e1 in progress = %v", got)
	}
	save("s2e1", 2_950_000, true)
	save("s2e2", 2_950_000, true)
	save("s2e3", 2_950_000, true)
	if got := all(access.Limit{}); !slices.Equal(got, []string{"s0e1", "movie"}) {
		t.Errorf("after season 2 = %v, want the special then the movie", got)
	}

	// A restricted profile never sees a 16+ show here.
	e.exec(`INSERT INTO item_metadata (item_id, state, age_rating, updated_at) VALUES ('s0e1', 'matched', 16, '2026-10-27T12:00:00Z')`)
	twelve := 12
	if got := all(access.Limit{MaxAge: &twelve, AllowUnrated: true}); !slices.Equal(got, []string{"movie"}) {
		t.Errorf("12+ profile = %v", got)
	}

	// Paging.
	ids, next, err := e.store.ContinueWatching(ctx, "ada", access.Limit{}, "", 1)
	if err != nil || len(ids) != 1 || next == "" {
		t.Fatalf("page 1 = %v %q %v", ids, next, err)
	}
	ids, next, err = e.store.ContinueWatching(ctx, "ada", access.Limit{}, next, 1)
	if err != nil || !slices.Equal(ids, []string{"movie"}) || next != "" {
		t.Errorf("page 2 = %v %q %v", ids, next, err)
	}
	if _, _, err := e.store.ContinueWatching(ctx, "ada", access.Limit{}, "!!", 1); !errors.Is(err, ErrBadCursor) {
		t.Errorf("bad cursor: %v", err)
	}
}
