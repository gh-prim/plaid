package playback

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// stopGrace is how long a run gets to exit after SIGTERM (R28).
const stopGrace = 5 * time.Second

// stderrKeep is how much of FFmpeg's stderr a run keeps for error logs.
const stderrKeep = 64 << 10

// Run is one FFmpeg process of a session (R12, R14, R28): its own
// process group, killed with its session. It tracks which pieces are on
// disk: FFmpeg writes each piece to a temporary name and renames it when
// complete, so a piece file that exists is finished.
type Run struct {
	Spec    RunSpec
	Started time.Time

	cmd    *exec.Cmd
	done   chan struct{}
	stderr *ring

	mu       sync.Mutex
	produced int // last piece of an unbroken run on disk; StartPiece-1 for none
	outMs    int64
	speed    float64
	paused   bool
	pausedAt time.Time
	idle     time.Duration // time spent paused
	stopping bool
	exited   bool // reaped: its pid may belong to someone else now
	ended    time.Time
	exitErr  error
	changed  chan struct{} // closed and replaced whenever produced moves or the run ends
	initDone bool
}

// ErrRunFailed means FFmpeg exited with an error before finishing.
var ErrRunFailed = errors.New("ffmpeg run failed")

// StartRun starts FFmpeg for a run. The process is spawned from one
// locked OS thread so Linux's Pdeathsig, tied to the forking thread,
// fires only when PLAID dies (plan step 16).
func StartRun(ffmpeg string, spec RunSpec) (*Run, error) {
	if err := os.MkdirAll(spec.Dir, 0o750); err != nil {
		return nil, err
	}
	r := &Run{
		Spec: spec, done: make(chan struct{}), stderr: &ring{max: stderrKeep},
		produced: spec.StartPiece - 1, changed: make(chan struct{}),
	}
	// Not tied to a context: the run lives until Stop, which signals the
	// whole process group (R28).
	cmd := exec.CommandContext(context.Background(), ffmpeg, spec.Args()...) //nolint:gosec // admin-configured binary, argument list
	cmd.SysProcAttr = procAttr()
	cmd.Stderr = r.stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	r.cmd = cmd
	if err := spawn(cmd); err != nil {
		return nil, fmt.Errorf("starting ffmpeg: %w", err)
	}
	r.Started = time.Now()
	go r.watch(stdout)
	return r, nil
}

// watch reads -progress blocks until FFmpeg exits, scanning for finished
// pieces after each one.
func (r *Run) watch(stdout io.Reader) {
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		key, value, _ := strings.Cut(sc.Text(), "=")
		switch key {
		case "out_time_us":
			if us, err := strconv.ParseInt(value, 10, 64); err == nil {
				r.mu.Lock()
				r.outMs = us / 1000
				r.mu.Unlock()
			}
		case "speed":
			if s, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(value), "x"), 64); err == nil {
				r.mu.Lock()
				r.speed = s
				r.mu.Unlock()
			}
		case "progress":
			r.scan()
		}
	}
	_, _ = io.Copy(io.Discard, stdout)
	err := r.cmd.Wait()
	r.mu.Lock()
	r.exited, r.ended = true, time.Now()
	r.mu.Unlock()
	r.scan()
	r.mu.Lock()
	if err != nil && !r.stopping {
		r.exitErr = fmt.Errorf("%w: %w: %s", ErrRunFailed, err, strings.TrimSpace(r.stderr.String()))
	}
	r.notify()
	r.mu.Unlock()
	close(r.done)
}

// scan advances produced over the pieces now on disk, and makes the
// run's init segment the session's once the first piece exists.
func (r *Run) scan() {
	r.mu.Lock()
	defer r.mu.Unlock()
	moved := false
	for r.produced+1 < r.Spec.Layout.Pieces() {
		if _, err := os.Stat(r.piecePath(r.produced + 1)); err != nil {
			break
		}
		r.produced++
		moved = true
	}
	if moved && !r.initDone {
		r.initDone = promoteInit(r.Spec.Dir, filepath.Join(r.Spec.Dir, r.Spec.InitName()))
	}
	if moved {
		r.notify()
	}
}

// notify wakes everyone waiting on the run. Callers hold mu.
func (r *Run) notify() {
	close(r.changed)
	r.changed = make(chan struct{})
}

func (r *Run) piecePath(n int) string { return filepath.Join(r.Spec.Dir, strconv.Itoa(n)+".m4s") }

// promoteInit makes a run's init segment the session's init.mp4 when
// there's none yet; every run writes the same bytes (plan step 16).
func promoteInit(dir, initPath string) bool {
	dst := filepath.Join(dir, "init.mp4")
	if _, err := os.Stat(dst); err == nil {
		_ = os.Remove(initPath)
		return true
	}
	if st, err := os.Stat(initPath); err != nil || st.Size() == 0 {
		return false
	}
	return os.Rename(initPath, dst) == nil
}

// Produced returns the last piece on disk of this run's unbroken output.
func (r *Run) Produced() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.produced
}

// Progress returns how far FFmpeg got (output time) and its speed.
func (r *Run) Progress() (outMs int64, speed float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.outMs, r.speed
}

// Ran is how long the process ran, so far if it still runs.
func (r *Run) Ran() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.exited {
		return r.ended.Sub(r.Started)
	}
	return time.Since(r.Started)
}

// Speed is how fast the run encodes against real time: FFmpeg's own
// figure, else (a run throttled before its first progress report) the
// media time of its pieces over the time it wasn't paused.
func (r *Run) Speed() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.speed > 0 {
		return r.speed
	}
	if r.produced < r.Spec.StartPiece {
		return 0
	}
	end := time.Now()
	switch {
	case r.exited:
		end = r.ended
	case r.paused:
		end = r.pausedAt
	}
	active := end.Sub(r.Started) - r.idle
	l := r.Spec.Layout
	media := l.Segments[len(l.Segments)-1].EndMs - l.PieceStartMs[r.Spec.StartPiece]
	if r.produced+1 < len(l.PieceStartMs) {
		media = l.PieceStartMs[r.produced+1] - l.PieceStartMs[r.Spec.StartPiece]
	}
	if active <= 0 {
		return 0
	}
	return float64(media) / float64(active.Milliseconds()+1)
}

// Done is closed when the process has exited.
func (r *Run) Done() <-chan struct{} { return r.done }

// Err is the run's failure once Done is closed; nil after a stop.
func (r *Run) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.exitErr
}

// Stderr is the end of FFmpeg's error output.
func (r *Run) Stderr() string { return r.stderr.String() }

// WaitPiece waits until piece n is on disk, the run ends, or ctx is
// done. It reports whether the piece is there.
func (r *Run) WaitPiece(ctx context.Context, n int) bool {
	for {
		r.mu.Lock()
		ok, changed := r.produced >= n, r.changed
		r.mu.Unlock()
		if ok {
			return true
		}
		select {
		case <-r.done:
			return r.Produced() >= n
		case <-ctx.Done():
			return false
		case <-changed:
		}
	}
}

// Pause stops FFmpeg with SIGSTOP (throttling, R15).
func (r *Run) Pause() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.paused && !r.stopping {
		r.paused = r.signal(syscall.SIGSTOP) == nil
		r.pausedAt = time.Now()
	}
}

// Resume continues a paused run.
func (r *Run) Resume() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.paused {
		_ = r.signal(syscall.SIGCONT)
		r.paused = false
		r.idle += time.Since(r.pausedAt)
	}
}

// Paused reports whether the run is throttled.
func (r *Run) Paused() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.paused
}

// Stop ends the run and waits for it: SIGCONT first (a stopped process
// ignores SIGTERM until continued), SIGTERM to the group, SIGKILL after
// 5 s (R28).
func (r *Run) Stop() {
	r.mu.Lock()
	r.stopping = true
	_ = r.signal(syscall.SIGCONT)
	r.paused = false
	_ = r.signal(syscall.SIGTERM)
	r.mu.Unlock()
	select {
	case <-r.done:
		return
	case <-time.After(stopGrace):
	}
	r.mu.Lock()
	_ = r.signal(syscall.SIGKILL)
	r.mu.Unlock()
	<-r.done
}

// signal sends a signal to the run's process group. Callers hold mu.
func (r *Run) signal(sig syscall.Signal) error {
	if r.exited {
		return os.ErrProcessDone
	}
	return syscall.Kill(-r.cmd.Process.Pid, sig)
}

// Pid is FFmpeg's process id (also its process group id).
func (r *Run) Pid() int { return r.cmd.Process.Pid }

// ring keeps the last max bytes written to it.
type ring struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func (b *ring) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if over := len(b.buf) - b.max; over > 0 {
		b.buf = append(b.buf[:0], b.buf[over:]...)
	}
	return len(p), nil
}

func (b *ring) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}
