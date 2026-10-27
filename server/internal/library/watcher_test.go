package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

type fakeNow struct {
	mu sync.Mutex
	t  time.Time
}

func (f *fakeNow) now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeNow) advance(d time.Duration) {
	f.mu.Lock()
	f.t = f.t.Add(d)
	f.mu.Unlock()
}

func (f *fakeNow) since(t0 time.Time) time.Duration { return f.now().Sub(t0) }

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestWatcherWaitsForFileToStopGrowing(t *testing.T) {
	root := t.TempDir()
	clock := &fakeNow{t: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
	t0 := clock.now()
	tick := make(chan time.Time)
	settled := make(chan time.Duration, 4)
	fw, err := startWatcher(root, func() { settled <- clock.since(t0) }, nil, nil, discardLog(),
		watcherHooks{now: clock.now, tick: tick})
	if err != nil {
		t.Fatal(err)
	}
	defer fw.close()

	// A copy in a new sub-folder, one chunk every 2 s for 14 s.
	dir := filepath.Join(root, "Dune Part Two (2024)")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "folder event", func() bool { return fw.pendingCount() > 0 })
	waitFor(t, "sub-folder watch", func() bool { fw.mu.Lock(); defer fw.mu.Unlock(); return fw.watches == 2 })
	f, err := os.Create(filepath.Join(dir, "Dune Part Two (2024).mkv"))
	if err != nil {
		t.Fatal(err)
	}
	chunk := make([]byte, 64<<10)
	for range 7 {
		if _, err := f.Write(chunk); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond) // let the event land before time moves
		clock.advance(pollInterval)
		tick <- clock.now()
	}
	_ = f.Close()
	lastWrite := clock.since(t0)
	time.Sleep(20 * time.Millisecond)

	var at time.Duration
	for at == 0 {
		if clock.since(t0) > lastWrite+30*time.Second {
			t.Fatal("never settled")
		}
		clock.advance(pollInterval)
		tick <- clock.now()
		select {
		case at = <-settled:
		case <-time.After(150 * time.Millisecond):
		}
	}
	if at < lastWrite+stableFor-pollInterval {
		t.Fatalf("settled at %v, last write at %v: processed while still growing", at, lastWrite)
	}
	if fw.pendingCount() != 0 {
		t.Error("pending paths kept after settling")
	}
}

func TestWatcherLimitReached(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	used := 0
	calls := 0
	add := func(w *fsnotify.Watcher, p string) error {
		calls++
		if calls == 2 {
			return syscall.ENOSPC
		}
		return w.Add(p)
	}
	_, err := startWatcher(root, func() {}, nil, func(d int) { used += d }, discardLog(), watcherHooks{add: add})
	if !errors.Is(err, ErrWatchLimit) {
		t.Fatalf("err = %v, want ErrWatchLimit", err)
	}
	if used != 0 {
		t.Errorf("%d watches still counted after the fallback", used)
	}
}

func TestWatcherSkipsIgnoredFolders(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"Movie (2019)", "@eaDir", ".hidden", "Movie (2019)/Extras"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var added []string
	add := func(w *fsnotify.Watcher, p string) error {
		rel, _ := filepath.Rel(root, p)
		added = append(added, filepath.ToSlash(rel))
		return w.Add(p)
	}
	fw, err := startWatcher(root, func() {}, nil, nil, discardLog(), watcherHooks{add: add})
	if err != nil {
		t.Fatal(err)
	}
	fw.close()
	if len(added) != 2 || added[0] != "." || added[1] != "Movie (2019)" {
		t.Fatalf("watched %v", added)
	}
}

func TestNetworkFS(t *testing.T) {
	for fs, want := range map[string]bool{
		"nfs": true, "nfs4": true, "cifs": true, "smb2": true, "smbfs": true, "9p": true,
		"fuse": true, "fuse.sshfs": true, "fuseblk": true,
		"ext4": false, "btrfs": false, "apfs": false, "zfs": false, "": false,
	} {
		if got := networkFS(fs); got != want {
			t.Errorf("networkFS(%q) = %v, want %v", fs, got, want)
		}
	}
}

func TestEngineWatchesLocalFolders(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	root := t.TempDir()
	lib, err := NewStore(d).CreateLibrary(ctx, "Movies", TypeMovies, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	ft := &fakeTimers{}
	e := NewEngine(d, EngineOptions{Log: discardLog(), after: ft.after})
	e.Start(ctx)
	<-e.loaded
	got, err := e.Store().GetLibrary(ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if f := got.Folders[0]; !f.Watched || f.FSType == "" {
		t.Fatalf("folder after start: %+v", f)
	}
	// Watched library, never scanned: first periodic scan in 6 h.
	if a := ft.active(); len(a) != 1 || a[0].d != DefaultIntervalWatched {
		t.Fatalf("timers: %+v", a)
	}
	caps, _ := e.Capabilities(ctx)
	if caps.InotifyUsed != 1 {
		t.Errorf("InotifyUsed = %d, want 1", caps.InotifyUsed)
	}
	e.Close()
	caps, _ = e.Capabilities(ctx)
	if caps.InotifyUsed != 0 {
		t.Errorf("InotifyUsed after close = %d", caps.InotifyUsed)
	}
}
