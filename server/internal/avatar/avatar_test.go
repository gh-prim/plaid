package avatar

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ptr(s string) *string { return &s }

func TestEveryVariantDraws(t *testing.T) {
	for _, part := range Parts() {
		if len(part.Variants) == 0 {
			t.Fatalf("%s has no variants", part.Name)
		}
		for _, v := range part.Variants {
			o := Defaults()
			switch part.Name {
			case "head":
				o.Head = v
			case "hair":
				o.Hair = v
			case "eyes":
				o.Eyes = v
			case "eyebrows":
				o.Eyebrows = v
			case "mouth":
				o.Mouth = v
			case "nose":
				o.Nose = v
			case "glasses":
				o.Glasses = ptr(v)
			case "beard":
				o.Beard = ptr(v)
			case "earrings":
				o.Earrings = ptr(v)
			case "freckles":
				o.Freckles = ptr(v)
			}
			if err := o.Validate(); err != nil {
				t.Fatalf("%s %s: %v", part.Name, v, err)
			}
			if _, err := PNG(o, "", 48); err != nil {
				t.Fatalf("%s %s: %v", part.Name, v, err)
			}
		}
	}
}

func TestPNGDrawsTheFace(t *testing.T) {
	o := Defaults()
	o.Glasses = ptr("variant01")
	b, err := PNG(o, "#d8c79a", 256)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds() != image.Rect(0, 0, 256, 256) {
		t.Fatalf("bounds %v", img.Bounds())
	}
	// The background shows in a corner, the skin in the middle of the face.
	if r, g, b, _ := img.At(2, 2).RGBA(); r>>8 != 0xd8 || g>>8 != 0xc7 || b>>8 != 0x9a {
		t.Errorf("corner = %x %x %x, want the background", r>>8, g>>8, b>>8)
	}
	skin := 0
	for y := 100; y < 180; y++ {
		for x := 100; x < 160; x++ {
			if r, g, b, _ := img.At(x, y).RGBA(); r>>8 == 0xd9 && g>>8 == 0xa0 && b>>8 == 0x7a {
				skin++
			}
		}
	}
	if skin < 500 {
		t.Errorf("only %d skin pixels in the face", skin)
	}
	if dir := os.Getenv("AVATAR_OUT"); dir != "" {
		_ = os.WriteFile(filepath.Join(dir, "avatar.png"), b, 0o600)
	}
}

func TestParse(t *testing.T) {
	_, norm, err := Parse([]byte(`{"head":"variant02","hair":"variant10","eyes":"variant03","eyebrows":"variant01",
		"mouth":"sad01","nose":"variant02","beard":"variant01","hairColor":"#5B3A26","skinColor":"#d9a07a"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(norm), `"hairColor":"#5b3a26"`) || !strings.Contains(string(norm), `"glasses":null`) {
		t.Errorf("normalized = %s", norm)
	}
	for _, c := range []struct{ json, field string }{
		{`{"head":"nope","hair":"variant01","eyes":"variant01","eyebrows":"variant01","mouth":"happy01","nose":"variant01","hairColor":"#000000","skinColor":"#ffffff"}`, "head"},
		{`{"head":"variant01","hair":"variant01","eyes":"variant01","eyebrows":"variant01","mouth":"happy01","nose":"variant01","hairColor":"red","skinColor":"#ffffff"}`, "hairColor"},
		{`{"head":"variant01","hair":"variant01","eyes":"variant01","eyebrows":"variant01","mouth":"happy01","nose":"variant01","glasses":"x","hairColor":"#000000","skinColor":"#ffffff"}`, "glasses"},
		{`{"extra":1}`, "options"},
	} {
		_, _, err := Parse([]byte(c.json))
		var inv *InvalidError
		if !errors.As(err, &inv) || inv.Field != c.field {
			t.Errorf("%s: err = %v, want invalid %s", c.json, err, c.field)
		}
	}
}

func TestSVGHasNoScript(t *testing.T) {
	// Colors come from the client: they must never break out of the attribute.
	o := Defaults()
	o.HairColor = `#000"/><script>`
	if strings.Contains(SVG(o, ""), "<script>") {
		t.Fatal("unescaped color")
	}
}

func TestNormalizePhoto(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for x := 0; x < 800; x++ {
		for y := 0; y < 600; y++ {
			src.Set(x, y, color.RGBA{R: uint8(x / 4), G: 90, B: 160, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	out, err := NormalizePhoto(&buf)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != PhotoSize || img.Bounds().Dy() != PhotoSize {
		t.Fatalf("size %v", img.Bounds())
	}
	if _, err := NormalizePhoto(strings.NewReader("not an image")); !errors.Is(err, ErrBadPhoto) {
		t.Fatalf("err = %v", err)
	}
	small, err := ResizeJPEG(out, 64)
	if err != nil {
		t.Fatal(err)
	}
	if img, _ := jpeg.Decode(bytes.NewReader(small)); img.Bounds().Dx() != 64 {
		t.Fatal("not resized")
	}
}

func TestBeardShadingIsMasked(t *testing.T) {
	o := Defaults()
	o.Beard = ptr("variant01")
	ls := layers(o, "")
	masked := 0
	for _, l := range ls {
		if l.mask != "" {
			masked++
			if l.opacity != 0.4 || !strings.Contains(l.mask, "<path") || strings.Contains(l.svg, "mask=") {
				t.Errorf("masked layer: opacity %v", l.opacity)
			}
		}
	}
	if masked != 1 || len(ls) != 3 {
		t.Fatalf("%d layers, %d masked; want 3 and 1", len(ls), masked)
	}
	with, err := PNG(o, "#ffffff", 128)
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("AVATAR_OUT"); dir != "" {
		_ = os.WriteFile(filepath.Join(dir, "beard.png"), with, 0o600)
	}
}
