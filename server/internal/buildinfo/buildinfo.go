// Package buildinfo exposes build metadata injected at link time:
//
//	go build -ldflags "-X github.com/gh-prim/plaid/server/internal/buildinfo.Version=0.1.0 \
//	                   -X github.com/gh-prim/plaid/server/internal/buildinfo.Commit=abc1234"
package buildinfo

import "time"

var (
	Version = "0.1.0-dev"
	Commit  = "unknown"
)

var startedAt = time.Now()

// Uptime returns how long the process has been running.
func Uptime() time.Duration {
	return time.Since(startedAt)
}
