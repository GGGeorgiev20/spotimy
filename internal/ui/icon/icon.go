// Package icon renders the app's 16x16 SVG-path icon set with Gio.
package icon

import (
	"image"
	"image/color"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

const (
	viewBox     = 16
	strokeWidth = 1.5 // in viewBox units; Gio strokes always use round caps and joins
)

type shape struct {
	fill bool
	box  float32 // viewBox size
	cmds []cmd
}

var (
	once   sync.Once
	icons  map[string][]shape
	sorted []string
)

func load() {
	icons = make(map[string][]shape, len(sources))
	for name, ps := range sources {
		sorted = append(sorted, name)
		for _, p := range ps {
			d, fill, box := split(p)
			cmds, err := parsePath(d)
			if err != nil {
				panic("icon " + name + ": " + err.Error())
			}
			icons[name] = append(icons[name], shape{fill, box, cmds})
		}
	}
	sort.Strings(sorted)
}

// split reads a source path's prefix: "f:" = filled, "f24:" = filled in a
// 24x24 viewBox (brand glyphs).
func split(p string) (d string, fill bool, box float32) {
	if d, ok := strings.CutPrefix(p, "f24:"); ok {
		return d, true, 24
	}
	d, fill = strings.CutPrefix(p, "f:")
	return d, fill, viewBox
}

// Names returns all icon names (sorted), for tests and a debug gallery.
func Names() []string {
	once.Do(load)
	return append([]string(nil), sorted...)
}

// Layout draws icon `name` in a size×size box (size in dp) filled/stroked with col.
// Unknown names draw nothing but still return the box size.
func Layout(gtx layout.Context, name string, size unit.Dp, col color.NRGBA) layout.Dimensions {
	once.Do(load)
	px := gtx.Dp(size)
	dims := layout.Dimensions{Size: image.Pt(px, px)}
	for _, s := range icons[name] {
		scale := float32(px) / s.box
		var p clip.Path
		p.Begin(gtx.Ops)
		for _, c := range s.cmds {
			pt := func(i int) f32.Point { return c.p[i].Mul(scale) }
			switch c.op {
			case 'M':
				p.MoveTo(pt(0))
			case 'L':
				p.LineTo(pt(0))
			case 'C':
				p.CubeTo(pt(0), pt(1), pt(2))
			case 'Z':
				p.Close()
			}
		}
		spec := p.End()
		var st clip.Stack
		if s.fill {
			st = clip.Outline{Path: spec}.Op().Push(gtx.Ops)
		} else {
			st = clip.Stroke{Path: spec, Width: strokeWidth * scale}.Op().Push(gtx.Ops)
		}
		paint.ColorOp{Color: col}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		st.Pop()
	}
	return dims
}

// cmd is an absolute path command: M/L use p[0], C uses p[0..2], Z uses none.
type cmd struct {
	op byte
	p  [3]f32.Point
}

type parser struct {
	s   string
	i   int
	err string
}

func (p *parser) skip() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == ',' || p.s[p.i] == '\n' || p.s[p.i] == '\t' || p.s[p.i] == '\r') {
		p.i++
	}
}

// more reports whether another number (not a command letter) follows.
func (p *parser) more() bool {
	p.skip()
	if p.i >= len(p.s) {
		return false
	}
	c := p.s[p.i]
	return c == '-' || c == '+' || c == '.' || c >= '0' && c <= '9'
}

func (p *parser) num() float32 {
	p.skip()
	start := p.i
	digits := func() {
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
		}
	}
	if p.i < len(p.s) && (p.s[p.i] == '-' || p.s[p.i] == '+') {
		p.i++
	}
	digits()
	if p.i < len(p.s) && p.s[p.i] == '.' {
		p.i++
		digits()
	}
	if p.i+1 < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		j := p.i + 1
		if p.s[j] == '-' || p.s[j] == '+' {
			j++
		}
		if j < len(p.s) && p.s[j] >= '0' && p.s[j] <= '9' {
			p.i = j
			digits()
		}
	}
	v, err := strconv.ParseFloat(p.s[start:p.i], 32)
	if err != nil && p.err == "" {
		p.err = "bad number at " + strconv.Itoa(start)
	}
	return float32(v)
}

// flag reads an arc flag: a single 0/1 char that may be unseparated from what follows.
func (p *parser) flag() bool {
	p.skip()
	if p.i < len(p.s) && (p.s[p.i] == '0' || p.s[p.i] == '1') {
		p.i++
		return p.s[p.i-1] == '1'
	}
	if p.err == "" {
		p.err = "bad arc flag at " + strconv.Itoa(p.i)
	}
	return false
}

func (p *parser) pt(rel bool, cur f32.Point) f32.Point {
	x, y := p.num(), p.num()
	if rel {
		x, y = x+cur.X, y+cur.Y
	}
	return f32.Pt(x, y)
}

type parseError string

func (e parseError) Error() string { return string(e) }

// parsePath converts SVG path data into absolute M, L, C and Z commands.
func parsePath(d string) ([]cmd, error) {
	p := &parser{s: d}
	var out []cmd
	var cur, start, lastC, lastQ f32.Point // lastC/lastQ: previous control point, for S and T
	var prev byte                          // previous command, upper-cased
	for p.skip(); p.i < len(p.s) && p.err == ""; p.skip() {
		c := p.s[p.i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') {
			return nil, parseError("expected command at " + strconv.Itoa(p.i))
		}
		p.i++
		rel := c >= 'a'
		up := c &^ 0x20
		if up == 'Z' {
			out = append(out, cmd{op: 'Z'})
			cur, prev = start, 'Z'
			continue
		}
		if !strings.ContainsRune("MLHVCSQTA", rune(up)) {
			return nil, parseError("bad command " + string(c))
		}
		for first := true; first || p.more(); first = false {
			if !p.more() && p.err == "" { // command with no arguments at all
				p.err = "missing arguments for " + string(c)
				break
			}
			switch up {
			case 'M':
				cur = p.pt(rel, cur)
				out = append(out, cmd{op: 'M', p: [3]f32.Point{cur}})
				if first {
					start = cur
				}
				up, prev = 'L', 'M' // further pairs are implicit lineto
				continue
			case 'L':
				cur = p.pt(rel, cur)
				out = append(out, cmd{op: 'L', p: [3]f32.Point{cur}})
			case 'H':
				x := p.num()
				if rel {
					x += cur.X
				}
				cur.X = x
				out = append(out, cmd{op: 'L', p: [3]f32.Point{cur}})
			case 'V':
				y := p.num()
				if rel {
					y += cur.Y
				}
				cur.Y = y
				out = append(out, cmd{op: 'L', p: [3]f32.Point{cur}})
			case 'C', 'S':
				c1 := cur
				if up == 'S' {
					if prev == 'C' || prev == 'S' {
						c1 = cur.Mul(2).Sub(lastC)
					}
				} else {
					c1 = p.pt(rel, cur)
				}
				c2 := p.pt(rel, cur)
				to := p.pt(rel, cur)
				out = append(out, cmd{op: 'C', p: [3]f32.Point{c1, c2, to}})
				lastC, cur = c2, to
			case 'Q', 'T':
				q := cur
				if up == 'T' {
					if prev == 'Q' || prev == 'T' {
						q = cur.Mul(2).Sub(lastQ)
					}
				} else {
					q = p.pt(rel, cur)
				}
				to := p.pt(rel, cur)
				out = append(out, cmd{op: 'C', p: [3]f32.Point{
					cur.Add(q.Sub(cur).Mul(2. / 3)), to.Add(q.Sub(to).Mul(2. / 3)), to}})
				lastQ, cur = q, to
			case 'A':
				rx, ry, rot := p.num(), p.num(), p.num()
				large, sweep := p.flag(), p.flag()
				to := p.pt(rel, cur)
				out = arcTo(out, cur, rx, ry, rot, large, sweep, to)
				cur = to
			}
			prev = up
		}
	}
	if p.err != "" {
		return nil, parseError(p.err)
	}
	return out, nil
}

// arcTo appends cubic Béziers approximating an SVG elliptical arc (SVG 1.1 F.6.5/F.6.6).
func arcTo(out []cmd, from f32.Point, rx, ry, rotDeg float32, large, sweep bool, to f32.Point) []cmd {
	if from == to {
		return out
	}
	rx, ry = float32(math.Abs(float64(rx))), float32(math.Abs(float64(ry)))
	if rx == 0 || ry == 0 {
		return append(out, cmd{op: 'L', p: [3]f32.Point{to}})
	}
	phi := float64(rotDeg) * math.Pi / 180
	sinP, cosP := math.Sincos(phi)
	dx, dy := float64(from.X-to.X)/2, float64(from.Y-to.Y)/2
	x1, y1 := cosP*dx+sinP*dy, -sinP*dx+cosP*dy
	r1, r2 := float64(rx), float64(ry)
	if l := x1*x1/(r1*r1) + y1*y1/(r2*r2); l > 1 { // radii too small: scale up
		r1, r2 = r1*math.Sqrt(l), r2*math.Sqrt(l)
	}
	num := r1*r1*r2*r2 - r1*r1*y1*y1 - r2*r2*x1*x1
	den := r1*r1*y1*y1 + r2*r2*x1*x1
	k := 0.0
	if num > 0 {
		k = math.Sqrt(num / den)
	}
	if large == sweep {
		k = -k
	}
	cxp, cyp := k*r1*y1/r2, -k*r2*x1/r1
	cx := cosP*cxp - sinP*cyp + float64(from.X+to.X)/2
	cy := sinP*cxp + cosP*cyp + float64(from.Y+to.Y)/2
	th1 := math.Atan2((y1-cyp)/r2, (x1-cxp)/r1)
	th2 := math.Atan2((-y1-cyp)/r2, (-x1-cxp)/r1)
	dth := th2 - th1
	if sweep && dth < 0 {
		dth += 2 * math.Pi
	} else if !sweep && dth > 0 {
		dth -= 2 * math.Pi
	}
	n := int(math.Ceil(math.Abs(dth)/(math.Pi/2) - 1e-9))
	step := dth / float64(n)
	t := 4.0 / 3 * math.Tan(step/4)
	at := func(a float64) (pt, d f32.Point) { // point and derivative on the ellipse at angle a
		s, c := math.Sincos(a)
		px, py := r1*c, r2*s
		dx, dy := -r1*s, r2*c
		return f32.Pt(float32(cx+cosP*px-sinP*py), float32(cy+sinP*px+cosP*py)),
			f32.Pt(float32(cosP*dx-sinP*dy), float32(sinP*dx+cosP*dy))
	}
	a := th1
	p0, d0 := at(a)
	for i := 0; i < n; i++ {
		a += step
		p1, d1 := at(a)
		if i == n-1 {
			p1 = to // avoid accumulated drift
		}
		out = append(out, cmd{op: 'C', p: [3]f32.Point{
			p0.Add(d0.Mul(float32(t))), p1.Sub(d1.Mul(float32(t))), p1}})
		p0, d0 = p1, d1
	}
	return out
}
