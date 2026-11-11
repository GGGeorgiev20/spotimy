package ui

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
)

// A middle click fires on release over the clickable, not on press, and not
// when the pointer was dragged off it first.
func TestMiddleClickOnRelease(t *testing.T) {
	var r input.Router
	var c Click
	frame := func() bool {
		gtx := layout.Context{Ops: new(op.Ops), Source: r.Source(), Constraints: layout.Exact(image.Pt(100, 100))}
		got := c.MiddleClicked(gtx)
		c.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Max} })
		r.Frame(gtx.Ops)
		return got
	}
	ev := func(k pointer.Kind, x float32, b pointer.Buttons) {
		r.Queue(pointer.Event{Kind: k, Source: pointer.Mouse, Position: f32.Pt(x, 50), Buttons: b})
	}
	frame()
	ev(pointer.Press, 50, pointer.ButtonTertiary)
	if frame() {
		t.Fatal("fired on press")
	}
	ev(pointer.Release, 50, 0)
	if !frame() {
		t.Fatal("no click on release")
	}
	ev(pointer.Press, 50, pointer.ButtonTertiary)
	frame()
	ev(pointer.Release, 150, 0)
	if frame() {
		t.Fatal("fired on release off the clickable")
	}
}
