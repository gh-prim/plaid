package library

import (
	"context"
	"database/sql"
	"log/slog"
	"runtime"
	"sync"
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
	e := &Engine{store: store, scanner: scanner, opts: o, log: o.Log, loaded: make(chan struct{})}
	e.sched = newScheduler(scanner.ScanLibrary, o.after, o.Now, o.Log)
	return e
}

// Store gives read access to the library tables.
func (e *Engine) Store() *Store { return e.store }

// FFprobe reports whether ffprobe was found, and its version.
func (e *Engine) FFprobe() (found bool, version string) {
	return e.opts.Prober != nil, e.opts.FFprobeVersion
}

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
