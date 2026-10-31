package api

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// DefaultTrustedProxies are the ranges a reverse proxy on the same host or
// Docker network connects from: loopback, RFC 1918 and IPv6 ULA.
var DefaultTrustedProxies = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fc00::/7"),
}

// clientIP returns the address the request came from. When the peer is a
// trusted proxy, X-Forwarded-For is read from the right, skipping trusted
// hops: the first other entry is the one our proxy added, while anything to
// its left was written by the client and can be forged.
func (h *handler) clientIP(r *http.Request) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	peer = peer.Unmap().WithZone("")
	if !h.trusted(peer) {
		return peer, true
	}
	var hops []string
	for _, v := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(v, ",")...)
	}
	for i := len(hops) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			// A garbled hop: nothing to its left can be trusted either.
			return peer, true
		}
		a = a.Unmap().WithZone("")
		if !h.trusted(a) {
			return a, true
		}
		peer = a
	}
	// Every hop is trusted: the client is on the proxy's own network.
	return peer, true
}

func (h *handler) trusted(a netip.Addr) bool {
	for _, p := range h.cfg.TrustedProxies {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
