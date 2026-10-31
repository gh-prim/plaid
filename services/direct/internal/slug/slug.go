// Package slug builds and parses server labels: "<slug>-<id>", the first
// label of every hostname under the zone.
package slug

import (
	"crypto/rand"
	"strings"
)

const (
	// MaxLen is the longest slug: labels stay short enough to read aloud.
	MaxLen = 20
	// IDLen is the length of a server id.
	IDLen = 6
	// Fallback is the slug of a name with no usable character.
	Fallback = "plaid"

	idAlphabet = "abcdefghijklmnopqrstuvwxyz234567" // base32, lowercase
)

// Make turns a server name into a slug: lowercased, every run of characters
// outside [a-z0-9] collapsed into one "-", trimmed of "-", cut to MaxLen.
func Make(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
			continue
		}
		dash = true
	}
	s := b.String()
	if len(s) > MaxLen {
		// Cutting can leave a trailing dash ("living-room-" from a longer name).
		s = strings.TrimRight(s[:MaxLen], "-")
	}
	if s == "" {
		return Fallback
	}
	return s
}

// NewID returns IDLen random characters from [a-z2-7]. 256 is a multiple of
// 32, so taking each byte modulo 32 has no bias.
func NewID() string {
	b := make([]byte, IDLen)
	_, _ = rand.Read(b) // crypto/rand.Read never fails (Go 1.24+)
	for i := range b {
		b[i] = idAlphabet[b[i]%32]
	}
	return string(b)
}

// Label joins a slug and an id.
func Label(slug, id string) string { return slug + "-" + id }

// ValidID reports whether id has the shape of a server id.
func ValidID(id string) bool {
	if len(id) != IDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		if !strings.ContainsRune(idAlphabet, rune(id[i])) {
			return false
		}
	}
	return true
}

// Parse splits a label into its slug and id. It only checks the shape: the
// caller still has to look the id up and compare the slug.
func Parse(label string) (slug, id string, ok bool) {
	i := strings.LastIndexByte(label, '-')
	if i < 1 {
		return "", "", false
	}
	slug, id = label[:i], label[i+1:]
	if !ValidID(id) || !validSlug(slug) {
		return "", "", false
	}
	return slug, id, true
}

func validSlug(s string) bool {
	if s == "" || len(s) > MaxLen || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	prevDash := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '-':
			if prevDash {
				return false
			}
			prevDash = true
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			prevDash = false
		default:
			return false
		}
	}
	return true
}
