package avatar

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// normalizePath rewrites SVG path data with absolute M, L, C and Z only:
// arcs and quadratic curves become cubic ones. The style's paths are
// minified (flags run into numbers: "a8 8 0 01-5 5"), which oksvg misreads;
// this reads them like a browser does.
func normalizePath(d string) (string, error) {
	p := pathParser{s: d}
	var out strings.Builder
	var (
		x, y, sx, sy float64
		// The last control point, for the smooth commands S and T.
		cx, cy float64
		last   byte
	)
	emit := func(cmd byte, v ...float64) {
		out.WriteByte(cmd)
		for i, f := range v {
			if i > 0 {
				out.WriteByte(' ')
			}
			out.WriteString(strconv.FormatFloat(f, 'f', 3, 64))
		}
	}
	cubic := func(x1, y1, x2, y2, ex, ey float64) {
		emit('C', x1, y1, x2, y2, ex, ey)
		cx, cy, x, y = x2, y2, ex, ey
	}
	var cmd byte
	for {
		p.space()
		if p.done() {
			break
		}
		if c := p.s[p.i]; isCommand(c) {
			cmd = c
			p.i++
		} else if cmd == 0 {
			return "", errors.New("avatar: path without a command")
		}
		rel := cmd >= 'a'
		abs := func(dx, dy float64) (float64, float64) {
			if rel {
				return x + dx, y + dy
			}
			return dx, dy
		}
		upper := cmd &^ 0x20
		switch upper {
		case 'Z':
			emit('Z')
			x, y = sx, sy
			last = 'Z'
			continue
		case 'M':
			v, err := p.numbers(2)
			if err != nil {
				return "", err
			}
			x, y = abs(v[0], v[1])
			sx, sy = x, y
			emit('M', x, y)
			// Pairs after a moveto are linetos.
			cmd = 'L' | (cmd & 0x20)
			last = 'M'
			continue
		case 'L':
			v, err := p.numbers(2)
			if err != nil {
				return "", err
			}
			x, y = abs(v[0], v[1])
			emit('L', x, y)
		case 'H':
			v, err := p.numbers(1)
			if err != nil {
				return "", err
			}
			if rel {
				x += v[0]
			} else {
				x = v[0]
			}
			emit('L', x, y)
		case 'V':
			v, err := p.numbers(1)
			if err != nil {
				return "", err
			}
			if rel {
				y += v[0]
			} else {
				y = v[0]
			}
			emit('L', x, y)
		case 'C':
			v, err := p.numbers(6)
			if err != nil {
				return "", err
			}
			x1, y1 := abs(v[0], v[1])
			x2, y2 := abs(v[2], v[3])
			ex, ey := abs(v[4], v[5])
			cubic(x1, y1, x2, y2, ex, ey)
		case 'S':
			v, err := p.numbers(4)
			if err != nil {
				return "", err
			}
			x1, y1 := x, y
			if last == 'C' || last == 'S' {
				x1, y1 = 2*x-cx, 2*y-cy
			}
			x2, y2 := abs(v[0], v[1])
			ex, ey := abs(v[2], v[3])
			cubic(x1, y1, x2, y2, ex, ey)
		case 'Q', 'T':
			var qx, qy, ex, ey float64
			if upper == 'Q' {
				v, err := p.numbers(4)
				if err != nil {
					return "", err
				}
				qx, qy = abs(v[0], v[1])
				ex, ey = abs(v[2], v[3])
			} else {
				v, err := p.numbers(2)
				if err != nil {
					return "", err
				}
				qx, qy = x, y
				if last == 'Q' || last == 'T' {
					qx, qy = 2*x-cx, 2*y-cy
				}
				ex, ey = abs(v[0], v[1])
			}
			x0, y0 := x, y
			emit('C', x0+2.0/3*(qx-x0), y0+2.0/3*(qy-y0), ex+2.0/3*(qx-ex), ey+2.0/3*(qy-ey), ex, ey)
			cx, cy, x, y = qx, qy, ex, ey
		case 'A':
			v, err := p.arc()
			if err != nil {
				return "", err
			}
			ex, ey := abs(v[5], v[6])
			for _, c := range arcToCubics(x, y, v[0], v[1], v[2], v[3] != 0, v[4] != 0, ex, ey) {
				emit('C', c[0], c[1], c[2], c[3], c[4], c[5])
			}
			x, y = ex, ey
		default:
			return "", errors.New("avatar: unknown path command " + string(cmd))
		}
		last = upper
	}
	return out.String(), nil
}

func isCommand(c byte) bool { return strings.IndexByte("MmLlHhVvCcSsQqTtAaZz", c) >= 0 }

type pathParser struct {
	s string
	i int
}

func (p *pathParser) done() bool { return p.i >= len(p.s) }

func (p *pathParser) space() {
	for !p.done() && (p.s[p.i] == ' ' || p.s[p.i] == ',' || p.s[p.i] == '\n' || p.s[p.i] == '\t' || p.s[p.i] == '\r') {
		p.i++
	}
}

// number reads one number: "-.5", "1e-3", and "1.5.5" as 1.5 then .5.
func (p *pathParser) number() (float64, error) {
	p.space()
	start := p.i
	if !p.done() && (p.s[p.i] == '-' || p.s[p.i] == '+') {
		p.i++
	}
	digits, dot := 0, false
	for !p.done() {
		c := p.s[p.i]
		switch {
		case c >= '0' && c <= '9':
			digits++
		case c == '.' && !dot:
			dot = true
		default:
			goto exp
		}
		p.i++
	}
exp:
	if digits > 0 && !p.done() && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		j := p.i + 1
		if j < len(p.s) && (p.s[j] == '-' || p.s[j] == '+') {
			j++
		}
		if j < len(p.s) && p.s[j] >= '0' && p.s[j] <= '9' {
			p.i = j
			for !p.done() && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
				p.i++
			}
		}
	}
	if digits == 0 {
		return 0, errors.New("avatar: bad number in path")
	}
	return strconv.ParseFloat(p.s[start:p.i], 64)
}

func (p *pathParser) numbers(n int) ([]float64, error) {
	out := make([]float64, n)
	for i := range out {
		v, err := p.number()
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// arc reads rx ry rotation large-arc sweep x y, the flags being one
// character each, separators or not.
func (p *pathParser) arc() ([]float64, error) {
	out := make([]float64, 7)
	for i := range out {
		if i == 3 || i == 4 {
			p.space()
			if p.done() || (p.s[p.i] != '0' && p.s[p.i] != '1') {
				return nil, errors.New("avatar: bad arc flag in path")
			}
			out[i] = float64(p.s[p.i] - '0')
			p.i++
			continue
		}
		v, err := p.number()
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// arcToCubics turns an SVG arc from (x1, y1) to (x2, y2) into cubic Bézier
// curves of at most 90° each (SVG 1.1, appendix F.6).
func arcToCubics(x1, y1, rx, ry, angle float64, large, sweep bool, x2, y2 float64) [][6]float64 {
	if x1 == x2 && y1 == y2 {
		return nil
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	if rx == 0 || ry == 0 {
		return [][6]float64{{x1, y1, x2, y2, x2, y2}}
	}
	phi := angle * math.Pi / 180
	cos, sin := math.Cos(phi), math.Sin(phi)
	dx, dy := (x1-x2)/2, (y1-y2)/2
	x1p, y1p := cos*dx+sin*dy, -sin*dx+cos*dy
	// Radii too small to reach are scaled up.
	if l := x1p*x1p/(rx*rx) + y1p*y1p/(ry*ry); l > 1 {
		s := math.Sqrt(l)
		rx, ry = rx*s, ry*s
	}
	num := rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	coef := math.Sqrt(math.Max(0, num/den))
	if large == sweep {
		coef = -coef
	}
	cxp, cyp := coef*rx*y1p/ry, -coef*ry*x1p/rx
	cx, cy := cos*cxp-sin*cyp+(x1+x2)/2, sin*cxp+cos*cyp+(y1+y2)/2
	angleOf := func(ux, uy, vx, vy float64) float64 {
		a := math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
		return a
	}
	t1 := angleOf(1, 0, (x1p-cxp)/rx, (y1p-cyp)/ry)
	dt := angleOf((x1p-cxp)/rx, (y1p-cyp)/ry, (-x1p-cxp)/rx, (-y1p-cyp)/ry)
	if !sweep && dt > 0 {
		dt -= 2 * math.Pi
	} else if sweep && dt < 0 {
		dt += 2 * math.Pi
	}
	n := int(math.Ceil(math.Abs(dt) / (math.Pi / 2)))
	step := dt / float64(n)
	k := 4.0 / 3 * math.Tan(step/4)
	point := func(t float64) (float64, float64) {
		ex, ey := rx*math.Cos(t), ry*math.Sin(t)
		return cos*ex - sin*ey + cx, sin*ex + cos*ey + cy
	}
	deriv := func(t float64) (float64, float64) {
		ex, ey := -rx*math.Sin(t), ry*math.Cos(t)
		return cos*ex - sin*ey, sin*ex + cos*ey
	}
	out := make([][6]float64, 0, n)
	for i := range n {
		a, b := t1+float64(i)*step, t1+float64(i+1)*step
		ax, ay := point(a)
		bx, by := point(b)
		dax, day := deriv(a)
		dbx, dby := deriv(b)
		out = append(out, [6]float64{ax + k*dax, ay + k*day, bx - k*dbx, by - k*dby, bx, by})
	}
	// The last point is exactly the arc's end.
	out[n-1][4], out[n-1][5] = x2, y2
	return out
}
