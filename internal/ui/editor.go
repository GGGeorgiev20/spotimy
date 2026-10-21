package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Input is a text field.
type Input struct {
	widget.Editor
	focused bool
}

type InputStyle struct {
	Hint     string
	Size     float32 // font size (default 14)
	Color    color.NRGBA
	HintCol  color.NRGBA
	MaxLines int // 0 = single line
}

// Focused reports keyboard focus (as of the last frame).
func (in *Input) Focused() bool { return in.focused }

func (in *Input) Focus(gtx layout.Context) { gtx.Execute(key.FocusCmd{Tag: &in.Editor}) }

// Blur drops keyboard focus.
func (in *Input) Blur(gtx layout.Context) { gtx.Execute(key.FocusCmd{}) }

// Poll consumes the editor's events: text edits and Enter (single line).
func (in *Input) Poll(gtx layout.Context) (changed, submitted bool) {
	for {
		e, ok := in.Editor.Update(gtx)
		if !ok {
			return
		}
		switch e.(type) {
		case widget.ChangeEvent:
			changed = true
		case widget.SubmitEvent:
			submitted = true
		}
	}
}

func (in *Input) Layout(gtx layout.Context, s InputStyle) layout.Dimensions {
	if s.Size == 0 {
		s.Size = 14
	}
	if s.Color == ColNone {
		s.Color = ColText
	}
	if s.HintCol == ColNone {
		s.HintCol = ColText2
	}
	in.SingleLine = s.MaxLines == 0
	in.Submit = in.SingleLine
	in.focused = gtx.Focused(&in.Editor)
	if in.focused {
		inputFocused = true
	}
	f := font.Font{Typeface: "Figtree"}
	sel := Material(gtx, Alpha(ColAccent, .4))
	if in.Editor.Len() == 0 {
		m := op.Record(gtx.Ops)
		Label{Text: s.Hint, Size: s.Size, Color: s.HintCol}.Layout(gtx)
		hint := m.Stop()
		hint.Add(gtx.Ops)
	}
	d := in.Editor.Layout(gtx, Shaper, f, unit.Sp(s.Size), Material(gtx, s.Color), sel)
	if d.Size.X < gtx.Constraints.Min.X {
		d.Size.X = gtx.Constraints.Min.X
	}
	st := clip.Rect{Max: d.Size}.Push(gtx.Ops)
	pointer.CursorText.Add(gtx.Ops)
	st.Pop()
	return d
}

// FieldBox draws the dialog-style input background (light translucent box,
// highlighted while focused) around an Input of height h dp.
func FieldBox(gtx layout.Context, in *Input, s InputStyle, h float32) layout.Dimensions {
	w := gtx.Constraints.Max.X
	hp := Dp(gtx, h)
	bg := Alpha(ColText, .1)
	if in.Focused() {
		bg = RGB(0x333333)
	}
	FillRect(gtx.Ops, image.Rect(0, 0, w, hp), Dp(gtx, 4), bg)
	if in.Focused() {
		StrokeRRect(gtx.Ops, image.Rect(0, 0, w, hp), Dp(gtx, 4), max(1, Dp(gtx, 1)), RGB(0x535353))
	}
	Offset(gtx, Dp(gtx, 12), Dp(gtx, 10), func() {
		g := gtx
		g.Constraints = layout.Constraints{Min: image.Pt(w-Dp(gtx, 24), 0), Max: image.Pt(w-Dp(gtx, 24), hp-Dp(gtx, 20))}
		in.Layout(g, s)
	})
	return layout.Dimensions{Size: image.Pt(w, hp)}
}
