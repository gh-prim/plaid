// Package keyframes reads the video keyframe times of a media file from
// its container index (Matroska Cues, MP4 stss), which Direct Stream needs
// to cut segments (F006 R13). It reads the file's header and index only,
// never the media data: a file without a usable index has no keyframes
// here and is transcoded instead.
package keyframes

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// ErrNoIndex means the file has no index listing its video keyframes.
var ErrNoIndex = errors.New("no keyframe index")

// maxIndexBytes bounds what a reader loads from one index element: a
// three-hour film's Cues or moov is a few MB.
const maxIndexBytes = 64 << 20

// Read returns the video keyframe presentation times of a file in
// milliseconds, ascending, from its container index. container is the
// short name media.go derives from the extension.
func Read(r io.ReaderAt, size int64, container string) ([]int64, error) {
	switch container {
	case "mkv", "webm":
		return readMatroska(r, size)
	case "mp4", "mov":
		return readMP4(r, size)
	}
	return nil, ErrNoIndex
}

// ReadFile is Read on a file path.
func ReadFile(path, container string) ([]int64, error) {
	f, err := os.Open(path) //nolint:gosec // path comes from the library, never from a request
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return Read(f, st.Size(), container)
}

// Encode stores ascending times as uvarint deltas.
func Encode(times []int64) []byte {
	out := make([]byte, 0, len(times)*2)
	var prev int64
	for _, t := range times {
		out = binary.AppendUvarint(out, uint64(t-prev)) //nolint:gosec // times are ascending
		prev = t
	}
	return out
}

// Decode reverses Encode.
func Decode(b []byte) ([]int64, error) {
	var (
		out  []int64
		prev int64
	)
	for len(b) > 0 {
		d, n := binary.Uvarint(b)
		if n <= 0 {
			return nil, errors.New("keyframes: bad encoding")
		}
		prev += int64(d) //nolint:gosec // written by Encode
		out = append(out, prev)
		b = b[n:]
	}
	return out, nil
}

// Store caches keyframe lists in media_keyframes, per file and mtime. An
// empty list is cached too: it means the file has no usable index.
type Store struct {
	DB  *sql.DB
	Now func() time.Time
}

// Get returns the cached list for a file at this mtime.
func (s *Store) Get(ctx context.Context, fileID string, mtime time.Time) (times []int64, ok bool, err error) {
	var (
		stored string
		blob   []byte
	)
	err = s.DB.QueryRowContext(ctx, `SELECT file_mtime, keyframes FROM media_keyframes WHERE file_id = ?`, fileID).Scan(&stored, &blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if stored != mtime.UTC().Format(time.RFC3339Nano) {
		return nil, false, nil
	}
	times, err = Decode(blob)
	return times, err == nil, err
}

// Put caches a file's list.
func (s *Store) Put(ctx context.Context, fileID string, mtime time.Time, times []int64) error {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO media_keyframes (file_id, file_mtime, keyframes, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (file_id) DO UPDATE SET file_mtime = excluded.file_mtime, keyframes = excluded.keyframes, created_at = excluded.created_at`,
		fileID, mtime.UTC().Format(time.RFC3339Nano), append([]byte{}, Encode(times)...), now().UTC().Format(time.RFC3339))
	return err
}

// Load returns a file's keyframes from the cache, reading the file's index
// on a miss. A file without an index gives an empty list and no error.
func (s *Store) Load(ctx context.Context, fileID, path, container string, mtime time.Time) ([]int64, error) {
	if times, ok, err := s.Get(ctx, fileID, mtime); err != nil || ok {
		return times, err
	}
	times, err := ReadFile(path, container)
	switch {
	case errors.Is(err, ErrNoIndex):
		times = nil
	case err != nil:
		return nil, fmt.Errorf("keyframes of %s: %w", fileID, err)
	}
	return times, s.Put(ctx, fileID, mtime, times)
}

// readAt reads exactly n bytes at off, refusing reads past the end or
// above maxIndexBytes.
func readAt(r io.ReaderAt, size, off, n int64) ([]byte, error) {
	if off < 0 || n < 0 || n > maxIndexBytes || off+n > size {
		return nil, ErrNoIndex
	}
	b := make([]byte, n)
	if _, err := r.ReadAt(b, off); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoIndex, err)
	}
	return b, nil
}
