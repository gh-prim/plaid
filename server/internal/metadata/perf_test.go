//go:build perf

package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/db"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
	"github.com/gh-prim/plaid/server/internal/metadata/tmdb"
	"github.com/gh-prim/plaid/server/internal/metadata/tmdbtest"
)

// Reference library from the spec's NFR: 1,000 movies, 100 shows of
// 9 seasons × 10 episodes. Run with:
//
//	go -C server test -tags perf -run Perf -v -timeout 30m ./internal/metadata/
const (
	perfSeasons  = 9
	perfEpisodes = 10
	stubLatency  = 50 * time.Millisecond
)

// PERF_SMALL=1 runs a 60-movie, 5-show version with logs, for debugging.
var (
	perfMovies = pick(1000, 60)
	perfShows  = pick(100, 5)
)

func pick(full, small int) int {
	if os.Getenv("PERF_SMALL") != "" {
		return small
	}
	return full
}

// perfLog keeps warnings: a silent failure would look like slowness.
func perfLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

// dynamicTMDB answers every title the perf library has, with 50 ms of
// latency, and records requests per second.
type dynamicTMDB struct {
	mu      sync.Mutex
	perSec  map[int64]int
	total   atomic.Int64
	started time.Time
}

func (d *dynamicTMDB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	time.Sleep(stubLatency)
	d.total.Add(1)
	d.mu.Lock()
	d.perSec[int64(time.Since(d.started)/time.Second)]++
	d.mu.Unlock()
	q := r.URL.Query()
	path := r.URL.Path
	write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	images := map[string]any{
		"posters":   []any{map[string]any{"file_path": "/p" + strings.ReplaceAll(path, "/", "") + ".jpg", "iso_639_1": "en", "width": 780, "height": 1170}},
		"backdrops": []any{map[string]any{"file_path": "/b" + strings.ReplaceAll(path, "/", "") + ".jpg", "width": 1280, "height": 720}},
	}
	switch {
	case strings.HasPrefix(path, "/search/"):
		title := q.Get("query")
		id, _ := strconv.Atoi(strings.TrimPrefix(strings.TrimPrefix(title, "Movie "), "Show "))
		if strings.HasPrefix(title, "Show ") {
			id += 100000
		}
		write(map[string]any{"results": []any{map[string]any{"id": id, "title": title, "name": title,
			"release_date": "2000-01-01", "first_air_date": "2000-01-01", "popularity": 10}}})
	case strings.HasPrefix(path, "/movie/"):
		id := strings.TrimPrefix(path, "/movie/")
		write(map[string]any{"id": atoi(id), "title": "Movie " + id, "release_date": "2000-01-01", "overview": "x",
			"images": images, "release_dates": map[string]any{"results": []any{map[string]any{"iso_3166_1": "US",
				"release_dates": []any{map[string]any{"type": 3, "certification": "PG"}}}}}})
	case strings.Contains(path, "/season/"):
		parts := strings.Split(path, "/")
		var eps []any
		for e := 1; e <= perfEpisodes; e++ {
			eps = append(eps, map[string]any{"episode_number": e, "season_number": atoi(parts[4]), "name": "E" + strconv.Itoa(e),
				"overview": "x", "air_date": "2000-01-01"})
		}
		write(map[string]any{"season_number": atoi(parts[4]), "name": "Season " + parts[4], "episodes": eps})
	case strings.HasPrefix(path, "/tv/"):
		id := atoi(strings.TrimPrefix(path, "/tv/"))
		var seasons []any
		for s := 1; s <= perfSeasons; s++ {
			seasons = append(seasons, map[string]any{"season_number": s, "name": "Season " + strconv.Itoa(s), "episode_count": perfEpisodes})
		}
		write(map[string]any{"id": id, "name": "Show " + strconv.Itoa(id-100000), "first_air_date": "2000-01-01", "status": "Ended",
			"seasons": seasons, "images": images})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func TestPerfReferenceLibrary(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	d, err := db.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	stub := &dynamicTMDB{perSec: map[int64]int{}}
	api := httptest.NewTLSServer(stub)
	defer api.Close()
	images := tmdbtest.New(t)
	h := httpx.New(httpx.Options{Name: "tmdb", Hosts: []string{"127.0.0.1"}, Transport: api.Client().Transport,
		RPS: tmdbRPS, Burst: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	svc := New(Options{DB: d, Provider: tmdb.New(h, api.URL, "k"), Log: perfLog(),
		DataDir: dir, ImageHTTP: images.HTTP(), ImageHosts: images.ImageHosts(), QueueIdle: 50 * time.Millisecond})

	root := t.TempDir()
	write := func(rel string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("x"), 0o644)
	}
	// Titles start at 1: the stub gives "Movie N" TMDB id N, and 0 isn't one.
	for i := 1; i <= perfMovies; i++ {
		write(fmt.Sprintf("movies/Movie %d (2000)/Movie %d (2000).mkv", i, i))
	}
	for s := 1; s <= perfShows; s++ {
		for se := 1; se <= perfSeasons; se++ {
			for e := 1; e <= perfEpisodes; e++ {
				write(fmt.Sprintf("shows/Show %d/Season %d/Show %d S%02dE%02d.mkv", s, se, s, se, e))
			}
		}
	}
	st := library.NewStore(d)
	movies, _ := st.CreateLibrary(ctx, "Movies", library.TypeMovies, []string{filepath.Join(root, "movies")})
	shows, _ := st.CreateLibrary(ctx, "Shows", library.TypeShows, []string{filepath.Join(root, "shows")})
	sc := &library.Scanner{Store: st, Hooks: library.Hooks{ItemsCreated: svc.ItemsCreated, Sidecars: svc.SidecarsSeen}}

	var peak atomic.Uint64
	stopMem := make(chan struct{})
	go func() {
		var m runtime.MemStats
		for {
			select {
			case <-stopMem:
				return
			case <-time.After(100 * time.Millisecond):
			}
			runtime.ReadMemStats(&m)
			if m.HeapInuse > peak.Load() {
				peak.Store(m.HeapInuse)
			}
		}
	}()
	stub.started = time.Now()
	start := time.Now()
	svc.Start(ctx)
	defer svc.Close()
	for _, lib := range []library.Library{movies, shows} {
		if _, err := sc.ScanLibrary(ctx, lib.ID, library.ScanOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	count := func(q string) int {
		var n int
		_ = d.QueryRow(q).Scan(&n)
		return n
	}
	var postersAt time.Duration
	posters := 0
	for time.Since(start) < 45*time.Minute {
		time.Sleep(time.Second)
		posters = count(`SELECT COUNT(*) FROM artwork a JOIN items i ON i.id = a.owner_id WHERE i.kind = 'movie' AND a.kind = 'poster' AND a.selected = 1 AND a.sha256 IS NOT NULL`)
		if postersAt == 0 && posters == perfMovies {
			postersAt = time.Since(start)
		}
		pending := count(`SELECT COUNT(*) FROM item_metadata WHERE state = 'pending'`)
		if int(time.Since(start).Seconds())%30 == 0 {
			t.Logf("%v: pending %d, matched %d, movie posters %d", time.Since(start).Round(time.Second), pending,
				count(`SELECT COUNT(*) FROM item_metadata WHERE state = 'matched'`), posters)
		}
		if pending == 0 {
			break
		}
	}
	total := time.Since(start)
	close(stopMem)

	maxRate := 0
	stub.mu.Lock()
	for _, n := range stub.perSec {
		maxRate = max(maxRate, n)
	}
	stub.mu.Unlock()
	t.Logf("%d of %d movie posters (all of them after %v); everything (%d items) after %v", posters, perfMovies,
		postersAt.Round(time.Second), count(`SELECT COUNT(*) FROM items`), total.Round(time.Second))
	t.Logf("%d TMDB requests, at most %d in one second; peak heap in use %d MB", stub.total.Load(), maxRate, peak.Load()>>20)
	if posters < perfMovies*99/100 || postersAt > 10*time.Minute {
		t.Errorf("%d posters, all of them after %v; budget: every poster within 10 min", posters, postersAt)
	}
	if total > 45*time.Minute {
		t.Errorf("enrichment took %v, budget 45 min", total)
	}
	if maxRate > tmdbRPS+1 { // +1: a second boundary can see the burst and the next token
		t.Errorf("%d requests in one second, limit %d", maxRate, tmdbRPS)
	}
	if peak.Load() > 400<<20 {
		t.Errorf("peak heap %d MB", peak.Load()>>20)
	}
}
