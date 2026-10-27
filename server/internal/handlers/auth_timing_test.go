//go:build !race

package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/auth"
)

// TestSignInTimingDoesNotRevealUsernames measures with the real argon2id
// cost (F003 acceptance: mean difference under 50 ms). Excluded from -race,
// where timings are meaningless; the hash-count test is the structural guard.
func TestSignInTimingDoesNotRevealUsernames(t *testing.T) {
	if testing.Short() {
		t.Skip("slow: real argon2id parameters")
	}
	e := newEnvWithParams(t, auth.DefaultParams)
	e.setup()
	const samples = 20
	var wrong, unknown time.Duration
	for i := range samples {
		ip := fmt.Sprintf("10.0.%d.%d:1", i/200, i%200+1)
		start := time.Now()
		e.expect(e.do("POST", "/auth/login", map[string]any{"username": "ada", "password": "not it at all", "device": laptop}, from(ip)),
			http.StatusUnauthorized, "invalid_credentials")
		wrong += time.Since(start)

		start = time.Now()
		e.expect(e.do("POST", "/auth/login", map[string]any{"username": fmt.Sprintf("ghost%d", i), "password": "not it at all", "device": laptop}, from(ip)),
			http.StatusUnauthorized, "invalid_credentials")
		unknown += time.Since(start)

		if i%4 == 3 {
			e.login("ada", adminPassword) // resets the backoff before it kicks in
		}
	}
	diff := (wrong - unknown) / samples
	if diff < 0 {
		diff = -diff
	}
	t.Logf("mean sign-in: wrong password %v, unknown user %v", wrong/samples, unknown/samples)
	if diff > 50*time.Millisecond {
		t.Errorf("mean difference %v, want < 50ms", diff)
	}
}
