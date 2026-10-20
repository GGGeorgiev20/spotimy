package ui

import (
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// CategoryTile is a "Browse all" tile: colored 16:9 card, name top left and
// the artwork tilted into the bottom right corner.
type CategoryTile struct {
	Click Click
}

// Layout draws the tile at the constraint's max width.
func (t *CategoryTile) Layout(gtx layout.Context, c spotify.Category) layout.Dimensions {
	if t.Click.Clicked(gtx) {
		OpenURI(c.URI)
	}
	t.Click.Label = c.Name
	w := gtx.Constraints.Max.X
	h := w * 9 / 16
	return t.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		r := image.Rect(0, 0, w, h)
		defer clip.UniformRRect(r, Dp(gtx, 8)).Push(gtx.Ops).Pop()
		FillRect(gtx.Ops, r, 0, Hex(c.Color))
		if len(c.Images) > 0 {
			// CSS: right/bottom 0, 45% wide, rotate(25deg) translate(18%, -2%).
			s := w * 45 / 100
			ctr := f32.Pt(float32(w)-float32(s)/2, float32(h)-float32(s)/2)
			m := f32.AffineId().Offset(f32.Pt(.18*float32(s), -.02*float32(s))).
				Rotate(ctr, 25*math.Pi/180)
			st := op.Affine(m).Push(gtx.Ops)
			Offset(gtx, w-s, h-s, func() {
				Image{URL: Pick(c.Images, s), Radius: Dp(gtx, 4)}.Layout(gtx, image.Pt(s, s))
			})
			st.Pop()
		}
		// Name: 9% of the tile width, between 16 and 24.
		size := max(16, min(24, float32(w)/gtx.Metric.PxPerSp*.09))
		pad := Dp(gtx, 16)
		Offset(gtx, pad, pad, func() {
			g := gtx
			g.Constraints = layout.Constraints{Max: image.Pt(w-2*pad, h-pad)}
			Label{Text: c.Name, Size: size, Weight: Bold, MaxLines: 3}.Layout(g)
		})
		return layout.Dimensions{Size: r.Max}
	})
}

// CategoryGrid lays category tiles out as rows of a List (auto-fill columns
// of at least 232dp, 24dp apart, inside 24dp of padding).
type CategoryGrid struct {
	tiles []CategoryTile
}

func categoryCols(gtx layout.Context) int {
	gap := Dp(gtx, 24)
	return max(1, (gtx.Constraints.Max.X-2*gap+gap)/(Dp(gtx, 232)+gap))
}

// Count is the number of rows n tiles need at the constraint's width.
func (g *CategoryGrid) Count(gtx layout.Context, n int) int {
	return (n + categoryCols(gtx) - 1) / categoryCols(gtx)
}

// Row draws row r (with the 24dp gap that separates it from the next).
func (g *CategoryGrid) Row(gtx layout.Context, r int, cats []spotify.Category) layout.Dimensions {
	if len(g.tiles) < len(cats) {
		g.tiles = append(g.tiles, make([]CategoryTile, len(cats)-len(g.tiles))...)
	}
	pad, gap := Dp(gtx, 24), Dp(gtx, 24)
	cols := categoryCols(gtx)
	tw := (gtx.Constraints.Max.X - 2*pad - (cols-1)*gap) / cols
	for c := 0; c < cols && r*cols+c < len(cats); c++ {
		i := r*cols + c
		Offset(gtx, pad+c*(tw+gap), 0, func() {
			tg := gtx
			tg.Constraints = layout.Constraints{Max: image.Pt(tw, 1<<20)}
			g.tiles[i].Layout(tg, cats[i])
		})
	}
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, tw*9/16+gap)}
}
