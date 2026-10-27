package httpx

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const secret = "s3cr3t-key-value"

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type env struct {
	srv    *httptest.Server
	hits   atomic.Int64
	status atomic.Int64
	logs   *bytes.Buffer
	clock  *clock
	c      *Client
}

// newEnv serves status (200 by default) with body "ok" over TLS.
func newEnv(t *testing.T, tune func(*Options)) *env {
	t.Helper()
	e := &env{logs: &bytes.Buffer{}, clock: &clock{t: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}}
	e.status.Store(200)
	e.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.hits.Add(1)
		switch r.URL.Path {
		case "/redirect-out":
			http.Redirect(w, r, "https://example.com/elsewhere", http.StatusFound)
			return
		case "/big":
			_, _ = w.Write(bytes.Repeat([]byte("x"), 2048))
			return
		case "/slow":
			time.Sleep(300 * time.Millisecond)
		case "/limited":
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(int(e.status.Load()))
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(e.srv.Close)
	o := Options{
		Name: "test", Hosts: []string{"127.0.0.1"}, Transport: e.srv.Client().Transport,
		Log: slog.New(slog.NewTextHandler(e.logs, nil)), Now: e.clock.now,
	}
	if tune != nil {
		tune(&o)
	}
	e.c = New(o)
	return e
}

func (e *env) get(path string) ([]byte, error) {
	return e.c.Get(context.Background(), e.srv.URL+path+"?api_key="+secret, nil)
}

func TestGetAndStatusCodes(t *testing.T) {
	e := newEnv(t, nil)
	body, err := e.get("/movie/603")
	if err != nil || string(body) != "ok" {
		t.Fatalf("Get = %q, %v", body, err)
	}
	e.status.Store(404)
	if _, err := e.get("/movie/0"); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: %v", err)
	}
	e.status.Store(400)
	var se *StatusError
	if _, err := e.get("/x"); !errors.As(err, &se) || se.Code != 400 {
		t.Errorf("400: %v", err)
	}
	if st := e.c.Status(); st.State != StateOK {
		t.Errorf("client errors changed the state: %+v", st)
	}
}

func TestCircuitBreaker(t *testing.T) {
	e := newEnv(t, nil)
	e.status.Store(503)
	for i := range failureThreshold {
		if _, err := e.get("/x"); err == nil {
			t.Fatalf("request %d succeeded", i)
		}
	}
	if st := e.c.Status(); st.State != StateUnreachable || st.LastError == "" {
		t.Fatalf("after %d failures: %+v", failureThreshold, st)
	}
	before := e.hits.Load()
	for range 10 {
		if _, err := e.get("/x"); !errors.Is(err, ErrUnreachable) {
			t.Fatalf("open circuit: %v", err)
		}
	}
	if e.hits.Load() != before {
		t.Errorf("%d requests went out while unreachable", e.hits.Load()-before)
	}
	// One probe a minute; it fails, so the circuit stays open.
	e.clock.advance(probeEvery)
	_, _ = e.get("/x")
	_, err := e.get("/x")
	if e.hits.Load() != before+1 || !errors.Is(err, ErrUnreachable) {
		t.Errorf("probe: hits +%d, second err %v", e.hits.Load()-before, err)
	}
	// The provider recovers: the next probe closes the circuit.
	e.status.Store(200)
	e.clock.advance(probeEvery)
	if _, err := e.get("/x"); err != nil {
		t.Fatalf("recovery probe: %v", err)
	}
	if st := e.c.Status(); st.State != StateOK {
		t.Errorf("after recovery: %+v", st)
	}
}

func TestInvalidKeyPausesUntilReset(t *testing.T) {
	e := newEnv(t, nil)
	e.status.Store(401)
	if _, err := e.get("/x"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("401: %v", err)
	}
	e.status.Store(200)
	before := e.hits.Load()
	if _, err := e.get("/x"); !errors.Is(err, ErrInvalidKey) || e.hits.Load() != before {
		t.Errorf("request went out with a rejected key: %v", err)
	}
	if st := e.c.Status(); st.State != StateInvalidKey {
		t.Errorf("state %+v", st)
	}
	e.c.ResetKey()
	if _, err := e.get("/x"); err != nil {
		t.Errorf("after ResetKey: %v", err)
	}
}

func TestHostAllowlist(t *testing.T) {
	e := newEnv(t, nil)
	for _, u := range []string{
		"https://evil.example/x",
		strings.Replace(e.srv.URL, "https://", "http://", 1) + "/x",
		"https://127.0.0.1.evil.example/x",
	} {
		if _, err := e.c.Get(context.Background(), u, nil); !errors.Is(err, ErrHostNotAllowed) {
			t.Errorf("%s: %v", u, err)
		}
	}
	if e.hits.Load() != 0 {
		t.Errorf("%d requests to disallowed hosts", e.hits.Load())
	}
	if _, err := e.get("/redirect-out"); !errors.Is(err, ErrHostNotAllowed) {
		t.Errorf("redirect out of the allowlist: %v", err)
	}
	if st := e.c.Status(); st.LastError != "" {
		t.Errorf("a refused redirect counted as a provider failure: %+v", st)
	}
}

func TestKeysNeverLeak(t *testing.T) {
	e := newEnv(t, func(o *Options) { o.Timeout = 100 * time.Millisecond })
	var errs []string
	e.status.Store(500)
	for range failureThreshold {
		if _, err := e.get("/x"); err != nil {
			errs = append(errs, err.Error())
		}
	}
	e.c = New(Options{Name: "t", Hosts: []string{"127.0.0.1"}, Transport: e.srv.Client().Transport,
		Timeout: 100 * time.Millisecond, Log: slog.New(slog.NewTextHandler(e.logs, nil))})
	if _, err := e.get("/slow"); err != nil {
		errs = append(errs, err.Error())
		if !strings.Contains(err.Error(), "timeout") {
			t.Errorf("timeout error = %v", err)
		}
	}
	e.status.Store(401)
	if _, err := e.get("/x"); err != nil {
		errs = append(errs, err.Error())
	}
	all := strings.Join(errs, "\n") + e.logs.String() + e.c.Status().LastError
	if strings.Contains(all, secret) {
		t.Errorf("the key leaked:\n%s", all)
	}
	if !strings.Contains(all, "REDACTED") {
		t.Errorf("errors don't show the redacted URL:\n%s", all)
	}
	if got := Redact("https://webservice.fanart.tv/v3/movies/603?api_key=a&client_key=b&x=1"); strings.Contains(got, "=a") || strings.Contains(got, "=b") || !strings.Contains(got, "x=1") {
		t.Errorf("Redact = %s", got)
	}
}

func TestMaxBody(t *testing.T) {
	e := newEnv(t, func(o *Options) { o.MaxBody = 1024 })
	if _, err := e.get("/big"); !errors.Is(err, ErrTooLarge) {
		t.Errorf("oversized body: %v", err)
	}
}

func TestRetryAfterHoldsRequests(t *testing.T) {
	e := newEnv(t, func(o *Options) { o.Now = time.Now })
	if _, err := e.get("/limited"); err == nil {
		t.Fatal("429 returned no error")
	}
	start := time.Now()
	if _, err := e.get("/x"); err != nil {
		t.Fatal(err)
	}
	if waited := time.Since(start); waited < 900*time.Millisecond {
		t.Errorf("next request went out after %v, want ≥ 1 s (Retry-After)", waited)
	}
}

func TestRateLimit(t *testing.T) {
	e := newEnv(t, func(o *Options) { o.RPS, o.Burst = 20, 1 })
	start := time.Now()
	for range 5 {
		if _, err := e.get("/x"); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 190*time.Millisecond {
		t.Errorf("5 requests at 20/s took %v, want ≥ 200 ms", d)
	}
}
