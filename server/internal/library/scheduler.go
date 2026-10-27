package library

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Scan states (R21).
const (
	StateIdle      = "idle"
	StateQueued    = "queued"
	StateScanning  = "scanning"
	StateAnalyzing = "analyzing"
)

// Periodic scan intervals (R19).
const (
	DefaultIntervalUnwatched = 15 * time.Minute
	DefaultIntervalWatched   = 6 * time.Hour
	MinScanInterval          = 5 * time.Minute
	MaxScanInterval          = 24 * time.Hour
)

// Event types sent to scan subscribers.
const (
	EventProgress  = "progress"
	EventCompleted = "completed"
	EventFailed    = "failed"
)

// subscriberBuffer is how many events a subscriber can lag behind. Progress
// events beyond it are dropped (the next one carries the latest counters);
// completed and failed events push out the oldest pending event instead.
const subscriberBuffer = 16

// Progress is the scan state of one library (R21).
type Progress struct {
	State string
	// Queued is true when another scan will start once this one ends.
	Queued    bool
	Full      bool
	Folder    string
	StartedAt time.Time // zero when idle
	Report    Report
}

// Event is what subscribers of a library's scans receive.
type Event struct {
	Type     string
	Progress Progress
	// Error is set on failed events.
	Error string
}

// afterFunc runs f once after d; stop cancels it. Tests replace it.
type afterFunc func(d time.Duration, f func()) (stop func() bool)

func realAfterFunc(d time.Duration, f func()) func() bool {
	return time.AfterFunc(d, f).Stop
}

// scanFunc runs one scan of a library.
type scanFunc func(ctx context.Context, libraryID string, opts ScanOptions) (Report, error)

// scheduler runs at most one scan per library at a time, queues one more
// request during a scan, and rescans every library periodically (R19–R21).
type scheduler struct {
	run   scanFunc
	after afterFunc
	now   func() time.Time
	log   *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu     sync.Mutex
	libs   map[string]*libSched
	closed bool
}

type libSched struct {
	id        string
	interval  time.Duration
	stopTimer func() bool
	// Set while a scan goroutine runs.
	cancel     context.CancelFunc
	done       chan struct{}
	queued     bool
	queuedFull bool
	progress   Progress
	subs       map[chan Event]struct{}
}

func newScheduler(run scanFunc, after afterFunc, now func() time.Time, log *slog.Logger) *scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &scheduler{
		run: run, after: after, now: now, log: log,
		ctx: ctx, cancel: cancel, libs: map[string]*libSched{},
	}
}

// track starts scheduling a library: its first periodic scan runs after
// first, the next ones every interval. Tracking a known library only
// updates its interval.
func (s *scheduler) track(id string, interval, first time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	l, ok := s.libs[id]
	if ok {
		l.interval = interval
		return
	}
	l = &libSched{id: id, interval: interval, progress: Progress{State: StateIdle}, subs: map[chan Event]struct{}{}}
	s.libs[id] = l
	s.armLocked(l, first)
}

func (s *scheduler) tracked(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.libs[id]
	return ok
}

// setInterval changes the period and restarts the timer when idle.
func (s *scheduler) setInterval(id string, interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.libs[id]
	if !ok || s.closed {
		return
	}
	l.interval = interval
	if l.done == nil {
		s.armLocked(l, interval)
	}
}

func (s *scheduler) armLocked(l *libSched, d time.Duration) {
	if l.stopTimer != nil {
		l.stopTimer()
	}
	id := l.id
	l.stopTimer = s.after(d, func() {
		if err := s.request(id, false); err != nil && !errors.Is(err, ErrNotFound) {
			s.log.Warn("periodic scan not started", "library", id, "err", err)
		}
	})
}

// request starts a scan, or queues one if a scan is running. Requests made
// while one is already queued are merged into it (R20).
func (s *scheduler) request(id string, full bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return context.Canceled
	}
	l, ok := s.libs[id]
	if !ok {
		return ErrNotFound
	}
	if l.done != nil {
		l.queued = true
		l.queuedFull = l.queuedFull || full
		l.progress.Queued = true
		s.publishLocked(l, Event{Type: EventProgress})
		return nil
	}
	s.startLocked(l, full)
	return nil
}

func (s *scheduler) startLocked(l *libSched, full bool) {
	if l.stopTimer != nil {
		l.stopTimer()
		l.stopTimer = nil
	}
	ctx, cancel := context.WithCancel(s.ctx)
	l.cancel, l.done = cancel, make(chan struct{})
	l.progress = Progress{State: StateQueued, Full: full, StartedAt: s.now()}
	s.publishLocked(l, Event{Type: EventProgress})
	s.wg.Add(1)
	go s.loop(ctx, l, full)
}

func (s *scheduler) loop(ctx context.Context, l *libSched, full bool) {
	defer s.wg.Done()
	for {
		opts := ScanOptions{Full: full, Progress: func(p ScanProgress) { s.onProgress(l, p) }}
		rep, err := s.run(ctx, l.id, opts)

		s.mu.Lock()
		ev := Event{Type: EventCompleted}
		if err != nil {
			ev = Event{Type: EventFailed, Error: err.Error()}
			if ctx.Err() == nil {
				s.log.Error("library scan failed", "library", l.id, "err", err)
			}
		}
		requeue := l.queued && ctx.Err() == nil
		if requeue {
			full = l.queuedFull
			l.progress = Progress{State: StateQueued, Full: full, StartedAt: s.now(), Report: rep}
		} else {
			l.progress = Progress{State: StateIdle, Report: rep}
		}
		l.queued, l.queuedFull = false, false
		s.publishLocked(l, ev)
		if requeue {
			s.mu.Unlock()
			continue
		}
		close(l.done)
		l.cancel()
		l.cancel, l.done = nil, nil
		if !s.closed && s.libs[l.id] == l {
			s.armLocked(l, l.interval)
		}
		s.mu.Unlock()
		return
	}
}

func (s *scheduler) onProgress(l *libSched, p ScanProgress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l.progress.State = p.Phase
	l.progress.Folder = p.Folder
	l.progress.Report = p.Report
	s.publishLocked(l, Event{Type: EventProgress})
}

// publishLocked sends ev, stamped with the current progress, to every
// subscriber without blocking.
func (s *scheduler) publishLocked(l *libSched, ev Event) {
	ev.Progress = l.progress
	for ch := range l.subs {
		select {
		case ch <- ev:
			continue
		default:
		}
		if ev.Type == EventProgress {
			continue
		}
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- ev:
		default:
		}
	}
}

// snapshot returns a library's current progress.
func (s *scheduler) snapshot(id string) (Progress, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.libs[id]
	if !ok {
		return Progress{}, false
	}
	return l.progress, true
}

// subscribe returns a channel of the library's scan events. It is closed
// when the library is removed or the scheduler closes; cancel stops it.
func (s *scheduler) subscribe(id string) (<-chan Event, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, nil, context.Canceled
	}
	l, ok := s.libs[id]
	if !ok {
		return nil, nil, ErrNotFound
	}
	ch := make(chan Event, subscriberBuffer)
	l.subs[ch] = struct{}{}
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			if _, ok := l.subs[ch]; ok {
				delete(l.subs, ch)
				close(ch)
			}
		})
	}
	return ch, cancel, nil
}

// remove stops scheduling a library, cancels its scan and waits for it.
func (s *scheduler) remove(id string) {
	s.mu.Lock()
	l, ok := s.libs[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.libs, id)
	if l.stopTimer != nil {
		l.stopTimer()
	}
	done := l.done
	if l.cancel != nil {
		l.cancel()
	}
	s.mu.Unlock()
	if done != nil {
		<-done
	}
	s.mu.Lock()
	closeSubsLocked(l)
	s.mu.Unlock()
}

func closeSubsLocked(l *libSched) {
	for ch := range l.subs {
		delete(l.subs, ch)
		close(ch)
	}
}

// close cancels every scan and timer and waits for scan goroutines.
func (s *scheduler) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	for _, l := range s.libs {
		if l.stopTimer != nil {
			l.stopTimer()
		}
	}
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
	s.mu.Lock()
	for _, l := range s.libs {
		closeSubsLocked(l)
	}
	s.mu.Unlock()
}

// ProbePool bounds concurrent probes server-wide (R24): every library's
// scanner shares it.
type ProbePool struct {
	prober Prober
	sem    chan struct{}
}

// NewProbePool wraps p so that at most n probes run at once.
func NewProbePool(p Prober, n int) *ProbePool {
	return &ProbePool{prober: p, sem: make(chan struct{}, max(n, 1))}
}

// Probe waits for a free slot, then probes.
func (pp *ProbePool) Probe(ctx context.Context, path string) (*ProbeResult, error) {
	select {
	case pp.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-pp.sem }()
	return pp.prober.Probe(ctx, path)
}
