package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// tokenBytes is the entropy of session tokens and pairing poll secrets (F003-R8).
const tokenBytes = 32

// NewToken returns a random URL-safe token (32 bytes, base64url, no padding).
func NewToken() string {
	b := make([]byte, tokenBytes)
	_, _ = rand.Read(b) // never fails on supported platforms (crypto/rand docs)
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken returns the SHA-256 of token, the only form stored in the database.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// EqualSecret compares two secrets in constant time.
func EqualSecret(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// codeAlphabet has no look-alike characters (no 0 O 1 I L), for codes people
// read on a TV or in a log and type elsewhere (F003-R20).
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// NewCode returns n random characters from codeAlphabet.
func NewCode(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = codeAlphabet[randIndex(len(codeAlphabet))]
	}
	return string(out)
}

// randIndex returns a uniform random int in [0, n) without modulo bias.
func randIndex(n int) int {
	limit := 256 - 256%n
	var b [1]byte
	for {
		_, _ = rand.Read(b[:])
		if int(b[0]) < limit {
			return int(b[0]) % n
		}
	}
}
