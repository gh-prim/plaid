package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gh-prim/plaid/server/internal/apierr"
)

// CookieName is the web app's session cookie (F003-R9).
const CookieName = "plaid_session"

// ErrNoSession is returned by a SessionLookup for an unknown, expired or
// revoked token.
var ErrNoSession = errors.New("no such session")

// SessionLookup resolves a session token into its Principal.
type SessionLookup interface {
	Lookup(ctx context.Context, token string) (Principal, error)
}

// NoSessions knows no session: every protected route is refused. It stands in
// until accounts exist.
type NoSessions struct{}

// Lookup always fails with ErrNoSession.
func (NoSessions) Lookup(context.Context, string) (Principal, error) {
	return Principal{}, ErrNoSession
}

// publicRoutes are the only API operations reachable without a session
// (F003-R12), as "METHOD /path" relative to /api/v1. They must match the
// operations with `security: []` in api/openapi.yaml (public_routes_test.go).
var publicRoutes = []string{
	"GET /health",
}

// PublicRoutes returns a copy of the public route list.
func PublicRoutes() []string { return append([]string(nil), publicRoutes...) }

// Authenticate guards every /api/ path: public routes pass through, anything
// else needs a valid bearer token or session cookie, or gets 401. Other paths
// (the web app) are never guarded.
func Authenticate(sessions SessionLookup, next http.Handler) http.Handler {
	public := http.NewServeMux()
	for _, route := range publicRoutes {
		method, path, _ := strings.Cut(route, " ")
		public.Handle(method+" /api/v1"+path, next)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if _, pattern := public.Handler(r); pattern != "" {
			next.ServeHTTP(w, r)
			return
		}
		token := tokenFrom(r)
		if token == "" {
			apierr.Write(w, apierr.Unauthorized())
			return
		}
		p, err := sessions.Lookup(r.Context(), token)
		if err != nil {
			// Lookup errors other than ErrNoSession (database down) are
			// still a refusal: never let a request through unauthenticated.
			apierr.Write(w, apierr.Unauthorized())
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
	})
}

// tokenFrom returns the bearer token, else the session cookie, else "".
func tokenFrom(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		scheme, token, ok := strings.Cut(h, " ")
		if ok && strings.EqualFold(scheme, "Bearer") {
			return strings.TrimSpace(token)
		}
		return ""
	}
	if c, err := r.Cookie(CookieName); err == nil {
		return c.Value
	}
	return ""
}
