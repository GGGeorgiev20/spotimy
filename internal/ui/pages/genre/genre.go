// Package genre is a genre or mood page of "Browse all" (/genre/<id>).
package genre

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() {
	ui.Register("genre", func(args []string) ui.Page {
		p := &page{color: ui.RGB(0x535353)}
		p.res.Load(func() (*spotify.BrowsePage, error) { return ui.B.Browse("spotify:page:" + args[0]) })
		return p
	})
}

type page struct {
	res        ui.Res[*spotify.BrowsePage]
	color      color.NRGBA
	ready      bool
	list       ui.List
	shelves    []ui.Shelf
	grid       ui.CategoryGrid
	heroBottom int
}

func (p *page) Chrome() ui.Chrome {
	title := ""
	if p.res.Ok() {
		title = p.res.V.Title
	}
	return ui.Chrome{Title: title, Color: p.color, Threshold: p.heroBottom, List: &p.list}
}

func (p *page) Layout(gtx layout.Context) layout.Dimensions {
	switch {
	case !p.res.Done:
		return ui.Loading(gtx)
	case p.res.Err != nil:
		return ui.Unavailable(gtx, "This page", p.res.Err)
	}
	b := p.res.V
	if !p.ready {
		p.ready = true
		p.color = ui.Hex(b.Color)
		p.shelves = make([]ui.Shelf, len(b.Sections))
	}
	ui.HeaderFade(gtx, p.color, ui.Dp(gtx, heroH)-p.list.ScrollTop())

	ns := len(b.Sections)
	rows := p.grid.Count(gtx, len(b.Categories))
	browse := 0 // the "Browse all" heading, if there are categories
	if rows > 0 {
		browse = 1
	}
	return p.list.Layout(gtx, 1+ns+browse+rows, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == 0:
			return p.hero(gtx, b.Title)
		case i <= ns:
			s := b.Sections[i-1]
			d := p.shelves[i-1].Layout(gtx, s.Items, ui.ShelfOpts{Title: s.Title})
			if i == 1 { // the first shelf sits well below the banner
				d.Size.Y += ui.Dp(gtx, 67)
			}
			return d
		case browse == 1 && i == ns+1:
			return ui.PageInset(gtx, 16, 16, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, "Browse all", 24, ui.Bold, ui.ColText)
			})
		case i < 1+ns+browse+rows:
			return p.grid.Row(gtx, i-1-ns-browse, b.Categories)
		}
		return layout.Dimensions{}
	})
}

const heroH = 320

// hero is the banner: the genre's color with the title at its bottom left.
func (p *page) hero(gtx layout.Context, title string) layout.Dimensions {
	w, h := gtx.Constraints.Max.X, ui.Dp(gtx, heroH)
	p.heroBottom = h
	ui.VGradient(gtx.Ops, image.Rect(0, 0, w, h), p.color, ui.Scale(p.color, .4))
	// clamp(48px, 10cqi, 96px)
	size := max(48, min(96, float32(w)/gtx.Metric.PxPerSp*.1))
	pad := ui.ContentPad(gtx)
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(w-2*pad, h-2*pad)}
	d := ui.Label{Text: title, Size: size, Weight: ui.ExtraBold, MaxLines: 2}.Layout(g)
	call := m.Stop()
	ui.Offset(gtx, pad, h-pad-d.Size.Y, func() { call.Add(gtx.Ops) })
	return layout.Dimensions{Size: image.Pt(w, h)}
}
