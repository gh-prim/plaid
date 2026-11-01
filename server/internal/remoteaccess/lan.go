package remoteaccess

import (
	"context"
	"net"
	"net/netip"
)

// LANAddr returns the private IPv4 address this machine uses to reach the
// internet, or "" when that address isn't private (a VPS, or no network).
// Dialing UDP sends no packet: it only asks the kernel for the route.
func LANAddr() string {
	c, err := (&net.Dialer{}).DialContext(context.Background(), "udp4", "192.0.2.1:9") // TEST-NET-1, never routed for real
	if err != nil {
		return ""
	}
	defer func() { _ = c.Close() }()
	a, ok := c.LocalAddr().(*net.UDPAddr)
	if !ok {
		return ""
	}
	ip, ok := netip.AddrFromSlice(a.IP)
	if !ok {
		return ""
	}
	ip = ip.Unmap()
	if !ip.IsPrivate() && !cgnat.Contains(ip) {
		return ""
	}
	return ip.String()
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")
