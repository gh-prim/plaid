package artwork

import (
	"fmt"
	"image"
	"math"
	"sort"

	xdraw "golang.org/x/image/draw"
)

// Colors are an image's ambient colors (R21), as #rrggbb.
type Colors struct {
	Dominant   string `json:"dominant"`
	Vibrant    string `json:"vibrant"`
	Muted      string `json:"muted"`
	Dark       string `json:"dark"`
	OnDominant string `json:"onDominant"` // light or dark text on Dominant
}

// boxes is how many colors median cut keeps.
const boxes = 5

// Palette extracts the ambient colors from a 64 px thumbnail.
func Palette(img image.Image) Colors {
	b := img.Bounds()
	w := 64
	h := max(1, b.Dy()*w/max(1, b.Dx()))
	small := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(small, small.Bounds(), img, b, xdraw.Src, nil)
	var px []rgb
	for y := range h {
		for x := range w {
			c := small.NRGBAAt(x, y)
			if c.A < 128 {
				continue // transparent parts of logos don't count
			}
			px = append(px, rgb{int(c.R), int(c.G), int(c.B)})
		}
	}
	if len(px) == 0 {
		return Colors{Dominant: "#000000", Vibrant: "#000000", Muted: "#000000", Dark: "#000000", OnDominant: "light"}
	}
	groups := medianCut(px, boxes)
	type swatch struct {
		c rgb
		n int
	}
	sw := make([]swatch, len(groups))
	for i, g := range groups {
		sw[i] = swatch{average(g), len(g)}
	}
	sort.SliceStable(sw, func(i, j int) bool { return sw[i].n > sw[j].n })
	out := Colors{Dominant: sw[0].c.hex()}
	best := func(score func(rgb) float64) rgb {
		top, topScore := sw[0].c, math.Inf(-1)
		for _, s := range sw {
			if v := score(s.c); v > topScore {
				top, topScore = s.c, v
			}
		}
		return top
	}
	out.Vibrant = best(func(c rgb) float64 { s, v := c.sv(); return s * v }).hex()
	out.Muted = best(func(c rgb) float64 { s, v := c.sv(); return -s + 0.2*v }).hex()
	out.Dark = best(func(c rgb) float64 { return -c.luminance() }).hex()
	out.OnDominant = "light"
	if sw[0].c.luminance() >= 0.5 {
		out.OnDominant = "dark"
	}
	return out
}

type rgb struct{ r, g, b int }

func (c rgb) hex() string { return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b) }

// luminance is the WCAG relative luminance, 0–1.
func (c rgb) luminance() float64 {
	lin := func(v int) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.r) + 0.7152*lin(c.g) + 0.0722*lin(c.b)
}

// sv returns HSV saturation and value, 0–1.
func (c rgb) sv() (s, v float64) {
	r, g, b := float64(c.r)/255, float64(c.g)/255, float64(c.b)/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	if mx > 0 {
		s = (mx - mn) / mx
	}
	return s, mx
}

func average(px []rgb) rgb {
	var r, g, b int
	for _, p := range px {
		r, g, b = r+p.r, g+p.g, b+p.b
	}
	n := len(px)
	return rgb{r / n, g / n, b / n}
}

// medianCut splits pixels into up to n boxes along their widest channel.
func medianCut(px []rgb, n int) [][]rgb {
	out := [][]rgb{px}
	for len(out) < n {
		// Split the box with the widest range.
		idx, widest, ch := -1, 0, 0
		for i, box := range out {
			if len(box) < 2 {
				continue
			}
			for c := range 3 {
				lo, hi := 255, 0
				for _, p := range box {
					v := [3]int{p.r, p.g, p.b}[c]
					lo, hi = min(lo, v), max(hi, v)
				}
				if hi-lo > widest {
					idx, widest, ch = i, hi-lo, c
				}
			}
		}
		if idx < 0 || widest == 0 {
			break
		}
		box := out[idx]
		sort.Slice(box, func(i, j int) bool {
			return [3]int{box[i].r, box[i].g, box[i].b}[ch] < [3]int{box[j].r, box[j].g, box[j].b}[ch]
		})
		mid := len(box) / 2
		out = append(out[:idx], append([][]rgb{box[:mid], box[mid:]}, out[idx+1:]...)...)
	}
	return out
}
