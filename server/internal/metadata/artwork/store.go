// Package artwork stores images and serves them resized (F005 R17–R21).
// Downloaded originals are kept once per content (SHA-256) under
// <data>/artwork/originals: they're needed offline, so they're never
// evicted. Resized variants live under <data>/cache/artwork and can be
// rebuilt. Local images next to the media are read in place, never copied.
// Nothing is ever written inside library folders.
package artwork

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // decoders for DecodeConfig and Decode
	_ "image/png"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp" // WebP decoder: local images may be WebP (R13)

	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
)

// Limits (NFR security).
const (
	MaxBytes  = 20 << 20
	MaxPixels = 8000
)

// Errors callers act on.
var (
	ErrTooLarge    = errors.New("artwork: image over 8000 × 8000")
	ErrUnsupported = errors.New("artwork: not a JPEG, PNG or WebP image")
)

// Store holds originals and variants.
type Store struct {
	dir  string
	http *httpx.Client
	// hosts rewrites image hosts (tests point them at a local server).
	hosts    map[string]string
	variants *variantCache
	log      *slog.Logger
}

// Options configure a Store.
type Options struct {
	// DataDir is PLAID_DATA_DIR.
	DataDir string
	// HTTP downloads images; it must allow image.tmdb.org and assets.fanart.tv.
	HTTP *httpx.Client
	// MaxVariantBytes caps the variant cache (R20); 0 means 2 GiB.
	MaxVariantBytes int64
	// Hosts maps image hosts to other base URLs (tests only).
	Hosts map[string]string
	Log   *slog.Logger
}

// New returns a store. It doesn't touch the disk until it's used.
func New(o Options) *Store {
	if o.MaxVariantBytes == 0 {
		o.MaxVariantBytes = 2 << 30
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	s := &Store{dir: o.DataDir, http: o.HTTP, hosts: o.Hosts, log: o.Log}
	s.variants = newVariantCache(filepath.Join(o.DataDir, "cache", "artwork"), o.MaxVariantBytes, o.Log)
	return s
}

// Original is a stored or local image.
type Original struct {
	SHA256 string
	Ext    string // jpg, png or webp
	Width  int
	Height int
}

// Download fetches an image and stores it once by content (R17, R18).
func (s *Store) Download(ctx context.Context, rawURL string) (Original, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Original{}, err
	}
	if base, ok := s.hosts[u.Hostname()]; ok {
		rawURL = strings.TrimSuffix(base, "/") + u.EscapedPath()
	}
	body, err := s.http.Get(ctx, rawURL, nil)
	if err != nil {
		return Original{}, err
	}
	o, err := inspect(body)
	if err != nil {
		return Original{}, err
	}
	path := s.OriginalPath(o)
	if _, err := os.Stat(path); err == nil {
		return o, nil
	}
	if err := writeAtomic(path, body); err != nil {
		return Original{}, err
	}
	return o, nil
}

// Fetch downloads an image without storing it (candidate previews, R27).
func (s *Store) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if base, ok := s.hosts[u.Hostname()]; ok {
		rawURL = strings.TrimSuffix(base, "/") + u.EscapedPath()
	}
	body, err := s.http.Get(ctx, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if _, err := inspect(body); err != nil {
		return nil, err
	}
	return body, nil
}

// Inspect reads a local image's hash and size without copying it (R13).
func Inspect(path string) (Original, error) {
	f, err := os.Open(path) //nolint:gosec // a library folder, read-only
	if err != nil {
		return Original{}, err
	}
	defer func() { _ = f.Close() }()
	body, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return Original{}, err
	}
	if len(body) > MaxBytes {
		return Original{}, ErrTooLarge
	}
	return inspect(body)
}

func inspect(body []byte) (Original, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return Original{}, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	ext := map[string]string{"jpeg": "jpg", "png": "png", "webp": "webp"}[format]
	if ext == "" {
		return Original{}, ErrUnsupported
	}
	if cfg.Width > MaxPixels || cfg.Height > MaxPixels || cfg.Width == 0 || cfg.Height == 0 {
		return Original{}, ErrTooLarge
	}
	sum := sha256.Sum256(body)
	return Original{SHA256: hex.EncodeToString(sum[:]), Ext: ext, Width: cfg.Width, Height: cfg.Height}, nil
}

// OriginalPath is where a downloaded original lives.
func (s *Store) OriginalPath(o Original) string {
	return filepath.Join(s.dir, "artwork", "originals", o.SHA256[:2], o.SHA256+"."+o.Ext)
}

// Cleanup deletes downloaded originals no artwork row uses (R18). It
// returns how many it removed. It works inside an os.Root, so a symlink
// planted in the data directory can't make it delete anything outside.
func (s *Store) Cleanup(ctx context.Context, used func(sha string) (bool, error)) (int, error) {
	root, err := os.OpenRoot(filepath.Join(s.dir, "artwork", "originals"))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer func() { _ = root.Close() }()
	n := 0
	err = fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}
		sha := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		keep, err := used(sha)
		if err != nil || keep {
			return err
		}
		if err := root.Remove(path); err != nil {
			return err
		}
		s.variants.dropSHA(sha)
		n++
		return nil
	})
	return n, err
}

func writeAtomic(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
