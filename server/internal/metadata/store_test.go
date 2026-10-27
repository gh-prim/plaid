package metadata

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/db"
)

type testDB struct {
	t     *testing.T
	db    *sql.DB
	store *Store
	lib   string
	now   time.Time
	n     int
}

func newTestDB(t *testing.T) *testDB {
	t.Helper()
	d, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	e := &testDB{t: t, db: d, lib: "lib1", now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
	e.store = NewStore(d)
	e.store.Now = func() time.Time { return e.now }
	e.exec(`INSERT INTO libraries (id, name, type, created_at, updated_at) VALUES ('lib1', 'Lib', 'movies', 'x', 'x')`)
	return e
}

func (e *testDB) exec(q string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Exec(q, args...); err != nil {
		e.t.Fatalf("%s: %v", q, err)
	}
}

// item inserts an item and returns its id.
func (e *testDB) item(kind, parent string) string {
	e.t.Helper()
	e.n++
	id := kind + "-" + string(rune('a'+e.n))
	var p any
	if parent != "" {
		p = parent
	}
	e.exec(`INSERT INTO items (id, library_id, kind, parent_id, title, created_at, updated_at) VALUES (?, 'lib1', ?, ?, ?, 'x', 'x')`,
		id, kind, p, "Title "+id)
	return id
}

func (e *testDB) col(itemID, col string) sql.NullString {
	e.t.Helper()
	var v sql.NullString
	if err := e.db.QueryRow(`SELECT `+col+` FROM item_metadata WHERE item_id = ?`, itemID).Scan(&v); err != nil {
		e.t.Fatalf("%s.%s: %v", itemID, col, err)
	}
	return v
}

func (e *testDB) claim() (Job, bool) {
	e.t.Helper()
	j, ok, err := e.store.Claim(context.Background(), 10*time.Minute)
	if err != nil {
		e.t.Fatal(err)
	}
	return j, ok
}

func TestAddPendingAndSweep(t *testing.T) {
	e := newTestDB(t)
	movie := e.item("movie", "")
	show := e.item("show", "")
	season := e.item("season", show)
	ctx := context.Background()
	if err := e.store.AddPending(ctx, []string{movie, "missing-item"}); err != nil {
		t.Fatal(err)
	}
	if s := e.col(movie, "state").String; s != "pending" {
		t.Errorf("state = %q", s)
	}
	e.exec(`UPDATE item_metadata SET state = 'matched' WHERE item_id = ?`, movie)
	if err := e.store.AddPending(ctx, []string{movie}); err != nil {
		t.Fatal(err)
	}
	if s := e.col(movie, "state").String; s != "matched" {
		t.Errorf("AddPending reset a matched item to %q", s)
	}
	n, err := e.store.Sweep(ctx)
	if err != nil || n != 2 {
		t.Errorf("Sweep added %d (%v), want the show and its season", n, err)
	}
	if e.col(season, "state").String != "pending" {
		t.Error("season not swept")
	}
	if n, _ := e.store.Sweep(ctx); n != 0 {
		t.Errorf("second sweep added %d", n)
	}
}

func TestClaimOrderAndScope(t *testing.T) {
	e := newTestDB(t)
	ctx := context.Background()
	refresh := e.item("movie", "")
	retry := e.item("movie", "")
	fresh := e.item("movie", "")
	notDue := e.item("movie", "")
	hidden := e.item("movie", "")
	show := e.item("show", "")
	season := e.item("season", show)
	_ = e.store.AddPending(ctx, []string{refresh, retry, fresh, notDue, hidden, show, season})
	past, future := ts(e.now.Add(-time.Hour)), ts(e.now.Add(time.Hour))
	e.exec(`UPDATE item_metadata SET state = 'matched', next_attempt_at = NULL, refresh_at = ? WHERE item_id = ?`, past, refresh)
	e.exec(`UPDATE item_metadata SET attempts = 2, next_attempt_at = ? WHERE item_id = ?`, past, retry)
	e.exec(`UPDATE item_metadata SET next_attempt_at = ? WHERE item_id = ?`, future, notDue)
	e.exec(`UPDATE items SET hidden = 1 WHERE id = ?`, hidden)

	var order []string
	for {
		j, ok := e.claim()
		if !ok {
			break
		}
		order = append(order, j.ItemID)
		if j.ItemID == refresh && !j.Refresh || j.ItemID != refresh && j.Refresh {
			t.Errorf("%s: Refresh = %v", j.ItemID, j.Refresh)
		}
	}
	// New items (fresh, show) first, then the retry, then the refresh.
	if len(order) != 4 || order[2] != retry || order[3] != refresh {
		t.Errorf("claim order = %v", order)
	}
	for _, id := range order {
		if id == notDue || id == hidden || id == season {
			t.Errorf("%s was claimed", id)
		}
	}
}

func TestClaimIsExclusive(t *testing.T) {
	e := newTestDB(t)
	ctx := context.Background()
	var ids []string
	for range 20 {
		ids = append(ids, e.item("movie", ""))
	}
	_ = e.store.AddPending(ctx, ids)
	var (
		mu   sync.Mutex
		seen = map[string]int{}
		wg   sync.WaitGroup
	)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				j, ok, err := e.store.Claim(ctx, time.Minute)
				if err != nil {
					t.Error(err)
					return
				}
				if !ok {
					return
				}
				mu.Lock()
				seen[j.ItemID]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(seen) != 20 {
		t.Errorf("%d items claimed, want 20", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("%s claimed %d times", id, n)
		}
	}
}

func TestFailBackoffAndRelease(t *testing.T) {
	e := newTestDB(t)
	ctx := context.Background()
	id := e.item("movie", "")
	_ = e.store.AddPending(ctx, []string{id})
	j, _ := e.claim()
	if err := e.store.Fail(ctx, j, errors.New("GET https://api.themoviedb.org/3/movie/1?api_key=REDACTED: timeout")); err != nil {
		t.Fatal(err)
	}
	if a := e.col(id, "attempts").String; a != "1" {
		t.Errorf("attempts = %s", a)
	}
	next, _ := time.Parse(time.RFC3339Nano, e.col(id, "next_attempt_at").String)
	if d := next.Sub(e.now); d < 48*time.Second || d > 72*time.Second {
		t.Errorf("first retry in %v, want 1 min ± 20 %%", d)
	}
	if e.col(id, "state").String != "pending" || e.col(id, "last_error").String == "" {
		t.Error("Fail changed the state or kept no error")
	}
	if _, ok := e.claim(); ok {
		t.Error("a failed job was claimable before its retry time")
	}
	e.now = next
	j, ok := e.claim()
	if !ok || j.Attempts != 1 {
		t.Fatalf("retry claim = %+v, %v", j, ok)
	}
	if err := e.store.Release(ctx, j, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	if a := e.col(id, "attempts").String; a != "1" {
		t.Errorf("Release counted an attempt: %s", a)
	}

	for attempts, want := range map[int]time.Duration{1: time.Minute, 2: 2 * time.Minute, 5: 16 * time.Minute, 20: 6 * time.Hour} {
		for range 20 {
			if d := Backoff(attempts); d < want*8/10 || d > want*12/10 {
				t.Errorf("Backoff(%d) = %v, want %v ± 20 %%", attempts, d, want)
			}
		}
	}
}

func TestCounts(t *testing.T) {
	e := newTestDB(t)
	ctx := context.Background()
	a, b, c := e.item("movie", ""), e.item("movie", ""), e.item("movie", "")
	show := e.item("show", "")
	ep := e.item("season", show)
	_ = e.store.AddPending(ctx, []string{a, b, c, show, ep})
	e.exec(`UPDATE item_metadata SET state = 'matched' WHERE item_id IN (?, ?)`, a, show)
	e.exec(`UPDATE item_metadata SET state = 'needs_review' WHERE item_id = ?`, b)
	got, err := e.store.Counts(ctx, "lib1")
	if err != nil || got != (Counters{Pending: 1, Matched: 2, NeedsReview: 1}) {
		t.Errorf("counts = %+v, %v (seasons don't count)", got, err)
	}
}
