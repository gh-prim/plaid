package auth

import (
	"encoding/base64"
	"net/netip"
	"strings"
	"testing"
)

func TestNewToken(t *testing.T) {
	a, b := NewToken(), NewToken()
	if a == b {
		t.Fatal("two tokens are equal")
	}
	raw, err := base64.RawURLEncoding.DecodeString(a)
	if err != nil || len(raw) != 32 {
		t.Errorf("token %q: %d bytes, err %v; want 32 bytes base64url", a, len(raw), err)
	}
	if len(HashToken(a)) != 32 {
		t.Error("HashToken is not SHA-256")
	}
}

func TestNewCode(t *testing.T) {
	seen := map[rune]bool{}
	for range 200 {
		c := NewCode(6)
		if len(c) != 6 {
			t.Fatalf("code %q has length %d", c, len(c))
		}
		for _, r := range c {
			if !strings.ContainsRune(codeAlphabet, r) {
				t.Fatalf("code %q has %q outside the alphabet", c, r)
			}
			seen[r] = true
		}
	}
	for _, bad := range "0O1IL" {
		if strings.ContainsRune(codeAlphabet, bad) {
			t.Errorf("alphabet contains ambiguous %q", bad)
		}
	}
	if len(seen) < 25 {
		t.Errorf("only %d distinct characters in 1,200 draws", len(seen))
	}
}

func TestIsLocal(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1":       true,
		"127.9.9.9":       true,
		"::1":             true,
		"10.1.2.3":        true,
		"172.16.0.1":      true,
		"172.31.255.255":  true,
		"192.168.1.1":     true,
		"fd12:3456::1":    true,
		"::ffff:10.0.0.1": true,
		"172.32.0.1":      false,
		"8.8.8.8":         false,
		"100.64.0.1":      false, // CGNAT is not RFC 1918
		"2001:db8::1":     false,
		"fe80::1":         false,
		"169.254.10.10":   false,
	} {
		if got := IsLocal(netip.MustParseAddr(addr)); got != want {
			t.Errorf("IsLocal(%s) = %v, want %v", addr, got, want)
		}
	}
	if IsLocal(netip.Addr{}) {
		t.Error("IsLocal(invalid) = true")
	}
}
