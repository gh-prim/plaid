package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLogKeepsFlushWorking(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var flushErr error
	h := withAccessLog(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "data: hello\n\n")
		flushErr = http.NewResponseController(w).Flush()
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil))
	if flushErr != nil {
		t.Fatalf("Flush through the access log: %v", flushErr)
	}
	if !rec.Flushed {
		t.Error("the underlying writer was not flushed")
	}
}

// TestAccessLogHidesPlaybackTokens: a stream token is a credential for as
// long as its session lives, so it never reaches the logs (F006 R25).
func TestAccessLogHidesPlaybackTokens(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	h := withAccessLog(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for _, p := range []string{"/api/v1/stream/s3cr3t-T0ken/file", "/api/v1/stream/s3cr3t-T0ken"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, p, nil))
	}
	if strings.Contains(buf.String(), "s3cr3t") {
		t.Errorf("token in the access log:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "path=/api/v1/stream/…/file") {
		t.Errorf("redacted path missing:\n%s", buf.String())
	}
	if got := redactPath("/api/v1/libraries"); got != "/api/v1/libraries" {
		t.Errorf("other paths changed: %s", got)
	}
}
