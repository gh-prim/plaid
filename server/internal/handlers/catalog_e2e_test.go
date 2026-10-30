package handlers_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/metadata/tmdbtest"
)

// These tests drive F008-R26: a library's top-level items sorted, filtered
// and paged by offset, with a total that respects the age limit.

type catalogPage struct {
	Items []struct {
		ID      string
		Title   *string
		AddedAt *time.Time
	}
	NextCursor *string
	Total      *int
}

func (e *libEnv) catalog(libraryID, query string) catalogPage {
	e.t.Helper()
	r := e.do("GET", "/libraries/"+libraryID+"/items"+query, nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	var p catalogPage
	r.json(&p)
	return p
}

func (p catalogPage) titles() []string {
	out := []string{}
	for _, it := range p.Items {
		out = append(out, *it.Title)
	}
	return out
}

func TestListLibraryItemsSortAndFilter(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	movies := t.TempDir()
	touch(t, movies, "Alien (1979).mkv", "Dune (2021).mkv", "Paddington (2014).mkv")
	ml := e.create("Movies", "movies", movies)
	e.waitIdle(ml.ID)
	ids := map[string]string{}
	for _, it := range e.catalog(ml.ID, "?sort=title").Items {
		ids[*it.Title] = it.ID
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := e.db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`UPDATE items SET created_at = '2026-01-01T00:00:00Z' WHERE id = ?`, ids["Alien"])
	exec(`UPDATE items SET created_at = '2026-10-01T00:00:00Z' WHERE id = ?`, ids["Paddington"])
	exec(`UPDATE items SET created_at = '2026-05-01T00:00:00Z' WHERE id = ?`, ids["Dune"])
	e.rate(ids["Alien"], 18)
	e.rate(ids["Paddington"], 0)
	exec(`UPDATE item_metadata SET genres = '[{"id":878,"name":"Science Fiction"}]' WHERE item_id = ?`, ids["Alien"])

	if got := e.catalog(ml.ID, "?sort=title&order=desc").titles(); !slices.Equal(got, []string{"Paddington", "Dune", "Alien"}) {
		t.Errorf("title desc: %v", got)
	}
	p := e.catalog(ml.ID, "?sort=added&total=true&limit=2")
	if got := p.titles(); !slices.Equal(got, []string{"Paddington", "Dune"}) {
		t.Errorf("added (desc by default): %v", got)
	}
	if p.Total == nil || *p.Total != 3 || p.NextCursor != nil {
		t.Errorf("total %v, nextCursor %v; want 3 and none", p.Total, p.NextCursor)
	}
	if a := p.Items[0].AddedAt; a == nil || !a.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("addedAt = %v", a)
	}
	if got := e.catalog(ml.ID, "?sort=added&limit=2&offset=2").titles(); !slices.Equal(got, []string{"Alien"}) {
		t.Errorf("offset 2: %v", got)
	}
	if got := e.catalog(ml.ID, "?genre=878&genre=12").titles(); !slices.Equal(got, []string{"Alien"}) {
		t.Errorf("genre: %v", got)
	}
	if got := e.catalog(ml.ID, "?yearFrom=2000&yearTo=2015").titles(); !slices.Equal(got, []string{"Paddington"}) {
		t.Errorf("years: %v", got)
	}
	if got := e.catalog(ml.ID, "?watch=unwatched&total=true"); got.Total == nil || *got.Total != 3 {
		t.Errorf("nothing watched yet: total %v", got.Total)
	}
	a := e.catalog(ml.ID, "?sort=random&seed=7").titles()
	if b := e.catalog(ml.ID, "?sort=random&seed=7").titles(); !slices.Equal(a, b) || len(a) != 3 {
		t.Errorf("random with one seed: %v then %v", a, b)
	}
	// The F004 keyset listing is unchanged without the new parameters.
	if got := e.catalog(ml.ID, "").titles(); !slices.Equal(got, []string{"Alien", "Dune", "Paddington"}) {
		t.Errorf("plain listing: %v", got)
	}

	// Facets (F008-R27).
	if f := e.facets(ml.ID); f.Total != 3 || len(f.Genres) != 1 || f.Genres[0].ID != 878 || f.Genres[0].Count != 1 ||
		len(f.Letters) != 3 || f.Letters[0].Letter != "A" || len(f.Decades) != 3 {
		t.Errorf("facets = %+v", f)
	}

	// A kids profile: the total and the filters never count what it can't see.
	r := e.do("POST", "/me/profiles", map[string]any{"name": "Kids", "maxAge": 12}, bearer(e.admin))
	e.expect(r, http.StatusCreated, "")
	var kids struct{ ID string }
	r.json(&kids)
	e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": kids.ID}, bearer(e.admin)), http.StatusOK, "")
	if p := e.catalog(ml.ID, "?total=true"); p.Total == nil || *p.Total != 1 || !slices.Equal(p.titles(), []string{"Paddington"}) {
		t.Errorf("kids: total %v, %v", p.Total, p.titles())
	}
	if p := e.catalog(ml.ID, "?genre=878&total=true"); p.Total == nil || *p.Total != 0 {
		t.Errorf("kids, a genre only Alien has: total %v", p.Total)
	}
	if f := e.facets(ml.ID); f.Total != 1 || len(f.Genres) != 0 || len(f.Letters) != 1 || f.Letters[0].Letter != "P" {
		t.Errorf("kids facets = %+v", f)
	}
	e.expect(e.do("GET", "/libraries/nope/facets", nil, bearer(e.admin)), http.StatusNotFound, "not_found")
}

type facetsJSON struct {
	Total  int
	Genres []struct {
		ID    int `json:"id"`
		Name  string
		Count int
	}
	Decades []struct{ From, Count int }
	Letters []struct {
		Letter string
		Count  int
	}
	Hdr int
}

func (e *libEnv) facets(libraryID string) facetsJSON {
	e.t.Helper()
	r := e.do("GET", "/libraries/"+libraryID+"/facets", nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	var f facetsJSON
	r.json(&f)
	return f
}

func TestListLibraryItemsValidation(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	ml := e.create("Movies", "movies", t.TempDir())
	e.waitIdle(ml.ID)
	for _, q := range []string{
		"?sort=popular", "?order=up", "?watch=half", "?resolution=480i", "?offset=-1",
		"?sort=title&cursor=abc", "?sort=title&parentId=x", "?sort=title&kind=show",
		"?yearFrom=2010&yearTo=2000", "?total=true&limit=500",
	} {
		e.expect(e.do("GET", "/libraries/"+ml.ID+"/items"+q, nil, bearer(e.admin)), http.StatusUnprocessableEntity, "validation_failed")
	}
	e.expect(e.do("GET", "/libraries/nope/items?sort=title", nil, bearer(e.admin)), http.StatusNotFound, "not_found")
}

type showRefPage struct {
	Items []struct {
		ID   string
		Kind string
		Show *struct {
			ID           string
			DisplayTitle string
			Logo         *struct{ ImageID string }
		}
	}
}

// F008-R30: seasons and episodes name their show, with its logo.
func TestItemSummaryShow(t *testing.T) {
	e := newLibEnv(t, libOpts{tmdb: tmdbtest.New(t)})
	shows := t.TempDir()
	touch(t, shows, "Dark (2017)/Season 1/Dark S01E01.mkv", "Dark (2017)/Season 1/Dark S01E02.mkv")
	sl := e.create("Shows", "shows", shows)
	e.waitIdle(sl.ID)
	eventually(t, "Dark matched", func() bool { return e.firstItem(sl.ID, "?kind=show").MatchStatus == "matched" })
	var page showRefPage
	e.do("GET", "/libraries/"+sl.ID+"/items", nil, bearer(e.admin)).json(&page)
	dark := page.Items[0].ID
	if page.Items[0].Show != nil {
		t.Errorf("a show names a show: %+v", page.Items[0].Show)
	}
	var title, logo string
	if err := e.db.QueryRow(`SELECT title FROM item_metadata WHERE item_id = ?`, dark).Scan(&title); err != nil {
		t.Fatal(err)
	}
	eventually(t, "Dark's logo selected", func() bool {
		return e.db.QueryRow(`SELECT id FROM artwork WHERE owner_type = 'item' AND owner_id = ? AND kind = 'logo' AND selected = 1`, dark).Scan(&logo) == nil
	})

	e.do("GET", "/libraries/"+sl.ID+"/items?kind=episode", nil, bearer(e.admin)).json(&page)
	if len(page.Items) != 2 {
		t.Fatalf("%d episodes", len(page.Items))
	}
	for _, ep := range page.Items {
		if s := ep.Show; s == nil || s.ID != dark || s.DisplayTitle != title || s.Logo == nil || s.Logo.ImageID != logo {
			t.Errorf("episode %s show = %+v, want %s %q %s", ep.ID, s, dark, title, logo)
		}
	}
	first := page.Items[0].ID
	e.do("GET", "/libraries/"+sl.ID+"/items?parentId="+dark, nil, bearer(e.admin)).json(&page)
	if s := page.Items[0].Show; page.Items[0].Kind != "season" || s == nil || s.ID != dark {
		t.Errorf("season show = %+v", s)
	}

	// Continue watching offers the next episode, named with its show.
	e.expect(e.do("PUT", "/items/"+first+"/watched", nil, bearer(e.admin)), http.StatusNoContent, "")
	e.do("GET", "/me/continue-watching", nil, bearer(e.admin)).json(&page)
	if len(page.Items) != 1 || page.Items[0].Show == nil || page.Items[0].Show.DisplayTitle != title {
		t.Errorf("continue watching = %+v", page.Items)
	}
}
