package metadata

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// ErrPaused means the provider can't take requests for now (no key, or the
// key was rejected): the job is released without counting an attempt.
var ErrPaused = errors.New("metadata provider paused")

// Queue runs jobs from the store on a fixed pool of workers (R2).
type Queue struct {
	Store *Store
	// Process does one job. It returns ErrPaused to release the job, any
	// other error to count a failed attempt.
	Process func(ctx context.Context, j Job) error
	Workers int // 0 means 4
	// Lease is how long a claimed job stays reserved (default 10 min).
	Lease time.Duration
	// Idle is how long an idle worker waits before looking again when
	// nothing woke it (default 1 min).
	Idle time.Duration
	// PauseFor is how long a paused job waits (default 10 min).
	PauseFor time.Duration
	Log      *slog.Logger

	wake chan struct{}
	stop context.CancelFunc
	wg   sync.WaitGroup
}

// Start runs the workers until Close. It returns at once.
func (q *Queue) Start(ctx context.Context) {
	if q.Workers <= 0 {
		q.Workers = 4
	}
	if q.Lease == 0 {
		q.Lease = 10 * time.Minute
	}
	if q.Idle == 0 {
		q.Idle = time.Minute
	}
	if q.PauseFor == 0 {
		q.PauseFor = 10 * time.Minute
	}
	if q.Log == nil {
		q.Log = slog.Default()
	}
	q.wake = make(chan struct{}, q.Workers)
	ctx, q.stop = context.WithCancel(ctx)
	for range q.Workers {
		q.wg.Add(1)
		go q.work(ctx)
	}
}

// Wake tells idle workers there may be work.
func (q *Queue) Wake() {
	for range q.Workers {
		select {
		case q.wake <- struct{}{}:
		default:
			return
		}
	}
}

// Close stops the workers and waits for them.
func (q *Queue) Close() {
	if q.stop != nil {
		q.stop()
	}
	q.wg.Wait()
}

func (q *Queue) work(ctx context.Context) {
	defer q.wg.Done()
	for ctx.Err() == nil {
		j, ok, err := q.Store.Claim(ctx, q.Lease)
		if err != nil {
			if ctx.Err() == nil {
				q.Log.Error("metadata queue: claim failed", "err", err)
			}
			q.sleep(ctx)
			continue
		}
		if !ok {
			q.sleep(ctx)
			continue
		}
		q.run(ctx, j)
	}
}

func (q *Queue) run(ctx context.Context, j Job) {
	err := q.Process(ctx, j)
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		return // interrupted by shutdown; the lease expires and the job comes back
	}
	// Record the outcome even if shutdown started meanwhile.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	switch {
	case err == nil:
	case errors.Is(err, ErrPaused):
		if err := q.Store.Release(ctx, j, q.PauseFor); err != nil {
			q.Log.Error("metadata queue: release failed", "item", j.ItemID, "err", err)
		}
	default:
		q.Log.Warn("metadata fetch failed, will retry", "item", j.ItemID, "attempt", j.Attempts+1, "err", err)
		if err := q.Store.Fail(ctx, j, err); err != nil {
			q.Log.Error("metadata queue: recording a failure failed", "item", j.ItemID, "err", err)
		}
	}
}

func (q *Queue) sleep(ctx context.Context) {
	t := time.NewTimer(q.Idle)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-q.wake:
	case <-t.C:
	}
}
