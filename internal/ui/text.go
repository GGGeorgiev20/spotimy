package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Label is a run of text. Size is in sp (= CSS px at 1x). MaxLines 0 means 1;
// use -1 for unlimited. Overflow is ellipsized.
type Label struct {
	Text     string
	Size     float32
	Weight   font.Weight
	Color    color.NRGBA
	MaxLines int
	Align    text.Alignment
	// LineHeight in sp; 0 uses the font's default (about 1.2×).
	LineHeight float32
}

func (l Label) Layout(gtx layout.Context) layout.Dimensions {
	size := l.Size
	if size == 0 {
		size = 14
	}
	c := l.Color
	if c == (color.NRGBA{}) {
		c = ColText
	}
	lines := l.MaxLines
	switch {
	case lines == 0:
		lines = 1
	case lines < 0:
		lines = 0
	}
	gtx.Constraints.Min = image.Point{}
	w := widget.Label{MaxLines: lines, Truncator: "…", Alignment: l.Align, LineHeight: unit.Sp(l.LineHeight), WrapPolicy: text.WrapWords}
	return w.Layout(gtx, Shaper, font.Font{Typeface: "Figtree", Weight: l.Weight}, unit.Sp(size), l.Text, Material(gtx, c))
}

// Material records a paint color for text.
func Material(gtx layout.Context, c color.NRGBA) op.CallOp {
	m := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	return m.Stop()
}

// Text lays out a single ellipsized line.
func Text(gtx layout.Context, s string, size float32, w font.Weight, c color.NRGBA) layout.Dimensions {
	return Label{Text: s, Size: size, Weight: w, Color: c}.Layout(gtx)
}

// Measure returns the width of s on one line, without drawing it.
func Measure(gtx layout.Context, s string, size float32, w font.Weight) int {
	m := op.Record(gtx.Ops)
	gtx.Constraints = layout.Constraints{Max: image.Pt(1e6, 1e6)}
	d := Label{Text: s, Size: size, Weight: w}.Layout(gtx)
	m.Stop()
	return d.Size.X
}

// Link is text that underlines on hover and runs onClick (if set) when clicked.
type Link struct {
	Click Click
}

// Layout draws s; hoverColor is used while hovered (ColNone keeps c).
func (k *Link) Layout(gtx layout.Context, s string, size float32, w font.Weight, c, hoverColor color.NRGBA) layout.Dimensions {
	return k.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		col := c
		if k.Click.Hovered() && hoverColor != ColNone {
			col = hoverColor
		}
		d := Text(gtx, s, size, w, col)
		if k.Click.Hovered() {
			y := d.Size.Y - d.Baseline + max(1, Dp(gtx, 1))
			FillRect(gtx.Ops, image.Rect(0, y, d.Size.X, y+max(1, Dp(gtx, 1))), 0, col)
		}
		return d
	})
}

// Seg is one piece of an Inline run. A segment with a Link is clickable.
type Seg struct {
	Text   string
	Link   *Link
	Color  color.NRGBA // ColNone = the run's color
	Weight font.Weight // 0 = the run's weight
	Widget layout.Widget
}

// Inline lays out segments on one line; the first one that doesn't fit is
// ellipsized and the rest are dropped. Hovered links turn hover (if set).
func Inline(gtx layout.Context, size float32, w font.Weight, c, hover color.NRGBA, segs ...Seg) layout.Dimensions {
	maxW := gtx.Constraints.Max.X
	x, h, base := 0, 0, 0
	type placed struct {
		call op.CallOp
		d    layout.Dimensions
	}
	var out []placed
	for _, s := range segs {
		if x >= maxW {
			break
		}
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(maxW-x, gtx.Constraints.Max.Y)}
		m := op.Record(gtx.Ops)
		col, wt := s.Color, s.Weight
		if col == ColNone {
			col = c
		}
		if wt == 0 {
			wt = w
		}
		var d layout.Dimensions
		switch {
		case s.Widget != nil:
			d = s.Widget(g)
		case s.Link != nil:
			d = s.Link.Layout(g, s.Text, size, wt, col, hover)
		default:
			d = Text(g, s.Text, size, wt, col)
		}
		out = append(out, placed{m.Stop(), d})
		x += d.Size.X
		h = max(h, d.Size.Y)
		base = max(base, d.Size.Y-d.Baseline)
	}
	x = 0
	for _, p := range out {
		// Align baselines.
		y := base - (p.d.Size.Y - p.d.Baseline)
		st := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		p.call.Add(gtx.Ops)
		st.Pop()
		x += p.d.Size.X
	}
	return layout.Dimensions{Size: image.Pt(x, h), Baseline: h - base}
}
