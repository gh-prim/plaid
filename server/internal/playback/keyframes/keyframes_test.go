package keyframes_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/db"
	"github.com/gh-prim/plaid/server/internal/playback/keyframes"
	"github.com/gh-prim/plaid/server/internal/playback/testmedia"
)

// probeKeyframes lists keyframe times in ms the way FFmpeg sees them.
func probeKeyframes(t *testing.T, path string) []int64 {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-skip_frame", "nokey",
		"-show_entries", "frame=pts_time", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	var times []int64
	for _, line := range strings.Fields(string(out)) {
		f, err := strconv.ParseFloat(strings.TrimSuffix(line, ","), 64)
		if err != nil {
			t.Fatal(err)
		}
		times = append(times, int64(f*1000+0.5))
	}
	return times
}

// countingReader counts the bytes read through it.
type countingReader struct {
	r io.ReaderAt
	n atomic.Int64
}

func (c *countingReader) ReadAt(p []byte, off int64) (int, error) {
	n, err := c.r.ReadAt(p, off)
	c.n.Add(int64(n))
	return n, err
}

func readCounting(t *testing.T, path, container string) ([]int64, int64, error) {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // test file
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	st, _ := f.Stat()
	c := &countingReader{r: f}
	times, err := keyframes.Read(c, st.Size(), container)
	return times, c.n.Load(), err
}

func TestReadMatchesFFprobe(t *testing.T) {
	for _, tc := range []struct{ clip, container string }{
		{testmedia.MKV, "mkv"},
		{testmedia.MP4, "mp4"},
		{testmedia.Long, "mkv"},
		{testmedia.HEVCHDR, "mkv"},
	} {
		t.Run(tc.clip, func(t *testing.T) {
			path := testmedia.Clip(t, tc.clip)
			got, read, err := readCounting(t, path, tc.container)
			if err != nil {
				t.Fatal(err)
			}
			want := probeKeyframes(t, path)
			if len(got) != len(want) {
				t.Fatalf("got %d keyframes, ffprobe %d\ngot  %v\nwant %v", len(got), len(want), got[:min(8, len(got))], want[:min(8, len(want))])
			}
			for i := range got {
				if d := got[i] - want[i]; d < -1 || d > 1 {
					t.Fatalf("keyframe %d at %d ms, ffprobe %d ms", i, got[i], want[i])
				}
			}
			// Header plus index only: a few KB per minute, never the media.
			st, _ := os.Stat(path)
			if limit := int64(64<<10) + int64(len(got))*64; read > limit || read*20 > st.Size() {
				t.Errorf("read %d bytes of a %d-byte file for %d keyframes", read, st.Size(), len(got))
			}
		})
	}
}

func TestReadWithoutIndex(t *testing.T) {
	dir := t.TempDir()
	mkv := testmedia.Clip(t, testmedia.MKV)

	t.Run("ts", func(t *testing.T) {
		_, read, err := readCounting(t, testmedia.Clip(t, testmedia.TS), "ts")
		if !errors.Is(err, keyframes.ErrNoIndex) || read != 0 {
			t.Fatalf("err %v after %d bytes, want ErrNoIndex without reading", err, read)
		}
	})
	t.Run("truncated", func(t *testing.T) {
		b, _ := os.ReadFile(mkv) //nolint:gosec // test file
		p := filepath.Join(dir, "truncated.mkv")
		if err := os.WriteFile(p, b[:len(b)/2], 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := keyframes.ReadFile(p, "mkv"); !errors.Is(err, keyframes.ErrNoIndex) {
			t.Fatalf("err %v, want ErrNoIndex", err)
		}
	})
	t.Run("cues voided", func(t *testing.T) {
		b, _ := os.ReadFile(mkv) //nolint:gosec // test file
		// The Cues element (1C 53 BB 6B, then its size) becomes a Void
		// element (EC) of the same total length.
		i := bytes.LastIndex(b, []byte{0x1C, 0x53, 0xBB, 0x6B})
		if i < 0 {
			t.Fatal("no Cues in the clip")
		}
		sizeLen := 0
		for j := 0; j < 8; j++ {
			if b[i+4]&(0x80>>j) != 0 {
				sizeLen = j + 1
				break
			}
		}
		total := 4 + sizeLen
		var size uint64
		size = uint64(b[i+4] & (0xFF >> sizeLen))
		for _, c := range b[i+5 : i+4+sizeLen] {
			size = size<<8 | uint64(c)
		}
		total += int(size)
		// Void: id EC, an 8-byte size, then zeros.
		void := []byte{0xEC, 0x01}
		bodyLen := uint64(total - 1 - 8)
		for k := 6; k >= 0; k-- {
			void = append(void, byte(bodyLen>>(8*k)))
		}
		copy(b[i:], void)
		clear(b[i+len(void) : i+total])
		p := filepath.Join(dir, "nocues.mkv")
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", p).Output(); err != nil || len(out) == 0 {
			t.Fatalf("the voided file should still be valid: %v", err)
		}
		if _, err := keyframes.ReadFile(p, "mkv"); !errors.Is(err, keyframes.ErrNoIndex) {
			t.Fatalf("err %v, want ErrNoIndex", err)
		}
	})
	t.Run("garbage", func(t *testing.T) {
		for _, c := range []string{"mkv", "mp4"} {
			if _, err := keyframes.Read(bytes.NewReader([]byte("not a media file at all")), 23, c); !errors.Is(err, keyframes.ErrNoIndex) {
				t.Errorf("%s: err %v, want ErrNoIndex", c, err)
			}
		}
	})
}

func TestEncodeDecode(t *testing.T) {
	in := []int64{0, 2000, 4000, 6001, 10_000, 7_200_000}
	got, err := keyframes.Decode(keyframes.Encode(in))
	if err != nil || !slices.Equal(got, in) {
		t.Fatalf("got %v, %v", got, err)
	}
	if got, err := keyframes.Decode(nil); err != nil || got != nil {
		t.Fatalf("empty: %v, %v", got, err)
	}
	if _, err := keyframes.Decode([]byte{0xFF}); err == nil {
		t.Fatal("a truncated varint should fail")
	}
}

func TestStoreCache(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.ExecContext(ctx, `
		INSERT INTO libraries (id, name, type, created_at, updated_at) VALUES ('l', 'L', 'movies', 'x', 'x');
		INSERT INTO library_folders (id, library_id, path, created_at) VALUES ('lf', 'l', '/m', 'x');
		INSERT INTO media_files (id, library_id, folder_id, path, size, mtime, parse_status, created_at, updated_at)
		VALUES ('f', 'l', 'lf', '/m/a.mkv', 1, 'x', 'ok', 'x', 'x');`); err != nil {
		t.Fatal(err)
	}
	s := &keyframes.Store{DB: database}
	mtime := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	if _, ok, err := s.Get(ctx, "f", mtime); ok || err != nil {
		t.Fatalf("empty cache: %v %v", ok, err)
	}
	if err := s.Put(ctx, "f", mtime, []int64{0, 2000}); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := s.Get(ctx, "f", mtime); !ok || err != nil || !slices.Equal(got, []int64{0, 2000}) {
		t.Fatalf("got %v %v %v", got, ok, err)
	}
	if _, ok, _ := s.Get(ctx, "f", mtime.Add(time.Second)); ok {
		t.Fatal("a new mtime must miss")
	}
	// No index is cached as an empty list.
	if err := s.Put(ctx, "f", mtime, nil); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := s.Get(ctx, "f", mtime); !ok || err != nil || len(got) != 0 {
		t.Fatalf("got %v %v %v", got, ok, err)
	}
}
