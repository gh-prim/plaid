package handlers_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gh-prim/plaid/server/internal/metadata/tmdbtest"
)

// TestProblemsFeed: a library folder that goes away twice is one problem
// seen twice, resolved when it's back; members can't read the feed (F007
// R29).
func TestProblemsFeed(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	member := e.member(e.admin, "carol").Token
	e.expect(e.do("GET", "/problems", nil, bearer(member)), http.StatusForbidden, "forbidden")

	root := t.TempDir()
	touch(t, root, "Heat (1995)/Heat (1995).mkv")
	lib := e.create("Movies", "movies", root)
	e.waitIdle(lib.ID)

	away := root + ".away"
	for range 2 {
		if err := os.Rename(root, away); err != nil {
			t.Fatal(err)
		}
		e.expect(e.do("POST", "/libraries/"+lib.ID+"/scan", nil, bearer(e.admin)), http.StatusAccepted, "")
		e.waitIdle(lib.ID)
		if err := os.Rename(away, root); err != nil {
			t.Fatal(err)
		}
		e.expect(e.do("POST", "/libraries/"+lib.ID+"/scan", nil, bearer(e.admin)), http.StatusAccepted, "")
		e.waitIdle(lib.ID)
	}

	var out struct {
		Problems []struct {
			Code       string
			Params     map[string]string
			Count      int
			ResolvedAt *string
		}
	}
	r := e.do("GET", "/problems", nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&out)
	if len(out.Problems) != 1 {
		t.Fatalf("problems = %+v, want one", out.Problems)
	}
	p := out.Problems[0]
	resolved, _ := filepath.EvalSymlinks(root) // the library keeps resolved paths
	if p.Code != "folder_unavailable" || p.Params["path"] != resolved || p.Count != 2 || p.ResolvedAt == nil {
		t.Errorf("problem = %+v", p)
	}
}

// TestHardwareTestRoute: admins re-run the hardware test; members can't
// (F007 R28).
func TestHardwareTestRoute(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	member := e.member(e.admin, "carol").Token
	e.expect(e.do("POST", "/server/capabilities/hardware-test", nil, bearer(member)), http.StatusForbidden, "forbidden")

	r := e.do("POST", "/server/capabilities/hardware-test", nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	var st struct{ Configured string }
	r.json(&st)
	if st.Configured == "" {
		t.Errorf("status = %s", r.Body.String())
	}
}

// TestMatchedEventsOnTheScanStream: a movie that matches during a scan
// reaches the stream with its title, year and poster (F007 R27).
func TestMatchedEventsOnTheScanStream(t *testing.T) {
	e := newLibEnv(t, libOpts{tmdb: tmdbtest.New(t)})
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
	touch(t, root, "Alien (1979) {tmdb-348}/Alien (1979) {tmdb-348}.mkv")
	e.expect(e.do("POST", "/libraries/"+lib.ID+"/scan", nil, bearer(e.admin)), http.StatusAccepted, "")

	sc := bufio.NewScanner(resp.Body)
	event := ""
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "event: ") {
			event = strings.TrimPrefix(line, "event: ")
		}
		if event != "matched" || !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev struct {
			Type string
			Item struct {
				ID, Kind, Title string
				Year            int
				Poster          *struct{ ImageID string }
			}
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type != "matched" || ev.Item.Kind != "movie" || !strings.HasPrefix(ev.Item.Title, "Alien") || ev.Item.Year != 1979 ||
			ev.Item.ID == "" || ev.Item.Poster == nil || ev.Item.Poster.ImageID == "" {
			t.Errorf("matched event = %s", line)
		}
		return
	}
	t.Fatalf("no matched event: %v", sc.Err())
}

// TestServerStats: the dashboard's numbers, for admins only; library
// folders on one drive share a row (V2).
func TestServerStats(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	member := e.member(e.admin, "carol").Token
	e.expect(e.do("GET", "/server/stats", nil, bearer(member)), http.StatusForbidden, "forbidden")

	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, d := range []string{a, b} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	e.create("Movies", "movies", a)
	e.create("Shows", "shows", b)

	r := e.do("GET", "/server/stats", nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	var st struct {
		UptimeSeconds int64
		MaxTranscodes int
		Drives        []struct {
			Libraries  []string
			Available  bool
			TotalBytes int64
		}
	}
	r.json(&st)
	if st.MaxTranscodes < 1 || len(st.Drives) != 1 || !st.Drives[0].Available || st.Drives[0].TotalBytes <= 0 ||
		len(st.Drives[0].Libraries) != 2 {
		t.Errorf("stats = %s", r.Body.String())
	}
}
