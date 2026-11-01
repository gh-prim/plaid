// Package probe checks from the outside whether a PLAID server answers on
// its public address, the way a remote client would reach it.
package probe

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Failure codes, as in the protocol.
const (
	ErrTimeout  = "timeout"
	ErrRefused  = "refused"
	ErrNotPlaid = "not_plaid"
	ErrTLS      = "tls"
)

// DialFunc opens a connection; tests inject one.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// Prober runs reachability checks.
type Prober struct {
	// Dial defaults to a net.Dialer.
	Dial DialFunc
	// Timeout bounds the whole check; 5 s when zero.
	Timeout time.Duration
	// AllowPrivate lets tests probe loopback and private addresses. In
	// production it stays false: the service must not become a way to scan
	// its own network (SSRF).
	AllowPrivate bool
}

// Result is the outcome of one check.
type Result struct {
	Reachable bool
	Error     string // one of the Err* codes when not reachable
}

// Check opens https://ip:port/api/v1/health without verifying TLS and
// reports whether a PLAID server answered (a JSON body with "version").
func (p *Prober) Check(ctx context.Context, ip netip.Addr, port int) Result {
	ip = ip.Unmap()
	if !p.AllowPrivate && !Public(ip) {
		return Result{Error: ErrRefused}
	}
	timeout := p.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dial := p.Dial
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: dial,
			// The certificate is for the server's hostname, which may not
			// resolve yet: verifying it here would answer a different question.
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // see above
			DisableKeepAlives: true,
			ForceAttemptHTTP2: false,
		},
		// A redirect could point the probe anywhere; a PLAID server doesn't redirect health checks.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()

	u := "https://" + net.JoinHostPort(ip.String(), strconv.Itoa(port)) + "/api/v1/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Result{Error: ErrRefused}
	}
	req.Header.Set("User-Agent", "getplaid.direct reachability probe")
	resp, err := client.Do(req)
	if err != nil {
		return Result{Error: classify(ctx, err)}
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		Version *string `json:"version"`
	}
	if resp.StatusCode != http.StatusOK ||
		json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body) != nil ||
		body.Version == nil {
		return Result{Error: ErrNotPlaid}
	}
	return Result{Reachable: true}
}

func classify(ctx context.Context, err error) string {
	var (
		recordErr tls.RecordHeaderError
		certErr   *tls.CertificateVerificationError
		alertErr  tls.AlertError
		netErr    net.Error
	)
	switch {
	case errors.As(err, &recordErr), errors.As(err, &certErr), errors.As(err, &alertErr), tlsHint(err):
		// Something answered, but not with TLS: a plain-HTTP service, or a
		// handshake that failed.
		return ErrTLS
	case errors.Is(err, syscall.ECONNREFUSED), errors.Is(err, syscall.ECONNRESET):
		return ErrRefused
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, os.ErrDeadlineExceeded),
		ctx.Err() != nil, errors.As(err, &netErr) && netErr.Timeout():
		return ErrTimeout
	default:
		return ErrRefused
	}
}

// tlsHint catches handshake failures that crypto/tls and net/http only
// report as text (net/http turns a plain-HTTP answer into a string error).
func tlsHint(err error) bool {
	s := err.Error()
	return strings.Contains(s, "tls: ") || strings.Contains(s, "TLS handshake") ||
		strings.Contains(s, "HTTP response to HTTPS client")
}

// Public reports whether ip is a globally routable unicast address, i.e.
// not loopback, private (RFC 1918, ULA), CGNAT, link-local, multicast or
// unspecified.
func Public(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, p := range nonPublic {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

var nonPublic = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // CGNAT
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2001:db8::/32"), // documentation
}

// String makes Result readable in test failures.
func (r Result) String() string { return fmt.Sprintf("reachable=%v error=%q", r.Reachable, r.Error) }
