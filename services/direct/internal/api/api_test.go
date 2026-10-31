package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gh-prim/plaid/services/direct/internal/probe"
	"github.com/gh-prim/plaid/services/direct/internal/store"
)

type fakeProber struct {
	mu    sync.Mutex
	calls []netip.AddrPort
}

func (f *fakeProber) Check(_ context.Context, ip netip.Addr, port int) probe.Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, netip.AddrPortFrom(ip, uint16(port)))
	return probe.Result{Reachable: true}
}

type env struct {
	t      *testing.T
	h      http.Handler
	store  *store.Store
	prober *fakeProber
	now    time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	e := &env{t: t, store: st, prober: &fakeProber{}, now: time.Unix(1_800_000_000, 0).UTC()}
	e.h = New(Config{
		Store:          st,
		Prober:         e.prober,
		Zone:           "getplaid.direct",
		SiteURL:        "https://getplaid.tv",
		TrustedProxies: DefaultTrustedProxies,
		Now:            func() time.Time { return e.now },
	})
	return e
}

type call struct {
	method, path, body, token, remote, xff string
}

func (e *env) do(c call) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
	req.RemoteAddr = "203.0.113.7:51000"
	if c.remote != "" {
		req.RemoteAddr = c.remote
	}
	if c.xff != "" {
		req.Header.Set("X-Forwarded-For", c.xff)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func (e *env) register(name string, port int) serverView {
	e.t.Helper()
	body, _ := json.Marshal(map[string]any{"name": name, "port": port, "version": "0.3.0"})
	rec := e.do(call{method: "POST", path: "/api/v1/servers", body: string(body)})
	if rec.Code != http.StatusCreated {
		e.t.Fatalf("register: %d %s", rec.Code, rec.Body)
	}
	return decodeJSON[serverView](e.t, rec)
}

func TestRegisterUpdateDelete(t *testing.T) {
	e := newEnv(t)
	v := e.register("Living room", 32501)

	if len(v.Token) != 43 || v.Port != 32501 || v.PublicIP != "203.0.113.7" {
		t.Fatalf("register view: %+v", v)
	}
	if v.Hostname != "living-room-"+v.ID+".getplaid.direct" ||
		v.URL != "https://living-room-"+v.ID+".getplaid.direct:32501" ||
		v.ShortURL != "https://getplaid.direct/living-room-"+v.ID {
		t.Fatalf("register names: %+v", v)
	}
	srv, err := e.store.Get(context.Background(), v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(srv.TokenHash, []byte(v.Token)) || len(srv.TokenHash) != 32 {
		t.Fatal("token must be stored hashed")
	}

	// Rename, new port, from a new address (behind the proxy).
	e.now = e.now.Add(time.Minute)
	rec := e.do(call{method: "PUT", path: "/api/v1/servers/" + v.ID, token: v.Token,
		body: `{"name":"Cinéma","port":443}`, remote: "172.18.0.2:4000", xff: "198.51.100.9"})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	u := decodeJSON[map[string]any](t, rec)
	if _, ok := u["token"]; ok {
		t.Fatal("update must not return the token")
	}
	if u["hostname"] != "cin-ma-"+v.ID+".getplaid.direct" || u["url"] != "https://cin-ma-"+v.ID+".getplaid.direct" ||
		u["publicIp"] != "198.51.100.9" || u["port"] != float64(443) {
		t.Fatalf("update view: %v", u)
	}
	srv, _ = e.store.Get(context.Background(), v.ID)
	if srv.IPv4.String() != "198.51.100.9" || !srv.LastSeen.Equal(e.now) || srv.Version != "0.3.0" {
		t.Fatalf("stored after update: %+v", srv)
	}

	// Empty body is a plain heartbeat.
	if rec := e.do(call{method: "PUT", path: "/api/v1/servers/" + v.ID, token: v.Token}); rec.Code != http.StatusOK {
		t.Fatalf("heartbeat: %d %s", rec.Code, rec.Body)
	}

	if rec := e.do(call{method: "DELETE", path: "/api/v1/servers/" + v.ID, token: v.Token}); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := e.do(call{method: "PUT", path: "/api/v1/servers/" + v.ID, token: v.Token}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after delete: %d", rec.Code)
	}
}

func TestRegisterValidation(t *testing.T) {
	e := newEnv(t)
	for _, body := range []string{``, `{`, `{"name":"x"}`, `{"port":0}`, `{"port":70000}`, `{"port":1,"version":"\u0007"}`,
		`{"port":1,"name":"` + strings.Repeat("a", 201) + `"}`} {
		rec := e.do(call{method: "POST", path: "/api/v1/servers", body: body})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", body, rec.Code)
		}
		if got := decodeJSON[map[string]string](t, rec); got["error"] != "bad_request" || got["message"] == "" {
			t.Fatalf("%s: %v", body, got)
		}
	}
	rec := e.do(call{method: "POST", path: "/api/v1/servers", body: `{"port":32501,"pad":"` + strings.Repeat("a", 9000) + `"}`})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large body: %d", rec.Code)
	}
	// No name: the slug falls back to "plaid".
	if v := e.register("", 32501); !strings.HasPrefix(v.Hostname, "plaid-") {
		t.Fatalf("hostname %q", v.Hostname)
	}
}

func TestAuth(t *testing.T) {
	e := newEnv(t)
	a := e.register("a", 1000)
	b := e.register("b", 2000)

	paths := []call{
		{method: "PUT", path: "/api/v1/servers/%s"},
		{method: "DELETE", path: "/api/v1/servers/%s"},
		{method: "PUT", path: "/api/v1/servers/%s/acme-challenge", body: `{"values":["x"]}`},
		{method: "DELETE", path: "/api/v1/servers/%s/acme-challenge"},
		{method: "POST", path: "/api/v1/servers/%s/reachability"},
	}
	cases := []struct{ id, token string }{
		{a.ID, ""},            // no token
		{a.ID, b.Token},       // someone else's token
		{a.ID, a.Token + "x"}, // wrong token
		{"zzzzzz", a.Token},   // unknown id
		{"NOT-AN-ID", a.Token},
	}
	for _, p := range paths {
		for _, c := range cases {
			p := p
			p.path = strings.Replace(p.path, "%s", c.id, 1)
			p.token = c.token
			rec := e.do(p)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s token=%q: %d", p.method, p.path, c.token, rec.Code)
			}
			if got := decodeJSON[map[string]string](t, rec); got["error"] != "unauthorized" {
				t.Fatalf("%v", got)
			}
		}
	}
	if _, err := e.store.Get(context.Background(), a.ID); err != nil {
		t.Fatal("server a was touched by failed calls")
	}
}

func TestACME(t *testing.T) {
	e := newEnv(t)
	v := e.register("a", 1000)
	path := "/api/v1/servers/" + v.ID + "/acme-challenge"
	ctx := context.Background()

	for _, body := range []string{`{"values":[]}`, `{"values":["a","b","c","d","e"]}`, `{"values":["has space"]}`, `{"values":[""]}`, `{}`} {
		if rec := e.do(call{method: "PUT", path: path, token: v.Token, body: body}); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", body, rec.Code)
		}
	}
	rec := e.do(call{method: "PUT", path: path, token: v.Token, body: `{"values":["dGVzdA-_one","two"]}`})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("set: %d %s", rec.Code, rec.Body)
	}
	vals, _ := e.store.ACMEValues(ctx, v.ID, e.now)
	if !slices.Equal(vals, []string{"dGVzdA-_one", "two"}) {
		t.Fatalf("values %v", vals)
	}
	// Replaces, not appends.
	e.do(call{method: "PUT", path: path, token: v.Token, body: `{"values":["three"]}`})
	if vals, _ := e.store.ACMEValues(ctx, v.ID, e.now); !slices.Equal(vals, []string{"three"}) {
		t.Fatalf("values %v", vals)
	}
	if vals, _ := e.store.ACMEValues(ctx, v.ID, e.now.Add(ACMETTL)); len(vals) != 0 {
		t.Fatalf("values must expire after an hour: %v", vals)
	}
	if rec := e.do(call{method: "DELETE", path: path, token: v.Token}); rec.Code != http.StatusNoContent {
		t.Fatalf("clear: %d", rec.Code)
	}
	if vals, _ := e.store.ACMEValues(ctx, v.ID, e.now); len(vals) != 0 {
		t.Fatalf("values after clear %v", vals)
	}
}

func TestShortLink(t *testing.T) {
	e := newEnv(t)
	v := e.register("Living room", 32501)

	rec := e.do(call{method: "GET", path: "/living-room-" + v.ID})
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://living-room-"+v.ID+".getplaid.direct:32501/" {
		t.Fatalf("short link: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	// An old slug still finds the server by id and lands on the current name.
	rec = e.do(call{method: "GET", path: "/old-name-" + v.ID})
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://living-room-"+v.ID+".getplaid.direct:32501/" {
		t.Fatalf("old slug: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	for _, p := range []string{"/living-room-zzzzzz", "/nope", "/favicon.ico"} {
		rec = e.do(call{method: "GET", path: p})
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `href="https://getplaid.tv"`) {
			t.Fatalf("%s: %d %s", p, rec.Code, rec.Body)
		}
	}
	if rec = e.do(call{method: "GET", path: "/"}); rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://getplaid.tv" {
		t.Fatalf("home: %d", rec.Code)
	}
	if rec = e.do(call{method: "GET", path: "/api/v1/nope"}); rec.Code != http.StatusNotFound || decodeJSON[map[string]string](t, rec)["error"] != "not_found" {
		t.Fatalf("api 404: %d %s", rec.Code, rec.Body)
	}
}

func TestRegisterRateLimit(t *testing.T) {
	e := newEnv(t)
	for range 5 {
		e.register("a", 1)
	}
	rec := e.do(call{method: "POST", path: "/api/v1/servers", body: `{"port":1}`})
	if rec.Code != http.StatusTooManyRequests || decodeJSON[map[string]string](t, rec)["error"] != "rate_limited" {
		t.Fatalf("6th registration: %d %s", rec.Code, rec.Body)
	}
	// Another source is not affected; a forged X-Forwarded-For from an
	// untrusted peer doesn't change the source.
	if rec := e.do(call{method: "POST", path: "/api/v1/servers", body: `{"port":1}`, xff: "198.51.100.1"}); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("forged XFF bypassed the limit: %d", rec.Code)
	}
	if rec := e.do(call{method: "POST", path: "/api/v1/servers", body: `{"port":1}`, remote: "198.51.100.1:1"}); rec.Code != http.StatusCreated {
		t.Fatalf("other source: %d", rec.Code)
	}
	// An hour later the window has slid.
	e.now = e.now.Add(time.Hour + time.Second)
	e.register("a", 1)

	// IPv6 sources share a limit per /64.
	for i := range 6 {
		rec := e.do(call{method: "POST", path: "/api/v1/servers", body: `{"port":1}`, remote: "[2001:db8:1:2::" + string(rune('a'+i)) + "]:1"})
		want := http.StatusCreated
		if i == 5 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Fatalf("ipv6 #%d: %d", i, rec.Code)
		}
	}
}

func TestReachability(t *testing.T) {
	e := newEnv(t)
	v := e.register("a", 32501)
	path := "/api/v1/servers/" + v.ID + "/reachability"
	for i := range 11 {
		rec := e.do(call{method: "POST", path: path, token: v.Token})
		if i == 10 {
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("11th check: %d", rec.Code)
			}
			break
		}
		got := decodeJSON[map[string]any](t, rec)
		if rec.Code != http.StatusOK || got["reachable"] != true || got["publicIp"] != "203.0.113.7" || got["port"] != float64(32501) {
			t.Fatalf("check: %d %v", rec.Code, got)
		}
		if _, ok := got["error"]; ok {
			t.Fatalf("error on success: %v", got)
		}
	}
	if len(e.prober.calls) != 10 || e.prober.calls[0].String() != "203.0.113.7:32501" {
		t.Fatalf("prober calls %v", e.prober.calls)
	}
}

func TestClientIP(t *testing.T) {
	e := newEnv(t)
	tests := []struct{ remote, xff, want string }{
		{"203.0.113.7:1", "", "203.0.113.7"},
		{"203.0.113.7:1", "198.51.100.1", "203.0.113.7"},           // untrusted peer: header ignored
		{"10.0.0.2:1", "198.51.100.1", "198.51.100.1"},             // proxy
		{"10.0.0.2:1", "6.6.6.6, 198.51.100.1", "198.51.100.1"},    // forged left entry ignored
		{"10.0.0.2:1", "198.51.100.1, 172.18.0.5", "198.51.100.1"}, // two proxies
		{"10.0.0.2:1", "garbage, 198.51.100.1", "198.51.100.1"},
		{"10.0.0.2:1", "198.51.100.1, garbage", "10.0.0.2"},
		{"10.0.0.2:1", "", "10.0.0.2"},
		{"[::ffff:10.0.0.2]:1", "2001:db8::5", "2001:db8::5"},
	}
	for _, tt := range tests {
		rec := e.do(call{method: "GET", path: "/api/v1/ip", remote: tt.remote, xff: tt.xff})
		if got := decodeJSON[map[string]string](t, rec)["ip"]; got != tt.want {
			t.Errorf("remote=%s xff=%q: ip=%s, want %s", tt.remote, tt.xff, got, tt.want)
		}
	}
}

func TestHealthz(t *testing.T) {
	e := newEnv(t)
	rec := e.do(call{method: "GET", path: "/healthz"})
	body, _ := io.ReadAll(rec.Body)
	if rec.Code != http.StatusOK || string(body) != "ok\n" {
		t.Fatalf("healthz: %d %q", rec.Code, body)
	}
}
