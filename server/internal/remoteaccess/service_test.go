package remoteaccess

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/db"
	"github.com/gh-prim/plaid/server/internal/settings"
)

type fakeMapper struct {
	mu      sync.Mutex
	err     error
	mapped  map[int]int // external -> internal
	takenBy int         // an external port someone else holds
}

func (f *fakeMapper) Method() string { return "upnp" }

func (f *fakeMapper) Map(_ context.Context, internal, ext int) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return 0, f.err
	}
	if ext == f.takenBy {
		ext = 45678
	}
	f.mapped[ext] = internal
	return ext, nil
}

func (f *fakeMapper) Unmap(_ context.Context, ext int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.mapped, ext)
	return nil
}

type fakeDirectory struct {
	mu          sync.Mutex
	servers     map[string]string // id -> token
	ports       map[string]int
	challenge   []string
	registered  int
	unreachable bool
}

func (f *fakeDirectory) reg(id string, port int) Registration {
	host := "living-room-" + id + ".getplaid.direct"
	return Registration{ID: id, Hostname: host, URL: "https://" + host + ":" + itoa(port),
		ShortURL: "https://getplaid.direct/living-room-" + id, PublicIP: "203.0.113.7", Port: port}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func (f *fakeDirectory) Register(_ context.Context, _ string, port int, _ string) (Registration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registered++
	id := "abc23" + itoa(f.registered)
	f.servers[id], f.ports[id] = "tok-"+id, port
	r := f.reg(id, port)
	r.Token = "tok-" + id
	return r, nil
}

func (f *fakeDirectory) check(id, token string) error {
	if f.servers[id] == "" || f.servers[id] != token {
		return errUnauthorized
	}
	return nil
}

func (f *fakeDirectory) Update(_ context.Context, id, token, _ string, port int, _ string) (Registration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check(id, token); err != nil {
		return Registration{}, err
	}
	f.ports[id] = port
	return f.reg(id, port), nil
}

func (f *fakeDirectory) Unregister(_ context.Context, id, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check(id, token); err != nil {
		return err
	}
	delete(f.servers, id)
	return nil
}

func (f *fakeDirectory) SetChallenge(_ context.Context, id, token string, v []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.challenge = v
	return f.check(id, token)
}

func (f *fakeDirectory) ClearChallenge(_ context.Context, id, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.challenge = nil
	return f.check(id, token)
}

func (f *fakeDirectory) Reachability(_ context.Context, id, token string) (Reachability, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check(id, token); err != nil {
		return Reachability{}, err
	}
	if f.unreachable {
		return Reachability{Error: "timeout"}, nil
	}
	return Reachability{Reachable: true}, nil
}

// fakeIssuer self-signs for the host, after checking the challenge flow.
type fakeIssuer struct {
	err    error
	issued []string
}

func (f *fakeIssuer) Issue(ctx context.Context, host string, publish func(context.Context, []string) error, unpublish func(context.Context) error) (*tls.Certificate, error) {
	if err := publish(ctx, []string{"v1", "v2"}); err != nil {
		return nil, err
	}
	defer func() { _ = unpublish(ctx) }()
	if f.err != nil {
		return nil, f.err
	}
	f.issued = append(f.issued, host)
	return selfSigned(time.Now(), host, "*."+host)
}

type harness struct {
	svc    *Service
	mapper *fakeMapper
	dir    *fakeDirectory
	issuer *fakeIssuer
	store  *settings.Store
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	ctx := context.Background()
	data := t.TempDir()
	database, err := db.Open(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	h := &harness{
		mapper: &fakeMapper{mapped: map[int]int{}},
		dir:    &fakeDirectory{servers: map[string]string{}, ports: map[string]int{}},
		issuer: &fakeIssuer{},
		store:  settings.New(database),
	}
	h.svc = h.open(t, data)
	return h
}

func (h *harness) open(t *testing.T, data string) *Service {
	t.Helper()
	svc, err := New(context.Background(), Options{
		Settings: h.store, Directory: h.dir, Issuer: h.issuer,
		DiscoverMapper: func(context.Context) (Mapper, error) { return h.mapper, nil },
		ServerName:     "Living room", Version: "0.3.0", TLSPort: 32501, CertDir: data + "/remote",
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func on() *bool { b := true; return &b }

func TestOffByDefault(t *testing.T) {
	h := newHarness(t)
	h.svc.refresh(context.Background())
	st := h.svc.Status()
	if st.Enabled || st.State != StateOff || st.PortMode != PortAuto || h.dir.registered != 0 || len(h.mapper.mapped) != 0 {
		t.Fatalf("status %+v, registered %d, mapped %v: want off and untouched", st, h.dir.registered, h.mapper.mapped)
	}
	if h.svc.RemoteURL() != "" {
		t.Error("RemoteURL is set while off")
	}
}

func TestTurnOnMapsRegistersAndGetsACertificate(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.svc.Update(ctx, Update{Enabled: on()}); err != nil {
		t.Fatal(err)
	}
	h.svc.refresh(ctx)
	st := h.svc.Status()
	if st.State != StateOK {
		t.Fatalf("state %q, status %+v", st.State, st)
	}
	if h.mapper.mapped[32501] != 32501 || st.ExternalPort != 32501 || st.PortMapping.Method != "upnp" {
		t.Errorf("mapping %v, status %+v", h.mapper.mapped, st.PortMapping)
	}
	if st.Hostname != "living-room-abc231.getplaid.direct" || st.URL != "https://living-room-abc231.getplaid.direct:32501" || st.PublicIP != "203.0.113.7" {
		t.Errorf("registration %+v", st)
	}
	if len(h.issuer.issued) != 1 || st.Certificate.State != StepOK || st.Certificate.At.IsZero() {
		t.Errorf("certificate %+v, issued %v", st.Certificate, h.issuer.issued)
	}
	if h.dir.challenge != nil {
		t.Errorf("challenge left behind: %v", h.dir.challenge)
	}
	if st.Reachability.State != StepOK {
		t.Errorf("reachability %+v", st.Reachability)
	}
	c, _ := h.svc.GetCertificate(nil)
	if c.Leaf.VerifyHostname(st.Hostname) != nil {
		t.Error("served certificate doesn't cover the hostname")
	}

	// The heartbeat reuses the registration and the certificate.
	h.svc.refresh(ctx)
	if h.dir.registered != 1 || len(h.issuer.issued) != 1 {
		t.Errorf("second refresh: registered %d times, %d certificates", h.dir.registered, len(h.issuer.issued))
	}
}

func TestRestartKeepsRegistrationAndCertificate(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, _ = h.svc.Update(ctx, Update{Enabled: on()})
	h.svc.refresh(ctx)
	data := h.svc.o.CertDir[:len(h.svc.o.CertDir)-len("/remote")]

	again := h.open(t, data)
	if again.RemoteURL() != "https://living-room-abc231.getplaid.direct:32501" {
		t.Errorf("RemoteURL after restart = %q", again.RemoteURL())
	}
	again.refresh(ctx)
	if h.dir.registered != 1 || len(h.issuer.issued) != 1 {
		t.Errorf("after restart: registered %d times, %d certificates", h.dir.registered, len(h.issuer.issued))
	}
}

func TestForgottenRegistrationRegistersAgain(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, _ = h.svc.Update(ctx, Update{Enabled: on()})
	h.svc.refresh(ctx)
	delete(h.dir.servers, "abc231") // expired on the service side
	h.svc.refresh(ctx)
	if st := h.svc.Status(); h.dir.registered != 2 || st.Hostname != "living-room-abc232.getplaid.direct" || len(h.issuer.issued) != 2 {
		t.Errorf("registered %d, hostname %q, certificates %v", h.dir.registered, st.Hostname, h.issuer.issued)
	}
}

func TestTakenPortFallsBackAndIsRemembered(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.mapper.takenBy = 32501
	_, _ = h.svc.Update(ctx, Update{Enabled: on()})
	h.svc.refresh(ctx)
	if st := h.svc.Status(); st.ExternalPort != 45678 || h.dir.ports["abc231"] != 45678 {
		t.Fatalf("external port %d, registered port %d", st.ExternalPort, h.dir.ports["abc231"])
	}
	if v, _, _ := h.store.Get(ctx, settings.RemoteExternalPort); v != "45678" {
		t.Errorf("saved external port %q", v)
	}
}

func TestNoRouterIsDegraded(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.svc.o.DiscoverMapper = func(context.Context) (Mapper, error) { return nil, errNoGateway }
	_, _ = h.svc.Update(ctx, Update{Enabled: on()})
	h.svc.refresh(ctx)
	st := h.svc.Status()
	if st.State != StateDegraded || st.PortMapping.State != StepFailed || st.PortMapping.Error != "no_gateway" || h.dir.registered != 0 {
		t.Errorf("status %+v, registered %d", st, h.dir.registered)
	}
}

func TestManualPortSkipsTheRouter(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.svc.Update(ctx, Update{Enabled: on(), PortMode: ptr(PortManual)}); !errors.Is(err, ErrInvalidManualPort) {
		t.Fatalf("manual without a port: err %v", err)
	}
	if _, err := h.svc.Update(ctx, Update{Enabled: on(), PortMode: ptr(PortManual), ManualPort: ptr(443)}); err != nil {
		t.Fatal(err)
	}
	h.svc.refresh(ctx)
	st := h.svc.Status()
	if st.ExternalPort != 443 || st.PortMapping.Method != PortManual || len(h.mapper.mapped) != 0 || h.dir.ports["abc231"] != 443 {
		t.Errorf("status %+v, mapped %v", st, h.mapper.mapped)
	}
}

func TestCertificateFailureBacksOff(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.issuer.err = errACME
	_, _ = h.svc.Update(ctx, Update{Enabled: on()})
	h.svc.refresh(ctx)
	if st := h.svc.Status(); st.Certificate.Error != "acme_failed" || st.State != StateDegraded {
		t.Fatalf("status %+v", st)
	}
	h.issuer.err = nil
	h.svc.refresh(ctx)
	if len(h.issuer.issued) != 0 {
		t.Error("retried before the backoff ended")
	}
	h.svc.o.Now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	h.svc.refresh(ctx)
	if len(h.issuer.issued) != 1 || h.svc.Status().State != StateOK {
		t.Errorf("after the backoff: issued %v, status %+v", h.issuer.issued, h.svc.Status())
	}
}

func TestUnreachableIsDegradedAndTestRechecks(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.svc.Test(ctx); !errors.Is(err, ErrOff) {
		t.Errorf("Test while off: %v", err)
	}
	h.dir.unreachable = true
	_, _ = h.svc.Update(ctx, Update{Enabled: on()})
	h.svc.refresh(ctx)
	if st := h.svc.Status(); st.State != StateDegraded || st.Reachability.Error != "timeout" {
		t.Fatalf("status %+v", st)
	}
	h.dir.unreachable = false
	st, err := h.svc.Test(ctx)
	if err != nil || st.Reachability.State != StepOK || st.Reachability.At.IsZero() {
		t.Errorf("Test: %+v, %v", st.Reachability, err)
	}
}

func TestTurnOffUnmapsAndUnregisters(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, _ = h.svc.Update(ctx, Update{Enabled: on()})
	h.svc.refresh(ctx)
	off := false
	_, _ = h.svc.Update(ctx, Update{Enabled: &off})
	h.svc.refresh(ctx)
	st := h.svc.Status()
	if st.State != StateOff || st.Hostname != "" || len(h.mapper.mapped) != 0 || len(h.dir.servers) != 0 {
		t.Errorf("status %+v, mapped %v, servers %v", st, h.mapper.mapped, h.dir.servers)
	}
	if v, ok, _ := h.store.Get(ctx, settings.RemoteDirectToken); ok {
		t.Errorf("token kept: %q", v)
	}
}

func TestGetCertificateFallsBackToSelfSigned(t *testing.T) {
	h := newHarness(t)
	c, err := h.svc.GetCertificate(nil)
	if err != nil || c == nil || c.Leaf.Subject.CommonName != "PLAID" {
		t.Fatalf("fallback certificate %v, %v", c, err)
	}
}

func ptr[T any](v T) *T { return &v }

func TestDirectClient(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/servers":
			var body serverBody
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Name != "Living room" || body.Port != 32501 || body.Version != "0.3.0" {
				t.Errorf("register body %+v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"k3x7qa","token":"secret","hostname":"living-room-k3x7qa.getplaid.direct","url":"https://living-room-k3x7qa.getplaid.direct:32501","shortUrl":"https://getplaid.direct/living-room-k3x7qa","publicIp":"203.0.113.7","port":32501}`))
		case r.URL.Path == "/api/v1/servers/k3x7qa" && r.Header.Get("Authorization") != "Bearer secret":
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/api/v1/servers/k3x7qa/reachability":
			_, _ = w.Write([]byte(`{"reachable":false,"error":"refused"}`))
		case r.URL.Path == "/api/v1/servers/k3x7qa/acme-challenge":
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	c := NewDirectClient(srv.URL + "/api/v1/")
	ctx := context.Background()
	reg, err := c.Register(ctx, "Living room", 32501, "0.3.0")
	if err != nil || reg.Token != "secret" || reg.ShortURL != "https://getplaid.direct/living-room-k3x7qa" {
		t.Fatalf("Register: %+v, %v", reg, err)
	}
	if _, err := c.Update(ctx, "k3x7qa", "wrong", "x", 1, "v"); !errors.Is(err, errUnauthorized) {
		t.Errorf("Update with a wrong token: %v", err)
	}
	if r, err := c.Reachability(ctx, "k3x7qa", "secret"); err != nil || r.Reachable || r.Error != "refused" {
		t.Errorf("Reachability: %+v, %v", r, err)
	}
	if err := c.SetChallenge(ctx, "k3x7qa", "secret", []string{"a"}); code(err) != "rate_limited" {
		t.Errorf("SetChallenge: %v", err)
	}
	if err := c.Unregister(ctx, "k3x7qa", "secret"); err != nil {
		t.Errorf("Unregister: %v", err)
	}
	srv.Close()
	if _, err := c.Register(ctx, "x", 1, "v"); code(err) != "direct_unreachable" {
		t.Errorf("Register on a closed server: %v", err)
	}
}
