package handlers_test

import (
	"context"
	"testing"
)

// BenchmarkSessionLookup checks the F003 budget of 1 ms per session lookup
// without a cache: go test -bench SessionLookup ./internal/handlers/
func BenchmarkSessionLookup(b *testing.B) {
	e := newEnv(b)
	admin := e.setup()
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		if _, err := e.acct.Lookup(ctx, admin.Token); err != nil {
			b.Fatal(err)
		}
	}
}
