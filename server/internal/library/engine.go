package library

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gh-prim/plaid/server/internal/problems"
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
	// Hooks report scan results to F005 (metadata).
	Hooks Hooks
	// Problems records folder and watch-limit problems (F007 R29).
	Problems problems.Reporter
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
	scanner := &Scanner{Store: store, Workers: o.Workers, Log: o.Log, Hooks: o.Hooks, Problems: o.Problems}
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
				problems.Of(e.opts.Problems).Report(problems.WatchLimit, map[string]string{"path": f.Path})
			case err != nil:
				e.log.Warn("folder not watched", "folder", f.Path, "err", err)
			default:
				e.watchers[f.ID] = w
				f.Watched = true
				problems.Of(e.opts.Problems).Resolve(problems.WatchLimit, map[string]string{"path": f.Path})
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
	problems.Of(e.opts.Problems).Report(problems.WatchLimit, map[string]string{"path": w.root})
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

// BrowseRoots returns the folder picker roots (PLAID_BROWSE_ROOTS).
func (e *Engine) BrowseRoots() []string { return e.opts.BrowseRoots }

// View is a library with its counts and scan state, as the API shows it.
type View struct {
	Library
	Counts Counts
	Scan   Progress
}

// Libraries lists every library with counts and scan state.
func (e *Engine) Libraries(ctx context.Context) ([]View, error) {
	libs, err := e.store.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := e.store.LibraryCounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(libs))
	for _, lib := range libs {
		p, _ := e.sched.snapshot(lib.ID)
		if p.State == "" {
			p.State = StateIdle
		}
		out = append(out, View{Library: lib, Counts: counts[lib.ID], Scan: p})
	}
	return out, nil
}

// View returns one library with counts and scan state.
func (e *Engine) View(ctx context.Context, id string) (View, error) {
	lib, err := e.store.GetLibrary(ctx, id)
	if err != nil {
		return View{}, err
	}
	counts, err := e.store.LibraryCounts(ctx)
	if err != nil {
		return View{}, err
	}
	p, _ := e.sched.snapshot(id)
	if p.State == "" {
		p.State = StateIdle
	}
	return View{Library: lib, Counts: counts[id], Scan: p}, nil
}

// validateFolders cleans and checks every folder (R2) and rejects overlaps
// with each other and with other libraries' folders.
func (e *Engine) validateFolders(ctx context.Context, folders []string, selfID string) ([]string, error) {
	var clean []string
	for _, f := range folders {
		c, err := ValidateFolder(f)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(clean, c) {
			clean = append(clean, c)
		}
	}
	existing, err := e.store.AllFolders(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkOverlap(clean, existing, selfID); err != nil {
		return nil, err
	}
	return clean, nil
}

// NewLibrary is what CreateLibrary needs. Name, type and interval bounds
// are checked by the caller.
type NewLibrary struct {
	Name          string
	Type          string
	Folders       []string
	ScanIntervalS *int64
	// MetadataLanguage and MetadataRegion default to en/US when empty.
	MetadataLanguage string
	MetadataRegion   string
}

// CreateLibrary validates the folders, stores the library, arms its
// watchers and queues a full scan (R1, R2). Nothing is created on error.
func (e *Engine) CreateLibrary(ctx context.Context, n NewLibrary) (Library, error) {
	folders, err := e.validateFolders(ctx, n.Folders, "")
	if err != nil {
		return Library{}, err
	}
	lib, err := e.store.CreateLibrary(ctx, n.Name, n.Type, folders, CreateOptions{
		ScanIntervalS: n.ScanIntervalS, MetadataLanguage: n.MetadataLanguage, MetadataRegion: n.MetadataRegion,
	})
	if err != nil {
		return Library{}, err
	}
	e.track(lib)
	if err := e.sched.request(lib.ID, true); err != nil {
		return Library{}, err
	}
	return e.store.GetLibrary(ctx, lib.ID)
}

// UpdateLibrary applies p. New folders are validated, watched and scanned;
// removed ones stop being watched.
func (e *Engine) UpdateLibrary(ctx context.Context, id string, p Patch) (Library, error) {
	before, err := e.store.GetLibrary(ctx, id)
	if err != nil {
		return Library{}, err
	}
	if p.Folders != nil {
		if p.Folders, err = e.validateFolders(ctx, p.Folders, id); err != nil {
			return Library{}, err
		}
	}
	added, err := e.store.UpdateLibrary(ctx, id, p)
	if err != nil {
		return Library{}, err
	}
	if p.Folders != nil {
		var removed []string
		for _, f := range before.Folders {
			if !slices.Contains(p.Folders, f.Path) {
				removed = append(removed, f.ID)
			}
		}
		e.unwatch(removed...)
	}
	lib, err := e.store.GetLibrary(ctx, id)
	if err != nil {
		return Library{}, err
	}
	lib = e.armFolders(lib)
	if !e.sched.tracked(id) {
		e.track(lib)
	} else {
		e.sched.setInterval(id, interval(lib))
	}
	if len(added) > 0 || len(lib.Folders) < len(before.Folders) {
		// New folders' files are found, removed ones' items cleaned up.
		if err := e.sched.request(id, false); err != nil {
			return Library{}, err
		}
	}
	return lib, nil
}

// DeleteLibrary cancels the library's scan, stops its watchers and deletes
// its rows. Nothing on disk is touched (R23).
func (e *Engine) DeleteLibrary(ctx context.Context, id string) error {
	lib, err := e.store.GetLibrary(ctx, id)
	if err != nil {
		return err
	}
	e.sched.remove(id)
	ids := make([]string, 0, len(lib.Folders))
	for _, f := range lib.Folders {
		ids = append(ids, f.ID)
	}
	e.unwatch(ids...)
	return e.store.DeleteLibrary(ctx, id)
}
