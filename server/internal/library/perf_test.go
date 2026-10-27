//go:build perf

package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPerf10kFiles checks the NFR performance targets on 10,000 sparse
// files laid out like a real library (4,000 movies, 600 shows of 10
// episodes), with a fake prober so ffprobe's cost is left out:
//   - first walk and parse < 2 min
//   - unchanged rescan < 60 s, with zero probes
//
// Run it with `go test -tags perf -run Perf -v ./internal/library/`, on the
// throttled profile (see CLAUDE.md) to approximate a Raspberry Pi.
func TestPerf10kFiles(t *testing.T) {
	root := t.TempDir()
	start := time.Now()
	for i := range 4000 {
		title := fmt.Sprintf("Movie Number %04d (%d)", i, 1950+i%75)
		sparse(t, filepath.Join(root, "movies", title, title+".mkv"), 2<<30)
		if i%4 == 0 {
			writeSmall(t, filepath.Join(root, "movies", title, title+".en.srt"))
		}
	}
	for s := range 600 {
		show := fmt.Sprintf("Show Number %03d", s)
		for e := range 10 {
			season := 1 + e/5
			name := fmt.Sprintf("%s S%02dE%02d 1080p WEB-DL.mkv", show, season, 1+e%5)
			sparse(t, filepath.Join(root, "shows", show, fmt.Sprintf("Season %02d", season), name), 1<<30)
		}
	}
	t.Logf("generated 10,000 sparse files in %v", time.Since(start).Round(time.Millisecond))

	d := openTestDB(t)
	st := NewStore(d)
	ctx := context.Background()
	movies, err := st.CreateLibrary(ctx, "Movies", TypeMovies, []string{filepath.Join(root, "movies")})
	if err != nil {
		t.Fatal(err)
	}
	shows, err := st.CreateLibrary(ctx, "Shows", TypeShows, []string{filepath.Join(root, "shows")})
	if err != nil {
		t.Fatal(err)
	}
	prober := &fakeProber{}
	sc := &Scanner{Store: st, Prober: prober, Log: discardLog()}

	peak := trackHeap(t)

	// First scan: the walk and parse phase ends when analysis starts.
	var walkDone time.Duration
	var files int
	start = time.Now()
	for _, lib := range []Library{movies, shows} {
		libStart := time.Now()
		var analyzing time.Duration
		rep, err := sc.ScanLibrary(ctx, lib.ID, ScanOptions{Progress: func(p ScanProgress) {
			if p.Phase == PhaseAnalyzing && analyzing == 0 {
				analyzing = time.Since(libStart)
			}
		}})
		if err != nil {
			t.Fatal(err)
		}
		walkDone += analyzing
		files += rep.Found
		if rep.Unrecognized != 0 {
			t.Errorf("%s: %d unrecognized", lib.Name, rep.Unrecognized)
		}
	}
	first := time.Since(start)
	if files != 10_000 || prober.count() != 10_000 {
		t.Fatalf("first scan found %d files", files)
	}

	// Unchanged rescan.
	start = time.Now()
	for _, lib := range []Library{movies, shows} {
		rep, err := sc.ScanLibrary(ctx, lib.ID, ScanOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if rep.New != 0 || rep.Changed != 0 {
			t.Errorf("%s rescan: %+v", lib.Name, rep)
		}
	}
	rescan := time.Since(start)
	if n := prober.count(); n != 0 {
		t.Errorf("unchanged rescan probed %d files", n)
	}

	t.Logf("CPUs: %d (GOMAXPROCS %d)", runtime.NumCPU(), runtime.GOMAXPROCS(0))
	t.Logf("first scan, walk and parse: %v", walkDone.Round(time.Millisecond))
	t.Logf("first scan, total with fake analysis (fingerprints): %v", first.Round(time.Millisecond))
	t.Logf("unchanged rescan: %v", rescan.Round(time.Millisecond))
	t.Logf("peak heap in use: %d MB", peak()/(1<<20))
	if walkDone > 2*time.Minute {
		t.Errorf("walk and parse took %v, target < 2 min", walkDone)
	}
	if rescan > time.Minute {
		t.Errorf("unchanged rescan took %v, target < 60 s", rescan)
	}
}

func sparse(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeSmall(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("1\n00:00:01,000 --> 00:00:02,000\nHi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// trackHeap samples HeapInuse until the test ends and returns a function
// reporting the peak.
func trackHeap(t *testing.T) func() uint64 {
	var peak atomic.Uint64
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		var ms runtime.MemStats
		for {
			runtime.ReadMemStats(&ms)
			if ms.HeapInuse > peak.Load() {
				peak.Store(ms.HeapInuse)
			}
			select {
			case <-done:
				return
			case <-tick.C:
			}
		}
	})
	t.Cleanup(func() { close(done); wg.Wait() })
	return peak.Load
}
