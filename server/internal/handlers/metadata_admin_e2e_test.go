package handlers_test

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/metadata/tmdbtest"
)

// These tests drive F005 PR D: the admin routes, keys, status, live
// counters, attribution, collections and people.

type metaItem struct {
	ID           string
	Kind         string
	DisplayTitle string
	MatchStatus  string
	Metadata     *struct {
		MatchSource  string
		ProviderIDs  struct{ Tmdb int } `json:"providerIds"`
		LockedFields []string
		Collection   *struct{ ID, Name string }
		Credits      []struct{ PersonID, Name, Role string } `json:"credits"`
	}
}

// eventually polls f until it's true or fails the test.
func eventually(t *testing.T, what string, f func() bool) {
	t.Helper()
	for deadline := time.Now().Add(metadataWait); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if f() {
			return
		}
	}
	t.Fatalf("%s never happened", what)
}

func (e *libEnv) itemDetail(id string) metaItem {
	e.t.Helper()
	var it metaItem
	r := e.do("GET", "/items/"+id, nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&it)
	return it
}

func (e *libEnv) firstItem(libraryID, query string) metaItem {
	e.t.Helper()
	var page struct{ Items []metaItem }
	e.do("GET", "/libraries/"+libraryID+"/items"+query, nil, bearer(e.admin)).json(&page)
	if len(page.Items) == 0 {
		return metaItem{}
	}
	return page.Items[0]
}

func TestMetadataSearchAndCandidateImages(t *testing.T) {
	srv := tmdbtest.New(t)
	e := newLibEnv(t, libOpts{tmdb: srv})
	tok := bearer(e.admin)
	type result struct {
		Candidates []struct {
			TmdbID     int `json:"tmdbId"`
			Title      string
			PosterPath *string
			Confidence *float64
		}
	}
	var res result
	r := e.do("GET", "/metadata/search?kind=movie&query=The+Matrix&year=1999", nil, tok)
	e.expect(r, http.StatusOK, "")
	r.json(&res)
	if len(res.Candidates) == 0 || res.Candidates[0].TmdbID != 603 || res.Candidates[0].Confidence != nil {
		t.Errorf("free text = %+v", res.Candidates[:min(2, len(res.Candidates))])
	}
	for _, q := range []string{"tmdb:603", "imdb:tt0133093"} {
		var byID result
		e.do("GET", "/metadata/search?kind=movie&query="+q, nil, tok).json(&byID)
		if len(byID.Candidates) != 1 || byID.Candidates[0].TmdbID != 603 || byID.Candidates[0].PosterPath == nil {
			t.Errorf("%s = %+v", q, byID.Candidates)
		}
	}
	e.expect(e.do("GET", "/metadata/search?kind=movie&query=tmdb:999999", nil, tok), http.StatusNotFound, "match_not_found")

	// Candidate posters go through the server, TMDB file names only.
	r = e.do("GET", "/metadata/candidate-images/abc123.jpg", nil, tok)
	e.expect(r, http.StatusOK, "")
	if r.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("content type %s", r.Header().Get("Content-Type"))
	}
	before := srv.Requests()
	for _, bad := range []string{"..%2F..%2Fetc%2Fpasswd", "abc.gif", "a-b.jpg", "%2Fabc.jpg"} {
		e.expect(e.do("GET", "/metadata/candidate-images/"+bad, nil, tok), http.StatusBadRequest, "bad_request")
	}
	if srv.Requests() != before {
		t.Errorf("a refused path still reached TMDB (%d requests)", srv.Requests()-before)
	}
}

func TestFixMatchLockUnmatchAndReview(t *testing.T) {
	srv := tmdbtest.New(t)
	e := newLibEnv(t, libOpts{tmdb: srv})
	tok := bearer(e.admin)
	root := t.TempDir()
	touch(t, root, "The Office/Season 1/The Office S01E01.mkv")
	lib := e.create("Shows", "shows", root)
	e.waitIdle(lib.ID)
	eventually(t, "The Office needs review", func() bool { return e.firstItem(lib.ID, "").MatchStatus == "needs_review" })
	show := e.firstItem(lib.ID, "")

	var review struct {
		Items []struct {
			Item       struct{ ID string }
			Reason     string
			Candidates []struct {
				TmdbID     int `json:"tmdbId"`
				Confidence *float64
			}
		}
	}
	e.do("GET", "/libraries/"+lib.ID+"/review", nil, tok).json(&review)
	if len(review.Items) != 1 || review.Items[0].Reason != "needs_review" || len(review.Items[0].Candidates) < 2 ||
		review.Items[0].Candidates[0].Confidence == nil {
		t.Fatalf("review = %+v", review)
	}

	// Errors first: wrong kind of item, unknown id, a member.
	var season struct{ Items []struct{ ID string } }
	e.do("GET", "/libraries/"+lib.ID+"/items?parentId="+show.ID, nil, tok).json(&season)
	e.expect(e.do("POST", "/items/"+season.Items[0].ID+"/match", map[string]any{"tmdbId": 2316}, tok), http.StatusUnprocessableEntity, "item_kind_mismatch")
	e.expect(e.do("POST", "/items/"+show.ID+"/match", map[string]any{"tmdbId": 999999}, tok), http.StatusNotFound, "match_not_found")

	r := e.do("POST", "/items/"+show.ID+"/match", map[string]any{"tmdbId": 2316}, tok)
	e.expect(r, http.StatusOK, "")
	// A show answers at once; the queue fetches it (seasons, episodes,
	// images take tens of seconds on a real one).
	var matched metaItem
	r.json(&matched)
	if matched.MatchStatus != "pending" {
		t.Fatalf("right after fix match = %+v", matched)
	}
	eventually(t, "the show fetched", func() bool {
		matched = e.itemDetail(show.ID)
		return matched.MatchStatus == "matched"
	})
	if matched.Metadata == nil || matched.Metadata.MatchSource != "manual" || matched.Metadata.ProviderIDs.Tmdb != 2316 {
		t.Fatalf("after fix match = %+v", matched)
	}
	var episodes struct{ Items []metaItem }
	e.do("GET", "/libraries/"+lib.ID+"/items?kind=episode", nil, tok).json(&episodes)
	if len(episodes.Items) != 1 || episodes.Items[0].DisplayTitle == "" || episodes.Items[0].MatchStatus != "matched" {
		t.Errorf("the episodes follow the show: %+v", episodes.Items)
	}

	// The lock survives a full rescan and a refresh.
	e.expect(e.do("POST", "/libraries/"+lib.ID+"/scan", map[string]any{"full": true}, tok), http.StatusAccepted, "")
	e.waitIdle(lib.ID)
	e.expect(e.do("POST", "/items/"+show.ID+"/refresh", map[string]any{"images": true}, tok), http.StatusAccepted, "")
	time.Sleep(200 * time.Millisecond)
	if it := e.itemDetail(show.ID); it.Metadata.ProviderIDs.Tmdb != 2316 || it.Metadata.MatchSource != "manual" {
		t.Errorf("after rescan and refresh = %+v", it.Metadata)
	}
	var empty struct{ Items []any }
	e.do("GET", "/libraries/"+lib.ID+"/review", nil, tok).json(&empty)
	if len(empty.Items) != 0 {
		t.Errorf("review after the fix = %+v", empty.Items)
	}

	// Unmatch: back to automatic matching, which needs review again.
	e.expect(e.do("DELETE", "/items/"+show.ID+"/match", nil, tok), http.StatusNoContent, "")
	eventually(t, "unmatched show back to review", func() bool { return e.itemDetail(show.ID).MatchStatus == "needs_review" })
	e.expect(e.do("POST", "/libraries/"+lib.ID+"/metadata/refresh", nil, tok), http.StatusAccepted, "")
}

func TestIdentifyAndEditFields(t *testing.T) {
	srv := tmdbtest.New(t)
	e := newLibEnv(t, libOpts{tmdb: srv})
	tok := bearer(e.admin)
	root := t.TempDir()
	touch(t, root, "1080p.BluRay.x264.mkv", "The Matrix (1999).mkv")
	lib := e.create("Movies", "movies", root)
	e.waitIdle(lib.ID)
	var unrec struct{ Files []struct{ ID, Path string } }
	e.do("GET", "/libraries/"+lib.ID+"/unrecognized", nil, tok).json(&unrec)
	if len(unrec.Files) != 1 {
		t.Fatalf("unrecognized = %+v", unrec)
	}
	fileID := unrec.Files[0].ID
	e.expect(e.do("POST", "/media-files/"+fileID+"/identify", map[string]any{"tmdbId": 194, "seasonNumber": 1, "episodeNumbers": []int{1}}, tok),
		http.StatusUnprocessableEntity, "item_kind_mismatch")
	r := e.do("POST", "/media-files/"+fileID+"/identify", map[string]any{"tmdbId": 194}, tok)
	e.expect(r, http.StatusOK, "")
	var amelie metaItem
	r.json(&amelie)
	// The library is French (the admin's locale).
	if amelie.DisplayTitle != "Le Fabuleux Destin d'Amélie Poulain" || amelie.Metadata.MatchSource != "manual" {
		t.Fatalf("identified = %+v", amelie)
	}
	e.expect(e.do("POST", "/media-files/"+fileID+"/identify", map[string]any{"tmdbId": 194}, tok), http.StatusConflict, "already_identified")
	e.expect(e.do("POST", "/libraries/"+lib.ID+"/scan", map[string]any{"full": true}, tok), http.StatusAccepted, "")
	e.waitIdle(lib.ID)
	if it := e.itemDetail(amelie.ID); it.Metadata.ProviderIDs.Tmdb != 194 {
		t.Errorf("after a full rescan = %+v", it)
	}

	// Field edits lock; unlock gives the field back.
	r = e.do("PATCH", "/items/"+amelie.ID+"/metadata", map[string]any{"title": "Amélie", "genres": []string{"Conte"}}, tok)
	e.expect(r, http.StatusOK, "")
	var edited metaItem
	r.json(&edited)
	if edited.DisplayTitle != "Amélie" || !slices.Contains(edited.Metadata.LockedFields, "title") || !slices.Contains(edited.Metadata.LockedFields, "genres") {
		t.Errorf("edited = %+v %+v", edited, edited.Metadata)
	}
	e.expect(e.do("PATCH", "/items/"+amelie.ID+"/metadata", map[string]any{"unlock": []string{"hints"}}, tok), http.StatusUnprocessableEntity, "field_not_editable")
	e.expect(e.do("POST", "/items/"+amelie.ID+"/refresh", nil, tok), http.StatusAccepted, "")
	time.Sleep(200 * time.Millisecond)
	if it := e.itemDetail(amelie.ID); it.DisplayTitle != "Amélie" {
		t.Errorf("a refresh overwrote the locked title: %q", it.DisplayTitle)
	}
	e.expect(e.do("PATCH", "/items/"+amelie.ID+"/metadata", map[string]any{"unlock": []string{"title"}}, tok), http.StatusOK, "")
	eventually(t, "the unlocked title comes back from TMDB", func() bool {
		return e.itemDetail(amelie.ID).DisplayTitle == "Le Fabuleux Destin d'Amélie Poulain"
	})

	// Artwork choice.
	var arts struct {
		Candidates []struct {
			ID               string
			Selected, Locked bool
		}
	}
	e.do("GET", "/items/"+amelie.ID+"/artwork?kind=poster", nil, tok).json(&arts)
	if len(arts.Candidates) < 2 || !arts.Candidates[0].Selected {
		t.Fatalf("poster candidates = %+v", arts)
	}
	pick := arts.Candidates[len(arts.Candidates)-1].ID
	r = e.do("PUT", "/items/"+amelie.ID+"/artwork/poster", map[string]any{"candidateId": pick}, tok)
	e.expect(r, http.StatusOK, "")
	r.json(&arts)
	if arts.Candidates[0].ID != pick || !arts.Candidates[0].Selected || !arts.Candidates[0].Locked {
		t.Errorf("after picking = %+v", arts.Candidates[0])
	}
	e.expect(e.do("PUT", "/items/"+amelie.ID+"/artwork/poster", map[string]any{"candidateId": "nope"}, tok), http.StatusNotFound, "not_found")
	e.expect(e.do("DELETE", "/items/"+amelie.ID+"/artwork/poster", nil, tok), http.StatusNoContent, "")
}

func TestKeysStatusAttributionAndAdminOnly(t *testing.T) {
	srv := tmdbtest.New(t)
	e := newLibEnv(t, libOpts{tmdb: srv})
	tok := bearer(e.admin)

	type keys struct {
		Tmdb   struct{ KeySource string }
		Fanart struct {
			KeySource      string
			PersonalKeySet bool
		}
	}
	const secret = "sekret-tmdb-key-value"
	var bodies []string
	r := e.do("PATCH", "/metadata/settings", map[string]any{"tmdbApiKey": secret}, tok)
	e.expect(r, http.StatusUnprocessableEntity, "invalid_api_key")
	bodies = append(bodies, r.Body.String())
	r = e.do("GET", "/metadata/settings", nil, tok)
	bodies = append(bodies, r.Body.String())
	var k keys
	r.json(&k)
	if k.Tmdb.KeySource != "none" {
		t.Errorf("a rejected key was saved: %+v", k)
	}
	r = e.do("PATCH", "/metadata/settings", map[string]any{"tmdbApiKey": tmdbtest.Key, "fanartPersonalKey": "mine"}, tok)
	e.expect(r, http.StatusOK, "")
	bodies = append(bodies, r.Body.String())
	r.json(&k)
	if k.Tmdb.KeySource != "custom" || !k.Fanart.PersonalKeySet || strings.Contains(r.Body.String(), tmdbtest.Key) {
		t.Errorf("after saving = %s", r.Body.String())
	}
	r = e.do("PATCH", "/metadata/settings", map[string]any{"tmdbApiKey": nil}, tok)
	e.expect(r, http.StatusOK, "")
	r.json(&k)
	if k.Tmdb.KeySource != "none" {
		t.Errorf("null didn't clear the key: %+v", k)
	}

	var status struct {
		Providers []struct{ Provider, State string }
		Libraries []struct{ LibraryID string } `json:"libraries"`
	}
	r = e.do("GET", "/metadata/status", nil, tok)
	e.expect(r, http.StatusOK, "")
	bodies = append(bodies, r.Body.String())
	r.json(&status)
	if len(status.Providers) != 2 || status.Providers[0].Provider != "tmdb" || status.Providers[0].State != "ok" {
		t.Errorf("status = %+v", status)
	}

	var server struct {
		Attributions []struct{ Provider, Notice string }
	}
	e.do("GET", "/server", nil).json(&server)
	if len(server.Attributions) != 2 || !strings.Contains(server.Attributions[0].Notice, "not endorsed or certified by TMDB") {
		t.Errorf("attributions = %+v", server.Attributions)
	}

	for _, b := range append(bodies, e.logs.String()) {
		if strings.Contains(b, secret) {
			t.Errorf("the key leaked: %s", b)
		}
	}

	// A member gets 403 on every admin route of F005.
	member := bearer(e.member(e.admin, "kim").Token)
	for _, route := range [][2]string{
		{"GET", "/metadata/search?kind=movie&query=x"}, {"GET", "/metadata/candidate-images/a.jpg"},
		{"GET", "/metadata/status"}, {"GET", "/metadata/settings"}, {"PATCH", "/metadata/settings"},
		{"GET", "/libraries/x/review"}, {"POST", "/libraries/x/metadata/refresh"},
		{"POST", "/items/x/match"}, {"DELETE", "/items/x/match"}, {"PATCH", "/items/x/metadata"},
		{"POST", "/items/x/refresh"}, {"GET", "/items/x/artwork?kind=poster"}, {"PUT", "/items/x/artwork/poster"},
		{"DELETE", "/items/x/artwork/poster"}, {"POST", "/media-files/x/identify"},
	} {
		var b any
		if route[0] == "POST" || route[0] == "PATCH" || route[0] == "PUT" {
			b = map[string]any{"tmdbId": 1, "candidateId": "x"}
		}
		e.expect(e.do(route[0], route[1], b, member), http.StatusForbidden, "forbidden")
	}
}

func TestCollectionsAndPeople(t *testing.T) {
	srv := tmdbtest.New(t)
	e := newLibEnv(t, libOpts{tmdb: srv})
	tok := bearer(e.admin)
	root := t.TempDir()
	touch(t, root, "Alien (1979) {tmdb-348}/Alien (1979) {tmdb-348}.mkv")
	lib := e.create("Movies", "movies", root)
	e.waitIdle(lib.ID)
	var alien metaItem
	eventually(t, "Alien matched", func() bool {
		it := e.firstItem(lib.ID, "")
		if it.MatchStatus != "matched" {
			return false
		}
		alien = e.itemDetail(it.ID)
		return alien.Metadata != nil && len(alien.Metadata.Credits) > 0
	})

	var cols struct {
		Collections []struct {
			ID         string
			Name       string
			MovieCount int
		}
	}
	e.do("GET", "/collections?libraryId="+lib.ID, nil, tok).json(&cols)
	if len(cols.Collections) != 1 || cols.Collections[0].MovieCount != 1 {
		t.Fatalf("collections = %+v", cols)
	}
	var col struct{ Items []struct{ ID string } }
	e.do("GET", "/collections/"+cols.Collections[0].ID, nil, tok).json(&col)
	if len(col.Items) != 1 || col.Items[0].ID != alien.ID {
		t.Errorf("collection items = %+v", col.Items)
	}
	var cast string
	for _, c := range alien.Metadata.Credits {
		if c.Role == "cast" {
			cast = c.PersonID
			break
		}
	}
	var person struct {
		Name    string
		Credits []struct {
			Item struct{ ID string }
			Role string
		}
	}
	e.do("GET", "/people/"+cast, nil, tok).json(&person)
	if person.Name == "" || len(person.Credits) != 1 || person.Credits[0].Item.ID != alien.ID {
		t.Errorf("person = %+v", person)
	}
	e.expect(e.do("GET", "/people/nope", nil, tok), http.StatusNotFound, "not_found")

	// Alien is 12 in France (the library's region): a 10 profile sees
	// neither the collection nor the credit.
	r := e.do("POST", "/me/profiles", map[string]any{"name": "Kids", "maxAge": 10}, tok)
	e.expect(r, http.StatusCreated, "")
	var kids struct{ ID string }
	r.json(&kids)
	e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": kids.ID}, tok), http.StatusOK, "")
	e.do("GET", "/collections?libraryId="+lib.ID, nil, tok).json(&cols)
	if len(cols.Collections) != 0 {
		t.Errorf("kids see collections %+v", cols)
	}
	e.expect(e.do("GET", "/collections/"+col.Items[0].ID, nil, tok), http.StatusNotFound, "not_found")
	e.do("GET", "/people/"+cast, nil, tok).json(&person)
	if len(person.Credits) != 0 {
		t.Errorf("kids see credits %+v", person.Credits)
	}
}

// TestMetadataEventsOnTheScanStream: the scan SSE stream carries metadata
// counters (R35).
func TestMetadataEventsOnTheScanStream(t *testing.T) {
	srv := tmdbtest.New(t)
	e := newLibEnv(t, libOpts{tmdb: srv})
	root := t.TempDir()
	lib := e.create("Movies", "movies", root)
	e.waitIdle(lib.ID)

	s := httptest.NewServer(e.h)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), metadataWait)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", s.URL+"/api/v1/libraries/"+lib.ID+"/scan/events", nil)
	req.Header.Set("Authorization", "Bearer "+e.admin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	touch(t, root, "The Matrix (1999)/The Matrix (1999).mkv")
	e.expect(e.do("POST", "/libraries/"+lib.ID+"/scan", nil, bearer(e.admin)), http.StatusAccepted, "")

	sc := bufio.NewScanner(resp.Body)
	event := ""
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "event: ") {
			event = strings.TrimPrefix(line, "event: ")
		}
		if event == "metadata" && strings.HasPrefix(line, "data: ") {
			if !strings.Contains(line, `"type":"metadata"`) || !strings.Contains(line, `"needsReview"`) {
				t.Errorf("metadata event = %s", line)
			}
			if strings.Contains(line, `"matched":1`) {
				return
			}
		}
	}
	t.Fatalf("no metadata event with the match: %v", fmt.Sprint(sc.Err()))
}
