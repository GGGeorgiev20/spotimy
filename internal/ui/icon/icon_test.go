package icon

import (
	"math"
	"testing"

	"gioui.org/f32"
)

func TestParse(t *testing.T) {
	cmds, err := parsePath("M1 2h3v4z")
	if err != nil {
		t.Fatal(err)
	}
	want := []cmd{{'M', [3]f32.Point{f32.Pt(1, 2)}}, {'L', [3]f32.Point{f32.Pt(4, 2)}}, {'L', [3]f32.Point{f32.Pt(4, 6)}}, {op: 'Z'}}
	if len(cmds) != len(want) {
		t.Fatalf("got %v", cmds)
	}
	for i := range want {
		if cmds[i] != want[i] {
			t.Errorf("cmd %d: got %v want %v", i, cmds[i], want[i])
		}
	}

	// Compact numbers, implicit lineto after m, exponents, and relative moves after z.
	cmds, err = parsePath("m.5-.3 1.2.3 1e-1 2zm1 1")
	if err != nil {
		t.Fatal(err)
	}
	got := []f32.Point{cmds[0].p[0], cmds[1].p[0], cmds[2].p[0], cmds[4].p[0]}
	exp := []f32.Point{f32.Pt(.5, -.3), f32.Pt(1.7, 0), f32.Pt(1.8, 2), f32.Pt(1.5, .7)}
	for i := range exp {
		if math.Abs(float64(got[i].X-exp[i].X)) > 1e-5 || math.Abs(float64(got[i].Y-exp[i].Y)) > 1e-5 {
			t.Errorf("point %d: got %v want %v", i, got[i], exp[i])
		}
	}
}

func TestArc(t *testing.T) {
	// Half circle of radius 4 from (0,4) to (8,4), sweeping clockwise on screen via the top.
	cmds, err := parsePath("M0 4a4 4 0 0 1 8 0")
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 3 { // M + two quarter-circle cubics
		t.Fatalf("got %d cmds", len(cmds))
	}
	mid := cmds[1].p[2]
	if math.Abs(float64(mid.X-4)) > 1e-4 || math.Abs(float64(mid.Y)) > 1e-4 {
		t.Errorf("midpoint %v, want (4,0)", mid)
	}
	// Cubic midpoint must lie on the circle (catches wrong handle lengths).
	a, b, c, d := cmds[0].p[0], cmds[1].p[0], cmds[1].p[1], cmds[1].p[2]
	m := a.Add(b.Mul(3)).Add(c.Mul(3)).Add(d).Mul(1. / 8)
	if r := math.Hypot(float64(m.X-4), float64(m.Y-4)); math.Abs(r-4) > 1e-2 {
		t.Errorf("segment midpoint radius %v, want 4", r)
	}
	if end := cmds[2].p[2]; end != f32.Pt(8, 4) {
		t.Errorf("end %v, want (8,4)", end)
	}
}

func TestAllIconsParse(t *testing.T) {
	if len(Names()) != len(sources) {
		t.Fatal("name count mismatch")
	}
	for name, ps := range sources {
		for _, p := range ps {
			d := p
			if len(d) > 2 && d[:2] == "f:" {
				d = d[2:]
			}
			if cmds, err := parsePath(d); err != nil || len(cmds) == 0 {
				t.Errorf("%s %q: %v", name, p, err)
			}
		}
	}
}
