package handlers_test

import (
	"net/http"
	"slices"
	"sort"
	"testing"
)

// These tests drive F005 R25: the item read routes hide what the active
// profile's age limit doesn't allow, and answer 404 for it.

// rate sets an item's age rating the way F005's matching will, copying it
// to the item's children like a show's rating goes to its seasons and
// episodes. age < 0 means unrated.
func (e *libEnv) rate(itemID string, age int) {
	e.t.Helper()
	var v any
	if age >= 0 {
		v = age
	}
	if _, err := e.db.Exec(`WITH RECURSIVE d(id) AS (SELECT ? UNION ALL SELECT i.id FROM items i JOIN d ON i.parent_id = d.id)
		INSERT INTO item_metadata (item_id, state, age_rating, updated_at)
		SELECT id, 'matched', ?, '2026-10-27T12:00:00Z' FROM d
		WHERE true ON CONFLICT (item_id) DO UPDATE SET age_rating = excluded.age_rating`, itemID, v); err != nil {
		e.t.Fatal(err)
	}
}

func (e *libEnv) titles(libraryID, token string, query string) []string {
	e.t.Helper()
	r := e.do("GET", "/libraries/"+libraryID+"/items"+query, nil, bearer(token))
	e.expect(r, http.StatusOK, "")
	var page itemPage
	r.json(&page)
	out := []string{}
	for _, it := range page.Items {
		if it.Title != nil {
			out = append(out, *it.Title)
		} else {
			out = append(out, it.Kind)
		}
	}
	sort.Strings(out)
	return out
}

func TestAgeLimitsOnItems(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	movies := t.TempDir()
	touch(t, movies, "Dune (2021).mkv", "Paddington (2014).mkv", "Home Movie (2020).mkv")
	shows := t.TempDir()
	touch(t, shows, "Dark/Season 1/Dark S01E01.mkv")
	ml := e.create("Movies", "movies", movies)
	sl := e.create("Shows", "shows", shows)
	e.waitIdle(ml.ID)
	e.waitIdle(sl.ID)

	ids := map[string]string{}
	var page itemPage
	e.do("GET", "/libraries/"+ml.ID+"/items", nil, bearer(e.admin)).json(&page)
	for _, it := range page.Items {
		ids[*it.Title] = it.ID
	}
	e.do("GET", "/libraries/"+sl.ID+"/items", nil, bearer(e.admin)).json(&page)
	dark := page.Items[0].ID
	e.do("GET", "/libraries/"+sl.ID+"/items?kind=episode", nil, bearer(e.admin)).json(&page)
	episode := page.Items[0].ID
	e.rate(ids["Dune"], 13)       // PG-13
	e.rate(ids["Paddington"], 10) // PG
	e.rate(dark, 18)              // TV-MA, with its season and episode
	// Home Movie stays unrated.

	all := []string{"Dune", "Home Movie", "Paddington"}
	if got := e.titles(ml.ID, e.admin, ""); !slices.Equal(got, all) {
		t.Errorf("unrestricted admin profile sees %v, want %v", got, all)
	}

	// A kids profile on the admin's own account (F005 R25 applies to admins too).
	r := e.do("POST", "/me/profiles", map[string]any{"name": "Kids", "maxAge": 12}, bearer(e.admin))
	e.expect(r, http.StatusCreated, "")
	var kids struct{ ID string }
	r.json(&kids)
	e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": kids.ID}, bearer(e.admin)), http.StatusOK, "")

	if got := e.titles(ml.ID, e.admin, ""); !slices.Equal(got, []string{"Paddington"}) {
		t.Errorf("kids (12, no unrated) see %v, want [Paddington]", got)
	}
	e.expect(e.do("GET", "/items/"+ids["Paddington"], nil, bearer(e.admin)), http.StatusOK, "")
	for _, id := range []string{ids["Dune"], ids["Home Movie"], dark, episode} {
		e.expect(e.do("GET", "/items/"+id, nil, bearer(e.admin)), http.StatusNotFound, "not_found")
	}
	if got := e.titles(sl.ID, e.admin, ""); len(got) != 0 {
		t.Errorf("kids see shows %v, want none", got)
	}
	if got := e.titles(sl.ID, e.admin, "?kind=episode"); len(got) != 0 {
		t.Errorf("kids see episodes %v, want none", got)
	}
	if got := e.titles(sl.ID, e.admin, "?parentId="+dark); len(got) != 0 {
		t.Errorf("kids see Dark's seasons %v, want none", got)
	}

	// Back on the admin profile, unrated titles allowed for the kids.
	r = e.do("GET", "/me", nil, bearer(e.admin))
	var me struct {
		Profiles []struct {
			ID        string
			IsDefault bool `json:"isDefault"`
		}
	}
	r.json(&me)
	var own string
	for _, p := range me.Profiles {
		if p.IsDefault {
			own = p.ID
		}
	}
	e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": own}, bearer(e.admin)), http.StatusOK, "")
	e.expect(e.do("PATCH", "/me/profiles/"+kids.ID, map[string]any{"allowUnrated": true, "password": adminPassword}, bearer(e.admin)), http.StatusOK, "")
	e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": kids.ID}, bearer(e.admin)), http.StatusOK, "")
	if got := e.titles(ml.ID, e.admin, ""); !slices.Equal(got, []string{"Home Movie", "Paddington"}) {
		t.Errorf("kids with unrated allowed see %v", got)
	}

	// No profile picked: the default profile's limit applies.
	e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": own}, bearer(e.admin)), http.StatusOK, "")
	e.expect(e.do("PATCH", "/me/profiles/"+own, map[string]any{"maxAge": 10, "allowUnrated": false, "password": adminPassword}, bearer(e.admin)), http.StatusOK, "")
	fresh := e.login("Ada", adminPassword)
	if fresh.ActiveProfile != nil {
		t.Fatalf("a two-profile account got a profile picked: %+v", fresh.ActiveProfile)
	}
	if got := e.titles(ml.ID, fresh.Token, ""); !slices.Equal(got, []string{"Paddington"}) {
		t.Errorf("no profile picked sees %v, want the default profile's [Paddington]", got)
	}
}
