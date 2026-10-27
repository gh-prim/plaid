package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/clientinfo"
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
	"GET /server",
	"GET /setup",
	"POST /setup/admin",
	"POST /auth/login",
	"POST /pairing",
	"GET /pairing/{pairingId}",
	// Playback streams authenticate by the token in their path (F006 R25).
	"GET /stream/{token}/file",
	"HEAD /stream/{token}/file",
}

// passwordChangeRoutes stay usable while an account has a temporary
// password; everything else is 403 password_change_required (F003-R15).
var passwordChangeRoutes = []string{
	"GET /me",
	"POST /me/password",
	"POST /auth/logout",
}

// ClientHeader marks requests from the web app (F003-R10).
const ClientHeader = "X-Plaid-Client"

// PublicRoutes returns a copy of the public route list.
func PublicRoutes() []string { return append([]string(nil), publicRoutes...) }

// Authenticate guards every /api/ path: public routes pass through, anything
// else needs a valid bearer token or session cookie, or gets 401. Other paths
// (the web app) are never guarded. It also enforces CSRF rules on cookie
// sessions (F003-R10) and the temporary-password gate (F003-R15).
func Authenticate(sessions SessionLookup, next http.Handler) http.Handler {
	public := routeMatcher(publicRoutes)
	passwordChange := routeMatcher(passwordChangeRoutes)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), webKey{}, r.Header.Get(ClientHeader) == "web")
		jar := &cookieJar{}
		ctx = context.WithValue(ctx, jarKey{}, jar)
		w = &jarWriter{ResponseWriter: w, jar: jar}
		r = r.WithContext(ctx)

		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if matches(public, r) {
			// A web client asking for a session cookie must come from our
			// own origin, or another site could sign a browser in.
			if !isSafeMethod(r.Method) && IsWebClient(ctx) && !sameOrigin(r) {
				apierr.Write(w, apierr.CSRF())
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		token, viaCookie := tokenFrom(r)
		if token == "" {
			apierr.Write(w, apierr.Unauthorized())
			return
		}
		p, err := sessions.Lookup(ctx, token)
		if err != nil {
			// Lookup errors other than ErrNoSession (database down) are
			// still a refusal: never let a request through unauthenticated.
			apierr.Write(w, apierr.Unauthorized())
			return
		}
		if viaCookie && !isSafeMethod(r.Method) && (!IsWebClient(ctx) || !sameOrigin(r)) {
			apierr.Write(w, apierr.CSRF())
			return
		}
		if p.MustChangePassword && !matches(passwordChange, r) {
			apierr.Write(w, apierr.PasswordChangeRequired())
			return
		}
		ctx = context.WithValue(ctx, viaCookieKey{}, viaCookie)
		next.ServeHTTP(w, r.WithContext(WithPrincipal(ctx, p)))
	})
}

func routeMatcher(routes []string) *http.ServeMux {
	mux := http.NewServeMux()
	for _, route := range routes {
		method, path, _ := strings.Cut(route, " ")
		mux.Handle(method+" /api/v1"+path, http.NotFoundHandler())
	}
	return mux
}

func matches(mux *http.ServeMux, r *http.Request) bool {
	_, pattern := mux.Handler(r)
	return pattern != ""
}

func isSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// sameOrigin reports whether the Origin header names this server, as the
// client reached it: same scheme and same Host.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	scheme := "http"
	if clientinfo.FromContext(r.Context()).HTTPS {
		scheme = "https"
	}
	return u.Scheme == scheme && strings.EqualFold(u.Host, r.Host)
}

type (
	webKey       struct{}
	viaCookieKey struct{}
)

// IsWebClient reports whether the request comes from the web app
// (X-Plaid-Client: web), which gets sessions as a cookie instead of a token.
func IsWebClient(ctx context.Context) bool {
	web, _ := ctx.Value(webKey{}).(bool)
	return web
}

// ViaCookie reports whether the request was authenticated by the session cookie.
func ViaCookie(ctx context.Context) bool {
	v, _ := ctx.Value(viaCookieKey{}).(bool)
	return v
}

// tokenFrom returns the bearer token, else the session cookie, else "".
func tokenFrom(r *http.Request) (token string, viaCookie bool) {
	if h := r.Header.Get("Authorization"); h != "" {
		scheme, token, ok := strings.Cut(h, " ")
		if ok && strings.EqualFold(scheme, "Bearer") {
			return strings.TrimSpace(token), false
		}
		return "", false
	}
	if c, err := r.Cookie(CookieName); err == nil {
		return c.Value, true
	}
	return "", false
}
