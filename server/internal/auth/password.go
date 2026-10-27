package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Argon2Params are argon2id cost parameters. They are stored in every
// encoded hash, so they can be raised later without breaking old hashes.
type Argon2Params struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
}

// DefaultParams is the OWASP minimum for argon2id (F003-R5): 19 MiB, t=2, p=1.
var DefaultParams = Argon2Params{MemoryKiB: 19 * 1024, Time: 2, Threads: 1}

const (
	saltLen = 16
	keyLen  = 32
	// maxConcurrentHashes bounds memory under a burst of sign-ins: 2 × 19 MiB.
	maxConcurrentHashes = 2
)

// Password length limits (F003-R5). Counted in characters, not bytes.
const (
	MinPasswordLen = 8
	MaxPasswordLen = 128
)

// ErrMalformedHash is returned for a stored hash that can't be parsed.
var ErrMalformedHash = errors.New("malformed argon2id hash")

// Hasher hashes and verifies secrets (passwords, PINs) with argon2id.
// At most maxConcurrentHashes run at once; others wait.
type Hasher struct {
	params Argon2Params
	sem    chan struct{}
	dummy  string
}

// NewHasher returns a Hasher using params for new hashes.
func NewHasher(params Argon2Params) *Hasher {
	h := &Hasher{params: params, sem: make(chan struct{}, maxConcurrentHashes)}
	dummy, err := h.Hash(context.Background(), "plaid-dummy-password")
	if err != nil {
		panic(err) // only fails if crypto/rand fails
	}
	h.dummy = dummy
	return h
}

// Hash returns the PHC-encoded argon2id hash of secret.
func (h *Hasher) Hash(ctx context.Context, secret string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := h.derive(ctx, secret, salt, h.params)
	if err != nil {
		return "", err
	}
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.params.MemoryKiB, h.params.Time, h.params.Threads,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify reports whether secret matches encoded, in constant time.
func (h *Hasher) Verify(ctx context.Context, secret, encoded string) (bool, error) {
	params, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	got, err := h.derive(ctx, secret, salt, params)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// VerifyDummy does the same work as Verify against a hash nothing matches.
// Sign-in calls it for unknown usernames so timing doesn't reveal them (F003-R6).
func (h *Hasher) VerifyDummy(ctx context.Context, secret string) error {
	_, err := h.Verify(ctx, secret, h.dummy)
	return err
}

// NeedsRehash reports whether encoded was made with weaker params than h's.
func (h *Hasher) NeedsRehash(encoded string) bool {
	p, _, _, err := decodeHash(encoded)
	if err != nil {
		return true
	}
	return p.MemoryKiB < h.params.MemoryKiB || p.Time < h.params.Time || p.Threads < h.params.Threads
}

func (h *Hasher) derive(ctx context.Context, secret string, salt []byte, p Argon2Params) ([]byte, error) {
	select {
	case h.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-h.sem }()
	return argon2.IDKey([]byte(secret), salt, p.Time, p.MemoryKiB, p.Threads, keyLen), nil
}

func decodeHash(encoded string) (p Argon2Params, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return p, nil, nil, ErrMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return p, nil, nil, ErrMalformedHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Time, &p.Threads); err != nil {
		return p, nil, nil, ErrMalformedHash
	}
	b64 := base64.RawStdEncoding
	if salt, err = b64.DecodeString(parts[4]); err != nil {
		return p, nil, nil, ErrMalformedHash
	}
	if key, err = b64.DecodeString(parts[5]); err != nil || len(key) == 0 {
		return p, nil, nil, ErrMalformedHash
	}
	return p, salt, key, nil
}

// PasswordProblem returns the validation code for an unacceptable password,
// or "" when it's fine: "too_short", "too_long" or "too_common" (F003-R5).
func PasswordProblem(pw string) string {
	switch n := utf8.RuneCountInString(pw); {
	case n < MinPasswordLen:
		return "too_short"
	case n > MaxPasswordLen:
		return "too_long"
	case isCommonPassword(pw):
		return "too_common"
	}
	return ""
}
