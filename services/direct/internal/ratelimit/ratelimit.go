// Package ratelimit is an in-memory sliding-window limiter: at most N events
// per key within a window. State is lost on restart, which is fine for abuse
// limits that only need to hold for an hour.
package ratelimit

import (
	"sync"
	"time"
)

// maxKeys bounds memory: past it, keys with no recent event are dropped.
const maxKeys = 100_000

// Limiter allows Max events per key per Window.
type Limiter struct {
	Max    int
	Window time.Duration

	mu     sync.Mutex
	events map[string][]time.Time
}

// New returns a limiter allowing max events per key per window.
func New(maxEvents int, window time.Duration) *Limiter {
	return &Limiter{Max: maxEvents, Window: window, events: map[string][]time.Time{}}
}

// Allow records an event for key at now and reports whether it is within the
// limit. A refused event is not recorded, so retrying doesn't extend the wait.
func (l *Limiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-l.Window)
	ev := trim(l.events[key], cutoff)
	if len(ev) >= l.Max {
		l.events[key] = ev
		return false
	}
	if len(l.events) >= maxKeys {
		l.prune(cutoff)
	}
	l.events[key] = append(ev, now)
	return true
}

func (l *Limiter) prune(cutoff time.Time) {
	for k, ev := range l.events {
		if ev = trim(ev, cutoff); len(ev) == 0 {
			delete(l.events, k)
		} else {
			l.events[k] = ev
		}
	}
}

// trim drops events at or before cutoff; events are in time order.
func trim(ev []time.Time, cutoff time.Time) []time.Time {
	i := 0
	for i < len(ev) && !ev[i].After(cutoff) {
		i++
	}
	return ev[i:]
}
