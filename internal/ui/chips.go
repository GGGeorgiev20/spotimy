package ui

import (
	"image"
	"image/color"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// Chips is a wrapping row of filter chips (Home facets, Search tabs).
type Chips struct {
	// Base is the color behind the chips (default ColSurface); the translucent
	// chip fill is pre-blended onto it because Gio blends in linear light,
	// which makes alpha fills look much brighter than in a browser.
	Base color.NRGBA
	// Scroll keeps the chips on one row that scrolls sideways (Search tabs).
	Scroll bool
	pills  []Pill
	scroll gesture.Scroll
	off    int
}

// Layout draws one chip per label (sel is highlighted) and returns the index
// clicked this frame, or -1.
func (c *Chips) Layout(gtx layout.Context, labels []string, sel int) (layout.Dimensions, int) {
	if len(c.pills) < len(labels) {
		c.pills = append(c.pills, make([]Pill, len(labels)-len(c.pills))...)
	}
	clicked := -1
	for i := range labels {
		if c.pills[i].Click.Clicked(gtx) {
			clicked = i
		}
	}
	base := c.Base
	if base == ColNone {
		base = ColSurface
	}
	gap, h := Dp(gtx, 8), Dp(gtx, 32)
	maxW := gtx.Constraints.Max.X
	x, y := 0, 0
	if c.Scroll {
		total := 0
		for _, l := range labels {
			total += Measure(gtx, l, 14, Regular) + Dp(gtx, 24) + gap
		}
		maxOff := max(0, total-gap-maxW)
		d := c.scroll.Update(gtx.Metric, gtx.Source, gtx.Now, gesture.Horizontal,
			pointer.ScrollRange{Min: -c.off, Max: maxOff - c.off}, pointer.ScrollRange{})
		c.off = min(max(c.off+d, 0), maxOff)
		x = -c.off
		defer clip.Rect{Max: image.Pt(maxW, h)}.Push(gtx.Ops).Pop()
		c.scroll.Add(gtx.Ops)
	}
	for i, l := range labels {
		w := Measure(gtx, l, 14, Regular) + Dp(gtx, 24)
		if !c.Scroll && x > 0 && x+w > maxW {
			x, y = 0, y+h+gap
		}
		Offset(gtx, x, y, func() {
			c.pills[i].Layout(gtx, PillStyle{Text: l, PadX: 12, Bg: Mix(base, ColText, .07), BgHover: Mix(base, ColText, .1), Selected: i == sel})
		})
		x += w + gap
	}
	return layout.Dimensions{Size: image.Pt(maxW, y+h)}, clicked
}
