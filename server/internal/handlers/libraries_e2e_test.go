package handlers_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gh-prim/plaid/server/internal/handlers"
	"github.com/gh-prim/plaid/server/internal/httpserver"
	"github.com/gh-prim/plaid/server/internal/library"
)

// These tests drive the acceptance criteria of F004 that go through the
// API: roles, folder validation, library lifecycle, items and scan events.

// stubProber answers like ffprobe on a 4K HDR10 file with English and
// French audio. When gate is set, each probe waits for a value on it.
type stubProber struct {
	gate chan struct{}
}

func (p *stubProber) Probe(ctx context.Context, _ string) (*library.ProbeResult, error) {
	if p.gate != nil {
		select {
		case <-p.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &library.ProbeResult{
		Container: "matroska,webm", DurationMs: 60000, Bitrate: 20_000_000, Raw: []byte(`{}`),
		Streams: []library.Stream{
			{Index: 0, Type: "video", Codec: "hevc", Width: 3840, Height: 2160, HDR: library.HDR10, Language: "und"},
			{Index: 1, Type: "audio", Codec: "eac3", Language: "eng", Channels: 6, Default: true},
			{Index: 2, Type: "audio", Codec: "aac", Language: "fre", Channels: 2},
		},
	}, nil
}

type libEnv struct {
	*env
	engine    *library.Engine
	admin     string
	endStream context.CancelFunc
}

type libOpts struct {
	prober      library.Prober
	browseRoots []string
}

func newLibEnv(t *testing.T, o libOpts) *libEnv {
	t.Helper()
	e := newEnv(t)
	logger := slog.New(slog.NewTextHandler(e.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	engine := library.NewEngine(e.db, library.EngineOptions{Prober: o.prober, FFprobeVersion: "7.1.1",
		BrowseRoots: o.browseRoots, Log: logger, Workers: 2})
	engine.Start(context.Background())
	t.Cleanup(engine.Close)
	streams, end := context.WithCancel(context.Background())
	t.Cleanup(end)
	h := handlers.New(handlers.Deps{DB: e.db, Accounts: e.acct, ServerName: "test-server", Library: engine, Streams: streams})
	e.h = httpserver.New(h, fstest.MapFS{}, logger, httpserver.Options{Sessions: e.acct})
	le := &libEnv{env: e, engine: engine, endStream: end}
	le.admin = e.setup().Token
	return le
}

type libraryJSON struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Type                string `json:"type"`
	ScanIntervalSeconds *int   `json:"scanIntervalSeconds"`
	Folders             []struct {
		ID, Path, Status string
		Watched          bool
	} `json:"folders"`
	Counts struct{ Movies, Shows, Episodes, Files, Unrecognized int } `json:"counts"`
	Scan   scanJSON                                                   `json:"scan"`
}

type scanJSON struct {
	State    string `json:"state"`
	Queued   bool   `json:"queued"`
	Counters struct {
		Found, New, Changed, Missing, Analyzed, Failed, Unrecognized int
	} `json:"counters"`
}

func (e *libEnv) create(name, typ string, folders ...string) libraryJSON {
	e.t.Helper()
	r := e.do("POST", "/libraries", map[string]any{"name": name, "type": typ, "folders": folders}, bearer(e.admin))
	e.expect(r, http.StatusCreated, "")
	var lib libraryJSON
	r.json(&lib)
	return lib
}

// waitIdle polls the scan until it's idle and returns the last progress.
func (e *libEnv) waitIdle(id string) scanJSON {
	e.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var s scanJSON
		r := e.do("GET", "/libraries/"+id+"/scan", nil, bearer(e.admin))
		e.expect(r, http.StatusOK, "")
		r.json(&s)
		if s.State == "idle" && !s.Queued {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.t.Fatal("scan never finished")
	return scanJSON{}
}

func touch(t *testing.T, root string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("not really a video"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLibraryRoles(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	member := e.member(e.admin, "carol").Token
	dir := t.TempDir()

	e.expect(e.do("POST", "/libraries", map[string]any{"name": "Movies", "type": "movies", "folders": []string{dir}},
		bearer(member)), http.StatusForbidden, "forbidden")
	e.expect(e.do("GET", "/fs/browse?path=/", nil, bearer(member)), http.StatusForbidden, "forbidden")
	e.expect(e.do("GET", "/server/capabilities", nil, bearer(member)), http.StatusForbidden, "forbidden")

	lib := e.create("Movies", "movies", dir)
	e.expect(e.do("GET", "/libraries", nil, bearer(member)), http.StatusOK, "")
	e.expect(e.do("GET", "/libraries/"+lib.ID, nil, bearer(member)), http.StatusOK, "")
	e.expect(e.do("GET", "/libraries/"+lib.ID+"/items", nil, bearer(member)), http.StatusOK, "")
	for _, req := range []struct{ method, path string }{
		{"PATCH", "/libraries/" + lib.ID}, {"DELETE", "/libraries/" + lib.ID},
		{"POST", "/libraries/" + lib.ID + "/scan"}, {"GET", "/libraries/" + lib.ID + "/scan"},
		{"GET", "/libraries/" + lib.ID + "/scan/events"}, {"GET", "/libraries/" + lib.ID + "/unrecognized"},
	} {
		e.expect(e.do(req.method, req.path, map[string]any{}, bearer(member)), http.StatusForbidden, "forbidden")
	}
	e.expect(e.do("GET", "/libraries", nil), http.StatusUnauthorized, "unauthorized")
}

func TestLibraryFolderValidation(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	base := t.TempDir()
	movies := filepath.Join(base, "movies")
	touch(t, movies, "kids/Up (2009).mkv")

	r := e.do("POST", "/libraries", map[string]any{"name": "M", "type": "movies",
		"folders": []string{movies, filepath.Join(base, "nope")}}, bearer(e.admin))
	e.expect(r, http.StatusUnprocessableEntity, "folder_not_found")
	if !strings.Contains(r.Body.String(), `"field":"folders/1"`) {
		t.Errorf("details don't point at the folder: %s", r.Body)
	}
	e.expect(e.do("POST", "/libraries", map[string]any{"name": "M", "type": "movies",
		"folders": []string{"relative/path"}}, bearer(e.admin)), http.StatusUnprocessableEntity, "folder_not_found")
	e.expect(e.do("POST", "/libraries", map[string]any{"name": "M", "type": "movies",
		"folders": []string{filepath.Join(movies, "kids", "Up (2009).mkv")}}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "folder_not_found")

	if os.Geteuid() != 0 { // root reads everything
		locked := filepath.Join(base, "locked")
		if err := os.Mkdir(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
		e.expect(e.do("POST", "/libraries", map[string]any{"name": "Locked", "type": "movies",
			"folders": []string{locked}}, bearer(e.admin)), http.StatusUnprocessableEntity, "folder_unreadable")
	}

	var libs []libraryJSON
	e.do("GET", "/libraries", nil, bearer(e.admin)).json(&libs)
	if len(libs) != 0 {
		t.Fatalf("libraries created on validation errors: %+v", libs)
	}

	for name, body := range map[string]map[string]any{
		"empty name":     {"name": "  ", "type": "movies", "folders": []string{movies}},
		"bad type":       {"name": "M", "type": "music", "folders": []string{movies}},
		"no folders":     {"name": "M", "type": "movies", "folders": []string{}},
		"short interval": {"name": "M", "type": "movies", "folders": []string{movies}, "scanIntervalSeconds": 60},
	} {
		r := e.do("POST", "/libraries", body, bearer(e.admin))
		if r.Code != http.StatusUnprocessableEntity || r.code() != "validation_failed" {
			t.Errorf("%s: %d %s", name, r.Code, r.Body)
		}
	}

	// Overlaps: inside, containing, equal, and within one request.
	e.create("Movies", "movies", movies)
	for _, folders := range [][]string{
		{filepath.Join(movies, "kids")}, {base}, {movies + "/"},
	} {
		e.expect(e.do("POST", "/libraries", map[string]any{"name": "Kids", "type": "movies", "folders": folders},
			bearer(e.admin)), http.StatusConflict, "folder_overlap")
	}
	other := filepath.Join(base, "other")
	touch(t, other, "a/x.mkv")
	e.expect(e.do("POST", "/libraries", map[string]any{"name": "O", "type": "movies",
		"folders": []string{other, filepath.Join(other, "a")}}, bearer(e.admin)), http.StatusConflict, "folder_overlap")

	// A symlink to a library folder is the same folder.
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(movies, link); err != nil {
		t.Fatal(err)
	}
	e.expect(e.do("POST", "/libraries", map[string]any{"name": "L", "type": "movies", "folders": []string{link}},
		bearer(e.admin)), http.StatusConflict, "folder_overlap")
}

func TestLibraryLifecycle(t *testing.T) {
	e := newLibEnv(t, libOpts{prober: &stubProber{}})
	root := t.TempDir()
	touch(t, root,
		"The Matrix (1999)/The Matrix (1999).mkv",
		"The Matrix (1999)/The Matrix (1999).fr.forced.srt",
		"Movie (2019) - 1080p.mkv", "Movie (2019) - 2160p.mkv",
		"zz.mkv")
	lib := e.create("Movies", "movies", root)
	if lib.Type != "movies" || len(lib.Folders) != 1 || lib.ScanIntervalSeconds != nil {
		t.Fatalf("created: %+v", lib)
	}
	s := e.waitIdle(lib.ID)
	if s.Counters.Found != 4 || s.Counters.New != 4 || s.Counters.Analyzed != 4 || s.Counters.Unrecognized != 0 {
		t.Fatalf("first scan counters: %+v", s.Counters)
	}

	var got libraryJSON
	e.do("GET", "/libraries/"+lib.ID, nil, bearer(e.admin)).json(&got)
	// The Matrix, Movie (two versions, one item) and "zz".
	if got.Counts.Movies != 3 || got.Counts.Files != 4 {
		t.Fatalf("counts: %+v", got.Counts)
	}

	// Rescan without changes: nothing new, nothing probed.
	e.expect(e.do("POST", "/libraries/"+lib.ID+"/scan", nil, bearer(e.admin)), http.StatusAccepted, "")
	s = e.waitIdle(lib.ID)
	if s.Counters.New != 0 || s.Counters.Changed != 0 || s.Counters.Analyzed != 0 {
		t.Fatalf("rescan counters: %+v", s.Counters)
	}

	// Type is fixed; the same type is a no-op.
	e.expect(e.do("PATCH", "/libraries/"+lib.ID, map[string]any{"type": "shows"}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "library_type_immutable")
	r := e.do("PATCH", "/libraries/"+lib.ID, map[string]any{"type": "movies", "name": "Films", "scanIntervalSeconds": 600},
		bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&got)
	if got.Name != "Films" || got.ScanIntervalSeconds == nil || *got.ScanIntervalSeconds != 600 {
		t.Fatalf("patched: %+v", got)
	}
	e.expect(e.do("PATCH", "/libraries/"+lib.ID, map[string]any{"scanIntervalSeconds": 100_000}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "validation_failed")
	r = e.do("PATCH", "/libraries/"+lib.ID, map[string]any{"scanIntervalSeconds": nil}, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&got)
	if got.ScanIntervalSeconds != nil {
		t.Fatalf("interval not reset: %+v", got)
	}

	// Adding a folder scans it.
	second := t.TempDir()
	touch(t, second, "Alien (1979) {tmdb-348}.mkv")
	e.expect(e.do("PATCH", "/libraries/"+lib.ID, map[string]any{"folders": []string{root, second}}, bearer(e.admin)),
		http.StatusOK, "")
	e.waitIdle(lib.ID)
	e.do("GET", "/libraries/"+lib.ID, nil, bearer(e.admin)).json(&got)
	if len(got.Folders) != 2 || got.Counts.Movies != 4 {
		t.Fatalf("after adding a folder: %+v", got)
	}

	// Deleting removes rows only.
	e.expect(e.do("DELETE", "/libraries/"+lib.ID, nil, bearer(e.admin)), http.StatusNoContent, "")
	e.expect(e.do("GET", "/libraries/"+lib.ID, nil, bearer(e.admin)), http.StatusNotFound, "not_found")
	if _, err := os.Stat(filepath.Join(root, "The Matrix (1999)", "The Matrix (1999).mkv")); err != nil {
		t.Fatalf("file on disk touched: %v", err)
	}
	// Its folders are free again.
	e.create("Movies again", "movies", root)
}

func TestFFprobeMissing(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	var caps struct {
		Ffprobe struct {
			Found   bool
			Version *string
		}
		WatchLimitReached []string
	}
	r := e.do("GET", "/server/capabilities", nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&caps)
	if caps.Ffprobe.Found || caps.WatchLimitReached == nil {
		t.Fatalf("capabilities: %s", r.Body)
	}

	root := t.TempDir()
	touch(t, root, "Up (2009).mkv")
	lib := e.create("Movies", "movies", root)
	e.waitIdle(lib.ID)
	var page itemPage
	e.do("GET", "/libraries/"+lib.ID+"/items", nil, bearer(e.admin)).json(&page)
	if len(page.Items) != 1 {
		t.Fatalf("items: %+v", page)
	}
	var item itemDetail
	e.do("GET", "/items/"+page.Items[0].ID, nil, bearer(e.admin)).json(&item)
	if len(item.Files) != 1 || item.Files[0].ProbeStatus != "skipped" {
		t.Fatalf("item: %+v", item)
	}
}

type itemJSON struct {
	ID            string
	Kind          string
	Title         *string
	Year          *int
	SeasonNumber  *int
	EpisodeNumber *int
	AirDate       *string
	FileCount     int
	ChildCount    int
	Hints         *struct{ Tmdb, Imdb *string }
	Technical     struct {
		Resolution     *struct{ Width, Height int }
		HdrFormats     []string
		AudioLanguages []string
	}
}

type itemPage struct {
	Items      []itemJSON
	NextCursor *string
}

type itemDetail struct {
	itemJSON
	Files []struct {
		Path        string
		Edition     *string
		ProbeStatus string
		HdrFormat   *string
		Streams     []struct {
			Index    int
			Type     string
			Language string
			Forced   bool
			External bool
		}
	}
}

func TestItemsAPI(t *testing.T) {
	e := newLibEnv(t, libOpts{prober: &stubProber{}})
	root := t.TempDir()
	touch(t, root,
		"Breaking Bad/Season 01/Breaking.Bad.S01E02.720p.mkv",
		"Breaking Bad/Season 01/Breaking.Bad.S01E01.720p.mkv",
		"Breaking Bad/Season 01/Breaking.Bad.S01E01.720p.fr.forced.srt",
		"Breaking Bad/Season 02/Breaking Bad S02E01.mkv",
		"Lost/Staffel 1/Lost S01E01-E02.mkv",
		"The Daily Show/2024-10-27.mkv",
		"Random/notes.mkv")
	lib := e.create("Shows", "shows", root)
	if s := e.waitIdle(lib.ID); s.Counters.Unrecognized != 1 {
		t.Fatalf("counters: %+v", s.Counters)
	}

	var shows itemPage
	e.do("GET", "/libraries/"+lib.ID+"/items", nil, bearer(e.admin)).json(&shows)
	var titles []string
	for _, s := range shows.Items {
		titles = append(titles, *s.Title)
	}
	if strings.Join(titles, "|") != "Breaking Bad|Lost|The Daily Show" || shows.NextCursor != nil {
		t.Fatalf("shows: %v", titles)
	}
	bb := shows.Items[0]
	if bb.Kind != "show" || bb.ChildCount != 2 || bb.FileCount != 3 ||
		bb.Technical.Resolution == nil || bb.Technical.Resolution.Height != 2160 ||
		strings.Join(bb.Technical.AudioLanguages, ",") != "eng,fre" || strings.Join(bb.Technical.HdrFormats, ",") != "hdr10" {
		t.Fatalf("Breaking Bad summary: %+v", bb)
	}

	var seasons itemPage
	e.do("GET", "/libraries/"+lib.ID+"/items?parentId="+bb.ID, nil, bearer(e.admin)).json(&seasons)
	if len(seasons.Items) != 2 || *seasons.Items[0].SeasonNumber != 1 || *seasons.Items[1].SeasonNumber != 2 {
		t.Fatalf("seasons: %+v", seasons)
	}
	var episodes itemPage
	e.do("GET", "/libraries/"+lib.ID+"/items?parentId="+seasons.Items[0].ID, nil, bearer(e.admin)).json(&episodes)
	if len(episodes.Items) != 2 || *episodes.Items[0].EpisodeNumber != 1 || *episodes.Items[1].EpisodeNumber != 2 {
		t.Fatalf("episodes: %+v", episodes)
	}

	var ep itemDetail
	e.do("GET", "/items/"+episodes.Items[0].ID, nil, bearer(e.admin)).json(&ep)
	if len(ep.Files) != 1 || *ep.Files[0].HdrFormat != "hdr10" {
		t.Fatalf("episode: %+v", ep)
	}
	var langs []string
	forcedFrench := false
	for _, s := range ep.Files[0].Streams {
		if s.Type == "audio" {
			langs = append(langs, s.Language)
		}
		forcedFrench = forcedFrench || s.Type == "subtitle" && s.External && s.Forced && s.Language == "fre"
	}
	if strings.Join(langs, ",") != "eng,fre" || !forcedFrench {
		t.Fatalf("streams: %+v", ep.Files[0].Streams)
	}

	// The Daily Show: a date-based episode.
	var ds itemPage
	e.do("GET", "/libraries/"+lib.ID+"/items?kind=episode", nil, bearer(e.admin)).json(&ds)
	foundDate := false
	for _, it := range ds.Items {
		foundDate = foundDate || it.AirDate != nil && *it.AirDate == "2024-10-27"
	}
	if !foundDate {
		t.Fatalf("no date-based episode in %+v", ds.Items)
	}

	var unrec struct {
		Files []struct{ Path string }
	}
	e.do("GET", "/libraries/"+lib.ID+"/unrecognized", nil, bearer(e.admin)).json(&unrec)
	if len(unrec.Files) != 1 || !strings.HasSuffix(unrec.Files[0].Path, "Random/notes.mkv") {
		t.Fatalf("unrecognized: %+v", unrec)
	}

	e.expect(e.do("GET", "/items/nope", nil, bearer(e.admin)), http.StatusNotFound, "not_found")
	e.expect(e.do("GET", "/libraries/"+lib.ID+"/items?kind=movie&limit=500", nil, bearer(e.admin)),
		http.StatusUnprocessableEntity, "validation_failed")
	e.expect(e.do("GET", "/libraries/"+lib.ID+"/items?cursor=garbage", nil, bearer(e.admin)),
		http.StatusBadRequest, "bad_request")
	e.expect(e.do("GET", "/libraries/nope/items", nil, bearer(e.admin)), http.StatusNotFound, "not_found")
}

func TestItemsPagination(t *testing.T) {
	e := newLibEnv(t, libOpts{prober: &stubProber{}})
	root := t.TempDir()
	const n = 450
	for i := range n {
		touch(t, root, fmt.Sprintf("Movie %03d (%d).mkv", i, 1950+i%70))
	}
	lib := e.create("Movies", "movies", root)
	e.waitIdle(lib.ID)

	seen := map[string]bool{}
	var titles []string
	cursor := ""
	pages := 0
	for {
		path := "/libraries/" + lib.ID + "/items?limit=100"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		var page itemPage
		r := e.do("GET", path, nil, bearer(e.admin))
		e.expect(r, http.StatusOK, "")
		r.json(&page)
		pages++
		for _, it := range page.Items {
			if seen[it.ID] {
				t.Fatalf("item %s on two pages", it.ID)
			}
			seen[it.ID] = true
			titles = append(titles, *it.Title)
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if len(seen) != n || pages != 5 {
		t.Fatalf("%d items over %d pages, want %d over 5", len(seen), pages, n)
	}
	for i := 1; i < len(titles); i++ {
		if strings.ToLower(titles[i-1]) > strings.ToLower(titles[i]) {
			t.Fatalf("out of order: %q before %q", titles[i-1], titles[i])
		}
	}
}

func TestBrowseFolders(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"Movies", "shows", ".hidden"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	touch(t, root, "file.mkv")
	e := newLibEnv(t, libOpts{})

	type browse struct {
		Path    string
		Parent  *string
		Entries []struct {
			Name, Path string
			Readable   bool
		}
	}
	var b browse
	r := e.do("GET", "/fs/browse?path="+root, nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&b)
	if len(b.Entries) != 2 || b.Entries[0].Name != "Movies" || b.Entries[1].Name != "shows" || !b.Entries[0].Readable ||
		b.Parent == nil || *b.Parent != filepath.Dir(root) {
		t.Fatalf("browse: %s", r.Body)
	}
	e.do("GET", "/fs/browse", nil, bearer(e.admin)).json(&b)
	if b.Path != "/" || b.Parent != nil {
		t.Fatalf("default browse: %+v", b)
	}
	e.expect(e.do("GET", "/fs/browse?path=relative", nil, bearer(e.admin)), http.StatusBadRequest, "bad_request")
	e.expect(e.do("GET", "/fs/browse?path="+root+"/Movies/../..", nil, bearer(e.admin)), http.StatusBadRequest, "bad_request")
	e.expect(e.do("GET", "/fs/browse?path="+root+"/nope", nil, bearer(e.admin)), http.StatusNotFound, "not_found")

	// Restricted to roots.
	rooted := newLibEnv(t, libOpts{browseRoots: []string{filepath.Join(root, "Movies")}})
	rooted.do("GET", "/fs/browse", nil, bearer(rooted.admin)).json(&b)
	if b.Path != "" || len(b.Entries) != 1 || b.Entries[0].Path != filepath.Join(root, "Movies") {
		t.Fatalf("roots listing: %+v", b)
	}
	rooted.do("GET", "/fs/browse?path="+filepath.Join(root, "Movies"), nil, bearer(rooted.admin)).json(&b)
	if b.Parent != nil {
		t.Fatalf("parent above a root: %+v", b)
	}
	rooted.expect(rooted.do("GET", "/fs/browse?path="+root, nil, bearer(rooted.admin)), http.StatusNotFound, "not_found")
}

// sseClient reads events from a live server.
type sseEvent struct{ typ, data string }

func readEvents(t *testing.T, body io.Reader, out chan<- sseEvent) {
	t.Helper()
	buf := make([]byte, 0, 4096)
	chunk := make([]byte, 1024)
	var typ string
	for {
		n, err := body.Read(chunk)
		buf = append(buf, chunk[:n]...)
		for {
			i := strings.Index(string(buf), "\n")
			if i < 0 {
				break
			}
			line := string(buf[:i])
			buf = buf[i+1:]
			switch {
			case strings.HasPrefix(line, "event: "):
				typ = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				out <- sseEvent{typ, strings.TrimPrefix(line, "data: ")}
			case strings.HasPrefix(line, ": ping"):
				out <- sseEvent{"ping", ""}
			}
		}
		if err != nil {
			close(out)
			return
		}
	}
}

func TestScanEvents(t *testing.T) {
	defer handlers.SetSSETiming(30*time.Millisecond, 50*time.Millisecond)()
	gate := make(chan struct{})
	e := newLibEnv(t, libOpts{prober: &stubProber{gate: gate}})
	srv := newLiveServer(t, e.h)
	root := t.TempDir()
	touch(t, root, "Up (2009).mkv", "Alien (1979).mkv")
	lib := e.create("Movies", "movies", root)

	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/libraries/"+lib.ID+"/scan/events", nil)
	req.Header.Set("Authorization", "Bearer "+e.admin)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" || res.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("headers: %v", res.Header)
	}
	events := make(chan sseEvent, 64)
	go readEvents(t, res.Body, events)

	// While both probes wait, progress keeps coming, and pings.
	progress, pings := 0, 0
	timeout := time.After(5 * time.Second)
	for progress < 3 || pings < 1 {
		select {
		case ev := <-events:
			switch ev.typ {
			case "progress":
				progress++
				if !strings.Contains(ev.data, `"type":"progress"`) {
					t.Fatalf("progress data: %s", ev.data)
				}
			case "ping":
				pings++
			case "completed":
				t.Fatal("completed before the probes ran")
			}
		case <-timeout:
			t.Fatalf("%d progress events and %d pings in 5 s", progress, pings)
		}
	}
	close(gate)
	for ev := range events {
		if ev.typ == "completed" {
			if !strings.Contains(ev.data, `"analyzed":2`) || !strings.Contains(ev.data, `"state":"idle"`) {
				t.Fatalf("completed data: %s", ev.data)
			}
			return
		}
	}
	t.Fatal("stream ended without completed")
}

func TestScanEventsEndOnShutdown(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	root := t.TempDir()
	lib := e.create("Movies", "movies", root)
	srv := newLiveServer(t, e.h)
	srv.Config.RegisterOnShutdown(e.endStream)

	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/libraries/"+lib.ID+"/scan/events", nil)
	req.Header.Set("Authorization", "Bearer "+e.admin)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	events := make(chan sseEvent, 16)
	go readEvents(t, res.Body, events)
	<-events // the initial state

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	if err := srv.Config.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("shutdown took %v with an open stream", d)
	}
}

// liveServer is an httptest.Server on 127.0.0.1, which the auth layer
// treats as the local network.
type liveServer struct {
	URL    string
	Config *http.Server
}

func newLiveServer(t *testing.T, h http.Handler) *liveServer {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
	var wg sync.WaitGroup
	wg.Go(func() { _ = srv.Serve(ln) })
	t.Cleanup(func() { _ = srv.Close(); wg.Wait() })
	return &liveServer{URL: "http://" + ln.Addr().String(), Config: srv}
}
