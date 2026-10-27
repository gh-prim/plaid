package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/handlers"
)

type okPinger struct{}

func (okPinger) PingContext(context.Context) error { return nil }

var builtWeb = fstest.MapFS{
	"index.html":      {Data: []byte("<!doctype html><title>PLAID</title>")},
	"assets/app-1.js": {Data: []byte("console.log('plaid')")},
	"favicon.svg":     {Data: []byte("<svg/>")},
}

func newTestServer(web fstest.MapFS) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(handlers.New(okPinger{}), web, logger, Options{})
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealthRoute(t *testing.T) {
	rec := get(t, newTestServer(builtWeb), "/api/v1/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"status", "version", "commit", "uptimeSeconds", "database"} {
		if _, ok := body[k]; !ok {
			t.Errorf("missing %q in %s", k, rec.Body)
		}
	}
}

func TestUnknownAPIRouteIsJSON404(t *testing.T) {
	rec := get(t, newTestServer(builtWeb), "/api/v1/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}
	var body struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "not_found" {
		t.Errorf("error.code = %q, want not_found", body.Error.Code)
	}
}

func TestSPAFallback(t *testing.T) {
	h := newTestServer(builtWeb)
	for _, path := range []string{"/", "/library/123", "/movies/42/details"} {
		rec := get(t, h, path)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<title>PLAID</title>") {
			t.Errorf("%s: status %d, body %q, want index.html", path, rec.Code, rec.Body)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s: Content-Type = %q", path, ct)
		}
	}
}

func TestStaticAssets(t *testing.T) {
	rec := get(t, newTestServer(builtWeb), "/assets/app-1.js")
	if rec.Code != http.StatusOK || rec.Body.String() != "console.log('plaid')" {
		t.Fatalf("status %d body %q", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q, want immutable for hashed assets", cc)
	}
}

func TestWebNotBuilt(t *testing.T) {
	rec := get(t, newTestServer(fstest.MapFS{}), "/")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "make build") {
		t.Errorf("status %d body %q, want a 'web not built' page", rec.Code, rec.Body)
	}
}

func TestPanicIsRecovered(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := withRecover(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := get(t, h, "/api/v1/anything")
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "internal_error") {
		t.Errorf("status %d body %q", rec.Code, rec.Body)
	}
}

// erroringHandlers returns err from every operation.
type erroringHandlers struct{ err error }

func (e erroringHandlers) GetHealth(context.Context, api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return nil, e.err
}

func TestHandlerErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		status   int
		code     string
		wantLogs bool
	}{
		{"api error written as is", apierr.Forbidden(), http.StatusForbidden, "forbidden", false},
		{"wrapped api error", fmt.Errorf("check: %w", apierr.NotFound("no such thing")), http.StatusNotFound, "not_found", false},
		{"unexpected error is a logged 500", errors.New("disk on fire"), http.StatusInternalServerError, "internal_error", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			rec := get(t, New(erroringHandlers{tc.err}, builtWeb, logger, Options{}), "/api/v1/health")
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), `"code":"`+tc.code+`"`) {
				t.Errorf("status %d body %q, want %d %s", rec.Code, rec.Body, tc.status, tc.code)
			}
			if strings.Contains(rec.Body.String(), "disk on fire") {
				t.Errorf("internal error leaked to the client: %q", rec.Body)
			}
			if got := strings.Contains(logs.String(), "handler error"); got != tc.wantLogs {
				t.Errorf("logged = %v, want %v (%q)", got, tc.wantLogs, logs.String())
			}
		})
	}
}
