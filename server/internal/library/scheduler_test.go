package library

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"runtime"
	"sync"
	"testing"
	"time"
)

// fakeTimers records scheduled timers; tests fire them by hand.
type fakeTimers struct {
	mu     sync.Mutex
	timers []*fakeTimer
}

type fakeTimer struct {
	d       time.Duration
	f       func()
	stopped bool
}

func (ft *fakeTimers) after(d time.Duration, f func()) func() bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	t := &fakeTimer{d: d, f: f}
	ft.timers = append(ft.timers, t)
	return func() bool {
		ft.mu.Lock()
		defer ft.mu.Unlock()
		was := !t.stopped
		t.stopped = true
		return was
	}
}

// active returns the timers that are still armed.
func (ft *fakeTimers) active() []*fakeTimer {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	var out []*fakeTimer
	for _, t := range ft.timers {
		if !t.stopped {
			out = append(out, t)
		}
	}
	return out
}

func (ft *fakeTimers) fire(t *testing.T) {
	t.Helper()
	a := ft.active()
	if len(a) != 1 {
		t.Fatalf("%d active timers, want 1", len(a))
	}
	ft.mu.Lock()
	a[0].stopped = true
	ft.mu.Unlock()
	a[0].f()
}

// blockingRun is a scan function that waits for release and records calls.
type blockingRun struct {
	mu      sync.Mutex
	calls   []ScanOptions
	started chan struct{}
	release chan struct{}
}

func newBlockingRun() *blockingRun {
	return &blockingRun{started: make(chan struct{}, 10), release: make(chan struct{})}
}

func (b *blockingRun) run(ctx context.Context, _ string, opts ScanOptions) (Report, error) {
	b.mu.Lock()
	b.calls = append(b.calls, opts)
	b.mu.Unlock()
	opts.Progress(ScanProgress{Phase: PhaseScanning, Folder: "/media", Report: Report{Found: 1}})
	b.started <- struct{}{}
	select {
	case <-b.release:
	case <-ctx.Done():
		return Report{}, ctx.Err()
	}
	opts.Progress(ScanProgress{Phase: PhaseAnalyzing, Report: Report{Found: 1, Analyzed: 1}})
	return Report{Found: 1, Analyzed: 1}, nil
}

func (b *blockingRun) fulls() []bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []bool
	for _, c := range b.calls {
		out = append(out, c.Full)
	}
	return out
}

func discardLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newTestScheduler(run scanFunc) (*scheduler, *fakeTimers) {
	ft := &fakeTimers{}
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	return newScheduler(run, ft.after, func() time.Time { return now }, discardLog()), ft
}

func waitIdle(t *testing.T, s *scheduler, id string) Progress {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if p, _ := s.snapshot(id); p.State == StateIdle {
			return p
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("scan never went idle")
	return Progress{}
}

func TestSchedulerQueuesOnce(t *testing.T) {
	br := newBlockingRun()
	s, _ := newTestScheduler(br.run)
	defer s.close()
	s.track("lib", DefaultIntervalUnwatched, time.Hour)

	if err := s.request("lib", false); err != nil {
		t.Fatal(err)
	}
	<-br.started
	// Three more requests during the scan: one queued scan, full because
	// one of them asked for it.
	for _, full := range []bool{false, true, false} {
		if err := s.request("lib", full); err != nil {
			t.Fatal(err)
		}
	}
	if p, _ := s.snapshot("lib"); p.State != StateScanning || !p.Queued || p.Folder != "/media" {
		t.Fatalf("during scan: %+v", p)
	}
	br.release <- struct{}{}
	<-br.started
	br.release <- struct{}{}
	p := waitIdle(t, s, "lib")
	if got := br.fulls(); len(got) != 2 || got[0] || !got[1] {
		t.Fatalf("scans run with full = %v, want [false true]", got)
	}
	if p.Report.Analyzed != 1 || p.Queued {
		t.Errorf("idle progress: %+v", p)
	}
}

func TestSchedulerUnknownLibrary(t *testing.T) {
	s, _ := newTestScheduler(newBlockingRun().run)
	defer s.close()
	if err := s.request("nope", false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("request = %v, want ErrNotFound", err)
	}
	if _, _, err := s.subscribe("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("subscribe = %v, want ErrNotFound", err)
	}
}

func TestSchedulerPeriodicTimer(t *testing.T) {
	br := newBlockingRun()
	s, ft := newTestScheduler(br.run)
	defer s.close()
	s.track("lib", 15*time.Minute, 3*time.Minute)
	if a := ft.active(); len(a) != 1 || a[0].d != 3*time.Minute {
		t.Fatalf("first timer: %+v", a)
	}
	ft.fire(t)
	<-br.started
	if len(ft.active()) != 0 {
		t.Fatal("timer armed during a scan")
	}
	br.release <- struct{}{}
	waitIdle(t, s, "lib")
	// Rearmed for one interval after the scan.
	if a := ft.active(); len(a) != 1 || a[0].d != 15*time.Minute {
		t.Fatalf("timer after scan: %+v", a)
	}
	s.setInterval("lib", time.Hour)
	if a := ft.active(); len(a) != 1 || a[0].d != time.Hour {
		t.Fatalf("timer after setInterval: %+v", a)
	}
}

func TestSchedulerEvents(t *testing.T) {
	br := newBlockingRun()
	s, _ := newTestScheduler(br.run)
	defer s.close()
	s.track("lib", time.Hour, time.Hour)
	ch, cancel, err := s.subscribe("lib")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if err := s.request("lib", false); err != nil {
		t.Fatal(err)
	}
	<-br.started
	br.release <- struct{}{}

	var types []string
	for ev := range ch {
		types = append(types, ev.Type+":"+ev.Progress.State)
		if ev.Type == EventCompleted {
			if ev.Progress.Report.Analyzed != 1 {
				t.Errorf("completed report: %+v", ev.Progress.Report)
			}
			break
		}
	}
	want := []string{"progress:queued", "progress:scanning", "progress:analyzing", "completed:idle"}
	if len(types) != len(want) {
		t.Fatalf("events %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("events %v, want %v", types, want)
		}
	}
}

func TestSchedulerSlowSubscriberKeepsTerminalEvent(t *testing.T) {
	s, _ := newTestScheduler(nil)
	defer s.close()
	s.track("lib", time.Hour, time.Hour)
	ch, cancel, _ := s.subscribe("lib")
	defer cancel()
	s.mu.Lock()
	l := s.libs["lib"]
	for range subscriberBuffer * 2 {
		s.publishLocked(l, Event{Type: EventProgress})
	}
	s.publishLocked(l, Event{Type: EventCompleted})
	s.mu.Unlock()
	var last Event
	for range subscriberBuffer {
		last = <-ch
	}
	if last.Type != EventCompleted {
		t.Fatalf("last buffered event is %q, want completed", last.Type)
	}
}

func TestSchedulerRemoveCancelsScan(t *testing.T) {
	br := newBlockingRun()
	s, ft := newTestScheduler(br.run)
	defer s.close()
	s.track("lib", time.Hour, time.Hour)
	ch, _, _ := s.subscribe("lib")
	_ = s.request("lib", false)
	<-br.started
	s.remove("lib")
	drain(ch) // closed by remove
	if s.tracked("lib") || len(ft.active()) != 0 {
		t.Fatal("library still scheduled after remove")
	}
}

func TestSchedulerCloseStopsEverything(t *testing.T) {
	before := runtime.NumGoroutine()
	br := newBlockingRun()
	s, ft := newTestScheduler(br.run)
	for _, id := range []string{"a", "b", "c"} {
		s.track(id, time.Hour, time.Hour)
		_ = s.request(id, false)
		<-br.started
	}
	ch, _, _ := s.subscribe("a")
	s.close()
	drain(ch)
	if len(ft.active()) != 0 {
		t.Error("timers still armed after close")
	}
	if err := s.request("a", false); err == nil {
		t.Error("request accepted after close")
	}
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > before {
		t.Errorf("%d goroutines after close, %d before", n, before)
	}
}

func TestInterval(t *testing.T) {
	custom := int64(600)
	for _, tc := range []struct {
		name string
		lib  Library
		want time.Duration
	}{
		{"unwatched", Library{Folders: []Folder{{Watched: false}}}, DefaultIntervalUnwatched},
		{"watched", Library{Folders: []Folder{{Watched: true}}}, DefaultIntervalWatched},
		{"one unwatched folder", Library{Folders: []Folder{{Watched: true}, {Watched: false}}}, DefaultIntervalUnwatched},
		{"custom", Library{ScanIntervalS: &custom, Folders: []Folder{{Watched: true}}}, 10 * time.Minute},
	} {
		if got := interval(tc.lib); got != tc.want {
			t.Errorf("%s: interval = %v, want %v", tc.name, got, tc.want)
		}
	}
}

type countingProber struct {
	mu        sync.Mutex
	running   int
	maxSeen   int
	gate      chan struct{}
	completed int
}

func (c *countingProber) Probe(_ context.Context, _ string) (*ProbeResult, error) {
	c.mu.Lock()
	c.running++
	c.maxSeen = max(c.maxSeen, c.running)
	c.mu.Unlock()
	<-c.gate
	c.mu.Lock()
	c.running--
	c.completed++
	c.mu.Unlock()
	return &ProbeResult{}, nil
}

func TestProbePoolBoundsConcurrency(t *testing.T) {
	cp := &countingProber{gate: make(chan struct{})}
	pool := NewProbePool(cp, 2)
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() { _, _ = pool.Probe(context.Background(), "x") })
	}
	for range 6 {
		cp.gate <- struct{}{}
	}
	wg.Wait()
	if cp.maxSeen > 2 || cp.completed != 6 {
		t.Fatalf("max concurrent %d, completed %d", cp.maxSeen, cp.completed)
	}
}

func TestEngineStartDoesNotScan(t *testing.T) {
	d := openTestDB(t)
	st := NewStore(d)
	ctx := context.Background()
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	st.Now = func() time.Time { return now.Add(-10 * time.Minute) }
	lib, err := st.CreateLibrary(ctx, "Movies", TypeMovies, []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.setLastScan(ctx, lib.ID); err != nil {
		t.Fatal(err)
	}
	ft := &fakeTimers{}
	e := NewEngine(d, EngineOptions{Log: discardLog(), Now: func() time.Time { return now }, after: ft.after})
	defer e.Close()
	e.Start(ctx)
	<-e.loaded
	p, err := e.Progress(ctx, lib.ID)
	if err != nil || p.State != StateIdle {
		t.Fatalf("after start: %+v %v", p, err)
	}
	// Last scanned 10 minutes ago, watched: next scan 6 h after that.
	if a := ft.active(); len(a) != 1 || a[0].d != DefaultIntervalWatched-10*time.Minute {
		t.Fatalf("timers after start: %+v", a)
	}
}

// drain reads ch until it is closed.
func drain(ch <-chan Event) {
	for range ch { //nolint:revive // only waiting for close
	}
}
