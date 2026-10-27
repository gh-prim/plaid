package auth

import "net/netip"

// IsLocal reports whether addr is loopback or private: 127.0.0.0/8, ::1,
// RFC 1918 and RFC 4193 (F003-R3).
func IsLocal(addr netip.Addr) bool {
	addr = addr.Unmap()
	return addr.IsValid() && (addr.IsLoopback() || addr.IsPrivate())
}
