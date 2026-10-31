package segments

import (
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/db"
	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
)

const now = "2026-10-31T00:00:00Z"

// stub answers TheIntroDB requests from a map of query → body; anything
// else is a 404. It counts the requests.
type stub struct {
	bodies map[string]string
	asked  []string
}

func (s *stub) RoundTrip(r *http.Request) (*http.Response, error) {
	s.asked = append(s.asked, r.URL.RawQuery)
	body, ok := s.bodies[r.URL.RawQuery]
	code := http.StatusOK
	if !ok {
		code, body = http.StatusNotFound, `{"error":"no submissions for this media","code":"no_submissions"}`
	}
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: r}, nil
}

func setup(t *testing.T, bodies map[string]string) (*Service, *stub, *time.Time) {
	t.Helper()
	d, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	for _, q := range []string{
		`INSERT INTO libraries (id, name, type, created_at, updated_at) VALUES ('lm', 'Movies', 'movies', '` + now + `', '` + now + `'), ('ls', 'Shows', 'shows', '` + now + `', '` + now + `')`,
		`INSERT INTO library_folders (id, library_id, path, created_at) VALUES ('fm', 'lm', '/m', '` + now + `'), ('fs', 'ls', '/s', '` + now + `')`,
		`INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES ('m', 'lm', 'movie', '1917', '` + now + `', '` + now + `'), ('dark', 'ls', 'show', 'Dark', '` + now + `', '` + now + `')`,
		`INSERT INTO items (id, library_id, kind, parent_id, season_number, created_at, updated_at) VALUES ('s1', 'ls', 'season', 'dark', 1, '` + now + `', '` + now + `')`,
		`INSERT INTO items (id, library_id, kind, parent_id, episode_number, created_at, updated_at) VALUES ('e2', 'ls', 'episode', 's1', 2, '` + now + `', '` + now + `'), ('e3', 'ls', 'episode', 's1', 3, '` + now + `', '` + now + `')`,
		`INSERT INTO item_metadata (item_id, state, tmdb_id, updated_at) VALUES ('m', 'matched', 530915, '` + now + `'), ('dark', 'matched', 70523, '` + now + `'), ('e2', 'matched', 70523, '` + now + `'), ('e3', 'matched', 70523, '` + now + `')`,
		`INSERT INTO media_files (id, library_id, folder_id, path, size, mtime, parse_status, duration_ms, created_at, updated_at) VALUES ('f', 'lm', 'fm', '/m/1917.mkv', 1, '` + now + `', 'ok', 7180000, '` + now + `', '` + now + `')`,
		`INSERT INTO item_files (item_id, file_id) VALUES ('m', 'f')`,
	} {
		if _, err := d.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	st := &stub{bodies: bodies}
	clock := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	h := httpx.New(httpx.Options{Name: "theintrodb", Hosts: Hosts, Transport: st})
	s := New(Options{DB: d, HTTP: h, Now: func() time.Time { return clock }})
	return s, st, &clock
}

func (s *Service) exec(t *testing.T, q string) {
	t.Helper()
	if _, err := s.db.Exec(q); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func TestMovieAndEpisode(t *testing.T) {
	s, st, _ := setup(t, map[string]string{
		// Measured on a cut 10 s shorter: the credits move with the end.
		"details=true&duration_ms=7180000&tmdb_id=530915": `{"tvdb_id":1,"type":"movie","group_duration_ms":7170000,"credits":[{"start_ms":6900000,"end_ms":null,"confidence":0.8,"submission_count":3}],"intro":[{"start_ms":null,"end_ms":23000}]}`,
		// One unsure submission of credits: left out.
		"details=true&episode=2&season=1&tmdb_id=70523": `{"tvdb_id":2,"type":"tv","season":1,"episode":2,"recap":[{"start_ms":0,"end_ms":60000}],"intro":[{"start_ms":60000,"end_ms":150000}],"preview":[{"start_ms":10,"end_ms":5}],"credits":[{"start_ms":2500000,"end_ms":null,"confidence":0.1,"submission_count":1}]}`,
	})
	ctx := context.Background()

	segs, err := s.For(ctx, "m")
	if err != nil {
		t.Fatal(err)
	}
	// In time order; an open start or end is nil.
	if len(segs) != 2 || segs[0].Kind != Intro || segs[0].StartMs != nil || *segs[0].EndMs != 23000 ||
		segs[1].Kind != Credits || *segs[1].StartMs != 6910000 || segs[1].EndMs != nil {
		t.Fatalf("movie: %+v", segs)
	}

	segs, err = s.For(ctx, "e2")
	if err != nil {
		t.Fatal(err)
	}
	// Show's TMDB id with season and episode; a backwards segment is dropped.
	if len(segs) != 2 || segs[0].Kind != Recap || segs[1].Kind != Intro {
		t.Fatalf("episode: %+v", segs)
	}

	// Kept: asked once each.
	_, _ = s.For(ctx, "m")
	_, _ = s.For(ctx, "e2")
	if len(st.asked) != 2 {
		t.Errorf("asked %v", st.asked)
	}
}

func TestNothingKnown(t *testing.T) {
	s, st, clock := setup(t, nil)
	ctx := context.Background()
	for _, id := range []string{"e3", "e3"} {
		segs, err := s.For(ctx, id)
		if err != nil || len(segs) != 0 {
			t.Fatalf("%v %+v", err, segs)
		}
	}
	if len(st.asked) != 1 {
		t.Fatalf("a 404 is kept: asked %v", st.asked)
	}
	// Asked again after a few days.
	*clock = clock.Add(4 * 24 * time.Hour)
	_, _ = s.For(ctx, "e3")
	if len(st.asked) != 2 {
		t.Errorf("not asked again: %v", st.asked)
	}
	// Shows and seasons have no segments and cost no request.
	for _, id := range []string{"dark", "s1", "nope"} {
		if segs, err := s.For(ctx, id); err != nil || len(segs) != 0 {
			t.Errorf("%s: %v %+v", id, err, segs)
		}
	}
	if len(st.asked) != 2 {
		t.Errorf("asked for a show: %v", st.asked)
	}
}

// fakeFFmpeg writes a script that prints blackframe lines for the times
// given, relative to the -ss it is called with, like FFmpeg does.
func fakeFFmpeg(t *testing.T, black string) string {
	t.Helper()
	path := t.TempDir() + "/ffmpeg"
	script := "#!/bin/sh\nfor t in " + black + "; do echo \"[Parsed_blackframe_2 @ 0x1] frame:1 pblack:95 pts:1 t:$t type:I\" >&2; done\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDetectedCreditsWin(t *testing.T) {
	s, _, _ := setup(t, map[string]string{
		"details=true&duration_ms=3600000&episode=2&season=1&tmdb_id=70523": `{"tvdb_id":2,"type":"tv","season":1,"episode":2,"credits":[{"start_ms":3000000,"end_ms":null,"confidence":0.9,"submission_count":5}]}`,
	})
	s.exec(t, `INSERT INTO media_files (id, library_id, folder_id, path, size, mtime, parse_status, duration_ms, created_at, updated_at)
		VALUES ('fe', 'ls', 'fs', '/s/e2.mkv', 1, '`+now+`', 'ok', 3600000, '`+now+`', '`+now+`')`)
	s.exec(t, `INSERT INTO item_files (item_id, file_id) VALUES ('e2', 'fe')`)
	// The window starts 600 s before the end: black from 3400 s to the end.
	var times []string
	for x := 400.0; x <= 598; x += 0.5 {
		times = append(times, strconv.FormatFloat(x, 'f', 1, 64))
	}
	s.ffmpeg = fakeFFmpeg(t, strings.Join(times, " "))
	ctx := context.Background()

	segs, _ := s.For(ctx, "e2")
	if len(segs) != 1 || *segs[0].StartMs != 3000000 {
		t.Fatalf("before the detection: %+v", segs)
	}
	s.background.Wait()
	segs, _ = s.For(ctx, "e2")
	if len(segs) != 1 || segs[0].Kind != Credits || *segs[0].StartMs != 3400000 {
		t.Fatalf("after: %+v", segs)
	}
}
