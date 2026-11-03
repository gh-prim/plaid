package handlers_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/metadata/tmdbtest"
)

// TestSignInBackdrop drives the sign-in page's poster wall: public, and
// only posters of titles rated 12 or under.
func TestSignInBackdrop(t *testing.T) {
	srv := tmdbtest.New(t)
	e := newLibEnv(t, libOpts{tmdb: srv})
	root := t.TempDir()
	touch(t, root, "Alien (1979) {tmdb-348}/Alien (1979) {tmdb-348}.mkv")
	lib := e.create("Movies", "movies", root) // fr: Alien is rated 12
	e.waitIdle(lib.ID)

	var wall struct{ Posters []string }
	for deadline := time.Now().Add(metadataWait); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		r := e.do("GET", "/auth/backdrop", nil)
		e.expect(r, http.StatusOK, "")
		r.json(&wall)
		if len(wall.Posters) == 1 {
			break
		}
	}
	if len(wall.Posters) != 1 {
		t.Fatalf("wall = %v, want Alien's poster", wall.Posters)
	}
	poster := wall.Posters[0]

	r := e.do("GET", "/auth/backdrop/"+poster+"?w=185", nil)
	e.expect(r, http.StatusOK, "")
	if h := r.Header(); h.Get("Content-Type") != "image/jpeg" || !strings.HasPrefix(h.Get("Cache-Control"), "public") {
		t.Errorf("headers = %v", h)
	}
	e.expect(e.do("GET", "/auth/backdrop/nope", nil), http.StatusNotFound, "not_found")

	// Any other image stays private: the backdrop is not.
	var item struct{ Backdrop *struct{ ImageID string } }
	var page struct{ Items []struct{ ID string } }
	e.do("GET", "/libraries/"+lib.ID+"/items", nil, bearer(e.admin)).json(&page)
	e.do("GET", "/items/"+page.Items[0].ID, nil, bearer(e.admin)).json(&item)
	if item.Backdrop == nil || slices.Contains(wall.Posters, item.Backdrop.ImageID) {
		t.Fatalf("backdrop = %+v", item.Backdrop)
	}
	e.expect(e.do("GET", "/auth/backdrop/"+item.Backdrop.ImageID, nil), http.StatusNotFound, "not_found")

	// Rated 16, Alien leaves the wall.
	if _, err := e.db.Exec(`UPDATE item_metadata SET age_rating = 16`); err != nil {
		t.Fatal(err)
	}
	e.do("GET", "/auth/backdrop", nil).json(&wall)
	if len(wall.Posters) != 0 {
		t.Errorf("wall = %v, want none", wall.Posters)
	}
	e.expect(e.do("GET", "/auth/backdrop/"+poster, nil), http.StatusNotFound, "not_found")
}
