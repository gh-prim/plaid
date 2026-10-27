package artwork

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"strconv"
	"sync/atomic"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/sync/singleflight"
)

// Widths is the ladder of variant widths (R19).
var Widths = []int{92, 185, 342, 500, 780, 1280, 1920, 3840}

// jpegQuality for photos (R19).
const jpegQuality = 82

// Source is the image a variant is made from: a downloaded original, or a
// local file read in place.
type Source struct {
	Original
	// LocalPath is set for a local image.
	LocalPath string
	// Alpha keeps transparency (logos, clearart): the variant is PNG.
	Alpha bool
}

// Variant is a resized image ready to serve.
type Variant struct {
	Path        string
	ContentType string
	Width       int
	ETag        string
}

// TargetWidth is the smallest ladder width ≥ want, never past the
// original width (R19).
func TargetWidth(want, original int) int {
	for _, w := range Widths {
		if w >= want {
			return min(w, original)
		}
	}
	return min(Widths[len(Widths)-1], original)
}

var (
	resizeSlots = make(chan struct{}, 2) // at most 2 resizes at once (R19)
	flights     singleflight.Group
	resizes     atomic.Int64 // counts resizes, for tests
)

// Variant returns src at the ladder width for want, making it on first
// request and caching it (R19, R20).
func (s *Store) Variant(ctx context.Context, src Source, want int) (Variant, error) {
	w := TargetWidth(want, src.Width)
	ext, ctype := "jpg", "image/jpeg"
	if src.Alpha {
		ext, ctype = "png", "image/png"
	}
	v := Variant{ContentType: ctype, Width: w, ETag: `"` + src.SHA256[:16] + "-" + strconv.Itoa(w) + `"`}
	key := src.SHA256 + "/" + strconv.Itoa(w) + "." + ext
	if p, ok := s.variants.get(key); ok {
		v.Path = p
		return v, nil
	}
	p, err, _ := flights.Do(key, func() (any, error) {
		if p, ok := s.variants.get(key); ok {
			return p, nil
		}
		select {
		case resizeSlots <- struct{}{}:
			defer func() { <-resizeSlots }()
		case <-ctx.Done():
			return "", ctx.Err()
		}
		body, err := s.resize(src, w, src.Alpha)
		if err != nil {
			return "", err
		}
		return s.variants.put(key, body)
	})
	if err != nil {
		return Variant{}, err
	}
	v.Path = p.(string)
	return v, nil
}

func (s *Store) resize(src Source, width int, alpha bool) ([]byte, error) {
	resizes.Add(1)
	path := src.LocalPath
	if path == "" {
		path = s.OriginalPath(src.Original)
	}
	raw, err := os.ReadFile(path) //nolint:gosec // our originals, or a library image read-only
	if err != nil {
		return nil, err
	}
	if _, err := inspect(raw); err != nil { // the size cap, again: local files can change
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("artwork: decode: %w", err)
	}
	b := img.Bounds()
	height := max(1, b.Dy()*width/b.Dx())
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	if !alpha {
		// JPEG has no alpha: flatten on black, like a dark UI shows it.
		draw.Draw(dst, dst.Bounds(), image.Black, image.Point{}, draw.Src)
	}
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	var out bytes.Buffer
	if alpha {
		err = png.Encode(&out, dst)
	} else {
		err = jpeg.Encode(&out, dst, &jpeg.Options{Quality: jpegQuality})
	}
	return out.Bytes(), err
}
