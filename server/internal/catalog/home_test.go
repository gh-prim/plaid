package catalog

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/access"
)

// many adds n movies to library lm with one genre, added a minute apart.
func (f *fixture) many(prefix string, n int, genre int, name string, ageRating *int) []string {
	f.t.Helper()
	var ids []string
	for i := range n {
		id := fmt.Sprintf("%s%02d", prefix, i)
		f.movie(movie{id: id, title: id, added: fmt.Sprintf("2026-09-01T00:%02d:00Z", i), age: ageRating,
			genres: fmt.Sprintf(`[{"id":%d,"name":%q}]`, genre, name)})
		ids = append(ids, id)
	}
	return ids
}

func (f *fixture) backdrop(itemID string) {
	f.t.Helper()
	f.exec(`INSERT INTO artwork (id, owner_type, owner_id, kind, source, selected, created_at) VALUES (?, 'item', ?, 'backdrop', 'tmdb', 1, ?)`, "bd-"+itemID, itemID, now)
}

func (f *fixture) home(q HomeQuery) (string, []Row) {
	f.t.Helper()
	if q.ProfileID == "" {
		q.ProfileID = "p1"
	}
	a, rows, err := f.c.Home(context.Background(), q)
	if err != nil {
		f.t.Fatal(err)
	}
	return a, rows
}

func rowIDs(rows []Row) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

func TestHomeEmpty(t *testing.T) {
	f := newFixture(t)
	a, rows := f.home(HomeQuery{})
	if a != "" || len(rows) != 0 {
		t.Errorf("empty home: %q %+v", a, rows)
	}
}

func TestHomeRows(t *testing.T) {
	f := newFixture(t)
	scifi := f.many("sf", 12, 878, "Science Fiction", age(12))
	f.many("dr", 11, 18, "Drama", age(16))
	f.many("co", 9, 35, "Comedy", age(0)) // under 10: no row
	f.many("an", 10, 16, "Animation", age(0))
	f.many("ho", 13, 27, "Horror", age(18))
	f.many("we", 14, 37, "Western", age(12))
	f.watch("p1", "dr03", 0, true, "2026-10-01T00:00:00Z")

	cw := []string{"sf01", "dr02"}
	_, rows := f.home(HomeQuery{ContinueWatching: cw})
	// Continue watching, Movies (Shows is empty), then 4 genres: watched first, then by size.
	want := []string{"continue_watching", "recently_added.lm", "genre.18", "genre.37", "genre.27", "genre.878"}
	if got := rowIDs(rows); !slices.Equal(got, want) {
		t.Fatalf("rows %v, want %v", got, want)
	}
	if !slices.Equal(rows[0].IDs, cw) || rows[0].Next != "" {
		t.Errorf("continue watching: %+v", rows[0])
	}
	ra := rows[1]
	if len(ra.IDs) != HomeRowSize || ra.IDs[0] != "we13" || ra.Next == "" || ra.LibraryName != "Movies" ||
		*ra.SeeAll != (SeeAll{LibraryID: "lm", Sort: SortAdded, Desc: true}) {
		t.Errorf("recently added: %+v", ra)
	}
	g := rows[5]
	if g.Genre == nil || *g.Genre != (GenreRef{878, "Science Fiction"}) || *g.SeeAll != (SeeAll{LibraryID: "lm", Sort: SortTitle, Genre: 878}) {
		t.Errorf("genre row: %+v", g)
	}
	if got := slices.Sorted(slices.Values(g.IDs)); !slices.Equal(got, scifi) {
		t.Errorf("genre items: %v", g.IDs)
	}

	// Paging a row tiles it.
	next, err := f.c.HomeRow(context.Background(), HomeQuery{ProfileID: "p1"}, "recently_added.lm", ra.Next, HomeRowSize)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.IDs) != HomeRowSize || next.Next == "" { // 69 movies
		t.Errorf("second page: %d items, next %q", len(next.IDs), next.Next)
	}
	if slices.Contains(next.IDs, ra.IDs[0]) {
		t.Error("pages overlap")
	}

	// A kid's home: what's above the limit is gone, rows and genres alike.
	_, kid := f.home(HomeQuery{Access: access.Limit{MaxAge: age(12)}})
	if got := rowIDs(kid); !slices.Equal(got, []string{"recently_added.lm", "genre.37", "genre.878", "genre.16"}) {
		t.Errorf("kid rows: %v", got)
	}
}

func TestHomeGenreOrderAndName(t *testing.T) {
	f := newFixture(t)
	f.many("sf", 15, 878, "Science Fiction", nil)
	day := time.Date(2026, 10, 30, 9, 0, 0, 0, time.Local)
	f.c.now = func() time.Time { return day }
	_, a := f.home(HomeQuery{})
	f.c.now = func() time.Time { return day.Add(8 * time.Hour) }
	_, b := f.home(HomeQuery{})
	if !slices.Equal(a[1].IDs, b[1].IDs) {
		t.Error("genre order changed within the day")
	}
	changed := false
	for d := 1; d <= 5 && !changed; d++ {
		f.c.now = func() time.Time { return day.AddDate(0, 0, d) }
		_, c := f.home(HomeQuery{})
		changed = !slices.Equal(a[1].IDs, c[1].IDs)
	}
	if !changed {
		t.Error("genre order never changes from day to day")
	}
	_, other := f.home(HomeQuery{ProfileID: "p2"})
	if slices.Equal(a[1].IDs, other[1].IDs) {
		t.Error("two profiles share the same order")
	}

	// A French library names the genre for a French profile.
	f.exec(`INSERT INTO libraries (id, name, type, metadata_language, created_at, updated_at) VALUES ('lf', 'Films', 'movies', 'fr', ?, ?)`, now, now)
	f.exec(`INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES ('fr1', 'lf', 'movie', 'Fr', ?, ?)`, now, now)
	f.exec(`INSERT INTO item_metadata (item_id, state, genres, updated_at) VALUES ('fr1', 'matched', '[{"id":878,"name":"Science-Fiction"}]', ?)`, now)
	f.exec(`UPDATE profiles SET locale = 'fr' WHERE id = 'p2'`)
	if _, r := f.home(HomeQuery{ProfileID: "p2"}); r[len(r)-1].Genre.Name != "Science-Fiction" {
		t.Errorf("fr profile: %+v", r[len(r)-1].Genre)
	}
	if _, r := f.home(HomeQuery{}); r[len(r)-1].Genre.Name != "Science Fiction" || len(r[len(r)-1].IDs) != 16 {
		t.Errorf("en profile: %+v", r[len(r)-1])
	}
}

func TestHomeAmbient(t *testing.T) {
	f := newFixture(t)
	seedMovies(f)
	seedShow(f)
	if a, _ := f.home(HomeQuery{}); a != "" {
		t.Errorf("no backdrop anywhere: %q", a)
	}
	f.backdrop("alien")
	f.backdrop("toy")
	if a, _ := f.home(HomeQuery{}); a != "toy" {
		t.Errorf("most recently added with a backdrop: %q", a)
	}
	f.exec(`UPDATE item_metadata SET state = 'pending' WHERE item_id = 'toy'`)
	if a, _ := f.home(HomeQuery{}); a != "alien" {
		t.Errorf("only matched items: %q", a)
	}
	// Continue watching wins; an episode lends its show's backdrop.
	f.backdrop("dark")
	if a, _ := f.home(HomeQuery{ContinueWatching: []string{"amelie", "s1e2", "alien"}}); a != "dark" {
		t.Errorf("episode in progress: %q", a)
	}
	if a, _ := f.home(HomeQuery{ContinueWatching: []string{"amelie", "alien"}}); a != "alien" {
		t.Errorf("first with a backdrop: %q", a)
	}
}

func TestHomeRowErrors(t *testing.T) {
	f := newFixture(t)
	for _, id := range []string{"nope", "genre.x", "genre.99", "recently_added.zz", "continue_watching.x"} {
		if _, err := f.c.HomeRow(context.Background(), HomeQuery{}, id, "", 20); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v", id, err)
		}
	}
	if _, err := f.c.HomeRow(context.Background(), HomeQuery{}, "continue_watching", "!!", 20); !errors.Is(err, ErrBadCursor) {
		t.Errorf("bad cursor: %v", err)
	}
}
