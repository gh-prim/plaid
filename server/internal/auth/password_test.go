package auth

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testParams keeps argon2id cheap so -race runs stay fast.
var testParams = Argon2Params{MemoryKiB: 64, Time: 1, Threads: 1}

func TestHashAndVerify(t *testing.T) {
	ctx := context.Background()
	h := NewHasher(testParams)
	enc, err := h.Hash(ctx, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "$argon2id$v=19$m=64,t=1,p=1$") {
		t.Errorf("encoded = %q, want params in the hash", enc)
	}
	if ok, err := h.Verify(ctx, "correct horse battery staple", enc); err != nil || !ok {
		t.Errorf("right password: ok=%v err=%v", ok, err)
	}
	if ok, err := h.Verify(ctx, "correct horse battery stapl", enc); err != nil || ok {
		t.Errorf("wrong password: ok=%v err=%v", ok, err)
	}
	other, _ := h.Hash(ctx, "correct horse battery staple")
	if other == enc {
		t.Error("two hashes of the same password are equal: salt missing")
	}
}

func TestVerifyUsesParamsFromTheHash(t *testing.T) {
	ctx := context.Background()
	old := NewHasher(Argon2Params{MemoryKiB: 32, Time: 1, Threads: 1})
	enc, _ := old.Hash(ctx, "a long password")
	stronger := NewHasher(testParams)
	if ok, err := stronger.Verify(ctx, "a long password", enc); err != nil || !ok {
		t.Errorf("old hash with new hasher: ok=%v err=%v", ok, err)
	}
	if !stronger.NeedsRehash(enc) {
		t.Error("NeedsRehash = false for weaker params")
	}
	if old.NeedsRehash(enc) {
		t.Error("NeedsRehash = true for the same params")
	}
}

func TestVerifyMalformed(t *testing.T) {
	h := NewHasher(testParams)
	for _, enc := range []string{"", "plain", "$argon2i$v=19$m=64,t=1,p=1$c2FsdA$a2V5", "$argon2id$v=18$m=64,t=1,p=1$c2FsdA$a2V5", "$argon2id$v=19$m=x$c2FsdA$a2V5"} {
		if _, err := h.Verify(context.Background(), "pw", enc); err == nil {
			t.Errorf("%q: no error", enc)
		}
	}
}

func TestVerifyDummy(t *testing.T) {
	if err := NewHasher(testParams).VerifyDummy(context.Background(), "whatever"); err != nil {
		t.Error(err)
	}
}

func TestAtMostTwoHashesAtOnce(t *testing.T) {
	h := NewHasher(Argon2Params{MemoryKiB: 4 * 1024, Time: 3, Threads: 1})
	var running, peak atomic.Int32
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			// Track concurrency from inside the semaphore.
			h.sem <- struct{}{}
			n := running.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			running.Add(-1)
			<-h.sem
		})
	}
	wg.Wait()
	if peak.Load() > maxConcurrentHashes {
		t.Errorf("peak concurrency = %d, want ≤ %d", peak.Load(), maxConcurrentHashes)
	}
}

func TestHashRespectsContext(t *testing.T) {
	h := NewHasher(testParams)
	h.sem <- struct{}{}
	h.sem <- struct{}{} // both slots busy
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.Hash(ctx, "a long password"); err == nil {
		t.Error("Hash with a cancelled context and no free slot: no error")
	}
}

func TestPasswordProblem(t *testing.T) {
	for pw, want := range map[string]string{
		"short":                        "too_short",
		"sept777":                      "too_short",
		"éléphant":                     "", // 8 characters, more bytes
		"Password1":                    "too_common",
		"AZERTYUIOP":                   "too_common",
		"a reasonably long passphrase": "",
		strings.Repeat("x", 128):       "",
		strings.Repeat("x", 129):       "too_long",
	} {
		if got := PasswordProblem(pw); got != want {
			t.Errorf("PasswordProblem(%q) = %q, want %q", pw, got, want)
		}
	}
}
