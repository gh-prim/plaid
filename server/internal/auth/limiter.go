package auth

import (
	"sync"
	"time"
)

// BackoffPolicy describes when a key gets blocked after repeated failures.
type BackoffPolicy struct {
	// Threshold is the number of failures before the first delay.
	Threshold int
	// Base is the first delay; each further failure doubles it, up to Max.
	Base, Max time.Duration
	// Forget resets a key's count when its last failure is older than this.
	Forget time.Duration
}

// Policies used by F003.
var (
	// LoginBackoff: 1 s after 5 failures, doubling, capped at 15 min (F003-R7).
	LoginBackoff = BackoffPolicy{Threshold: 5, Base: time.Second, Max: 15 * time.Minute, Forget: time.Hour}
	// PINBackoff: 5 wrong PINs lock the profile for 5 min (F003-R17).
	PINBackoff = BackoffPolicy{Threshold: 5, Base: 5 * time.Minute, Max: 5 * time.Minute, Forget: time.Hour}
	// PairingCreateBackoff: 10 new pairing codes per 10 min per IP (F003-R20).
	PairingCreateBackoff = BackoffPolicy{Threshold: 10, Base: 10 * time.Minute, Max: 10 * time.Minute, Forget: 10 * time.Minute}
)

// Backoff tracks failures per key in memory. Counts reset on success, after
// Forget, or on restart.
type Backoff struct {
	policy BackoffPolicy
	now    func() time.Time

	mu   sync.Mutex
	keys map[string]*backoffEntry
}

type backoffEntry struct {
	failures int
	last     time.Time
	until    time.Time
}

// NewBackoff returns a Backoff with policy; now is the clock (time.Now in production).
func NewBackoff(policy BackoffPolicy, now func() time.Time) *Backoff {
	return &Backoff{policy: policy, now: now, keys: map[string]*backoffEntry{}}
}

// Wait returns how long key must wait before its next attempt; 0 means go.
func (b *Backoff) Wait(key string) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.entry(key, false)
	if e == nil {
		return 0
	}
	if d := e.until.Sub(b.now()); d > 0 {
		return d
	}
	return 0
}

// Fail records a failure for key.
func (b *Backoff) Fail(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.entry(key, true)
	now := b.now()
	e.failures++
	e.last = now
	if over := e.failures - b.policy.Threshold; over >= 0 {
		delay := b.policy.Base
		for i := 0; i < over && delay < b.policy.Max; i++ {
			delay *= 2
		}
		e.until = now.Add(min(delay, b.policy.Max))
	}
}

// Reset forgets key, after a success.
func (b *Backoff) Reset(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.keys, key)
}

// Sweep drops keys that are no longer blocked and were forgotten. The janitor
// calls it so the map doesn't grow forever.
func (b *Backoff) Sweep() {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	for k, e := range b.keys {
		if now.After(e.until) && now.Sub(e.last) > b.policy.Forget {
			delete(b.keys, k)
		}
	}
}

// entry returns key's entry, resetting it when forgotten. Caller holds mu.
func (b *Backoff) entry(key string, create bool) *backoffEntry {
	e := b.keys[key]
	if e != nil && b.now().Sub(e.last) > b.policy.Forget && !b.now().Before(e.until) {
		e = nil
		delete(b.keys, key)
	}
	if e == nil && create {
		e = &backoffEntry{}
		b.keys[key] = e
	}
	return e
}
