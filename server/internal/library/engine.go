package library

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// startupDelay is the shortest wait before a library's first periodic scan
// after the server starts, so startup stays light (NFR startup, R15).
const startupDelay = time.Minute

// EngineOptions configure an Engine.
type EngineOptions struct {
	// Prober is nil when ffprobe is missing (R25): files are then listed
	// with probe_status = skipped.
	Prober Prober
	// FFprobeVersion is reported by the capabilities endpoint.
	FFprobeVersion string
	// Workers bounds concurrent probes server-wide; 0 means min(4, NumCPU).
	Workers int
	// BrowseRoots restricts the folder picker (PLAID_BROWSE_ROOTS); empty
	// means the whole file system.
	BrowseRoots []string
	Log         *slog.Logger
	// Now is the clock; tests replace it.
	Now func() time.Time

	// after schedules timers; tests replace it.
	after afterFunc
	// watch drives the watchers' clock; tests replace it.
	watch watcherHooks
}

// Engine owns the library store, the scan scheduler and the file system
// watchers. It is the entry point of the API handlers.
type Engine struct {
	store   *Store
	scanner *Scanner
	sched   *scheduler
	opts    EngineOptions
	log     *slog.Logger

	startOnce sync.Once
	loaded    chan struct{}

	watchMu   sync.Mutex
	watchers  map[string]*folderWatcher // folder id → watcher
	watchUsed atomic.Int64
	// overLimit lists folders that fell back to periodic scans because the
	// inotify limit was reached (R18).
	overLimit map[string]bool
}

// NewEngine builds an engine over an open database. Call Start to arm
// timers and watchers, Close to stop everything.
func NewEngine(db *sql.DB, o EngineOptions) *Engine {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.after == nil {
		o.after = realAfterFunc
	}
	if o.Workers <= 0 {
		o.Workers = min(4, runtime.NumCPU())
	}
	store := NewStore(db)
	store.Now = o.Now
	scanner := &Scanner{Store: store, Workers: o.Workers, Log: o.Log}
	if o.Prober != nil {
		scanner.Prober = NewProbePool(o.Prober, o.Workers)
	}
	e := &Engine{
		store: store, scanner: scanner, opts: o, log: o.Log, loaded: make(chan struct{}),
		watchers: map[string]*folderWatcher{}, overLimit: map[string]bool{},
	}
	e.sched = newScheduler(scanner.ScanLibrary, o.after, o.Now, o.Log)
	return e
}

// Store gives read access to the library tables.
func (e *Engine) Store() *Store { return e.store }

// Start loads libraries and arms their timers in the background; it
// returns immediately and never starts a full scan (R15, NFR startup).
func (e *Engine) Start(ctx context.Context) {
	e.startOnce.Do(func() {
		go func() {
			defer close(e.loaded)
			libs, err := e.store.ListLibraries(ctx)
			if err != nil {
				e.log.Error("libraries not loaded", "err", err)
				return
			}
			for _, lib := range libs {
				e.track(lib)
			}
			e.log.Info("libraries loaded", "count", len(libs))
		}()
	})
}

// Close stops every scan, timer and watcher.
func (e *Engine) Close() {
	e.sched.close()
	e.watchMu.Lock()
	ws := e.watchers
	e.watchers = map[string]*folderWatcher{}
	e.watchMu.Unlock()
	for _, w := range ws {
		w.close()
	}
}

// interval is a library's periodic scan interval (R19).
func interval(lib Library) time.Duration {
	if lib.ScanIntervalS != nil {
		return time.Duration(*lib.ScanIntervalS) * time.Second
	}
	for _, f := range lib.Folders {
		if !f.Watched {
			return DefaultIntervalUnwatched
		}
	}
	return DefaultIntervalWatched
}

// track schedules a library. Its first periodic scan is due one interval
// after its last scan, but no sooner than startupDelay.
func (e *Engine) track(lib Library) {
	lib = e.armFolders(lib)
	iv := interval(lib)
	first := iv
	if lib.LastScanAt != nil {
		first = lib.LastScanAt.Add(iv).Sub(e.opts.Now())
	}
	e.sched.track(lib.ID, iv, max(first, startupDelay))
}

// ensureTracked makes sure a library loaded after Start is scheduled.
func (e *Engine) ensureTracked(ctx context.Context, id string) error {
	if e.sched.tracked(id) {
		return nil
	}
	lib, err := e.store.GetLibrary(ctx, id)
	if err != nil {
		return err
	}
	e.track(lib)
	return nil
}

// Scan starts a scan of the library, or queues one if it's scanning (R20).
func (e *Engine) Scan(ctx context.Context, id string, full bool) error {
	if err := e.ensureTracked(ctx, id); err != nil {
		return err
	}
	return e.sched.request(id, full)
}

// Progress returns the library's scan state (R21).
func (e *Engine) Progress(ctx context.Context, id string) (Progress, error) {
	if err := e.ensureTracked(ctx, id); err != nil {
		return Progress{}, err
	}
	p, ok := e.sched.snapshot(id)
	if !ok {
		return Progress{}, ErrNotFound
	}
	return p, nil
}

// Subscribe streams the library's scan events until cancel is called, the
// library is deleted or the engine closes.
func (e *Engine) Subscribe(ctx context.Context, id string) (<-chan Event, func(), error) {
	if err := e.ensureTracked(ctx, id); err != nil {
		return nil, nil, err
	}
	return e.sched.subscribe(id)
}

// armFolders finds each folder's file system type and watches the local
// ones (R18), then stores both. It returns lib with updated folders.
func (e *Engine) armFolders(lib Library) Library {
	e.watchMu.Lock()
	defer e.watchMu.Unlock()
	for i := range lib.Folders {
		f := &lib.Folders[i]
		if _, ok := e.watchers[f.ID]; ok {
			f.Watched = true
			continue
		}
		f.FSType = fsType(f.Path)
		f.Watched = false
		switch {
		case networkFS(f.FSType):
			e.log.Info("network folder not watched, scanned periodically", "folder", f.Path, "fs_type", f.FSType)
		case e.overLimit[f.ID]:
		default:
			libID, folderID := lib.ID, f.ID
			w, err := startWatcher(f.Path,
				func() { e.changed(libID) },
				func() { e.watchLimitReached(libID, folderID) },
				e.countWatches, e.log, e.opts.watch)
			switch {
			case errors.Is(err, ErrWatchLimit):
				e.overLimit[f.ID] = true
				e.log.Warn("inotify watch limit reached, folder scanned periodically instead; see docs/install.md",
					"folder", f.Path, "limit", inotifyLimit())
			case err != nil:
				e.log.Warn("folder not watched", "folder", f.Path, "err", err)
			default:
				e.watchers[f.ID] = w
				f.Watched = true
			}
		}
		if err := e.store.SetFolderWatch(context.Background(), f.ID, f.FSType, f.Watched); err != nil {
			e.log.Error("folder watch state not saved", "folder", f.Path, "err", err)
		}
	}
	return lib
}

func (e *Engine) countWatches(delta int) { e.watchUsed.Add(int64(delta)) }

// changed is called once watched changes have settled.
func (e *Engine) changed(libraryID string) {
	if err := e.sched.request(libraryID, false); err != nil && !errors.Is(err, ErrNotFound) {
		e.log.Warn("scan after file change not started", "library", libraryID, "err", err)
	}
}

// watchLimitReached falls a folder back to periodic scans.
func (e *Engine) watchLimitReached(libraryID, folderID string) {
	e.watchMu.Lock()
	e.overLimit[folderID] = true
	w := e.watchers[folderID]
	delete(e.watchers, folderID)
	e.watchMu.Unlock()
	if w == nil {
		return
	}
	w.close()
	e.log.Warn("inotify watch limit reached, folder scanned periodically instead; see docs/install.md",
		"folder", w.root, "limit", inotifyLimit())
	ctx := context.Background()
	if err := e.store.SetFolderWatch(ctx, folderID, fsType(w.root), false); err != nil {
		e.log.Error("folder watch state not saved", "folder", w.root, "err", err)
	}
	if lib, err := e.store.GetLibrary(ctx, libraryID); err == nil {
		e.sched.setInterval(libraryID, interval(lib))
	}
}

// unwatch stops the watchers of the given folders.
func (e *Engine) unwatch(folderIDs ...string) {
	e.watchMu.Lock()
	var ws []*folderWatcher
	for _, id := range folderIDs {
		if w, ok := e.watchers[id]; ok {
			ws = append(ws, w)
			delete(e.watchers, id)
		}
		delete(e.overLimit, id)
	}
	e.watchMu.Unlock()
	for _, w := range ws {
		w.close()
	}
}

// Capabilities is what the server can do for libraries (R18, R25).
type Capabilities struct {
	FFprobeFound   bool
	FFprobeVersion string
	// InotifyLimit is the per-user watch limit, 0 when unknown or not Linux.
	InotifyLimit int
	// InotifyUsed counts the watches this server holds.
	InotifyUsed int
	// OverLimit lists the paths of folders that fell back to periodic
	// scans because the watch limit was reached.
	OverLimit []string
}

// Capabilities reports ffprobe and watch status.
func (e *Engine) Capabilities(ctx context.Context) (Capabilities, error) {
	c := Capabilities{FFprobeFound: e.opts.Prober != nil, FFprobeVersion: e.opts.FFprobeVersion, InotifyLimit: inotifyLimit()}
	c.InotifyUsed = int(e.watchUsed.Load())
	e.watchMu.Lock()
	over := make(map[string]bool, len(e.overLimit))
	for id := range e.overLimit {
		over[id] = true
	}
	e.watchMu.Unlock()
	if len(over) == 0 {
		return c, nil
	}
	libs, err := e.store.ListLibraries(ctx)
	if err != nil {
		return c, err
	}
	for _, lib := range libs {
		for _, f := range lib.Folders {
			if over[f.ID] {
				c.OverLimit = append(c.OverLimit, f.Path)
			}
		}
	}
	return c, nil
}
