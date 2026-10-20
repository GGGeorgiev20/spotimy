package ui

import "gioui.org/layout"

// mainWidth is the main view's width in px, set by layoutMain each frame.
var mainWidth int

// MainDp is the main view's width in dp (the width of the page being laid
// out, whatever nested constraint a helper is drawn in).
func MainDp(gtx layout.Context) float32 {
	w := mainWidth
	if w == 0 {
		w = gtx.Constraints.Max.X
	}
	return float32(w) / gtx.Metric.PxPerDp
}

// ContentT is how far the main view is between 600dp (0) and 1025dp (1)
// wide. The web player scales its page padding and header artwork with it.
func ContentT(gtx layout.Context) float32 {
	return clamp01((MainDp(gtx) - 600) / 425)
}

// ContentPadDp is the page's side padding in dp: 16 up to 24.
func ContentPadDp(gtx layout.Context) float32 { return 16 + 8*ContentT(gtx) }

// ContentPad is ContentPadDp in px.
func ContentPad(gtx layout.Context) int { return Dp(gtx, ContentPadDp(gtx)) }

// PageInset lays out w with the page's side padding and top/bottom dp above
// and below.
func PageInset(gtx layout.Context, top, bottom float32, w layout.Widget) layout.Dimensions {
	p := ContentPadDp(gtx)
	return Pad(gtx, top, p, bottom, p, w)
}
