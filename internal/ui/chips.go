package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
)

// Chips is a wrapping row of filter chips (Home facets, Search tabs).
type Chips struct {
	// Base is the color behind the chips (default ColSurface); the translucent
	// chip fill is pre-blended onto it because Gio blends in linear light,
	// which makes alpha fills look much brighter than in a browser.
	Base  color.NRGBA
	pills []Pill
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
	for i, l := range labels {
		w := Measure(gtx, l, 14, Regular) + Dp(gtx, 24)
		if x > 0 && x+w > maxW {
			x, y = 0, y+h+gap
		}
		Offset(gtx, x, y, func() {
			c.pills[i].Layout(gtx, PillStyle{Text: l, PadX: 12, Bg: Mix(base, ColText, .07), BgHover: Mix(base, ColText, .1), Selected: i == sel})
		})
		x += w + gap
	}
	return layout.Dimensions{Size: image.Pt(maxW, y+h)}, clicked
}
