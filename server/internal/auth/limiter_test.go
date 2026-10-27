package auth

import (
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newClock() *fakeClock { return &fakeClock{t: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)} }

func TestLoginBackoff(t *testing.T) {
	clock := newClock()
	b := NewBackoff(LoginBackoff, clock.now)
	for i := range 4 {
		b.Fail("user:bob")
		if w := b.Wait("user:bob"); w != 0 {
			t.Fatalf("after %d failures: wait %v, want 0", i+1, w)
		}
	}
	b.Fail("user:bob")
	if w := b.Wait("user:bob"); w != time.Second {
		t.Fatalf("after 5 failures: wait %v, want 1s", w)
	}
	want := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}
	for _, d := range want {
		clock.advance(time.Minute)
		b.Fail("user:bob")
		if w := b.Wait("user:bob"); w != d {
			t.Fatalf("wait %v, want %v", w, d)
		}
	}
	for range 20 {
		b.Fail("user:bob")
	}
	if w := b.Wait("user:bob"); w != 15*time.Minute {
		t.Errorf("wait %v, want the 15 min cap", w)
	}
	if w := b.Wait("user:alice"); w != 0 {
		t.Errorf("other key blocked: %v", w)
	}
	b.Reset("user:bob")
	if w := b.Wait("user:bob"); w != 0 {
		t.Errorf("after reset: %v", w)
	}
}

func TestBackoffForgets(t *testing.T) {
	clock := newClock()
	b := NewBackoff(LoginBackoff, clock.now)
	for range 4 {
		b.Fail("k")
	}
	clock.advance(2 * time.Hour)
	b.Fail("k")
	if w := b.Wait("k"); w != 0 {
		t.Errorf("old failures still counted: wait %v", w)
	}
}

func TestPINBackoffIsFixed(t *testing.T) {
	clock := newClock()
	b := NewBackoff(PINBackoff, clock.now)
	for range 5 {
		b.Fail("pin")
	}
	if w := b.Wait("pin"); w != 5*time.Minute {
		t.Fatalf("wait %v, want 5m", w)
	}
	clock.advance(5*time.Minute + time.Second)
	if w := b.Wait("pin"); w != 0 {
		t.Errorf("still locked after 5 min: %v", w)
	}
}

func TestSweep(t *testing.T) {
	clock := newClock()
	b := NewBackoff(LoginBackoff, clock.now)
	b.Fail("old")
	clock.advance(30 * time.Minute)
	b.Fail("recent")
	clock.advance(31 * time.Minute)
	b.Sweep()
	if _, ok := b.keys["old"]; ok {
		t.Error("old key not swept")
	}
	if _, ok := b.keys["recent"]; !ok {
		t.Error("recent key swept")
	}
}
