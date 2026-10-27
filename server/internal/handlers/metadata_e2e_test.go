package handlers_test

import (
	"bytes"
	"image/jpeg"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/metadata/tmdbtest"
)

// metadataWait bounds how long these tests wait for the background queue.
// Locally it takes well under a second; CI runners under -race are several
// times slower (decoding and resizing images dominates).
const metadataWait = 30 * time.Second

// TestImagesThroughTheAPI drives F005 R19 and R25 through GET /images.
func TestImagesThroughTheAPI(t *testing.T) {
	srv := tmdbtest.New(t)
	e := newLibEnv(t, libOpts{tmdb: srv})
	root := t.TempDir()
	touch(t, root, "Alien (1979) {tmdb-348}/Alien (1979) {tmdb-348}.mkv")
	lib := e.create("Movies", "movies", root)
	e.waitIdle(lib.ID)

	type art struct {
		ImageID string `json:"imageId"`
		Colors  *struct{ Dominant, OnDominant string }
	}
	var item struct {
		ID       string
		Poster   *art
		Backdrop *art
		Metadata struct{ Logo *art }
	}
	for deadline := time.Now().Add(metadataWait); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var page struct{ Items []struct{ ID string } }
		e.do("GET", "/libraries/"+lib.ID+"/items", nil, bearer(e.admin)).json(&page)
		if len(page.Items) == 1 {
			e.do("GET", "/items/"+page.Items[0].ID, nil, bearer(e.admin)).json(&item)
			if item.Poster != nil && item.Poster.Colors != nil && item.Metadata.Logo != nil {
				break
			}
		}
	}
	if item.Poster == nil || item.Poster.Colors == nil || item.Backdrop == nil || item.Metadata.Logo == nil {
		t.Fatalf("artwork never showed up: %+v", item)
	}

	r := e.do("GET", "/images/"+item.Poster.ImageID+"?w=300", nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	h := r.Header()
	if h.Get("Content-Type") != "image/jpeg" || h.Get("ETag") == "" || h.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(h.Get("Cache-Control"), "immutable") {
		t.Errorf("headers = %v", h)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(r.Body.Bytes()))
	if err != nil || cfg.Width != 342 {
		t.Errorf("poster at w=300: %dpx (%v), want 342", cfg.Width, err)
	}
	r2 := e.do("GET", "/images/"+item.Poster.ImageID+"?w=300", nil, bearer(e.admin), header("If-None-Match", h.Get("ETag")))
	e.expect(r2, http.StatusNotModified, "")

	logo := e.do("GET", "/images/"+item.Metadata.Logo.ImageID+"?w=185", nil, bearer(e.admin))
	e.expect(logo, http.StatusOK, "")
	if logo.Header().Get("Content-Type") != "image/png" {
		t.Errorf("logo type = %s", logo.Header().Get("Content-Type"))
	}
	e.expect(e.do("GET", "/images/nope", nil, bearer(e.admin)), http.StatusNotFound, "not_found")
	e.expect(e.do("GET", "/images/"+item.Poster.ImageID, nil), http.StatusUnauthorized, "unauthorized")

	// The library is French (the admin's locale), where Alien is rated 12:
	// a 10 profile gets a 404 for its poster.
	r = e.do("POST", "/me/profiles", map[string]any{"name": "Kids", "maxAge": 10}, bearer(e.admin))
	e.expect(r, http.StatusCreated, "")
	var kids struct{ ID string }
	r.json(&kids)
	e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": kids.ID}, bearer(e.admin)), http.StatusOK, "")
	e.expect(e.do("GET", "/images/"+item.Poster.ImageID+"?w=300", nil, bearer(e.admin)), http.StatusNotFound, "not_found")
}

// TestMetadataThroughTheAPI drives F005 PR B end to end: a scan feeds the
// queue, items get their metadata in the background, and the read API
// shows it (R1, R7, R8, R10, R22–R24, R36).
func TestMetadataThroughTheAPI(t *testing.T) {
	srv := tmdbtest.New(t)
	e := newLibEnv(t, libOpts{tmdb: srv})
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
		for deadline := time.Now().Add(metadataWait); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
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
