package avatar

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png" // decoders for uploaded photos
	"io"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // decoder for uploaded photos
)

// Photo limits (F009).
const (
	MaxPhotoBytes  = 8 << 20
	maxPhotoPixels = 50_000_000 // a decompression bomb stops here
	PhotoSize      = 512
	photoQuality   = 85
)

// Colors are the avatar colors, the `party-*` tokens of the dark theme as
// sRGB: the background of a built avatar.
var Colors = []string{"#e2b0a7", "#d5b993", "#c0c295", "#a3c9aa", "#90caca", "#9bc4e1", "#b9b9e4", "#d5b0d0"}

// ErrBadPhoto is a photo that isn't a JPEG, PNG or WebP, or is too big.
var ErrBadPhoto = errors.New("avatar: not a usable photo")

// NormalizePhoto reads an uploaded photo (cropped square by the client),
// crops it square in the middle if it isn't, and returns a PhotoSize JPEG
// without the original's metadata.
func NormalizePhoto(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxPhotoBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxPhotoBytes {
		return nil, ErrBadPhoto
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPhotoPixels {
		return nil, ErrBadPhoto
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrBadPhoto
	}
	return squareJPEG(img, PhotoSize)
}

// ResizeJPEG returns a stored photo at size pixels square.
func ResizeJPEG(src []byte, size int) ([]byte, error) {
	img, err := jpeg.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	return squareJPEG(img, size)
}

func squareJPEG(img image.Image, size int) ([]byte, error) {
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2, 0, 0)
	crop.Max = crop.Min.Add(image.Pt(side, side))
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(dst, dst.Bounds(), image.Black, image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, crop, draw.Over, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: photoQuality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
