package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
)

// An idle gap before the first step must not finish the animation at once.
func TestHoverStepAfterIdle(t *testing.T) {
	var h Hover
	gtx := layout.Context{Now: time.Unix(100, 0), Ops: new(op.Ops)}
	h.Step(gtx, false, 150*time.Millisecond)
	gtx.Now = gtx.Now.Add(3 * time.Second) // nothing drew while idle
	if v := h.Step(gtx, true, 150*time.Millisecond); v >= .5 {
		t.Fatalf("first step after idle went to %v", v)
	}
	for i := 0; i < 20; i++ {
		gtx.Now = gtx.Now.Add(16 * time.Millisecond)
		h.Step(gtx, true, 150*time.Millisecond)
	}
	if h.v != 1 {
		t.Fatalf("not finished after 320ms: %v", h.v)
	}
}

func TestAppear(t *testing.T) {
	var since time.Time
	gtx := layout.Context{Now: time.Unix(100, 0), Ops: new(op.Ops)}
	if v := Appear(gtx, &since, 100*time.Millisecond); v != 0 {
		t.Errorf("start %v", v)
	}
	gtx.Now = gtx.Now.Add(50 * time.Millisecond)
	if v := Appear(gtx, &since, 100*time.Millisecond); v <= .5 || v >= 1 {
		t.Errorf("halfway (eased out) %v", v)
	}
	gtx.Now = gtx.Now.Add(time.Second)
	if v := Appear(gtx, &since, 100*time.Millisecond); v != 1 {
		t.Errorf("end %v", v)
	}
}

func TestTipPos(t *testing.T) {
	win, el, tip := image.Pt(1000, 800), image.Pt(32, 32), image.Pt(120, 24)
	if x, y := tipPos(image.Pt(500, 400), el, tip, win, 8, false); x != -44 || y != -32 {
		t.Errorf("middle: %d %d", x, y)
	}
	if x, _ := tipPos(image.Pt(960, 400), el, tip, win, 8, false); 960+x+tip.X != 992 { // right edge
		t.Errorf("right edge: tip ends at %d", 960+x+tip.X)
	}
	if x, _ := tipPos(image.Pt(0, 400), el, tip, win, 8, false); x != 8 { // left edge
		t.Errorf("left edge: %d", x)
	}
	if _, y := tipPos(image.Pt(500, 10), el, tip, win, 8, false); y != 40 { // no room above
		t.Errorf("top: %d", y)
	}
}
