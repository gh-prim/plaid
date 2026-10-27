package playback

import "net/netip"

// IsRemote reports whether a client counts as outside the local networks
// for the remote bitrate limit (R22): anything but loopback, private
// (RFC 1918, RFC 4193) and link-local addresses, unless the admin lists it.
// Tailscale and other CGNAT addresses (100.64.0.0/10) stay remote unless
// listed: they're mostly used from far away.
func IsRemote(ip netip.Addr, localNetworks []netip.Prefix) bool {
	ip = ip.Unmap()
	if !ip.IsValid() {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, p := range localNetworks {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}
