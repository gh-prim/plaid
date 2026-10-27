package handlers_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/metadata/tmdbtest"
)

// TestMetadataThroughTheAPI drives F005 PR B end to end: a scan feeds the
// queue, items get their metadata in the background, and the read API
// shows it (R1, R7, R8, R10, R22–R24, R36).
func TestMetadataThroughTheAPI(t *testing.T) {
	srv := tmdbtest.New(t)
	e := newLibEnv(t, libOpts{metadata: srv.Client()})
	root := t.TempDir()
	touch(t, root, "Alien (1979) {tmdb-348}/Alien (1979) {tmdb-348}.mkv")
	lib := e.create("Movies", "movies", root) // the test admin's locale is fr
	e.waitIdle(lib.ID)

	type summary struct {
		ID            string
		Title         *string
		DisplayTitle  string
		MatchStatus   string
		Certification *string
		AgeRating     *int
		Genres        []string
		ReleaseDate   *string
	}
	waitFor := func(want string) summary {
		t.Helper()
		var page struct{ Items []summary }
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			e.do("GET", "/libraries/"+lib.ID+"/items", nil, bearer(e.admin)).json(&page)
			if len(page.Items) == 1 && page.Items[0].DisplayTitle == want {
				return page.Items[0]
			}
		}
		t.Fatalf("display title never became %q: %+v", want, page.Items)
		return summary{}
	}
	// The library was created in French (the admin's locale).
	alien := waitFor("Alien, le huitième passager")
	if alien.MatchStatus != "matched" || *alien.Title != "Alien" || alien.Certification == nil || alien.AgeRating == nil ||
		len(alien.Genres) == 0 || alien.ReleaseDate == nil || *alien.ReleaseDate != "1979-05-25" {
		t.Errorf("summary = %+v", alien)
	}

	var detail struct {
		DisplayTitle string
		Metadata     struct {
			MatchSource string
			Overview    string
			Trailers    []struct{ Key, Language string }
			Credits     []struct{ Name, Role, Character string }
			Collection  *struct{ Name string }
			ProviderIDs struct{ Tmdb int } `json:"providerIds"`
		}
	}
	e.do("GET", "/items/"+alien.ID, nil, bearer(e.admin)).json(&detail)
	m := detail.Metadata
	if m.MatchSource != "hint" || m.Overview == "" || m.ProviderIDs.Tmdb != 348 || m.Collection == nil || len(m.Trailers) == 0 {
		t.Errorf("detail = %+v", m)
	}
	if len(m.Credits) == 0 || m.Credits[0].Role != "director" {
		t.Errorf("credits start with the crew: %+v", m.Credits[:min(len(m.Credits), 2)])
	}

	// Back to English: the whole library refreshes, keeping the match.
	r := e.do("PATCH", "/libraries/"+lib.ID, map[string]any{"metadataLanguage": "en", "metadataRegion": "US"}, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	var l struct{ MetadataLanguage, MetadataRegion string }
	r.json(&l)
	if l.MetadataLanguage != "en" || l.MetadataRegion != "US" {
		t.Errorf("library = %+v", l)
	}
	waitFor("Alien")
	for _, p := range srv.Paths() {
		if strings.HasPrefix(p, "/search/") {
			t.Errorf("a language change searched again: %s", p)
		}
	}
	for _, bad := range []map[string]any{{"metadataLanguage": "FR"}, {"metadataLanguage": "fra"}, {"metadataRegion": "fr"}} {
		e.expect(e.do("PATCH", "/libraries/"+lib.ID, bad, bearer(e.admin)), http.StatusUnprocessableEntity, "validation_failed")
	}
}
