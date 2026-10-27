package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/db"
	"github.com/gh-prim/plaid/server/internal/handlers"
	"github.com/gh-prim/plaid/server/internal/httpserver"
)

// These tests drive the acceptance criteria of F003 through the real HTTP
// stack (middleware, generated router, handlers, SQLite).

var cheapParams = auth.Argon2Params{MemoryKiB: 64, Time: 1, Threads: 1}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// countingHasher counts argon2id derivations (hash or verify).
type countingHasher struct {
	*auth.Hasher
	n atomic.Int32
}

func (c *countingHasher) Hash(ctx context.Context, s string) (string, error) {
	c.n.Add(1)
	return c.Hasher.Hash(ctx, s)
}

func (c *countingHasher) Verify(ctx context.Context, s, enc string) (bool, error) {
	c.n.Add(1)
	return c.Hasher.Verify(ctx, s, enc)
}

func (c *countingHasher) VerifyDummy(ctx context.Context, s string) error {
	c.n.Add(1)
	return c.Hasher.VerifyDummy(ctx, s)
}

type env struct {
	t      testing.TB
	h      http.Handler
	acct   *accounts.Service
	db     *sql.DB
	clock  *clock
	hasher *countingHasher
	logs   *syncBuffer
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func newEnv(t testing.TB) *env {
	return newEnvWithParams(t, cheapParams)
}

func newEnvWithParams(t testing.TB, params auth.Argon2Params) *env {
	t.Helper()
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	e := &env{t: t, db: database, logs: &syncBuffer{}}
	e.clock = &clock{t: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
	e.hasher = &countingHasher{Hasher: auth.NewHasher(params)}
	logger := slog.New(slog.NewTextHandler(e.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	e.acct = accounts.New(database, accounts.Options{Hasher: e.hasher, Now: e.clock.now, Logger: logger})
	h := handlers.New(handlers.Deps{DB: database, Accounts: e.acct, ServerName: "test-server"})
	e.h = httpserver.New(h, fstest.MapFS{}, logger, httpserver.Options{Sessions: e.acct})
	return e
}

type reqOpt func(*http.Request)

func bearer(token string) reqOpt {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

// web makes a same-origin request from the web app.
func web() reqOpt {
	return func(r *http.Request) {
		r.Header.Set("X-Plaid-Client", "web")
		r.Header.Set("Origin", "http://"+r.Host)
	}
}

func cookie(value string) reqOpt {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: value}) }
}

func from(addr string) reqOpt { return func(r *http.Request) { r.RemoteAddr = addr } }

func header(k, v string) reqOpt { return func(r *http.Request) { r.Header.Set(k, v) } }

type resp struct {
	*httptest.ResponseRecorder
	t testing.TB
}

func (r resp) json(v any) {
	r.t.Helper()
	if err := json.Unmarshal(r.Body.Bytes(), v); err != nil {
		r.t.Fatalf("decode %q: %v", r.Body, err)
	}
}

func (r resp) code() string {
	var b struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(r.Body.Bytes(), &b)
	return b.Error.Code
}

func (e *env) do(method, path string, body any, opts ...reqOpt) resp {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, "/api/v1"+path, rd)
	r.RemoteAddr = "127.0.0.1:50000"
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	for _, o := range opts {
		o(r)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, r)
	return resp{rec, e.t}
}

func (e *env) expect(r resp, status int, code string) {
	e.t.Helper()
	if r.Code != status || (code != "" && r.code() != code) {
		e.t.Fatalf("status %d code %q body %s, want %d %s", r.Code, r.code(), r.Body, status, code)
	}
}

var tv = map[string]any{"name": "Living room TV", "type": "tvos"}
var laptop = map[string]any{"name": "Laptop", "type": "web"}

type authResult struct {
	Token   string              `json:"token"`
	Session struct{ ID string } `json:"session"`
	User    struct {
		ID                 string `json:"id"`
		Role               string `json:"role"`
		MustChangePassword bool   `json:"mustChangePassword"`
	} `json:"user"`
	Profiles []struct {
		ID     string `json:"id"`
		HasPin bool   `json:"hasPin"`
	} `json:"profiles"`
	ActiveProfile *struct{ ID string } `json:"activeProfile"`
}

const adminPassword = "plaid and chill forever"

// setup creates the admin and returns its bearer token.
func (e *env) setup() authResult {
	e.t.Helper()
	r := e.do("POST", "/setup/admin", map[string]any{
		"displayName": "Ada", "username": "Ada", "password": adminPassword, "locale": "fr",
	})
	e.expect(r, http.StatusCreated, "")
	var res authResult
	r.json(&res)
	return res
}

func (e *env) login(username, password string) authResult {
	e.t.Helper()
	r := e.do("POST", "/auth/login", map[string]any{"username": username, "password": password, "device": laptop})
	e.expect(r, http.StatusOK, "")
	var res authResult
	r.json(&res)
	return res
}

// member creates a member with a temporary password, changes it, and signs in.
func (e *env) member(admin, username string) authResult {
	e.t.Helper()
	e.expect(e.do("POST", "/users", map[string]any{
		"displayName": username, "username": username, "temporaryPassword": "temporary password", "role": "member",
	}, bearer(admin)), http.StatusCreated, "")
	tmp := e.login(username, "temporary password")
	e.expect(e.do("POST", "/me/password", map[string]any{
		"currentPassword": "temporary password", "newPassword": username + " has a long password",
	}, bearer(tmp.Token)), http.StatusNoContent, "")
	return tmp
}

func TestFirstRunSetup(t *testing.T) {
	e := newEnv(t)
	var state struct{ SetupComplete, RequiresSetupCode bool }
	e.do("GET", "/setup", nil).json(&state)
	if state.SetupComplete || state.RequiresSetupCode {
		t.Fatalf("fresh server from localhost: %+v", state)
	}
	var info struct {
		Name          string
		SetupComplete bool
	}
	e.do("GET", "/server", nil).json(&info)
	if info.Name != "test-server" || info.SetupComplete {
		t.Errorf("GET /server = %+v", info)
	}

	res := e.setup()
	if res.Token == "" || res.User.Role != "admin" || len(res.Profiles) != 1 || res.ActiveProfile == nil {
		t.Fatalf("setup result: %+v", res)
	}
	var me struct {
		User     struct{ Username, Role, Locale string }
		Profiles []struct{ Name string }
	}
	e.do("GET", "/me", nil, bearer(res.Token)).json(&me)
	if me.User.Role != "admin" || me.User.Username != "ada" || me.User.Locale != "fr" || len(me.Profiles) != 1 || me.Profiles[0].Name != "Ada" {
		t.Errorf("GET /me = %+v", me)
	}

	e.expect(e.do("POST", "/setup/admin", map[string]any{
		"displayName": "Eve", "username": "eve", "password": "another long password",
	}), http.StatusConflict, "setup_complete")
}

func TestConcurrentSetupOnlyOneWins(t *testing.T) {
	e := newEnv(t)
	var wg sync.WaitGroup
	var created, conflicts atomic.Int32
	for i := range 20 {
		wg.Go(func() {
			r := e.do("POST", "/setup/admin", map[string]any{
				"displayName": "Admin", "username": "admin" + string(rune('a'+i)), "password": adminPassword,
			})
			switch r.Code {
			case http.StatusCreated:
				created.Add(1)
			case http.StatusConflict:
				conflicts.Add(1)
			default:
				t.Errorf("unexpected %d %s", r.Code, r.Body)
			}
		})
	}
	wg.Wait()
	if created.Load() != 1 || conflicts.Load() != 19 {
		t.Errorf("created %d, conflicts %d; want 1 and 19", created.Load(), conflicts.Load())
	}
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil || n != 1 {
		t.Errorf("%d users in the database (%v)", n, err)
	}
}

func TestSetupFromOutsideNeedsTheCode(t *testing.T) {
	e := newEnv(t)
	code, err := e.acct.PrepareSetupCode(context.Background())
	if err != nil || len(code) != 8 {
		t.Fatalf("setup code %q (%v)", code, err)
	}
	body := map[string]any{"displayName": "Ada", "username": "ada", "password": adminPassword}

	var state struct{ RequiresSetupCode bool }
	e.do("GET", "/setup", nil, from("203.0.113.9:4000")).json(&state)
	if !state.RequiresSetupCode {
		t.Error("public client: requiresSetupCode = false")
	}
	e.expect(e.do("POST", "/setup/admin", body, from("203.0.113.9:4000")), http.StatusForbidden, "setup_code_required")
	// A local reverse proxy that isn't trusted doesn't make a request local.
	e.expect(e.do("POST", "/setup/admin", body, header("X-Forwarded-For", "203.0.113.9")), http.StatusForbidden, "setup_code_required")

	body["setupCode"] = strings.ToLower(code)
	e.expect(e.do("POST", "/setup/admin", body, from("203.0.113.9:4000")), http.StatusCreated, "")
}

func TestSignInDoesNotRevealUsernames(t *testing.T) {
	e := newEnv(t)
	e.setup()
	before := e.hasher.n.Load()
	wrong := e.do("POST", "/auth/login", map[string]any{"username": "ada", "password": "not the password", "device": laptop})
	afterWrong := e.hasher.n.Load()
	unknown := e.do("POST", "/auth/login", map[string]any{"username": "nobody", "password": "not the password", "device": laptop}, from("10.0.0.9:1"))
	afterUnknown := e.hasher.n.Load()

	e.expect(wrong, http.StatusUnauthorized, "invalid_credentials")
	e.expect(unknown, http.StatusUnauthorized, "invalid_credentials")
	if wrong.Body.String() != unknown.Body.String() {
		t.Errorf("bodies differ:\n%s\n%s", wrong.Body, unknown.Body)
	}
	if afterWrong-before != 1 || afterUnknown-afterWrong != 1 {
		t.Errorf("hashing work: wrong password %d, unknown user %d; want 1 each", afterWrong-before, afterUnknown-afterWrong)
	}
}

func TestSignInBackoff(t *testing.T) {
	e := newEnv(t)
	e.setup()
	bad := map[string]any{"username": "ada", "password": "not the password", "device": laptop}
	for range 5 {
		e.expect(e.do("POST", "/auth/login", bad), http.StatusUnauthorized, "invalid_credentials")
	}
	r := e.do("POST", "/auth/login", map[string]any{"username": "ada", "password": adminPassword, "device": laptop})
	e.expect(r, http.StatusTooManyRequests, "too_many_attempts")
	if r.Header().Get("Retry-After") != "1" {
		t.Errorf("Retry-After = %q, want 1", r.Header().Get("Retry-After"))
	}
	e.clock.advance(2 * time.Second)
	e.login("ada", adminPassword) // success resets the counters
	e.expect(e.do("POST", "/auth/login", bad), http.StatusUnauthorized, "invalid_credentials")
}

func TestProtectedRoutesAndRoles(t *testing.T) {
	e := newEnv(t)
	admin := e.setup()
	e.expect(e.do("GET", "/me", nil), http.StatusUnauthorized, "unauthorized")
	e.expect(e.do("GET", "/users", nil, bearer("forged")), http.StatusUnauthorized, "unauthorized")
	e.expect(e.do("GET", "/users", nil, bearer(admin.Token)), http.StatusOK, "")

	m := e.member(admin.Token, "bob")
	e.expect(e.do("GET", "/users", nil, bearer(m.Token)), http.StatusForbidden, "forbidden")
	e.expect(e.do("POST", "/users", map[string]any{
		"displayName": "Eve", "username": "eve", "temporaryPassword": "temporary password", "role": "admin",
	}, bearer(m.Token)), http.StatusForbidden, "forbidden")
}

func TestWebCookieSessionAndCSRF(t *testing.T) {
	e := newEnv(t)
	e.setup()
	r := e.do("POST", "/auth/login", map[string]any{"username": "ada", "password": adminPassword, "device": laptop}, web())
	e.expect(r, http.StatusOK, "")
	var res authResult
	r.json(&res)
	if res.Token != "" {
		t.Error("web client got the token in the body")
	}
	var sessionCookie *http.Cookie
	for _, c := range r.Result().Cookies() {
		if c.Name == auth.CookieName {
			sessionCookie = c
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode || sessionCookie.Secure {
		t.Fatalf("cookie = %+v, want HttpOnly, SameSite=Lax, not Secure over HTTP", sessionCookie)
	}
	tok := sessionCookie.Value
	patch := map[string]any{"displayName": "Ada L."}

	e.expect(e.do("GET", "/me", nil, cookie(tok)), http.StatusOK, "")
	e.expect(e.do("PATCH", "/me", patch, cookie(tok)), http.StatusForbidden, "csrf")
	e.expect(e.do("PATCH", "/me", patch, cookie(tok), header("X-Plaid-Client", "web")), http.StatusForbidden, "csrf")
	e.expect(e.do("PATCH", "/me", patch, cookie(tok), header("X-Plaid-Client", "web"), header("Origin", "http://evil.example")), http.StatusForbidden, "csrf")
	e.expect(e.do("PATCH", "/me", patch, cookie(tok), web()), http.StatusOK, "")

	native := e.login("ada", adminPassword)
	e.expect(e.do("PATCH", "/me", patch, bearer(native.Token)), http.StatusOK, "")

	// Cross-site sign-in into a cookie session is refused too.
	e.expect(e.do("POST", "/auth/login", map[string]any{"username": "ada", "password": adminPassword, "device": laptop},
		header("X-Plaid-Client", "web"), header("Origin", "http://evil.example")), http.StatusForbidden, "csrf")

	out := e.do("POST", "/auth/logout", nil, cookie(tok), web())
	e.expect(out, http.StatusNoContent, "")
	if c := out.Result().Cookies(); len(c) != 1 || c[0].MaxAge >= 0 {
		t.Errorf("logout cookies = %+v, want the session cookie cleared", c)
	}
	e.expect(e.do("GET", "/me", nil, cookie(tok)), http.StatusUnauthorized, "unauthorized")
}

func TestRevokeAndPasswordChange(t *testing.T) {
	e := newEnv(t)
	first := e.setup()
	second := e.login("ada", adminPassword)
	third := e.login("ada", adminPassword)

	var sessions []struct {
		ID      string `json:"id"`
		Current bool   `json:"current"`
	}
	e.do("GET", "/me/sessions", nil, bearer(first.Token)).json(&sessions)
	if len(sessions) != 3 {
		t.Fatalf("%d sessions, want 3", len(sessions))
	}
	e.expect(e.do("DELETE", "/me/sessions/"+second.Session.ID, nil, bearer(first.Token)), http.StatusNoContent, "")
	e.expect(e.do("GET", "/me", nil, bearer(second.Token)), http.StatusUnauthorized, "unauthorized")

	e.expect(e.do("POST", "/me/password", map[string]any{
		"currentPassword": "wrong one", "newPassword": "a brand new passphrase",
	}, bearer(first.Token)), http.StatusForbidden, "invalid_credentials")
	e.expect(e.do("POST", "/me/password", map[string]any{
		"currentPassword": adminPassword, "newPassword": "password1",
	}, bearer(first.Token)), http.StatusUnprocessableEntity, "weak_password")
	e.expect(e.do("POST", "/me/password", map[string]any{
		"currentPassword": adminPassword, "newPassword": "a brand new passphrase",
	}, bearer(first.Token)), http.StatusNoContent, "")
	e.expect(e.do("GET", "/me", nil, bearer(third.Token)), http.StatusUnauthorized, "unauthorized")
	e.expect(e.do("GET", "/me", nil, bearer(first.Token)), http.StatusOK, "")
	e.login("ada", "a brand new passphrase")
}

func TestTemporaryPasswordGate(t *testing.T) {
	e := newEnv(t)
	admin := e.setup()
	e.expect(e.do("POST", "/users", map[string]any{
		"displayName": "Bob", "username": "Bob", "temporaryPassword": "temporary password", "role": "member",
	}, bearer(admin.Token)), http.StatusCreated, "")
	e.expect(e.do("POST", "/users", map[string]any{
		"displayName": "Bob", "username": "BOB", "temporaryPassword": "temporary password", "role": "member",
	}, bearer(admin.Token)), http.StatusConflict, "username_taken")

	bob := e.login("bob", "temporary password")
	if !bob.User.MustChangePassword {
		t.Error("mustChangePassword = false for a temporary password")
	}
	e.expect(e.do("GET", "/me", nil, bearer(bob.Token)), http.StatusOK, "")
	e.expect(e.do("GET", "/me/sessions", nil, bearer(bob.Token)), http.StatusForbidden, "password_change_required")
	e.expect(e.do("POST", "/me/password", map[string]any{
		"currentPassword": "temporary password", "newPassword": "bob's own passphrase",
	}, bearer(bob.Token)), http.StatusNoContent, "")
	e.expect(e.do("GET", "/me/sessions", nil, bearer(bob.Token)), http.StatusOK, "")
}

func TestLastAdminIsProtected(t *testing.T) {
	e := newEnv(t)
	admin := e.setup()
	id := admin.User.ID
	e.expect(e.do("PATCH", "/users/"+id, map[string]any{"role": "member"}, bearer(admin.Token)), http.StatusConflict, "last_admin")
	e.expect(e.do("PATCH", "/users/"+id, map[string]any{"disabled": true}, bearer(admin.Token)), http.StatusConflict, "last_admin")
	e.expect(e.do("DELETE", "/users/"+id, nil, bearer(admin.Token)), http.StatusConflict, "last_admin")

	bob := e.member(admin.Token, "bob")
	e.expect(e.do("PATCH", "/users/"+bob.User.ID, map[string]any{"role": "admin"}, bearer(admin.Token)), http.StatusOK, "")
	e.expect(e.do("PATCH", "/users/"+id, map[string]any{"role": "member"}, bearer(admin.Token)), http.StatusOK, "")
	// Ada is a member now: the role change applies to her live session at once.
	e.expect(e.do("GET", "/users", nil, bearer(admin.Token)), http.StatusForbidden, "forbidden")
}

func TestDisabledAccount(t *testing.T) {
	e := newEnv(t)
	admin := e.setup()
	bob := e.member(admin.Token, "bob")
	e.expect(e.do("PATCH", "/users/"+bob.User.ID, map[string]any{"disabled": true}, bearer(admin.Token)), http.StatusOK, "")
	e.expect(e.do("GET", "/me", nil, bearer(bob.Token)), http.StatusUnauthorized, "unauthorized")
	e.expect(e.do("POST", "/auth/login", map[string]any{"username": "bob", "password": "bob has a long password", "device": laptop}),
		http.StatusUnauthorized, "invalid_credentials")
	e.expect(e.do("PATCH", "/users/"+bob.User.ID, map[string]any{"disabled": false}, bearer(admin.Token)), http.StatusOK, "")
	e.login("bob", "bob has a long password")
}

func TestProfileLimits(t *testing.T) {
	e := newEnv(t)
	admin := e.setup()
	tok := bearer(admin.Token)
	e.expect(e.do("DELETE", "/me/profiles/"+admin.Profiles[0].ID, nil, tok), http.StatusConflict, "last_profile")
	for i := range 7 {
		e.expect(e.do("POST", "/me/profiles", map[string]any{"name": "Kid " + string(rune('A'+i))}, tok), http.StatusCreated, "")
	}
	r := e.do("POST", "/me/profiles", map[string]any{"name": "One too many"}, tok)
	e.expect(r, http.StatusUnprocessableEntity, "validation_failed")

	e.expect(e.do("POST", "/me/profiles", map[string]any{"name": ""}, tok), http.StatusUnprocessableEntity, "validation_failed")
}

func TestKidsProfileCantLiftItsLimit(t *testing.T) {
	e := newEnv(t)
	admin := e.setup()
	tok := bearer(admin.Token)
	var kids struct {
		ID           string `json:"id"`
		MaxAge       *int   `json:"maxAge"`
		AllowUnrated bool   `json:"allowUnrated"`
	}
	r := e.do("POST", "/me/profiles", map[string]any{"name": "Kids", "maxAge": 12}, tok)
	e.expect(r, http.StatusCreated, "")
	r.json(&kids)
	if kids.MaxAge == nil || *kids.MaxAge != 12 || kids.AllowUnrated {
		t.Fatalf("kids profile = %+v, want maxAge 12 and allowUnrated false", kids)
	}
	e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": kids.ID}, tok), http.StatusOK, "")
	e.expect(e.do("PATCH", "/me/profiles/"+kids.ID, map[string]any{"maxAge": nil, "password": adminPassword}, tok), http.StatusForbidden, "forbidden")
	e.expect(e.do("POST", "/me/profiles", map[string]any{"name": "Kid 2"}, tok), http.StatusForbidden, "forbidden")

	e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": admin.Profiles[0].ID}, tok), http.StatusOK, "")
	e.expect(e.do("PATCH", "/me/profiles/"+kids.ID, map[string]any{"maxAge": nil}, tok), http.StatusForbidden, "invalid_credentials")
	r = e.do("PATCH", "/me/profiles/"+kids.ID, map[string]any{"maxAge": nil, "password": adminPassword}, tok)
	e.expect(r, http.StatusOK, "")
	r.json(&kids)
	if kids.MaxAge != nil {
		t.Errorf("maxAge = %v after clearing it", *kids.MaxAge)
	}
}

func TestProfilePIN(t *testing.T) {
	e := newEnv(t)
	admin := e.setup()
	tok := bearer(admin.Token)
	own := admin.Profiles[0].ID
	e.expect(e.do("PATCH", "/me/profiles/"+own, map[string]any{"pin": "1234"}, tok), http.StatusForbidden, "invalid_credentials")
	e.expect(e.do("PATCH", "/me/profiles/"+own, map[string]any{"pin": "12a4", "password": adminPassword}, tok), http.StatusUnprocessableEntity, "validation_failed")
	e.expect(e.do("PATCH", "/me/profiles/"+own, map[string]any{"pin": "1234", "password": adminPassword}, tok), http.StatusOK, "")
	e.expect(e.do("POST", "/me/profiles", map[string]any{"name": "Kids", "maxAge": 7}, tok), http.StatusCreated, "")

	// With two profiles and a PIN, a new sign-in starts with no profile.
	res := e.login("ada", adminPassword)
	if res.ActiveProfile != nil {
		t.Errorf("activeProfile = %v, want none until picked", res.ActiveProfile)
	}
	tok = bearer(res.Token)
	e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": own}, tok), http.StatusForbidden, "invalid_pin")
	for range 4 {
		e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": own, "pin": "0000"}, tok), http.StatusForbidden, "invalid_pin")
	}
	r := e.do("POST", "/session/profile", map[string]any{"profileId": own, "pin": "1234"}, tok)
	e.expect(r, http.StatusLocked, "profile_locked")
	if r.Header().Get("Retry-After") != "300" {
		t.Errorf("Retry-After = %q, want 300", r.Header().Get("Retry-After"))
	}
	e.clock.advance(5*time.Minute + time.Second)
	e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": own, "pin": "1234"}, tok), http.StatusOK, "")

	// The PIN can be removed with the PIN itself.
	e.expect(e.do("PATCH", "/me/profiles/"+own, map[string]any{"pin": nil, "currentPin": "1234"}, tok), http.StatusOK, "")
	e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": own}, tok), http.StatusOK, "")
}

func TestTVPairing(t *testing.T) {
	e := newEnv(t)
	admin := e.setup()
	tok := bearer(admin.Token)
	var kids struct{ ID string }
	e.do("POST", "/me/profiles", map[string]any{"name": "Kids", "maxAge": 7}, tok).json(&kids)

	var started struct {
		ID, Code, PollSecret string
	}
	r := e.do("POST", "/pairing", map[string]any{"device": tv}, from("192.168.1.50:3000"))
	e.expect(r, http.StatusCreated, "")
	r.json(&started)
	if len(started.Code) != 7 || started.Code[3] != '-' || started.PollSecret == "" {
		t.Fatalf("pairing = %+v", started)
	}
	poll := func() resp {
		return e.do("GET", "/pairing/"+started.ID, nil, header("X-Pairing-Secret", started.PollSecret))
	}
	e.expect(poll(), http.StatusAccepted, "")
	e.expect(e.do("GET", "/pairing/"+started.ID, nil, header("X-Pairing-Secret", "guess")), http.StatusNotFound, "pairing_not_found")

	e.expect(e.do("POST", "/pairing/claim", map[string]any{"code": "ZZZ-ZZZ", "profileId": kids.ID}, tok), http.StatusNotFound, "pairing_not_found")
	lower := strings.ToLower(strings.ReplaceAll(started.Code, "-", ""))
	e.expect(e.do("POST", "/pairing/claim", map[string]any{"code": lower, "profileId": kids.ID}, tok), http.StatusNoContent, "")
	e.expect(e.do("POST", "/pairing/claim", map[string]any{"code": started.Code, "profileId": kids.ID}, tok), http.StatusGone, "pairing_expired")

	r = poll()
	e.expect(r, http.StatusOK, "")
	var res authResult
	r.json(&res)
	if res.Token == "" || res.ActiveProfile == nil || res.ActiveProfile.ID != kids.ID {
		t.Fatalf("TV session = %+v, want a token on the Kids profile", res)
	}
	e.expect(poll(), http.StatusNotFound, "pairing_not_found")

	var me struct{ ActiveProfile struct{ ID string } }
	e.do("GET", "/me", nil, bearer(res.Token)).json(&me)
	if me.ActiveProfile.ID != kids.ID {
		t.Errorf("TV active profile = %q, want Kids", me.ActiveProfile.ID)
	}
	var sessions []struct{ DeviceName, DeviceType string }
	e.do("GET", "/me/sessions", nil, tok).json(&sessions)
	if sessions[0].DeviceName != "Living room TV" || sessions[0].DeviceType != "tvos" {
		t.Errorf("newest session = %+v, want the TV", sessions[0])
	}
}

func TestPairingExpires(t *testing.T) {
	e := newEnv(t)
	admin := e.setup()
	var started struct{ ID, Code, PollSecret string }
	e.do("POST", "/pairing", map[string]any{"device": tv}).json(&started)
	e.clock.advance(10*time.Minute + time.Second)
	e.expect(e.do("POST", "/pairing/claim", map[string]any{"code": started.Code, "profileId": admin.Profiles[0].ID}, bearer(admin.Token)),
		http.StatusGone, "pairing_expired")
	e.expect(e.do("GET", "/pairing/"+started.ID, nil, header("X-Pairing-Secret", started.PollSecret)), http.StatusGone, "pairing_expired")
}

func TestPairingTokenLostOnRestart(t *testing.T) {
	e := newEnv(t)
	admin := e.setup()
	var started struct{ ID, Code, PollSecret string }
	e.do("POST", "/pairing", map[string]any{"device": tv}).json(&started)
	e.expect(e.do("POST", "/pairing/claim", map[string]any{"code": started.Code, "profileId": admin.Profiles[0].ID}, bearer(admin.Token)),
		http.StatusNoContent, "")

	// A new service on the same database is a restarted server.
	restarted := accounts.New(e.db, accounts.Options{Hasher: e.hasher, Now: e.clock.now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	_, _, err := restarted.PollPairing(context.Background(), started.ID, started.PollSecret)
	if err == nil || !strings.HasPrefix(err.Error(), "pairing_not_found") {
		t.Errorf("poll after restart: %v, want pairing_not_found", err)
	}
}

func TestSessionExpiresAfter90DaysIdle(t *testing.T) {
	e := newEnv(t)
	admin := e.setup()
	e.clock.advance(80 * 24 * time.Hour)
	e.expect(e.do("GET", "/me", nil, bearer(admin.Token)), http.StatusOK, "") // slides the expiry
	e.clock.advance(80 * 24 * time.Hour)
	e.expect(e.do("GET", "/me", nil, bearer(admin.Token)), http.StatusOK, "")
	e.clock.advance(91 * 24 * time.Hour)
	e.expect(e.do("GET", "/me", nil, bearer(admin.Token)), http.StatusUnauthorized, "unauthorized")
}

func TestLastSeenIsThrottled(t *testing.T) {
	e := newEnv(t)
	admin := e.setup()
	lastSeen := func() string {
		var s string
		if err := e.db.QueryRow(`SELECT last_seen_at FROM sessions WHERE id = ?`, admin.Session.ID).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	start := lastSeen()
	e.clock.advance(4 * time.Minute)
	e.do("GET", "/me", nil, bearer(admin.Token))
	if lastSeen() != start {
		t.Error("last_seen_at written within 5 minutes")
	}
	e.clock.advance(2 * time.Minute)
	e.do("GET", "/me", nil, bearer(admin.Token))
	if lastSeen() == start {
		t.Error("last_seen_at not written after 6 minutes")
	}
}

func TestNoSecretsInLogs(t *testing.T) {
	e := newEnv(t)
	admin := e.setup()
	tok := bearer(admin.Token)
	e.do("PATCH", "/me/profiles/"+admin.Profiles[0].ID, map[string]any{"pin": "4321", "password": adminPassword}, tok)
	e.do("POST", "/auth/login", map[string]any{"username": "ada", "password": "wrong guess here", "device": laptop})
	var started struct{ ID, Code, PollSecret string }
	e.do("POST", "/pairing", map[string]any{"device": tv}).json(&started)
	e.do("POST", "/pairing/claim", map[string]any{"code": started.Code, "profileId": admin.Profiles[0].ID, "pin": "4321"}, tok)
	var tvRes authResult
	e.do("GET", "/pairing/"+started.ID, nil, header("X-Pairing-Secret", started.PollSecret)).json(&tvRes)

	logs := e.logs.String()
	for _, secret := range []string{adminPassword, "wrong guess here", "4321", admin.Token, tvRes.Token, started.PollSecret} {
		if secret != "" && strings.Contains(logs, secret) {
			t.Errorf("log contains a secret %q", secret)
		}
	}
	for _, event := range []string{"first admin created", "sign-in failed", "pairing claimed"} {
		if !strings.Contains(logs, event) {
			t.Errorf("log lacks %q", event)
		}
	}
}
