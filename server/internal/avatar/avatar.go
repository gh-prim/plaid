// Package avatar draws the avatars of the builder (F009). The style is
// "Lorelei" by Lisa Wischofsky (CC0 1.0), as published in DiceBear's
// definition format (lorelei.json, from @dicebear/styles 10.6.0). Unlike
// DiceBear, nothing is random: every part is chosen.
//
// The SVG is flattened (components inlined, no <use>), so a plain SVG
// rasterizer draws it, and every app gets a PNG.
package avatar

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

//go:embed lorelei.json
var definitionJSON []byte

// License is the credit line of the style.
const License = `"Lorelei" by Lisa Wischofsky, CC0 1.0, drawn with DiceBear's definition`

type element struct {
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Attributes map[string]any `json:"attributes"`
	Children   []element      `json:"children"`
}

type variant struct {
	Elements []element `json:"elements"`
}

type component struct {
	Width       float64            `json:"width"`
	Height      float64            `json:"height"`
	Probability *float64           `json:"probability"`
	Variants    map[string]variant `json:"variants"`
}

type definition struct {
	Canvas struct {
		Elements []element `json:"elements"`
		Width    float64   `json:"width"`
		Height   float64   `json:"height"`
	} `json:"canvas"`
	Components map[string]component `json:"components"`
	Colors     map[string]struct {
		Values []string `json:"values"`
	} `json:"colors"`
}

var def = func() definition {
	var d definition
	if err := json.Unmarshal(definitionJSON, &d); err != nil {
		panic("avatar: bad definition: " + err.Error())
	}
	return d
}()

// Part is one thing the builder lets you pick.
type Part struct {
	Name     string
	Optional bool
	Variants []string
}

// parts are the builder's parts, in the order of its tabs.
var parts = []string{"head", "hair", "eyes", "eyebrows", "mouth", "nose", "glasses", "beard", "earrings", "freckles"}

// HairColors and SkinColors are the builder's palettes, warm and muted to
// sit on the party colors.
var (
	HairColors = []string{"#2a211c", "#5b3a26", "#a86b3c", "#d9b26a", "#c9c2b8", "#b8503a", "#3d4b6b", "#7a5c8a"}
	SkinColors = []string{"#f6dcc5", "#ecc4a2", "#d9a07a", "#b97c55", "#8d5a3b", "#5e3b28"}
)

// Parts lists the parts with their variants, sorted.
func Parts() []Part {
	out := make([]Part, 0, len(parts))
	for _, name := range parts {
		c := def.Components[name]
		vs := make([]string, 0, len(c.Variants))
		for v := range c.Variants {
			vs = append(vs, v)
		}
		slices.Sort(vs)
		out = append(out, Part{Name: name, Optional: c.Probability != nil, Variants: vs})
	}
	return out
}

// Options are the choices of a built avatar. An optional part is nil for none.
type Options struct {
	Head      string  `json:"head"`
	Hair      string  `json:"hair"`
	Eyes      string  `json:"eyes"`
	Eyebrows  string  `json:"eyebrows"`
	Mouth     string  `json:"mouth"`
	Nose      string  `json:"nose"`
	Glasses   *string `json:"glasses"`
	Beard     *string `json:"beard"`
	Earrings  *string `json:"earrings"`
	Freckles  *string `json:"freckles"`
	HairColor string  `json:"hairColor"`
	SkinColor string  `json:"skinColor"`
}

// Defaults is the avatar the builder starts from.
func Defaults() Options {
	return Options{Head: "variant01", Hair: "variant01", Eyes: "variant01", Eyebrows: "variant01", Mouth: "happy01",
		Nose: "variant01", HairColor: HairColors[1], SkinColor: SkinColors[2]}
}

func (o Options) choice(part string) string {
	switch part {
	case "head":
		return o.Head
	case "hair":
		return o.Hair
	case "eyes":
		return o.Eyes
	case "eyebrows":
		return o.Eyebrows
	case "mouth":
		return o.Mouth
	case "nose":
		return o.Nose
	}
	var v *string
	switch part {
	case "glasses":
		v = o.Glasses
	case "beard":
		v = o.Beard
	case "earrings":
		v = o.Earrings
	case "freckles":
		v = o.Freckles
	}
	if v == nil {
		return ""
	}
	return *v
}

// InvalidError names the first field of Options that isn't valid.
type InvalidError struct{ Field string }

func (e *InvalidError) Error() string { return "avatar: invalid " + e.Field }

var hexColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Parse reads and checks options from JSON. It returns the normalized
// JSON to store.
func Parse(raw []byte) (Options, []byte, error) {
	var o Options
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&o); err != nil {
		return Options{}, nil, &InvalidError{Field: "options"}
	}
	if err := o.Validate(); err != nil {
		return Options{}, nil, err
	}
	norm, err := json.Marshal(o)
	return o, norm, err
}

// Validate checks every choice exists in the style.
func (o *Options) Validate() error {
	for _, part := range parts {
		v := o.choice(part)
		c := def.Components[part]
		if v == "" && c.Probability != nil {
			continue
		}
		if _, ok := c.Variants[v]; !ok {
			return &InvalidError{Field: part}
		}
	}
	o.HairColor, o.SkinColor = strings.ToLower(o.HairColor), strings.ToLower(o.SkinColor)
	if !hexColor.MatchString(o.HairColor) {
		return &InvalidError{Field: "hairColor"}
	}
	if !hexColor.MatchString(o.SkinColor) {
		return &InvalidError{Field: "skinColor"}
	}
	return nil
}

// layer is one pass of the drawing. A masked layer (the shading of a beard
// under the mouth) shows svg through the alpha of mask, at opacity: a plain
// SVG rasterizer has no masks, so PNG composites them itself.
type layer struct {
	svg     string
	mask    string
	opacity float64
}

// SVG draws o on background (a CSS color, "" for transparent), square,
// without the masked shading (see layer).
func SVG(o Options, background string) string {
	return layers(o, background)[0].svg
}

func layers(o Options, background string) []layer {
	w, h := def.Canvas.Width, def.Canvas.Height
	r := &renderer{o: o, b: &strings.Builder{},
		head: fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %s %s" width="%s" height="%s" fill="none">`,
			num(w), num(h), num(w), num(h))}
	if background != "" {
		fmt.Fprintf(r.b, `<rect width="%s" height="%s" fill="%s"/>`, num(w), num(h), escape(background))
	}
	r.elements(def.Canvas.Elements)
	r.flush()
	return r.layers
}

type renderer struct {
	o    Options
	b    *strings.Builder
	head string
	// open are the groups open from the root to here, to reopen them in
	// the next layer.
	open   []string
	layers []layer
}

// wrap returns body inside the open groups, as a whole document.
func (r *renderer) wrap(body string) string {
	return r.head + strings.Join(r.open, "") + body + strings.Repeat("</g>", len(r.open)) + "</svg>"
}

// flush ends the current layer.
func (r *renderer) flush() {
	r.layers = append(r.layers, layer{svg: r.head + r.b.String() + strings.Repeat("</g>", len(r.open)) + "</svg>"})
	r.b = &strings.Builder{}
	r.b.WriteString(strings.Join(r.open, ""))
}

// sub renders els on their own, inside the open groups.
func (r *renderer) sub(els []element) string {
	s := &renderer{o: r.o, b: &strings.Builder{}}
	s.elements(els)
	return r.wrap(s.b.String())
}

func (r *renderer) elements(els []element) {
	for _, e := range els {
		r.element(e)
	}
}

// group opens a <g> with attrs, runs inner, and closes it.
func (r *renderer) group(attrs map[string]any, inner func()) {
	var tag strings.Builder
	tag.WriteString(`<g`)
	r.attributes(&tag, attrs)
	tag.WriteString(`>`)
	r.b.WriteString(tag.String())
	r.open = append(r.open, tag.String())
	inner()
	r.open = r.open[:len(r.open)-1]
	r.b.WriteString(`</g>`)
}

func (r *renderer) element(e element) {
	switch e.Type {
	case "component":
		v := r.o.choice(e.Name)
		if e.Name == "hairAccessories" || v == "" {
			return
		}
		r.group(e.Attributes, func() { r.elements(def.Components[e.Name].Variants[v].Elements) })
	case "element":
		if r.masked(e) {
			return
		}
		if e.Name == "g" && len(e.Children) > 0 {
			r.group(e.Attributes, func() { r.elements(e.Children) })
			return
		}
		r.b.WriteString(`<` + e.Name)
		r.attributes(r.b, e.Attributes)
		if len(e.Children) == 0 {
			r.b.WriteString(`/>`)
			return
		}
		r.b.WriteString(`>`)
		r.elements(e.Children)
		r.b.WriteString(`</` + e.Name + `>`)
	}
}

// masked draws a group holding a <mask> and what it masks as a layer of
// its own, and reports whether e was one.
func (r *renderer) masked(e element) bool {
	if e.Name == "mask" || e.Attributes["mask"] != nil {
		return true // only reached outside a masked group: nothing to draw
	}
	i := slices.IndexFunc(e.Children, func(c element) bool { return c.Type == "element" && c.Name == "mask" })
	if e.Name != "g" || i < 0 {
		return false
	}
	opacity := 1.0
	if v, ok := e.Attributes["opacity"].(string); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			opacity = f
		}
	}
	var content []element
	for _, c := range e.Children {
		if c.Type == "element" && c.Name == "mask" {
			continue
		}
		if c.Attributes["mask"] != nil {
			attrs := maps.Clone(c.Attributes)
			delete(attrs, "mask")
			c.Attributes = attrs
		}
		content = append(content, c)
	}
	r.flush()
	r.layers = append(r.layers, layer{svg: r.sub(content), mask: r.sub(e.Children[i].Children), opacity: opacity})
	return true
}

func (r *renderer) attributes(b *strings.Builder, attrs map[string]any) {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		var v string
		switch a := attrs[k].(type) {
		case string:
			v = a
		case map[string]any:
			name, _ := a["name"].(string)
			v = r.color(name)
		default:
			continue
		}
		// oksvg reads "translate(76 3)" as one number: use commas.
		if k == "transform" {
			v = strings.Join(strings.Fields(v), ",")
		}
		// Minified paths are spelled out for the rasterizer (path.go).
		if k == "d" {
			if n, err := normalizePath(v); err == nil {
				v = n
			}
		}
		b.WriteString(` ` + k + `="` + escape(v) + `"`)
	}
}

func (r *renderer) color(name string) string {
	switch name {
	case "hair":
		return r.o.HairColor
	case "skin":
		return r.o.SkinColor
	}
	if c, ok := def.Colors[name]; ok && len(c.Values) > 0 {
		return c.Values[0]
	}
	return "none"
}

// PNG draws o on background, size pixels square.
func PNG(o Options, background string, size int) ([]byte, error) {
	if size <= 0 {
		return nil, errors.New("avatar: bad size")
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	for _, l := range layers(o, background) {
		img, err := raster(l.svg, size)
		if err != nil {
			return nil, err
		}
		if l.mask == "" {
			draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Over)
			continue
		}
		m, err := raster(l.mask, size)
		if err != nil {
			return nil, err
		}
		alpha := image.NewAlpha(dst.Bounds())
		for i := range alpha.Pix {
			alpha.Pix[i] = uint8(float64(m.Pix[i*4+3]) * l.opacity)
		}
		draw.DrawMask(dst, dst.Bounds(), img, image.Point{}, alpha, image.Point{}, draw.Over)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func raster(svg string, size int) (*image.RGBA, error) {
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.IgnoreErrorMode)
	if err != nil {
		return nil, err
	}
	icon.SetTarget(0, 0, float64(size), float64(size))
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
	icon.Draw(rasterx.NewDasher(size, size, scanner), 1)
	return img, nil
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

var escaper = strings.NewReplacer(`&`, "&amp;", `<`, "&lt;", `>`, "&gt;", `"`, "&quot;")

func escape(s string) string { return escaper.Replace(s) }
