package ui

import (
	"image"

	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// CardRows is a grid of cards for pages whose content is one List: Count
// gives the rows at the current width, Row draws one (CSS grid auto-fill,
// minmax(minW, 1fr), inside the page padding less 12dp).
type CardRows struct {
	cards []Card
}

// Count is the number of rows n cards need at the constraint's width.
func (g *CardRows) Count(gtx layout.Context, n int, minW float32) int {
	cols := GridColumns(narrow(gtx, ContentPadDp(gtx)-12), minW)
	return (n + cols - 1) / cols
}

// Row draws row r of items.
func (g *CardRows) Row(gtx layout.Context, r int, items []spotify.Card, minW float32, o CardOpts) layout.Dimensions {
	if len(g.cards) < len(items) {
		g.cards = append(g.cards, make([]Card, len(items)-len(g.cards))...)
	}
	pad := Dp(gtx, ContentPadDp(gtx)-12)
	gtx = narrow(gtx, ContentPadDp(gtx)-12)
	cols := GridColumns(gtx, minW)
	cw := gtx.Constraints.Max.X / cols
	h := 0
	for c := 0; c < cols && r*cols+c < len(items); c++ {
		i := r*cols + c
		Offset(gtx, pad+c*cw, 0, func() {
			cg := gtx
			cg.Constraints = layout.Constraints{Min: image.Pt(cw, 0), Max: image.Pt(cw, 1<<20)}
			h = max(h, g.cards[i].Layout(cg, items[i], o).Size.Y)
		})
	}
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X+2*pad, h)}
}

// narrow narrows the constraint's width by dp on both sides (layout only; the
// caller offsets).
func narrow(gtx layout.Context, dp float32) layout.Context {
	gtx.Constraints.Max.X = max(0, gtx.Constraints.Max.X-2*Dp(gtx, dp))
	gtx.Constraints.Min.X = min(gtx.Constraints.Min.X, gtx.Constraints.Max.X)
	return gtx
}
