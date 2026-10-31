// Package remoteaccess makes the server reachable from outside the home
// network without an IP or a port to type (F012): it opens a port on the
// router (UPnP or NAT-PMP, or a port the admin forwarded by hand),
// registers a hostname with getplaid.direct, keeps it pointed at the
// home's public IP, and gets a Let's Encrypt certificate for it.
package remoteaccess

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"log/slog"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/gh-prim/plaid/server/internal/settings"
)

// Port modes.
const (
	PortAuto   = "auto"
	PortManual = "manual"
)

// States of the whole feature and of each step.
const (
	StateOff      = "off"
	StateStarting = "starting"
	StateOK       = "ok"
	StateDegraded = "degraded"

	StepOff     = "off"
	StepPending = "pending"
	StepOK      = "ok"
	StepFailed  = "failed"
)

const (
	defaultRefresh = 15 * time.Minute
	// renewBefore renews a certificate a month before it expires, as
	// Let's Encrypt recommends for 90-day certificates.
	renewBefore = 30 * 24 * time.Hour
	maxBackoff  = 24 * time.Hour
)

// Options configure the service.
type Options struct {
	Settings  *settings.Store
	Directory Directory
	Issuer    Issuer
	// DiscoverMapper finds the router; DiscoverMapper (the function) in
	// production.
	DiscoverMapper func(ctx context.Context) (Mapper, error)
	ServerName     string
	Version        string
	// TLSPort is the local HTTPS port the router forwards to.
	TLSPort int
	// CertDir holds the certificate and the ACME account key.
	CertDir string
	Log     *slog.Logger
	// Refresh is the heartbeat period; 15 minutes when zero.
	Refresh time.Duration
	Now     func() time.Time
}

// Step is the state of one part of the setup.
type Step struct {
	State  string
	Method string
	// Error is a stable code clients translate.
	Error string
	// At is when the certificate expires, or when the check ran.
	At time.Time
}

// Status is what the admin page shows.
type Status struct {
	Enabled      bool
	PortMode     string
	ManualPort   int
	InternalPort int
	ExternalPort int
	PublicIP     string
	State        string
	PortMapping  Step
	Certificate  Step
	Reachability Step
	Hostname     string
	URL          string
	ShortURL     string
}

// Update changes the admin's choices; nil fields keep their value.
type Update struct {
	Enabled    *bool
	PortMode   *string
	ManualPort *int
}

// ErrInvalidManualPort is returned when manual mode has no port.
var ErrInvalidManualPort = errors.New("manual port mode needs a port between 1 and 65535")

// ErrOff is returned by Test when there is nothing to test.
var ErrOff = errors.New("remote access is off")

// Service runs remote access. Its zero value isn't usable: call New.
type Service struct {
	o    Options
	kick chan struct{}

	mu       sync.Mutex
	st       Status
	id       string
	token    string
	mapper   Mapper
	mappedAt int // the external port currently mapped, 0 when none
	cert     *tls.Certificate
	fallback *tls.Certificate
	retryAt  time.Time // no certificate order before this
	backoff  time.Duration
}

// New loads the saved settings and certificate.
func New(ctx context.Context, o Options) (*Service, error) {
	if o.Refresh == 0 {
		o.Refresh = defaultRefresh
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.DiscoverMapper == nil {
		o.DiscoverMapper = DiscoverMapper
	}
	s := &Service{o: o, kick: make(chan struct{}, 1)}
	get := func(k string) string {
		v, _, _ := o.Settings.Get(ctx, k)
		return v
	}
	s.st = Status{
		Enabled: get(settings.RemoteEnabled) == "true", PortMode: get(settings.RemotePortMode),
		InternalPort: o.TLSPort, Hostname: get(settings.RemoteHostname),
		URL: get(settings.RemoteURL), ShortURL: get(settings.RemoteShortURL),
	}
	if s.st.PortMode != PortManual {
		s.st.PortMode = PortAuto
	}
	s.st.ManualPort, _ = strconv.Atoi(get(settings.RemoteManualPort))
	s.st.ExternalPort, _ = strconv.Atoi(get(settings.RemoteExternalPort))
	s.id, s.token = get(settings.RemoteDirectID), get(settings.RemoteDirectToken)
	cert, err := loadCertificate(o.CertDir)
	if err != nil {
		o.Log.Warn("remote access: ignoring the saved certificate", "err", err)
	}
	s.cert = cert
	s.resetSteps()
	return s, nil
}

// resetSteps derives the steps' resting state from the settings. Caller
// holds mu.
func (s *Service) resetSteps() {
	if !s.st.Enabled {
		s.st.State = StateOff
		s.st.PortMapping, s.st.Reachability = Step{State: StepOff}, Step{State: StepOff}
		s.st.Certificate = Step{State: StepOff}
		return
	}
	s.st.State = StateStarting
	s.st.PortMapping, s.st.Reachability = Step{State: StepPending}, Step{State: StepPending}
	s.st.Certificate = Step{State: StepPending}
	if s.cert != nil && s.cert.Leaf != nil {
		s.st.Certificate = Step{State: StepOK, Method: "letsencrypt", At: s.cert.Leaf.NotAfter}
	}
}

// Status returns the current status.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st
}

// RemoteURL is the address apps remember, "" when remote access is off.
func (s *Service) RemoteURL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.st.Enabled {
		return ""
	}
	return s.st.URL
}

// Update saves the admin's choices and applies them in the background.
func (s *Service) Update(ctx context.Context, u Update) (Status, error) {
	s.mu.Lock()
	next := s.st
	s.mu.Unlock()
	if u.Enabled != nil {
		next.Enabled = *u.Enabled
	}
	if u.PortMode != nil {
		next.PortMode = *u.PortMode
	}
	if u.ManualPort != nil {
		next.ManualPort = *u.ManualPort
	}
	if next.PortMode == PortManual && (next.ManualPort < 1 || next.ManualPort > 65535) {
		return Status{}, ErrInvalidManualPort
	}
	st := s.o.Settings
	for k, v := range map[string]string{
		settings.RemoteEnabled: strconv.FormatBool(next.Enabled), settings.RemotePortMode: next.PortMode,
		settings.RemoteManualPort: strconv.Itoa(next.ManualPort),
	} {
		if err := st.Set(ctx, k, v); err != nil {
			return Status{}, err
		}
	}
	s.mu.Lock()
	changed := s.st.Enabled != next.Enabled || s.st.PortMode != next.PortMode || s.st.ManualPort != next.ManualPort
	s.st.Enabled, s.st.PortMode, s.st.ManualPort = next.Enabled, next.PortMode, next.ManualPort
	if changed {
		s.resetSteps()
	}
	out := s.st
	s.mu.Unlock()
	if changed {
		s.Kick()
	}
	return out, nil
}

// Kick runs a refresh now rather than at the next tick.
func (s *Service) Kick() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// Test asks getplaid.direct to reach the server and records the answer.
func (s *Service) Test(ctx context.Context) (Status, error) {
	s.mu.Lock()
	enabled, id, token := s.st.Enabled, s.id, s.token
	s.mu.Unlock()
	if !enabled || id == "" {
		return Status{}, ErrOff
	}
	s.checkReachability(ctx, id, token)
	return s.Status(), nil
}

// Run keeps remote access in the wanted state until ctx ends, then
// closes the port it opened.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(s.o.Refresh)
	defer t.Stop()
	for {
		s.refresh(ctx)
		select {
		case <-ctx.Done():
			s.unmap(context.WithoutCancel(ctx))
			return
		case <-t.C:
		case <-s.kick:
		}
	}
}

// refresh brings everything in line with the settings: one pass, each
// step recording its own result.
func (s *Service) refresh(ctx context.Context) {
	s.mu.Lock()
	enabled := s.st.Enabled
	s.mu.Unlock()
	if !enabled {
		s.turnOff(ctx)
		return
	}
	ext, ok := s.openPort(ctx)
	if !ok {
		s.finish()
		return
	}
	id, token, ok := s.register(ctx, ext)
	if !ok {
		s.finish()
		return
	}
	s.ensureCertificate(ctx, id, token)
	s.checkReachability(ctx, id, token)
	s.finish()
}

// finish sums the steps into the overall state.
func (s *Service) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.st.Enabled {
		return
	}
	s.st.State = StateOK
	for _, st := range []Step{s.st.PortMapping, s.st.Certificate, s.st.Reachability} {
		if st.State == StepFailed {
			s.st.State = StateDegraded
		}
	}
}

func (s *Service) openPort(ctx context.Context) (int, bool) {
	s.mu.Lock()
	mode, manual, prev := s.st.PortMode, s.st.ManualPort, s.st.ExternalPort
	s.mu.Unlock()
	if mode == PortManual {
		s.unmap(ctx) // a mapping from auto mode would only confuse
		s.setPort(ctx, manual, Step{State: StepOK, Method: PortManual})
		return manual, true
	}
	s.mu.Lock()
	m := s.mapper
	s.mu.Unlock()
	if m == nil {
		var err error
		if m, err = s.o.DiscoverMapper(ctx); err != nil {
			s.o.Log.Warn("remote access: no router answered UPnP or NAT-PMP; forward a port by hand", "code", "remote_no_gateway")
			s.setPort(ctx, prev, Step{State: StepFailed, Error: code(err)})
			return 0, false
		}
		s.mu.Lock()
		s.mapper = m
		s.mu.Unlock()
	}
	want := prev
	if want == 0 {
		want = s.o.TLSPort
	}
	ext, err := m.Map(ctx, s.o.TLSPort, want)
	if err != nil {
		s.o.Log.Warn("remote access: the router refused the port mapping", "code", "remote_mapping_refused", "method", m.Method(), "err", err)
		s.mu.Lock()
		s.mapper = nil // rediscover: the router may have changed
		s.mu.Unlock()
		s.setPort(ctx, prev, Step{State: StepFailed, Method: m.Method(), Error: code(err)})
		return 0, false
	}
	s.mu.Lock()
	s.mappedAt = ext
	s.mu.Unlock()
	s.setPort(ctx, ext, Step{State: StepOK, Method: m.Method()})
	return ext, true
}

func (s *Service) setPort(ctx context.Context, ext int, step Step) {
	s.mu.Lock()
	changed := s.st.ExternalPort != ext
	s.st.ExternalPort, s.st.PortMapping = ext, step
	s.mu.Unlock()
	if changed && ext != 0 {
		_ = s.o.Settings.Set(ctx, settings.RemoteExternalPort, strconv.Itoa(ext))
	}
}

func (s *Service) unmap(ctx context.Context) {
	s.mu.Lock()
	m, ext := s.mapper, s.mappedAt
	s.mappedAt = 0
	s.mu.Unlock()
	if m != nil && ext != 0 {
		if err := m.Unmap(ctx, ext); err != nil {
			s.o.Log.Debug("remote access: removing the port mapping", "err", err)
		}
	}
}

// register registers the server with getplaid.direct, or sends the
// heartbeat that keeps its hostname on the current public IP.
func (s *Service) register(ctx context.Context, ext int) (id, token string, ok bool) {
	s.mu.Lock()
	id, token = s.id, s.token
	s.mu.Unlock()
	var reg Registration
	var err error
	if id != "" {
		reg, err = s.o.Directory.Update(ctx, id, token, s.o.ServerName, ext, s.o.Version)
		if errors.Is(err, errUnauthorized) {
			// Expired after 30 days unseen, or the service lost it.
			id, token = "", ""
		}
	}
	if id == "" {
		reg, err = s.o.Directory.Register(ctx, s.o.ServerName, ext, s.o.Version)
		if err == nil {
			id, token = reg.ID, reg.Token
			_ = s.o.Settings.Set(ctx, settings.RemoteDirectID, id)
			_ = s.o.Settings.Set(ctx, settings.RemoteDirectToken, token)
			s.o.Log.Info("remote access: registered with getplaid.direct", "hostname", reg.Hostname)
		}
	}
	if err != nil {
		s.o.Log.Warn("remote access: getplaid.direct", "code", "remote_direct_failed", "err", err)
		s.mu.Lock()
		s.st.Reachability = Step{State: StepFailed, Error: code(err)}
		s.mu.Unlock()
		return "", "", false
	}
	for k, v := range map[string]string{settings.RemoteHostname: reg.Hostname, settings.RemoteURL: reg.URL, settings.RemoteShortURL: reg.ShortURL} {
		_ = s.o.Settings.Set(ctx, k, v)
	}
	s.mu.Lock()
	if s.st.Hostname != reg.Hostname {
		s.retryAt, s.backoff = time.Time{}, 0 // a new name deserves a fresh try
	}
	s.id, s.token = id, token
	s.st.Hostname, s.st.URL, s.st.ShortURL, s.st.PublicIP = reg.Hostname, reg.URL, reg.ShortURL, reg.PublicIP
	s.mu.Unlock()
	return id, token, true
}

func (s *Service) ensureCertificate(ctx context.Context, id, token string) {
	s.mu.Lock()
	host, cert, retryAt := s.st.Hostname, s.cert, s.retryAt
	s.mu.Unlock()
	now := s.o.Now()
	if cert != nil && cert.Leaf != nil && cert.Leaf.VerifyHostname(host) == nil && cert.Leaf.NotAfter.Sub(now) > renewBefore {
		s.mu.Lock()
		s.st.Certificate = Step{State: StepOK, Method: "letsencrypt", At: cert.Leaf.NotAfter}
		s.mu.Unlock()
		return
	}
	if now.Before(retryAt) {
		return // keep the last failure on show until the backoff ends
	}
	s.mu.Lock()
	s.st.Certificate = Step{State: StepPending}
	s.mu.Unlock()
	c, err := s.o.Issuer.Issue(ctx, host,
		func(ctx context.Context, v []string) error { return s.o.Directory.SetChallenge(ctx, id, token, v) },
		func(ctx context.Context) error { return s.o.Directory.ClearChallenge(ctx, id, token) })
	if err == nil {
		err = saveCertificate(s.o.CertDir, c)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.backoff = min(max(2*s.backoff, time.Hour), maxBackoff)
		s.retryAt = now.Add(s.backoff)
		s.o.Log.Warn("remote access: no certificate yet", "code", "remote_acme_failed", "hostname", host, "retry_in", s.backoff, "err", err)
		s.st.Certificate = Step{State: StepFailed, Error: code(err)}
		return
	}
	s.cert, s.retryAt, s.backoff = c, time.Time{}, 0
	s.st.Certificate = Step{State: StepOK, Method: "letsencrypt", At: c.Leaf.NotAfter}
	s.o.Log.Info("remote access: certificate ready", "hostname", host, "expires", c.Leaf.NotAfter)
}

func (s *Service) checkReachability(ctx context.Context, id, token string) {
	r, err := s.o.Directory.Reachability(ctx, id, token)
	step := Step{State: StepOK, At: s.o.Now()}
	switch {
	case err != nil:
		step.State, step.Error = StepFailed, code(err)
	case !r.Reachable:
		step.State, step.Error = StepFailed, r.Error
	}
	s.mu.Lock()
	s.st.Reachability = step
	s.mu.Unlock()
}

// turnOff closes the port and gives the hostname back.
func (s *Service) turnOff(ctx context.Context) {
	s.unmap(ctx)
	s.mu.Lock()
	id, token := s.id, s.token
	s.id, s.token, s.mapper = "", "", nil
	s.st.Hostname, s.st.URL, s.st.ShortURL, s.st.PublicIP = "", "", "", ""
	s.mu.Unlock()
	if id == "" {
		return
	}
	if err := s.o.Directory.Unregister(ctx, id, token); err != nil {
		s.o.Log.Warn("remote access: unregistering from getplaid.direct; the name expires after 30 days anyway", "err", err)
	}
	for _, k := range []string{settings.RemoteDirectID, settings.RemoteDirectToken, settings.RemoteHostname, settings.RemoteURL, settings.RemoteShortURL} {
		_ = s.o.Settings.Delete(ctx, k)
	}
}

// GetCertificate serves the Let's Encrypt certificate, or a self-signed
// one until there is one: getplaid.direct's reachability check doesn't
// verify it, and nothing else should come before the hostname works.
func (s *Service) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cert != nil {
		return s.cert, nil
	}
	if s.fallback == nil {
		c, err := selfSigned(s.o.Now())
		if err != nil {
			return nil, err
		}
		s.fallback = c
	}
	return s.fallback, nil
}

func selfSigned(now time.Time, names ...string) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()), Subject: pkix.Name{CommonName: "PLAID"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(365 * 24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames: names,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	return certificate([][]byte{der}, key)
}

// code turns an error into the code clients translate.
func code(err error) string {
	for _, e := range []error{errNoGateway, errMappingRefused, errRateLimited, errACME, errDirectUnreachable} {
		if errors.Is(err, e) {
			return e.Error()
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return errDirectUnreachable.Error()
}
