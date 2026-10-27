// Package httpx is the HTTP client every metadata provider goes through
// (F005 R32, R33, NFR security): HTTPS to allow-listed hosts only, a
// timeout and a size cap per request, a rate limit, Retry-After, and a
// circuit breaker that tracks whether the provider is reachable. API keys
// never reach the logs: URLs are redacted before they're logged or put in
// an error.
package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Provider states, as GET /metadata/status reports them (R35).
const (
	StateOK          = "ok"
	StateUnreachable = "unreachable"
	StateInvalidKey  = "invalid_key"
)

// Defaults from the spec.
const (
	DefaultTimeout = 10 * time.Second
	DefaultMaxBody = 20 << 20
	// failureThreshold consecutive failures open the circuit (R33).
	failureThreshold = 5
	// probeEvery is how often a request may go out while unreachable.
	probeEvery   = time.Minute
	maxRedirects = 5
)

// Errors callers act on.
var (
	ErrHostNotAllowed = errors.New("host not allowed")
	// ErrUnreachable is returned without a request while the circuit is open.
	ErrUnreachable = errors.New("provider unreachable")
	// ErrInvalidKey is returned after a 401, until ResetKey.
	ErrInvalidKey = errors.New("invalid API key")
	ErrNotFound   = errors.New("not found")
	ErrTooLarge   = errors.New("response too large")
	// ErrRequestFailed wraps network and transport errors.
	ErrRequestFailed = errors.New("request failed")
)

// StatusError is an unexpected HTTP status.
type StatusError struct {
	Code int
	URL  string // redacted
}

func (e *StatusError) Error() string { return fmt.Sprintf("GET %s: HTTP %d", e.URL, e.Code) }

// Options configure a Client.
type Options struct {
	// Name is the provider ("tmdb", "fanart"), for logs.
	Name string
	// Hosts are the only hosts requests and redirects may go to.
	Hosts []string
	// RPS and Burst bound the request rate; RPS 0 means no limit.
	RPS   float64
	Burst int
	// MaxConcurrent bounds requests in flight; 0 means no bound.
	MaxConcurrent int
	Timeout       time.Duration // 0 means DefaultTimeout
	MaxBody       int64         // 0 means DefaultMaxBody
	Log           *slog.Logger
	// Transport is the underlying transport; tests set it.
	Transport http.RoundTripper
	// Now is the clock; tests replace it.
	Now func() time.Time
}

// Status is a provider's health (R35).
type Status struct {
	State       string
	LastError   string
	LastErrorAt time.Time
}

// Client does GET requests for one provider.
type Client struct {
	o       Options
	http    *http.Client
	limiter *rate.Limiter
	sem     chan struct{}

	mu        sync.Mutex
	state     string
	failures  int
	lastProbe time.Time
	notBefore time.Time // from Retry-After
	lastErr   string
	lastErrAt time.Time
}

// New returns a client.
func New(o Options) *Client {
	if o.Timeout == 0 {
		o.Timeout = DefaultTimeout
	}
	if o.MaxBody == 0 {
		o.MaxBody = DefaultMaxBody
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Transport == nil {
		o.Transport = http.DefaultTransport
	}
	c := &Client{o: o, state: StateOK}
	c.http = &http.Client{
		Transport: o.Transport,
		Timeout:   o.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			return c.checkURL(req.URL)
		},
	}
	if o.RPS > 0 {
		c.limiter = rate.NewLimiter(rate.Limit(o.RPS), max(o.Burst, 1))
	}
	if o.MaxConcurrent > 0 {
		c.sem = make(chan struct{}, o.MaxConcurrent)
	}
	return c
}

func (c *Client) checkURL(u *url.URL) error {
	if u.Scheme != "https" || !slices.Contains(c.o.Hosts, u.Hostname()) {
		return fmt.Errorf("%w: %s", ErrHostNotAllowed, Redact(u.String()))
	}
	return nil
}

// Get fetches rawURL and returns its body. header may be nil.
func (c *Client) Get(ctx context.Context, rawURL string, header http.Header) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("bad URL: %w", err)
	}
	if err := c.checkURL(u); err != nil {
		return nil, err
	}
	if err := c.admit(); err != nil {
		return nil, err
	}
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	if c.sem != nil {
		select {
		case c.sem <- struct{}{}:
			defer func() { <-c.sem }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	redacted := Redact(u.String())
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var uerr *url.Error
		if errors.As(err, &uerr) && errors.Is(uerr.Err, ErrHostNotAllowed) {
			return nil, uerr.Err
		}
		// Never wrap err itself: *url.Error carries the unredacted URL.
		c.failure(fmt.Sprintf("GET %s: %s", redacted, describe(err)))
		return nil, fmt.Errorf("%w: GET %s: %s", ErrRequestFailed, redacted, describe(err))
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
		body, err := io.ReadAll(io.LimitReader(resp.Body, c.o.MaxBody+1))
		if err != nil {
			c.failure(fmt.Sprintf("GET %s: reading body: %s", redacted, describe(err)))
			return nil, fmt.Errorf("%w: GET %s: reading body: %s", ErrRequestFailed, redacted, describe(err))
		}
		if int64(len(body)) > c.o.MaxBody {
			c.success()
			return nil, fmt.Errorf("%w: GET %s", ErrTooLarge, redacted)
		}
		c.success()
		return body, nil
	case resp.StatusCode == http.StatusNotFound:
		c.success()
		return nil, fmt.Errorf("%w: GET %s", ErrNotFound, redacted)
	case resp.StatusCode == http.StatusUnauthorized:
		c.invalidKey(redacted)
		return nil, ErrInvalidKey
	case resp.StatusCode == http.StatusTooManyRequests:
		c.retryAfter(resp.Header.Get("Retry-After"))
		return nil, &StatusError{Code: resp.StatusCode, URL: redacted}
	case resp.StatusCode >= 500:
		c.failure(fmt.Sprintf("GET %s: HTTP %d", redacted, resp.StatusCode))
		return nil, &StatusError{Code: resp.StatusCode, URL: redacted}
	default:
		c.success()
		return nil, &StatusError{Code: resp.StatusCode, URL: redacted}
	}
}

// describe keeps the kind of a transport error without any URL in it.
func describe(err error) string {
	var uerr *url.Error
	if errors.As(err, &uerr) {
		err = uerr.Err
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Client.Timeout") {
		return "timeout"
	}
	return err.Error()
}

// admit applies the circuit state before a request.
func (c *Client) admit() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.state {
	case StateInvalidKey:
		return ErrInvalidKey
	case StateUnreachable:
		now := c.o.Now()
		if now.Sub(c.lastProbe) < probeEvery {
			return ErrUnreachable
		}
		c.lastProbe = now
	}
	return nil
}

// wait honors Retry-After and the rate limit.
func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	until := c.notBefore
	c.mu.Unlock()
	if d := until.Sub(c.o.Now()); d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if c.limiter != nil {
		return c.limiter.Wait(ctx)
	}
	return nil
}

func (c *Client) success() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == StateUnreachable {
		c.o.Log.Info("metadata provider reachable again", "provider", c.o.Name)
	}
	c.failures = 0
	if c.state != StateInvalidKey {
		c.state = StateOK
	}
}

func (c *Client) failure(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures++
	c.lastErr, c.lastErrAt = msg, c.o.Now()
	if c.failures >= failureThreshold && c.state == StateOK {
		c.state = StateUnreachable
		c.lastProbe = c.o.Now()
		c.o.Log.Warn("metadata provider unreachable, probing once a minute", "provider", c.o.Name, "err", msg)
	}
}

func (c *Client) invalidKey(redacted string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = StateInvalidKey
	c.lastErr, c.lastErrAt = "GET "+redacted+": HTTP 401", c.o.Now()
	c.o.Log.Warn("metadata provider rejected the API key", "provider", c.o.Name)
}

func (c *Client) retryAfter(h string) {
	d := time.Second
	if s, err := strconv.Atoi(strings.TrimSpace(h)); err == nil && s > 0 {
		d = time.Duration(s) * time.Second
	} else if t, err := http.ParseTime(h); err == nil {
		d = t.Sub(c.o.Now())
	}
	d = min(max(d, time.Second), 5*time.Minute)
	c.mu.Lock()
	defer c.mu.Unlock()
	if until := c.o.Now().Add(d); until.After(c.notBefore) {
		c.notBefore = until
	}
}

// ResetKey clears the invalid-key state after the key changed.
func (c *Client) ResetKey() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == StateInvalidKey {
		c.state = StateOK
		c.failures = 0
	}
}

// Status returns the provider's current health.
func (c *Client) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Status{State: c.state, LastError: c.lastErr, LastErrorAt: c.lastErrAt}
}

// secretParams are query parameters that carry keys.
var secretParams = []string{"api_key", "client_key"}

// Redact replaces API keys in a URL's query with REDACTED.
func Redact(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "(unparsable URL)"
	}
	q := u.Query()
	changed := false
	for _, p := range secretParams {
		if q.Has(p) {
			q.Set(p, "REDACTED")
			changed = true
		}
	}
	if changed {
		u.RawQuery = q.Encode()
	}
	return u.String()
}
