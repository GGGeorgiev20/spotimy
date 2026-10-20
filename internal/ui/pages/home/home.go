// Package home is the home page (/): filter chips, the shortcut tiles and
// the shelves of recommendations.
package home

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

// defaultTint is the page gradient's color until a shortcut is hovered.
var defaultTint = ui.RGB(0x535353)

func init() {
	ui.Register("", func([]string) ui.Page {
		p := &page{tint: defaultTint, target: defaultTint, labels: []string{"All"}}
		p.load()
		return p
	})
}

type page struct {
	facet   string
	res     ui.Res[*spotify.Home]
	cur     *spotify.Home // the result the fields below were built from
	chips   ui.Chips
	labels  []string
	tiles   []tile
	short   []spotify.Card
	shelves []spotify.Shelf
	shelf   []ui.Shelf
	list    ui.List

	// The page's top gradient follows the cover of the hovered shortcut.
	tint, target color.NRGBA
	hovered      int
	covers       map[string]color.NRGBA
}

func (p *page) load() { p.res.Load(func() (*spotify.Home, error) { return ui.B.Home(p.facet) }) }

// setup rebuilds the derived lists when a (new) result arrives.
func (p *page) setup(h *spotify.Home) {
	p.cur = h
	p.short, p.shelves = nil, nil
	for _, s := range h.Sections {
		if s.Kind == "shortcuts" {
			p.short = s.Items[:min(8, len(s.Items))]
		} else {
			p.shelves = append(p.shelves, s)
		}
	}
	p.tiles = make([]tile, len(p.short))
	p.shelf = make([]ui.Shelf, len(p.shelves))
	p.hovered = -1
	p.covers = map[string]color.NRGBA{}
	p.labels = []string{"All"}
	for _, c := range h.Chips {
		p.labels = append(p.labels, c.Label)
	}
}

// cover makes shortcut i's cover color the gradient's target.
func (p *page) cover(i int) {
	c := p.short[i]
	if col, ok := p.covers[c.URI]; ok {
		p.target = col
		return
	}
	h := p.cur
	ui.CoverColor(c.Images, func(col color.NRGBA) {
		if h != p.cur {
			return
		}
		p.covers[c.URI] = col
		if i == p.hovered {
			p.target = col
		}
	})
}

const (
	itemChips = iota
	itemTiles
	itemShelves
)

func (p *page) Layout(gtx layout.Context) layout.Dimensions {
	if p.res.Done && p.res.Err == nil && p.res.V != p.cur {
		p.setup(p.res.V)
	}
	// Gradient behind the list, scrolling with it.
	p.stepTint(gtx)
	h := ui.Dp(gtx, 256)
	top := -p.list.ScrollTop()
	ui.VGradient(gtx.Ops, image.Rect(0, top, gtx.Constraints.Max.X, top+h), ui.Scale(p.tint, .4), ui.ColSurface)

	n := itemShelves + len(p.shelves) + 1
	return p.list.Layout(gtx, n, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == itemChips:
			return p.chipRow(gtx)
		case !p.res.Done && p.cur == nil:
			if i == itemTiles {
				return ui.Loading(gtx)
			}
			return layout.Dimensions{}
		case p.res.Err != nil && p.cur == nil:
			if i == itemTiles {
				return ui.Unavailable(gtx, "Home", p.res.Err)
			}
			return layout.Dimensions{}
		case i == itemTiles:
			return p.grid(gtx)
		case i < itemShelves+len(p.shelves):
			return p.layoutShelf(gtx, i-itemShelves)
		}
		return ui.Gap(0, 24)(gtx)
	})
}

// gradAt is the page gradient's color y px below the top of the content.
func (p *page) gradAt(gtx layout.Context, y int) color.NRGBA {
	return ui.Mix(ui.Scale(p.tint, .4), ui.ColSurface, float32(y)/float32(ui.Dp(gtx, 256)))
}

// stepTint eases the gradient color toward the hovered cover.
func (p *page) stepTint(gtx layout.Context) {
	if p.tint == p.target {
		return
	}
	next := ui.Mix(p.tint, p.target, .12)
	if next == p.tint { // Mix rounds; close enough to snap
		next = p.target
	}
	p.tint = next
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
}

func (p *page) chipRow(gtx layout.Context) layout.Dimensions {
	sel := 0
	if p.cur != nil {
		for i, c := range p.cur.Chips {
			if c.ID == p.facet {
				sel = i + 1
			}
		}
	}
	var clicked int
	d := ui.Pad(gtx, 20, pagePad, 16, pagePad, func(gtx layout.Context) layout.Dimensions {
		p.chips.Base = p.gradAt(gtx, ui.Dp(gtx, 36))
		d, c := p.chips.Layout(gtx, p.labels, sel)
		clicked = c
		return d
	})
	if clicked >= 0 && p.cur != nil {
		facet := ""
		if clicked > 0 {
			facet = p.cur.Chips[clicked-1].ID
		}
		if facet != p.facet {
			p.facet = facet
			p.load()
		}
	}
	return d
}

func (p *page) layoutShelf(gtx layout.Context, i int) layout.Dimensions {
	s := p.shelves[i]
	return p.shelf[i].Layout(gtx, s.Items, ui.ShelfOpts{
		Title: s.Title, Label: s.Label, Header: s.Header, Inset: pagePad, Carousel: true,
		OnShowAll: func() { ui.Navigate(ui.URIToPath(s.URI)) },
	})
}

// pagePad is the page's side padding in dp.
const pagePad = 40

// grid is the shortcut tiles: 4 columns (2 below 800dp wide), 48dp tall and
// 8dp apart, or 64dp tall and 12dp apart from 1200dp.
func (p *page) grid(gtx layout.Context) layout.Dimensions {
	if len(p.short) == 0 {
		return layout.Dimensions{}
	}
	wdp := float32(gtx.Constraints.Max.X) / gtx.Metric.PxPerDp
	th, gp := float32(48), float32(8)
	if wdp >= 1200 {
		th, gp = 64, 12
	}
	gap, h := ui.Dp(gtx, gp), ui.Dp(gtx, th)
	pad := ui.Dp(gtx, pagePad)
	w := gtx.Constraints.Max.X - 2*pad
	cols := 4
	if wdp < 800 {
		cols = 2
	}
	cw := (w - (cols-1)*gap) / cols
	hovered := -1
	for i := range p.short {
		x, y := pad+(i%cols)*(cw+gap), (i/cols)*(h+gap)
		ui.Offset(gtx, x, y, func() {
			g := gtx
			g.Constraints = layout.Exact(image.Pt(cw, h))
			p.tiles[i].layout(g, p.short[i], p.gradAt(gtx, ui.Dp(gtx, 68)+(i/cols)*(h+gap)+h/2))
		})
		if p.tiles[i].click.Hovered() {
			hovered = i
		}
	}
	switch {
	case hovered >= 0 && hovered != p.hovered:
		p.hovered = hovered
		p.cover(hovered)
	case hovered < 0 && p.hovered >= 0: // back to the default gray
		p.hovered = -1
		p.target = defaultTint
	}
	rows := (len(p.short) + cols - 1) / cols
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, rows*(h+gap)-gap+ui.Dp(gtx, 40))}
}

// tile is one shortcut: cover, name, and a play button on hover.
type tile struct {
	click ui.Click
	play  ui.PlayButton
	hover ui.Hover
}

func (t *tile) layout(gtx layout.Context, c spotify.Card, base color.NRGBA) {
	if t.play.Click.Clicked(gtx) {
		ui.PlayContext(c.URI)
	}
	if t.click.Clicked(gtx) {
		ui.OpenURI(c.URI)
	}
	if _, ok := t.click.RightClicked(gtx); ok {
		ui.OpenMenu(ui.CardMenu(c, ui.CardMenuOpts{}))
	}
	t.click.Label = c.Name
	t.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		w, h := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
		hov := t.click.Hovered()
		bg := ui.Mix(base, ui.ColText, .1)
		if hov {
			bg = ui.Mix(base, ui.ColText, .2)
		}
		r := image.Rect(0, 0, w, h)
		defer clip.UniformRRect(r, ui.Dp(gtx, 4)).Push(gtx.Ops).Pop()
		ui.FillRect(gtx.Ops, r, 0, bg)
		img := h
		ui.Image{URL: ui.Pick(c.Images, img), Icon: "note"}.Layout(gtx, image.Pt(img, img))
		btn := ui.Dp(gtx, 32)
		x := img + ui.Dp(gtx, 8)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(max(0, w-x-ui.Dp(gtx, 8)), h)}
		m := op.Record(gtx.Ops)
		d := ui.Label{Text: c.Name, Size: 14, Weight: ui.Bold, MaxLines: 2}.Layout(g)
		call := m.Stop()
		ui.Offset(gtx, x, (h-d.Size.Y)/2, func() { call.Add(gtx.Ops) })
		if v := t.hover.Step(gtx, hov, 150*time.Millisecond); v > 0 {
			ui.Offset(gtx, w-ui.Dp(gtx, 8)-btn, (h-btn)/2, func() {
				st := paint.PushOpacity(gtx.Ops, v)
				ui.Shadow(gtx.Ops, image.Rect(0, 0, btn, btn), btn/2, ui.Dp(gtx, 4), ui.Dp(gtx, 4), .3)
				t.play.Layout(gtx, 32, ui.CtxPlaying(c.URI))
				st.Pop()
			})
		}
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}
