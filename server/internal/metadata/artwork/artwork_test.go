package artwork

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gh-prim/plaid/server/internal/metadata/tmdbtest"
)

func newStore(t *testing.T, srv *tmdbtest.Server, maxVariant int64) *Store {
	t.Helper()
	o := Options{DataDir: t.TempDir(), MaxVariantBytes: maxVariant}
	if srv != nil {
		o.HTTP, o.Hosts = srv.HTTP(), srv.ImageHosts()
	}
	return New(o)
}

func encode(t *testing.T, img image.Image, format string) []byte {
	t.Helper()
	var b bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&b, img)
	case "gif":
		err = gif.Encode(&b, img, nil)
	default:
		err = jpeg.Encode(&b, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func solid(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func TestDownloadStoresOnce(t *testing.T) {
	srv := tmdbtest.New(t)
	s := newStore(t, srv, 0)
	ctx := context.Background()
	o, err := s.Download(ctx, "https://image.tmdb.org/t/p/w780/poster.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if o.Ext != "jpg" || o.Width != 780 || o.Height != 1170 || len(o.SHA256) != 64 {
		t.Errorf("original = %+v", o)
	}
	if _, err := os.Stat(s.OriginalPath(o)); err != nil {
		t.Fatal(err)
	}
	again, err := s.Download(ctx, "https://image.tmdb.org/t/p/w780/poster.jpg")
	if err != nil || again != o {
		t.Errorf("same bytes again = %+v, %v", again, err)
	}
	files, _ := filepath.Glob(filepath.Join(s.dir, "artwork", "originals", "*", "*"))
	if len(files) != 1 {
		t.Errorf("%d files stored, want 1 (dedup by content)", len(files))
	}
	logo, err := s.Download(ctx, "https://assets.fanart.tv/fanart/logo.png")
	if err != nil || logo.Ext != "png" {
		t.Errorf("fanart.tv logo = %+v, %v", logo, err)
	}
}

func TestInspectLimits(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, b []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if _, err := Inspect(write("huge.png", encode(t, image.NewNRGBA(image.Rect(0, 0, MaxPixels+1, 1)), "png"))); !errors.Is(err, ErrTooLarge) {
		t.Errorf("over 8000 px: %v", err)
	}
	if _, err := Inspect(write("anim.gif", encode(t, image.NewPaletted(image.Rect(0, 0, 4, 4), []color.Color{color.Black}), "gif"))); !errors.Is(err, ErrUnsupported) {
		t.Errorf("GIF: %v", err)
	}
	if _, err := Inspect(write("garbage.jpg", []byte("not an image"))); !errors.Is(err, ErrUnsupported) {
		t.Errorf("garbage: %v", err)
	}
	o, err := Inspect(write("poster.jpg", encode(t, solid(40, 60, color.NRGBA{R: 255, A: 255}), "jpg")))
	if err != nil || o.Width != 40 || o.Height != 60 {
		t.Errorf("local poster = %+v, %v", o, err)
	}
}

func TestTargetWidth(t *testing.T) {
	for _, c := range [][3]int{{300, 780, 342}, {342, 780, 342}, {343, 780, 500}, {1, 780, 92}, {5000, 780, 780}, {5000, 6000, 3840}, {1000, 780, 780}} {
		if got := TargetWidth(c[0], c[1]); got != c[2] {
			t.Errorf("TargetWidth(%d, original %d) = %d, want %d", c[0], c[1], got, c[2])
		}
	}
}

func TestVariants(t *testing.T) {
	srv := tmdbtest.New(t)
	s := newStore(t, srv, 0)
	ctx := context.Background()
	o, err := s.Download(ctx, "https://image.tmdb.org/t/p/w780/poster.jpg")
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Variant(ctx, Source{Original: o}, 300)
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(mustOpen(t, v.Path))
	if err != nil || format != "jpeg" || cfg.Width != 342 || cfg.Height != 513 || v.ContentType != "image/jpeg" {
		t.Errorf("w=300 → %s %dx%d (%v)", format, cfg.Width, cfg.Height, err)
	}
	before := resizes.Load()
	again, err := s.Variant(ctx, Source{Original: o}, 300)
	if err != nil || again != v || resizes.Load() != before {
		t.Errorf("second request resized again or changed: %+v", again)
	}
	big, _ := s.Variant(ctx, Source{Original: o}, 5000)
	if big.Width != 780 {
		t.Errorf("w=5000 → %d, never upscaled past 780", big.Width)
	}

	logo, err := s.Download(ctx, "https://assets.fanart.tv/fanart/logo.png")
	if err != nil {
		t.Fatal(err)
	}
	lv, err := s.Variant(ctx, Source{Original: logo, Alpha: true}, 185)
	if err != nil {
		t.Fatal(err)
	}
	img, format, err := image.Decode(mustOpen(t, lv.Path))
	if err != nil || format != "png" || lv.ContentType != "image/png" {
		t.Fatalf("logo variant: %s %v", format, err)
	}
	if _, _, _, a := img.At(2, 2).RGBA(); a != 0 {
		t.Errorf("logo lost its transparency (alpha %d)", a)
	}
}

func TestConcurrentRequestsResizeOnce(t *testing.T) {
	srv := tmdbtest.New(t)
	s := newStore(t, srv, 0)
	o, err := s.Download(context.Background(), "https://image.tmdb.org/t/p/w780/burst.jpg")
	if err != nil {
		t.Fatal(err)
	}
	before := resizes.Load()
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Variant(context.Background(), Source{Original: o}, 185); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := resizes.Load() - before; n != 1 {
		t.Errorf("%d resizes for 10 identical requests, want 1", n)
	}
}

func TestVariantEvictionAndCleanup(t *testing.T) {
	srv := tmdbtest.New(t)
	s := newStore(t, srv, 30<<10) // 30 KiB
	ctx := context.Background()
	var origs []Original
	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		o, err := s.Download(ctx, "https://image.tmdb.org/t/p/w780/"+name)
		if err != nil {
			t.Fatal(err)
		}
		origs = append(origs, o)
		if _, err := s.Variant(ctx, Source{Original: o}, 500); err != nil {
			t.Fatal(err)
		}
	}
	if s.variants.size > 30<<10 {
		t.Errorf("variant cache holds %d bytes over a 30 KiB cap", s.variants.size)
	}
	if _, ok := s.variants.get(origs[2].SHA256 + "/500.jpg"); !ok {
		t.Error("the most recent variant was evicted")
	}
	// Originals are never evicted by the variant cap.
	for _, o := range origs {
		if _, err := os.Stat(s.OriginalPath(o)); err != nil {
			t.Errorf("original evicted: %v", err)
		}
	}
	n, err := s.Cleanup(ctx, func(sha string) (bool, error) { return sha == origs[0].SHA256, nil })
	if err != nil || n != 2 {
		t.Errorf("cleanup removed %d (%v), want 2", n, err)
	}
	if _, err := os.Stat(s.OriginalPath(origs[0])); err != nil {
		t.Error("a used original was removed")
	}
	// A new store rebuilds the index from disk.
	s2 := New(Options{DataDir: s.dir, MaxVariantBytes: 30 << 10})
	if _, ok := s2.variants.get(origs[2].SHA256 + "/500.jpg"); ok {
		t.Error("cleanup left the variants of a removed original")
	}
}

func TestPalette(t *testing.T) {
	red := Palette(solid(100, 150, color.NRGBA{R: 255, A: 255}))
	if red.Dominant != "#ff0000" || red.OnDominant != "light" {
		t.Errorf("red = %+v", red)
	}
	white := Palette(solid(10, 10, color.NRGBA{R: 255, G: 255, B: 255, A: 255}))
	if white.OnDominant != "dark" {
		t.Errorf("white = %+v", white)
	}
	// Mostly dark blue with a bright orange band: blue dominates, orange is vibrant.
	img := solid(100, 100, color.NRGBA{R: 20, G: 30, B: 80, A: 255})
	for y := range 20 {
		for x := range 100 {
			img.SetNRGBA(x, y, color.NRGBA{R: 250, G: 140, B: 20, A: 255})
		}
	}
	p := Palette(img)
	near := func(hex string, r, g, b int) bool {
		var cr, cg, cb int
		if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &cr, &cg, &cb); err != nil {
			return false
		}
		abs := func(n int) int { return max(n, -n) }
		return abs(cr-r) <= 3 && abs(cg-g) <= 3 && abs(cb-b) <= 3 // the thumbnail interpolates
	}
	if !near(p.Dominant, 20, 30, 80) || !near(p.Vibrant, 250, 140, 20) || !near(p.Dark, 20, 30, 80) || p.OnDominant != "light" {
		t.Errorf("two-tone = %+v", p)
	}
}

func mustOpen(t *testing.T, p string) *os.File {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func BenchmarkPosterVariant(b *testing.B) {
	dir := b.TempDir()
	p := filepath.Join(dir, "poster.jpg")
	img := solid(780, 1170, color.NRGBA{R: 120, G: 80, B: 40, A: 255})
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	_ = os.WriteFile(p, buf.Bytes(), 0o644)
	o, _ := Inspect(p)
	s := New(Options{DataDir: dir})
	b.ResetTimer()
	for b.Loop() {
		if _, err := s.resize(Source{Original: o, LocalPath: p}, 342, false); err != nil {
			b.Fatal(err)
		}
	}
}
