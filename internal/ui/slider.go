package ui

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// Slider is the thin bar of the player (progress and volume): a grey track
// with a white fill that turns green with a thumb while hovered or dragged.
// A press or drag moves it; Moved reports each new value and Released the
// final one.
type Slider struct {
	drag  gesture.Drag
	hover gesture.Hover
	w     int
	pos   float32 // while dragging, 0..1
	moved bool
	done  bool
	Label string
}

// SliderHeight is the hit area height in dp.
const SliderHeight = 12

// Dragging reports whether the thumb is held (or was just let go and Released
// hasn't been read yet).
func (s *Slider) Dragging() bool { return s.drag.Dragging() || s.done }

// Value is the dragged fraction (0..1); meaningful while Dragging or after Released.
func (s *Slider) Value() float32 { return s.pos }

// Moved reports (once) that the value changed by the pointer since the last call.
func (s *Slider) Moved() bool {
	m := s.moved
	s.moved = false
	return m
}

// Released reports (once) that the pointer was let go; Value is the result.
func (s *Slider) Released() bool {
	d := s.done
	s.done = false
	return d
}

func (s *Slider) update(gtx layout.Context, enabled bool) {
	for {
		e, ok := s.drag.Update(gtx.Metric, gtx.Source, gesture.Horizontal)
		if !ok {
			break
		}
		if !enabled || s.w == 0 {
			continue
		}
		switch e.Kind {
		case pointer.Press, pointer.Drag:
			s.pos, s.moved = clamp01(e.Position.X/float32(s.w)), true
		case pointer.Release:
			s.pos, s.done = clamp01(e.Position.X/float32(s.w)), true
		}
	}
}

// Layout draws the slider filling the available width at frac (0..1); while
// dragging it shows the dragged value instead. enabled=false is inert and
// never shows the thumb.
func (s *Slider) Layout(gtx layout.Context, frac float32, enabled bool) layout.Dimensions {
	s.update(gtx, enabled)
	w, h := gtx.Constraints.Max.X, Dp(gtx, SliderHeight)
	s.w = w
	hovered := s.hover.Update(gtx.Source)
	dragging := s.Dragging() && enabled
	if dragging {
		frac = s.pos
	}
	frac = clamp01(frac)

	st := clip.Rect{Max: image.Pt(w, h)}.Push(gtx.Ops)
	semantic.DescriptionOp(s.Label).Add(gtx.Ops)
	s.drag.Add(gtx.Ops)
	s.hover.Add(gtx.Ops)
	if enabled {
		pointer.CursorPointer.Add(gtx.Ops)
	}
	st.Pop()

	th := Dp(gtx, 4)
	y := (h - th) / 2
	FillRect(gtx.Ops, image.Rect(0, y, w, y+th), th/2, RGB(0x4d4d4d))
	fill := ColText
	on := enabled && (hovered || dragging)
	if on {
		fill = ColAccent
	}
	fw := int(frac * float32(w))
	if fw > 0 {
		FillRect(gtx.Ops, image.Rect(0, y, fw, y+th), th/2, fill)
	}
	if on {
		d := Dp(gtx, 12)
		x := max(0, min(w-d, fw-d/2))
		ty := (h - d) / 2
		Shadow(gtx.Ops, image.Rect(x, ty, x+d, ty+d), d/2, Dp(gtx, 4), Dp(gtx, 2), .5)
		Offset(gtx, x, ty, func() { FillCircle(gtx.Ops, d, ColText) })
	}
	return layout.Dimensions{Size: image.Pt(w, h)}
}
