package metadata

import (
	"context"
	"testing"
	"time"
)

// Items paused without a key run as soon as a key is there, not after
// their 10-minute pause.
func TestUnpause(t *testing.T) {
	e := newTestDB(t)
	ctx := context.Background()
	id := e.item("movie", "")
	_ = e.store.AddPending(ctx, []string{id})
	j, _ := e.claim()
	if err := e.store.Release(ctx, j, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, ok := e.claim(); ok {
		t.Fatal("a paused job was claimable")
	}
	n, err := e.store.Unpause(ctx)
	if err != nil || n != 1 {
		t.Fatalf("unpaused %d, %v; want 1", n, err)
	}
	if _, ok := e.claim(); !ok {
		t.Error("the job wasn't claimable after Unpause")
	}
}
