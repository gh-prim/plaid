package metadata

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// A matched item whose artwork couldn't be written is refreshed again soon,
// never pushed later than its scheduled refresh.
func TestRefreshBy(t *testing.T) {
	e := newTestDB(t)
	ctx := context.Background()
	id := e.item("movie", "")
	_ = e.store.AddPending(ctx, []string{id})
	now := e.store.now()
	far := now.Add(30 * 24 * time.Hour)
	if _, err := e.store.db.ExecContext(ctx, `UPDATE item_metadata SET state = 'matched', refresh_at = ? WHERE item_id = ?`, ts(far), id); err != nil {
		t.Fatal(err)
	}
	if _, ok := e.claim(); ok {
		t.Fatal("a matched item was due before its refresh")
	}
	if err := e.store.refreshBy(ctx, id, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, ok := e.claim(); !ok {
		t.Fatal("the item wasn't due after refreshBy")
	}
	if err := e.store.refreshBy(ctx, id, far.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var got string
	_ = e.store.db.QueryRowContext(ctx, `SELECT refresh_at FROM item_metadata WHERE item_id = ?`, id).Scan(&got)
	if got != ts(now.Add(-time.Second)) {
		t.Errorf("refresh_at = %s; refreshBy pushed it back", got)
	}
}

// A show's children are written one season (with its episodes) at a time.
func TestBySeasonGroup(t *testing.T) {
	got := bySeasonGroup([]child{
		{ID: "s1", Kind: "season"}, {ID: "s2", Kind: "season"},
		{ID: "e1", Kind: "episode", ParentID: "s1"}, {ID: "e2", Kind: "episode", ParentID: "s2"},
		{ID: "e3", Kind: "episode", ParentID: "s1"}, {ID: "x", Kind: "episode", ParentID: "gone"},
	})
	var ids [][]string
	for _, g := range got {
		var row []string
		for _, c := range g {
			row = append(row, c.ID)
		}
		ids = append(ids, row)
	}
	want := [][]string{{"s1", "e1", "e3"}, {"s2", "e2"}, {"x"}}
	if fmt.Sprint(ids) != fmt.Sprint(want) {
		t.Errorf("groups = %v; want %v", ids, want)
	}
}

// The profile picker is lit by what each profile played last; an episode
// lends its show's backdrop.
func TestLastWatchedBackdrops(t *testing.T) {
	e := newTestDB(t)
	ctx := context.Background()
	e.exec(`INSERT INTO users (id, username, display_name, password_hash, role, created_at, updated_at) VALUES ('u', 'u', 'U', 'x', 'admin', 'x', 'x')`)
	for _, p := range []string{"p1", "p2", "p3"} {
		e.exec(`INSERT INTO profiles (id, user_id, name, created_at, updated_at) VALUES (?, 'u', ?, 'x', 'x')`, p, p)
	}
	movie := e.item("movie", "")
	old := e.item("movie", "")
	show := e.item("show", "")
	season := e.item("season", show)
	ep := e.item("episode", season)
	for _, o := range []string{movie, old, show} {
		e.exec(`INSERT INTO artwork (id, owner_type, owner_id, kind, source, selected, created_at) VALUES (?, 'item', ?, 'backdrop', 'tmdb', 1, 'x')`, "bd-"+o, o)
	}
	play := func(p, item, at string) {
		e.exec(`INSERT INTO watch_state (profile_id, item_id, last_played_at, updated_at) VALUES (?, ?, ?, 'x')`, p, item, at)
	}
	play("p1", old, "2026-10-01T00:00:00Z")
	play("p1", movie, "2026-10-02T00:00:00Z")
	play("p2", ep, "2026-10-02T00:00:00Z")
	got, err := e.store.LastWatchedBackdrops(ctx, []string{"p1", "p2", "p3"})
	if err != nil {
		t.Fatal(err)
	}
	if got["p1"].ID != "bd-"+movie || got["p2"].ID != "bd-"+show {
		t.Errorf("backdrops = %+v", got)
	}
	if _, ok := got["p3"]; ok {
		t.Error("a profile that watched nothing got a backdrop")
	}
}
