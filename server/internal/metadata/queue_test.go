package metadata

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestQueueRunsEachJobOnce(t *testing.T) {
	e := newTestDB(t)
	ctx := context.Background()
	var ids []string
	for range 12 {
		ids = append(ids, e.item("movie", ""))
	}
	var (
		mu    sync.Mutex
		count = map[string]int{}
		done  = make(chan struct{})
	)
	q := &Queue{Store: e.store, Workers: 4, Idle: 20 * time.Millisecond, Process: func(_ context.Context, j Job) error {
		mu.Lock()
		count[j.ItemID]++
		n := len(count)
		mu.Unlock()
		e.exec(`UPDATE item_metadata SET state = 'matched', next_attempt_at = NULL, refresh_at = '9999' WHERE item_id = ?`, j.ItemID)
		if n == len(ids) {
			select {
			case <-done:
			default:
				close(done)
			}
		}
		return nil
	}}
	q.Start(ctx)
	defer q.Close()
	_ = e.store.AddPending(ctx, ids)
	q.Wake()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("queue didn't process every item")
	}
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	for id, n := range count {
		if n != 1 {
			t.Errorf("%s processed %d times", id, n)
		}
	}
}

func TestQueuePausedAndFailedJobs(t *testing.T) {
	e := newTestDB(t)
	ctx := context.Background()
	paused, failing := e.item("movie", ""), e.item("movie", "")
	calls := make(chan string, 4)
	q := &Queue{Store: e.store, Workers: 1, Idle: 10 * time.Millisecond, Process: func(_ context.Context, j Job) error {
		calls <- j.ItemID
		if j.ItemID == paused {
			return ErrPaused
		}
		return errors.New("boom")
	}}
	_ = e.store.AddPending(ctx, []string{paused, failing})
	q.Start(ctx)
	got := map[string]bool{<-calls: true, <-calls: true}
	q.Close()
	if !got[paused] || !got[failing] {
		t.Fatalf("calls = %v", got)
	}
	if a := e.col(paused, "attempts").String; a != "0" {
		t.Errorf("paused job attempts = %s, want 0", a)
	}
	if a := e.col(failing, "attempts").String; a != "1" {
		t.Errorf("failed job attempts = %s, want 1", a)
	}
	select {
	case id := <-calls:
		t.Errorf("%s ran again before its retry time", id)
	default:
	}
}
