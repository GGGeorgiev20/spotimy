package search

import (
	"image"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

// tabBar is the row of tabs. It's also drawn pinned (solid) at the top once
// the page has scrolled.
func (p *page) tabBar(gtx layout.Context, solid bool) layout.Dimensions {
	sel := 0
	for i, t := range tabs {
		if t.id == p.tab {
			sel = i
		}
	}
	m := op.Record(gtx.Ops)
	d := inBox(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.Pad(gtx, 16, 0, 16, 0, func(gtx layout.Context) layout.Dimensions {
			d, clicked := p.chips.Layout(gtx, tabLabels, sel)
			if clicked >= 0 && clicked != sel {
				ui.Navigate(p.tabPath(tabs[clicked].id))
			}
			return d
		})
	})
	call := m.Stop()
	if solid {
		ui.FillRect(gtx.Ops, image.Rectangle{Max: d.Size}, 0, ui.ColSurface)
	}
	call.Add(gtx.Ops)
	return d
}

// item 0 is the tab bar; the content follows.
func (p *page) layoutResults(gtx layout.Context) layout.Dimensions {
	p.list.Sticky = func(gtx layout.Context) layout.Dimensions {
		d := p.tabBar(gtx, true)
		if p.tab == "tracks" && p.paged != nil && p.list.Scrolled(1) {
			ui.Offset(gtx, 0, d.Size.Y, func() { p.table.Header(gtx, true) })
		}
		return d
	}
	p.list.StickyIndex = 0
	bar := func(gtx layout.Context) layout.Dimensions { return p.tabBar(gtx, false) }
	switch p.tab {
	case "all":
		return p.layoutAll(gtx, bar)
	case "tracks":
		return p.layoutSongs(gtx, bar)
	}
	return p.layoutCards(gtx, bar)
}

// single lays out the tab bar and one content widget.
func (p *page) single(gtx layout.Context, bar, content layout.Widget) layout.Dimensions {
	return p.list.Layout(gtx, 2, func(gtx layout.Context, i int) layout.Dimensions {
		if i == 0 {
			return bar(gtx)
		}
		return content(gtx)
	})
}

func hint(gtx layout.Context, q string) layout.Dimensions {
	return ui.PageInset(gtx, 64, 64, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Label{Text: `No results found for "` + q + `"`, Size: 24, Weight: ui.Bold, Align: text.Middle, MaxLines: 2}.Layout(gtx)
			}),
			layout.Rigid(ui.Gap(0, 8)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Label{Text: "Please make sure your words are spelled correctly, or use fewer or different keywords.", Color: ui.ColText2, Align: text.Middle, MaxLines: 3, Size: 16}.Layout(gtx)
			}),
		)
	})
}

// ---- All ----

func (p *page) layoutAll(gtx layout.Context, bar layout.Widget) layout.Dimensions {
	switch {
	case !p.all.Done:
		return p.single(gtx, bar, ui.Loading)
	case p.all.Err != nil:
		return p.single(gtx, bar, func(gtx layout.Context) layout.Dimensions { return ui.Unavailable(gtx, "Search", p.all.Err) })
	}
	r := p.all.V
	if r.Top == nil && len(r.Tracks) == 0 && len(r.Groups) == 0 {
		return p.single(gtx, bar, func(gtx layout.Context) layout.Dimensions { return hint(gtx, p.query) })
	}
	if !p.built {
		p.built = true
		p.shelves = make([]ui.Shelf, len(r.Groups))
		if len(r.Tracks) > 0 {
			p.topRows = &ui.Paged{}
			p.topRows.SetAll(r.Tracks[:min(3, len(r.Tracks))])
			p.topTab = ui.TrackTable{Variant: "compact", OnPlayed: func(t spotify.Track) { remember(t.URI) }}
		}
	}
	return p.list.Layout(gtx, len(r.Groups)+3, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == 0:
			return bar(gtx)
		case i == 1:
			return p.topRow(gtx, r.Top)
		case i < 2+len(r.Groups):
			g := r.Groups[i-2]
			o := ui.ShelfOpts{Title: g.Title, Carousel: true, Boxed: true, Pitch: 133.3, Card: ui.CardOpts{OnOpen: func(c spotify.Card) { remember(c.URI) }}}
			if tab, ok := groupTab[g.Title]; ok {
				o.OnShowAll = func() { ui.Navigate(p.tabPath(tab)) }
			}
			return inBox(gtx, func(gtx layout.Context) layout.Dimensions { return p.shelves[i-2].Layout(gtx, g.Items, o) })
		}
		return ui.Gap(0, 24)(gtx)
	})
}

// box is the page's content column: the main view less its padding, but at
// most 1152dp wide and centered.
func box(gtx layout.Context) (left, w int) {
	full := gtx.Constraints.Max.X
	w = min(full-2*ui.ContentPad(gtx), ui.Dp(gtx, 1152))
	return (full - w) / 2, w
}

// inBox lays f out in the content column (full height, the page's width).
func inBox(gtx layout.Context, f layout.Widget) layout.Dimensions {
	left, w := box(gtx)
	full := gtx.Constraints.Max.X
	g := gtx
	g.Constraints.Max.X, g.Constraints.Min.X = w, 0
	st := op.Offset(image.Pt(left, 0)).Push(gtx.Ops)
	d := f(g)
	st.Pop()
	return layout.Dimensions{Size: image.Pt(full, d.Size.Y)}
}

// topRow is the web player's best match block: a wide top result card and
// the first three songs under it.
func (p *page) topRow(gtx layout.Context, top *spotify.Card) layout.Dimensions {
	return inBox(gtx, func(gtx layout.Context) layout.Dimensions {
		w, y := gtx.Constraints.Max.X, 0
		if top != nil {
			p.top.layout(gtx, *top, w)
			y = ui.Dp(gtx, 104+16)
		}
		if p.topRows != nil {
			rh := ui.SongRowHeight(gtx)
			for i, t := range p.topRows.Items {
				ui.Offset(gtx, 0, y+i*rh, func() { p.topTab.Row(gtx, i, t) })
			}
			y += len(p.topRows.Items) * rh
		}
		return layout.Dimensions{Size: image.Pt(w, y)}
	})
}

// topCard is the "Top result" card.
type topCard struct {
	click ui.Click
	play  ui.PlayButton
	hover ui.Hover
}

func typeWord(t string) string {
	if t == "" {
		return ""
	}
	return strings.ToUpper(t[:1]) + t[1:]
}

// layout draws the 104dp card: cover, name over type, play button on hover.
func (t *topCard) layout(gtx layout.Context, c spotify.Card, w int) layout.Dimensions {
	if t.play.Click.Clicked(gtx) {
		remember(c.URI)
		ui.PlayContext(c.URI)
	}
	if t.click.Clicked(gtx) {
		remember(c.URI)
		ui.OpenURI(c.URI)
	}
	if _, ok := t.click.RightClicked(gtx); ok {
		ui.OpenMenu(ui.CardMenu(c, ui.CardMenuOpts{}))
	}
	t.click.Label = c.Name
	h := ui.Dp(gtx, 104)
	return t.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hov := t.click.Hovered()
		bg := ui.Mix(ui.ColSurface, ui.ColText, .1)
		if hov {
			bg = ui.Mix(ui.ColSurface, ui.ColText, .15)
		}
		ui.FillRect(gtx.Ops, image.Rect(0, 0, w, h), ui.Dp(gtx, 6), bg)
		pad, art := ui.Dp(gtx, 16), ui.Dp(gtx, 72)
		round := c.Type == "artist"
		radius := ui.Dp(gtx, 4)
		if round {
			radius = art / 2
		}
		icon := "note"
		if round {
			icon = "user"
		}
		ui.Offset(gtx, pad, pad, func() {
			ui.Image{URL: ui.Pick(c.Images, art), Radius: radius, Round: round, Icon: icon}.Layout(gtx, image.Pt(art, art))
		})
		bs := ui.Dp(gtx, 48)
		x := pad + art + ui.Dp(gtx, 24)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(max(0, w-pad-bs-ui.Dp(gtx, 8)-x), h)}
		size := float32(20)
		if ui.MainDp(gtx) >= 768 {
			size = 24
		}
		m := op.Record(gtx.Ops)
		nd := ui.Label{Text: c.Name, Size: size, Weight: ui.Bold, LineHeight: size}.Layout(g)
		segs := []ui.Seg{{Text: typeWord(c.Type), Color: ui.ColText2}}
		if c.Subtitle != "" && c.Subtitle != "Artist" {
			segs = append(segs, ui.Seg{Text: " • " + c.Subtitle})
		}
		var sd layout.Dimensions
		ui.Offset(gtx, 0, nd.Size.Y+ui.Dp(gtx, 2), func() { sd = ui.Inline(g, 14, ui.Regular, ui.ColText2, ui.ColNone, segs...) })
		call := m.Stop()
		ui.Offset(gtx, x, (h-nd.Size.Y-ui.Dp(gtx, 2)-sd.Size.Y)/2, func() { call.Add(gtx.Ops) })
		playing := ui.CtxPlaying(c.URI)
		if v := t.hover.Step(gtx, hov || playing, 200*time.Millisecond); v > 0 {
			ui.Offset(gtx, w-pad-bs, (h-bs)/2+int((1-v)*float32(ui.Dp(gtx, 8))), func() {
				st := paint.PushOpacity(gtx.Ops, v)
				ui.Shadow(gtx.Ops, image.Rect(0, 0, bs, bs), bs/2, ui.Dp(gtx, 4), ui.Dp(gtx, 6), .3)
				t.play.Layout(gtx, 48, playing)
				st.Pop()
			})
		}
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}

// ---- Songs ----

func (p *page) layoutSongs(gtx layout.Context, bar layout.Widget) layout.Dimensions {
	if p.res.Done && p.res.Err == nil && p.paged == nil {
		r := p.res.V
		p.paged = ui.NewPaged(func(off, lim int) (*spotify.TrackPage, error) {
			r, err := ui.B.SearchType(p.query, "tracks", off, lim)
			if err != nil {
				return nil, err
			}
			return &spotify.TrackPage{Total: r.Total, Offset: off, Items: r.Tracks}, nil
		}, 50)
		p.paged.Seed(spotify.TrackPage{Total: r.Total, Items: r.Tracks})
		p.table = ui.TrackTable{Variant: "search", OnPlayed: func(t spotify.Track) { remember(t.URI) }}
	}
	switch {
	case !p.res.Done:
		return p.single(gtx, bar, ui.Loading)
	case p.res.Err != nil:
		return p.single(gtx, bar, func(gtx layout.Context) layout.Dimensions { return ui.Unavailable(gtx, "Search", p.res.Err) })
	case p.paged.Total == 0:
		return p.single(gtx, bar, func(gtx layout.Context) layout.Dimensions { return hint(gtx, p.query) })
	}
	first, last := p.list.Range()
	p.paged.Ensure(first-2, last-1)
	n := p.paged.Total
	return p.list.Layout(gtx, n+3, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == 0:
			return bar(gtx)
		case i == 1:
			return p.table.Header(gtx, false)
		case i < n+2:
			return p.table.Row(gtx, i-2, p.paged.Items[i-2])
		}
		return ui.Gap(0, 24)(gtx)
	})
}

// ---- Artists, albums, playlists, shows, episodes, profiles ----

const minCard = 176

func (p *page) layoutCards(gtx layout.Context, bar layout.Widget) layout.Dimensions {
	if p.res.Done && p.res.Err == nil && p.cards == nil {
		p.cards, p.total = p.res.V.Cards, p.res.V.Total
		if p.cards == nil {
			p.cards = []spotify.Card{}
		}
	}
	switch {
	case !p.res.Done:
		return p.single(gtx, bar, ui.Loading)
	case p.res.Err != nil:
		return p.single(gtx, bar, func(gtx layout.Context) layout.Dimensions { return ui.Unavailable(gtx, "Search", p.res.Err) })
	case len(p.cards) == 0:
		return p.single(gtx, bar, func(gtx layout.Context) layout.Dimensions { return hint(gtx, p.query) })
	}
	p.cardGrid.Boxed = true
	_, bw := box(gtx)
	bg := gtx
	bg.Constraints.Max.X = bw
	rows := p.cardGrid.Count(bg, len(p.cards), minCard)
	if _, last := p.list.Range(); last >= rows-1 {
		p.more()
	}
	more := len(p.cards) < p.total
	return p.list.Layout(gtx, rows+3, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == 0:
			return bar(gtx)
		case i <= rows:
			row := func(gtx layout.Context) layout.Dimensions {
				return inBox(gtx, func(gtx layout.Context) layout.Dimensions {
					return p.cardGrid.Row(gtx, i-1, p.cards, minCard, ui.CardOpts{OnOpen: func(c spotify.Card) { remember(c.URI) }})
				})
			}
			if i == 1 { // 24dp between the tabs and the grid
				return ui.Pad(gtx, 24, 0, 0, 0, row)
			}
			return row(gtx)
		case more:
			return ui.Loading(gtx)
		}
		return ui.Gap(0, 24)(gtx)
	})
}

// more loads the next page of cards as the end comes into view.
func (p *page) more() {
	if p.loading || len(p.cards) >= p.total {
		return
	}
	p.loading = true
	off, kind := len(p.cards), p.tab
	ui.Async(func() (*spotify.SearchPage, error) { return ui.B.SearchType(p.query, kind, off, 50) }, func(r *spotify.SearchPage, err error) {
		p.loading = false
		if err != nil || len(r.Cards) == 0 {
			p.total = len(p.cards) // nothing more to get; stop asking
			return
		}
		p.cards = append(p.cards, r.Cards...)
	})
}
