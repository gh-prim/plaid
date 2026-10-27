package library

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Debounce settings (R18): a changed path is polled every pollInterval and
// counts as settled once its size hasn't changed for stableFor.
const (
	pollInterval = 2 * time.Second
	stableFor    = 10 * time.Second
)

// ErrWatchLimit means the inotify watch limit was reached (ENOSPC).
var ErrWatchLimit = errors.New("inotify watch limit reached")

// folderWatcher watches one library folder recursively and calls onSettled
// once every changed path has been stable for stableFor. Waiting for every
// path, not just one, keeps a scan from probing a file still being copied.
type folderWatcher struct {
	root      string
	onSettled func()
	// onLimit runs (in its own goroutine) when the watch limit is hit after
	// start; the engine then closes this watcher and falls back.
	onLimit func()
	log     *slog.Logger
	now     func() time.Time

	w *fsnotify.Watcher
	// add registers one directory; tests replace it.
	add func(string) error
	// count is told about every watch added, for the capabilities endpoint.
	count func(delta int)

	mu        sync.Mutex
	pending   map[string]*pendingPath
	watches   int
	done      chan struct{}
	stopped   chan struct{}
	closeOnce sync.Once
}

type pendingPath struct {
	size       int64 // -1 when the path is gone
	lastChange time.Time
}

// watcherHooks let tests drive time.
type watcherHooks struct {
	now  func() time.Time
	tick <-chan time.Time
	add  func(w *fsnotify.Watcher, path string) error
}

// startWatcher watches root and every directory below it that the walker
// would visit. It returns ErrWatchLimit when the inotify limit is hit; the
// caller then falls back to periodic scans.
func startWatcher(root string, onSettled, onLimit func(), count func(int), log *slog.Logger, h watcherHooks) (*folderWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, watchError(err)
	}
	fw := &folderWatcher{
		root: root, onSettled: onSettled, onLimit: onLimit, log: log, now: h.now, w: w, count: count,
		pending: map[string]*pendingPath{}, done: make(chan struct{}), stopped: make(chan struct{}),
	}
	if fw.now == nil {
		fw.now = time.Now
	}
	fw.add = w.Add
	if h.add != nil {
		fw.add = func(p string) error { return h.add(w, p) }
	}
	if err := fw.addTree(root); err != nil {
		fw.closeWatches()
		return nil, err
	}
	tick := h.tick
	var ticker *time.Ticker
	if tick == nil {
		ticker = time.NewTicker(pollInterval)
		tick = ticker.C
	}
	go fw.run(tick, ticker)
	return fw, nil
}

func watchError(err error) error {
	if errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EMFILE) {
		return ErrWatchLimit
	}
	return err
}

// addTree adds a watch on dir and every sub-directory not ignored by the
// built-in rules (R5). Unreadable sub-directories are skipped like the
// walker does.
func (fw *folderWatcher) addTree(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == dir {
				return err
			}
			return filepath.SkipDir
		}
		if !d.IsDir() {
			return nil
		}
		if path != fw.root && ignoredDir(d.Name()) {
			return filepath.SkipDir
		}
		if err := fw.add(path); err != nil {
			if err := watchError(err); errors.Is(err, ErrWatchLimit) {
				return err
			}
			fw.log.Debug("directory not watched", "path", path, "err", err)
			return filepath.SkipDir
		}
		fw.mu.Lock()
		fw.watches++
		fw.mu.Unlock()
		if fw.count != nil {
			fw.count(1)
		}
		return nil
	})
}

func (fw *folderWatcher) run(tick <-chan time.Time, ticker *time.Ticker) {
	defer close(fw.stopped)
	if ticker != nil {
		defer ticker.Stop()
	}
	for {
		select {
		case <-fw.done:
			return
		case ev, ok := <-fw.w.Events:
			if !ok {
				return
			}
			fw.event(ev)
		case err, ok := <-fw.w.Errors:
			if !ok {
				return
			}
			fw.log.Warn("file watcher error", "folder", fw.root, "err", err)
		case <-tick:
			if fw.poll() {
				fw.onSettled()
			}
		}
	}
}

func (fw *folderWatcher) event(ev fsnotify.Event) {
	if ev.Op == fsnotify.Chmod {
		return
	}
	if ev.Has(fsnotify.Create) {
		if info, err := os.Lstat(ev.Name); err == nil && info.IsDir() && !ignoredDir(info.Name()) {
			if err := fw.addTree(ev.Name); errors.Is(err, ErrWatchLimit) && fw.onLimit != nil {
				go fw.onLimit()
			}
		}
	}
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.pending[ev.Name] = &pendingPath{size: sizeOf(ev.Name), lastChange: fw.now()}
}

// poll updates every pending path's size and reports whether they have all
// settled; settled paths are then forgotten.
func (fw *folderWatcher) poll() bool {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	if len(fw.pending) == 0 {
		return false
	}
	now := fw.now()
	settled := true
	for path, p := range fw.pending {
		if size := sizeOf(path); size != p.size {
			p.size, p.lastChange = size, now
		}
		if now.Sub(p.lastChange) < stableFor {
			settled = false
		}
	}
	if settled {
		clear(fw.pending)
	}
	return settled
}

func (fw *folderWatcher) pendingCount() int {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return len(fw.pending)
}

func sizeOf(path string) int64 {
	info, err := os.Lstat(path)
	if err != nil {
		return -1
	}
	return info.Size()
}

// close stops the watcher and releases its watches.
func (fw *folderWatcher) close() {
	fw.closeOnce.Do(func() {
		close(fw.done)
		<-fw.stopped
		fw.closeWatches()
	})
}

func (fw *folderWatcher) closeWatches() {
	_ = fw.w.Close()
	fw.mu.Lock()
	n := fw.watches
	fw.watches = 0
	fw.mu.Unlock()
	if fw.count != nil && n > 0 {
		fw.count(-n)
	}
}
