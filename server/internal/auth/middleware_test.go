package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeSessions knows one token.
type fakeSessions struct {
	token string
	p     Principal
	err   error // returned for any other token; ErrNoSession if nil
}

func (f fakeSessions) Lookup(_ context.Context, token string) (Principal, error) {
	if token == f.token {
		return f.p, nil
	}
	if f.err != nil {
		return Principal{}, f.err
	}
	return Principal{}, ErrNoSession
}

var member = Principal{UserID: "u1", SessionID: "s1", Role: RoleMember}

// echo records the Principal the protected handler saw.
type echo struct {
	called bool
	p      Principal
	ok     bool
}

func (e *echo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.called = true
	e.p, e.ok = FromContext(r.Context())
	w.WriteHeader(http.StatusTeapot)
}

func serve(t *testing.T, sessions SessionLookup, r *http.Request) (*httptest.ResponseRecorder, *echo) {
	t.Helper()
	next := &echo{}
	rec := httptest.NewRecorder()
	Authenticate(sessions, next).ServeHTTP(rec, r)
	return rec, next
}

func TestPublicRouteNeedsNoSession(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec, next := serve(t, NoSessions{}, httptest.NewRequest(method, "/api/v1/health", nil))
		if !next.called || rec.Code != http.StatusTeapot {
			t.Errorf("%s /health: status %d, called=%v, want it to reach the handler", method, rec.Code, next.called)
		}
		if next.ok {
			t.Errorf("public route got a Principal: %+v", next.p)
		}
	}
}

func TestWebAppIsNotGuarded(t *testing.T) {
	for _, path := range []string{"/", "/login", "/assets/app.js", "/apiary"} {
		rec, next := serve(t, NoSessions{}, httptest.NewRequest(http.MethodGet, path, nil))
		if !next.called {
			t.Errorf("%s: status %d, want the web app", path, rec.Code)
		}
	}
}

func TestProtectedRoutesNeedASession(t *testing.T) {
	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodPost, "/api/v1/health"}, // public only for GET
		{http.MethodGet, "/api/v1/me"},
		{http.MethodGet, "/api/v1/does-not-exist"},
		{http.MethodGet, "/api/v1/health/../me"},
		{http.MethodGet, "/api/v2/anything"},
		{http.MethodGet, "/api/"},
	} {
		rec, next := serve(t, NoSessions{}, httptest.NewRequest(tc.method, tc.path, nil))
		if next.called || rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"code":"unauthorized"`) {
			t.Errorf("%s %s: status %d body %q called=%v, want 401 unauthorized", tc.method, tc.path, rec.Code, rec.Body, next.called)
		}
	}
}

func TestValidTokenOrCookieCarriesPrincipal(t *testing.T) {
	sessions := fakeSessions{token: "good", p: member}
	bearer := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	bearer.Header.Set("Authorization", "Bearer good")
	lowercase := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	lowercase.Header.Set("Authorization", "bearer good")
	cookie := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	cookie.AddCookie(&http.Cookie{Name: CookieName, Value: "good"})

	for name, r := range map[string]*http.Request{"bearer": bearer, "lowercase scheme": lowercase, "cookie": cookie} {
		rec, next := serve(t, sessions, r)
		if !next.called || !next.ok || next.p != member {
			t.Errorf("%s: status %d, principal %+v (ok=%v), want %+v", name, rec.Code, next.p, next.ok, member)
		}
	}
}

func TestInvalidCredentialsAreRefused(t *testing.T) {
	sessions := fakeSessions{token: "good", p: member}
	wrong := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	wrong.Header.Set("Authorization", "Bearer bad")
	basic := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	basic.Header.Set("Authorization", "Basic good")
	// A bad bearer token doesn't fall back to a good cookie.
	mixed := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	mixed.Header.Set("Authorization", "Bearer bad")
	mixed.AddCookie(&http.Cookie{Name: CookieName, Value: "good"})
	otherCookie := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	otherCookie.AddCookie(&http.Cookie{Name: "session", Value: "good"})

	for name, r := range map[string]*http.Request{
		"wrong token": wrong, "basic scheme": basic, "bad bearer + good cookie": mixed, "other cookie": otherCookie,
	} {
		rec, next := serve(t, sessions, r)
		if next.called || rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d called=%v, want 401", name, rec.Code, next.called)
		}
	}
}

func TestLookupFailureIsRefused(t *testing.T) {
	sessions := fakeSessions{token: "good", p: member, err: errors.New("database is locked")}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	r.Header.Set("Authorization", "Bearer other")
	rec, next := serve(t, sessions, r)
	if next.called || rec.Code != http.StatusUnauthorized {
		t.Errorf("status %d called=%v, want 401 when the lookup fails", rec.Code, next.called)
	}
}

func TestRequireAdmin(t *testing.T) {
	ctx := context.Background()
	if err := RequireAdmin(ctx); !isCode(err, "unauthorized") {
		t.Errorf("no principal: %v, want unauthorized", err)
	}
	if err := RequireAdmin(WithPrincipal(ctx, member)); !isCode(err, "forbidden") {
		t.Errorf("member: %v, want forbidden", err)
	}
	if err := RequireAdmin(WithPrincipal(ctx, Principal{UserID: "u0", Role: RoleAdmin})); err != nil {
		t.Errorf("admin: %v, want nil", err)
	}
	if err := RequireAdmin(WithPrincipal(ctx, Principal{UserID: "u9", Role: "friend"})); !isCode(err, "forbidden") {
		t.Errorf("unknown role: %v, want forbidden", err)
	}
}

func isCode(err error, code string) bool {
	return err != nil && strings.HasPrefix(err.Error(), code+":")
}
