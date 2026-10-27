package playback

import (
	"net/netip"
	"testing"
)

func TestIsRemote(t *testing.T) {
	tailnet := []netip.Prefix{netip.MustParsePrefix("100.64.0.0/10")}
	cases := []struct {
		ip     string
		local  []netip.Prefix
		remote bool
	}{
		{"127.0.0.1", nil, false},
		{"::1", nil, false},
		{"192.168.1.2", nil, false},
		{"10.0.0.7", nil, false},
		{"fd00::1", nil, false},
		{"fe80::1", nil, false},
		{"169.254.10.1", nil, false},
		{"::ffff:192.168.1.2", nil, false},
		{"8.8.8.8", nil, true},
		{"100.101.102.103", nil, true},
		{"100.101.102.103", tailnet, false},
		{"2001:db8::1", tailnet, true},
	}
	for _, c := range cases {
		if got := IsRemote(netip.MustParseAddr(c.ip), c.local); got != c.remote {
			t.Errorf("IsRemote(%s, %v) = %v, want %v", c.ip, c.local, got, c.remote)
		}
	}
	if !IsRemote(netip.Addr{}, nil) {
		t.Error("an unknown address must count as remote")
	}
}
