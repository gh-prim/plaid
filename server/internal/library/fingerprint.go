package library

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// fingerprintChunk is how much of each end of a file the fingerprint reads.
const fingerprintChunk = 1 << 20

// Fingerprint is the quick identity of a file used for move detection (R16):
// SHA-256 of the size (8 bytes, big endian), the first MiB and the last MiB.
// It reads at most 2 MiB whatever the file size, and only reads.
func Fingerprint(path string) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // files found by the walker inside a library folder
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()

	h := sha256.New()
	var sz [8]byte
	binary.BigEndian.PutUint64(sz[:], uint64(size)) //nolint:gosec // sizes are never negative
	h.Write(sz[:])

	n := min(size, fingerprintChunk)
	if _, err := io.CopyN(h, f, n); err != nil {
		return nil, fmt.Errorf("read head of %s: %w", path, err)
	}
	if _, err := io.Copy(h, io.NewSectionReader(f, size-n, n)); err != nil {
		return nil, fmt.Errorf("read tail of %s: %w", path, err)
	}
	return h.Sum(nil), nil
}
