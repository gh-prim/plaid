// Package clientinfo works out who is on the other end of a request: the
// client IP and whether it reached us over HTTPS. X-Forwarded-* headers are
// believed only when they come from a trusted proxy (PLAID_TRUSTED_PROXIES).
package clientinfo

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Info describes the client of one request.
type Info struct {
	// IP is the client address, unmapped (IPv4 stays IPv4). Invalid if unknown.
	IP netip.Addr
	// HTTPS is true when the client used HTTPS, directly or through a trusted proxy.
	HTTPS bool
	// UntrustedForward is true when an untrusted peer sent forwarding headers.
	// The request may come through a proxy we don't know about, so IP may be
	// the proxy's, not the client's (F003 setup locality check).
	UntrustedForward bool
}

type ctxKey struct{}

// FromContext returns the Info stored by Middleware. The zero Info (invalid
// IP, not HTTPS) is returned when there is none.
func FromContext(ctx context.Context) Info {
	info, _ := ctx.Value(ctxKey{}).(Info)
	return info
}

// WithInfo returns a context carrying info. Tests use it to fake a client.
func WithInfo(ctx context.Context, info Info) context.Context {
	return context.WithValue(ctx, ctxKey{}, info)
}

// Middleware computes the Info of every request and stores it in its context.
func Middleware(trusted []netip.Prefix, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(WithInfo(r.Context(), Resolve(r, trusted))))
	})
}

// Resolve computes the Info of r.
func Resolve(r *http.Request, trusted []netip.Prefix) Info {
	peer := parseAddr(r.RemoteAddr)
	info := Info{IP: peer, HTTPS: r.TLS != nil}

	forwardedFor := headerList(r.Header.Values("X-Forwarded-For"))
	if !isTrusted(peer, trusted) {
		info.UntrustedForward = len(forwardedFor) > 0 ||
			r.Header.Get("Forwarded") != "" || r.Header.Get("X-Real-IP") != ""
		return info
	}

	// Walk the chain from the closest hop: the first address that isn't one
	// of our proxies is the client. Everything left of it is client-supplied.
	for i := len(forwardedFor) - 1; i >= 0; i-- {
		addr := parseAddr(forwardedFor[i])
		if !addr.IsValid() {
			break
		}
		info.IP = addr
		if !isTrusted(addr, trusted) {
			break
		}
	}
	if protos := headerList(r.Header.Values("X-Forwarded-Proto")); len(protos) > 0 {
		info.HTTPS = strings.EqualFold(protos[len(protos)-1], "https")
	}
	return info
}

func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	if !addr.IsValid() {
		return false
	}
	for _, p := range trusted {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// parseAddr accepts "ip", "ip:port" and "[ipv6]:port".
func parseAddr(s string) netip.Addr {
	s = strings.TrimSpace(s)
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	addr, err := netip.ParseAddr(strings.Trim(s, "[]"))
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap().WithZone("")
}

// headerList flattens comma-separated header values into one list.
func headerList(values []string) []string {
	var out []string
	for _, v := range values {
		for _, item := range strings.Split(v, ",") {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
	}
	return out
}
