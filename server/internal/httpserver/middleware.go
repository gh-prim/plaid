package httpserver

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Unwrap lets http.ResponseController reach the real writer (Flush for
// server-sent events, deadlines) through the recorder.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// withAccessLog logs one line per request.
func withAccessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Info("request",
			"method", r.Method,
			"path", redactPath(r.URL.Path),
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// withRecover turns a panic into a 500 instead of killing the connection.
func withRecover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				logger.Error("panic", "path", redactPath(r.URL.Path), "panic", v)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// streamPrefix is where playback streams carry their token (F006 R25).
const streamPrefix = "/api/v1/stream/"

// redactPath hides the playback token of stream paths: it's a credential
// for as long as the session lives.
func redactPath(p string) string {
	rest, ok := strings.CutPrefix(p, streamPrefix)
	if !ok {
		return p
	}
	if _, after, found := strings.Cut(rest, "/"); found {
		return streamPrefix + "…/" + after
	}
	return streamPrefix + "…"
}
