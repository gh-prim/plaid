package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/handlers"
)

const app = "https://getplaid.direct"

func newCORSServer() http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(handlers.New(handlers.Deps{DB: okPinger{}}), builtWeb, logger,
		Options{Sessions: oneSession{}, AppOrigins: []string{app}})
}

func TestCORSAllowsTheHostedApp(t *testing.T) {
	h := newCORSServer()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	r.Header.Set("Origin", app)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Origin") != app || rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("status %d, headers %v", rec.Code, rec.Header())
	}
}

func TestCORSIgnoresOtherOrigins(t *testing.T) {
	h := newCORSServer()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	r.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Allow-Origin %q for another origin", got)
	}
}

func TestCORSPreflightAllowsPrivateNetwork(t *testing.T) {
	h := newCORSServer()
	r := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	r.Header.Set("Origin", app)
	r.Header.Set("Access-Control-Request-Method", "POST")
	r.Header.Set("Access-Control-Request-Headers", "content-type, x-plaid-client")
	r.Header.Set("Access-Control-Request-Private-Network", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	hd := rec.Header()
	if rec.Code != http.StatusNoContent || hd.Get("Access-Control-Allow-Private-Network") != "true" ||
		hd.Get("Access-Control-Allow-Headers") != "content-type, x-plaid-client" || hd.Get("Access-Control-Allow-Origin") != app {
		t.Fatalf("status %d, headers %v", rec.Code, hd)
	}
}

// A cookie request from the hosted app passes the CSRF check; the same
// request from an unknown origin doesn't.
func TestCSRFAcceptsTheHostedApp(t *testing.T) {
	h := newCORSServer()
	for origin, want := range map[string]int{app: http.StatusNotFound, "https://evil.example": http.StatusForbidden} {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/nope", nil)
		r.Header.Set("Origin", origin)
		r.Header.Set(auth.ClientHeader, "web")
		r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "valid"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != want {
			t.Errorf("origin %s: status %d, want %d (%s)", origin, rec.Code, want, rec.Body)
		}
	}
}
