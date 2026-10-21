package ui

import (
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// Switch is the settings toggle: a 42×24 pill, green when on, whose knob
// slides across in 100ms like the web player's.
type Switch struct {
	Click Click
	knob  Hover
}

// Layout draws the switch for on and reports a click (the new value is !on).
func (s *Switch) Layout(gtx layout.Context, on bool, label string) (layout.Dimensions, bool) {
	clicked := s.Click.Clicked(gtx)
	if clicked {
		on = !on
	}
	s.Click.Label = label
	v := s.knob.Step(gtx, on, 100*time.Millisecond)
	w, h := Dp(gtx, 42), Dp(gtx, 24)
	d := s.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		off, onC := RGB(0x535353), RGB(0x1db954)
		if s.Click.Hovered() {
			off, onC = RGB(0x727272), ColAccent
		}
		FillRect(gtx.Ops, image.Rect(0, 0, w, h), h/2, Mix(off, onC, v))
		k, in := Dp(gtx, 20), Dp(gtx, 2)
		x := in + int(v*float32(w-k-2*in)+.5)
		Offset(gtx, x, in, func() { FillCircle(gtx.Ops, k, ColText) })
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
	return d, clicked
}

// Option is one choice of a Select.
type Option struct {
	Value, Label string
}

// Select is the settings dropdown: the chosen option on a dark rounded box
// with a chevron; a click lists the options under it.
type Select struct {
	Click  Click
	list   List
	rows   []Click
	ptr    f32.Point // the pointer over the box, in its coordinates
	pick   string
	picked bool
}

// Layout draws the box w px wide showing cur's label and reports a newly
// picked value.
func (s *Select) Layout(gtx layout.Context, w int, opts []Option, cur string) (layout.Dimensions, string, bool) {
	picked, v := s.picked, s.pick
	s.picked = false
	label := cur
	for _, o := range opts {
		if o.Value == cur {
			label = o.Label
		}
	}
	h := Dp(gtx, 32)
	for {
		e, ok := gtx.Event(pointer.Filter{Target: &s.ptr, Kinds: pointer.Move | pointer.Enter | pointer.Press})
		if !ok {
			break
		}
		if pe, ok := e.(pointer.Event); ok {
			s.ptr = pe.Position
		}
	}
	if s.Click.Clicked(gtx) {
		at := PointerPos().Sub(s.ptr.Round()).Add(image.Pt(0, h+Dp(gtx, 4)))
		OpenMenuWide(at, float32(w)/gtx.Metric.PxPerDp, []MenuItem{{Widget: s.options(opts, cur, w)}})
	}
	s.Click.Label = label
	d := s.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := RGB(0x1f1f1f)
		if s.Click.Hovered() {
			bg = RGB(0x2a2a2a)
		}
		FillRect(gtx.Ops, image.Rect(0, 0, w, h), Dp(gtx, 4), bg)
		is := Dp(gtx, 16)
		Offset(gtx, Dp(gtx, 12), (h-Dp(gtx, 20))/2, func() {
			g := gtx
			g.Constraints.Max.X = max(0, w-Dp(gtx, 12+8+8)-is)
			Text(g, label, 14, Regular, ColText2)
		})
		Offset(gtx, w-Dp(gtx, 8)-is, (h-is)/2, func() { IconPx(gtx, "chevron-down", is, ColText2) })
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
	st := clip.Rect{Max: d.Size}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &s.ptr)
	pass.Pop()
	st.Pop()
	return d, v, picked
}

// options is the open list: 40dp rows (at most 10 shown, scrolling), the
// current one checked.
func (s *Select) options(opts []Option, cur string, w int) layout.Widget {
	if len(s.rows) < len(opts) {
		s.rows = make([]Click, len(opts))
	}
	return func(gtx layout.Context) layout.Dimensions {
		rowH := Dp(gtx, 40)
		rw := gtx.Constraints.Max.X
		g := gtx
		g.Constraints = layout.Exact(image.Pt(rw, rowH*min(len(opts), 10)))
		return s.list.Layout(g, len(opts), func(gtx layout.Context, i int) layout.Dimensions {
			o, c := opts[i], &s.rows[i]
			if c.Clicked(gtx) {
				s.pick, s.picked = o.Value, true
				CloseMenu()
			}
			c.Label = o.Label
			return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				fg := Alpha(ColText, .9)
				if c.Hovered() {
					FillRect(gtx.Ops, image.Rect(0, 0, rw, rowH), Dp(gtx, 2), Alpha(ColText, .1))
					fg = ColText
				}
				is := Dp(gtx, 16)
				Offset(gtx, Dp(gtx, 12), (rowH-Dp(gtx, 20))/2, func() {
					g := gtx
					g.Constraints.Max.X = max(0, rw-Dp(gtx, 12+8+12)-is)
					Text(g, o.Label, 14, Regular, fg)
				})
				if o.Value == cur {
					Offset(gtx, rw-Dp(gtx, 8)-is, (rowH-is)/2, func() { IconPx(gtx, "check", is, ColAccent) })
				}
				return layout.Dimensions{Size: image.Pt(rw, rowH)}
			})
		})
	}
}
