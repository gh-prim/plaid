package handlers

import "time"

// SetSSETiming shortens the server-sent events intervals for a test.
func SetSSETiming(progress, ping time.Duration) (restore func()) {
	p, q := sseProgressEvery, ssePingEvery
	sseProgressEvery, ssePingEvery = progress, ping
	return func() { sseProgressEvery, ssePingEvery = p, q }
}
