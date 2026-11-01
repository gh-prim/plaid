package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	l := New(2, time.Hour)
	t0 := time.Unix(1_800_000_000, 0)
	if !l.Allow("a", t0) || !l.Allow("a", t0.Add(time.Minute)) {
		t.Fatal("first two events refused")
	}
	if l.Allow("a", t0.Add(2*time.Minute)) {
		t.Fatal("third event allowed")
	}
	if !l.Allow("b", t0) {
		t.Fatal("keys are not independent")
	}
	if !l.Allow("a", t0.Add(time.Hour+time.Second)) {
		t.Fatal("first event did not slide out of the window")
	}
	if l.Allow("a", t0.Add(time.Hour+2*time.Second)) {
		t.Fatal("second event slid out too early")
	}
}
