package ui

import (
	"image"
	"math"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// carousel is the scroll state of a shelf's single row of cards. The web
// player's row has fixed-width cards, scrolls sideways and snaps to a card.
type carousel struct {
	scroll gesture.Scroll
	pos    float32 // where the user scrolled to, px
	cur    float32 // the eased, snapped offset drawn, px
	last   time.Time
}

// carouselPitch is the width of one card cell in dp: the web player uses a
// wider card once the main view is at least 1024dp wide.
func carouselPitch(widthDp float32) float32 {
	if widthDp >= 1024 {
		return 195.5
	}
	return 177.7
}

// carousel draws the cards of items; left is the row's start padding in dp.
func (s *Shelf) carousel(gtx layout.Context, items []spotify.Card, o ShelfOpts, left float32) layout.Dimensions {
	c := &s.car
	ppd := gtx.Metric.PxPerDp
	W := gtx.Constraints.Max.X
	pitch := carouselPitch(float32(W)/ppd) * ppd
	lp := Dp(gtx, left)
	maxOff := max(0, float32(lp)+pitch*float32(len(items))+48*ppd-float32(W))

	d := c.scroll.Update(gtx.Metric, gtx.Source, gtx.Now, gesture.Horizontal,
		pointer.ScrollRange{Min: -int(c.pos), Max: int(maxOff - c.pos)}, pointer.ScrollRange{})
	c.pos = min(max(c.pos+float32(d), 0), maxOff)
	snap := float32(math.Round(float64(c.pos/pitch))) * pitch
	if c.pos > maxOff-pitch/2 {
		snap = maxOff
	}
	c.cur = min(c.cur, maxOff)
	if diff := snap - c.cur; diff != 0 {
		dt := min(gtx.Now.Sub(c.last), 50*time.Millisecond)
		c.cur += diff * float32(1-math.Exp(-dt.Seconds()/0.07))
		if math.Abs(float64(snap-c.cur)) < .5 {
			c.cur = snap
		}
		gtx.Execute(op.InvalidateCmd{})
	}
	c.last = gtx.Now

	off := int(c.cur)
	h := 0
	m := op.Record(gtx.Ops)
	for i := max(0, int((float32(off)-float32(lp))/pitch)); i < len(items); i++ {
		x0, x1 := int(float32(i)*pitch), int(float32(i+1)*pitch)
		if lp+x0-off >= W {
			break
		}
		Offset(gtx, lp+x0-off, 0, func() {
			g := gtx
			g.Constraints = layout.Constraints{Min: image.Pt(x1-x0, 0), Max: image.Pt(x1-x0, 1<<20)}
			h = max(h, s.Cards[i].Layout(g, items[i], o.Card).Size.Y)
		})
	}
	call := m.Stop()
	defer clip.Rect(image.Rect(0, 0, W, h)).Push(gtx.Ops).Pop()
	c.scroll.Add(gtx.Ops)
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(W, h)}
}
