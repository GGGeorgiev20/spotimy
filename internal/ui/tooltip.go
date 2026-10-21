package ui

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
)

// hoverTip is the tooltip a panel asked for during this frame.
var hoverTip struct {
	title, sub string
	at         image.Point
	on         bool
	// shown is when the tooltip with this title appeared (it fades in).
	shown     time.Time
	lastTitle string
}

// ShowTip asks for a window-level tooltip (it escapes the panel's clip, like
// the library rail's) with its left edge at at, vertically centered there.
// Call it every frame while it should show.
func ShowTip(title, sub string, at image.Point) {
	hoverTip.title, hoverTip.sub, hoverTip.at, hoverTip.on = title, sub, at, true
}

func layoutHoverTip(gtx layout.Context) {
	if !hoverTip.on {
		hoverTip.shown = time.Time{}
		return
	}
	hoverTip.on = false
	if hoverTip.title != hoverTip.lastTitle {
		hoverTip.shown, hoverTip.lastTitle = time.Time{}, hoverTip.title
	}
	v := Appear(gtx, &hoverTip.shown, popupFade)
	g := gtx
	tw := max(Measure(gtx, hoverTip.title, 14, Bold), Measure(gtx, hoverTip.sub, 14, Regular))
	g.Constraints = layout.Constraints{Max: image.Pt(min(Dp(gtx, 300), tw+Dp(gtx, 16)), Dp(gtx, 100))}
	m := op.Record(gtx.Ops)
	d := Pad(g, 4, 8, 4, 8, func(gtx layout.Context) layout.Dimensions {
		rows := []layout.Widget{func(gtx layout.Context) layout.Dimensions { return tipLine(gtx, hoverTip.title, Bold, ColText) }}
		if hoverTip.sub != "" {
			rows = append(rows, func(gtx layout.Context) layout.Dimensions { return tipLine(gtx, hoverTip.sub, Regular, ColText2) })
		}
		return Column(gtx, rows...)
	})
	call := m.Stop()
	Offset(gtx, hoverTip.at.X, hoverTip.at.Y-d.Size.Y/2, func() {
		defer paint.PushOpacity(gtx.Ops, v).Pop()
		r := image.Rectangle{Max: d.Size}
		popupShadow(gtx, r)
		FillRect(gtx.Ops, r, Dp(gtx, 4), RGB(0x282828))
		call.Add(gtx.Ops)
	})
}
