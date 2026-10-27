package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
