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
	d := ui.PageInset(gtx, 16, 16, func(gtx layout.Context) layout.Dimensions {
		d, clicked := p.chips.Layout(gtx, tabLabels, sel)
		if clicked >= 0 && clicked != sel {
			ui.Navigate(p.tabPath(tabs[clicked].id))
		}
		return d
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
			p.topRows.SetAll(r.Tracks[:min(4, len(r.Tracks))])
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
			o := ui.ShelfOpts{Title: g.Title, Card: ui.CardOpts{OnOpen: func(c spotify.Card) { remember(c.URI) }}}
			if tab, ok := groupTab[g.Title]; ok {
				o.OnShowAll = func() { ui.Navigate(p.tabPath(tab)) }
			}
			return p.shelves[i-2].Layout(gtx, g.Items, o)
		}
		return ui.Gap(0, 24)(gtx)
	})
}

// topRow is the top result next to the first songs (stacked when narrow).
func (p *page) topRow(gtx layout.Context, top *spotify.Card) layout.Dimensions {
	w := gtx.Constraints.Max.X
	px := func(v float32) int { return ui.Dp(gtx, v) }
	pad, gap := ui.ContentPad(gtx), px(24)
	inner := w - 2*pad
	songs := 0
	if p.topRows != nil {
		songs = len(p.topRows.Items)
	}
	rowsH := songs * px(56)
	leftW, rightW := inner, inner
	if top != nil && songs > 0 && w > px(760) {
		leftW = max(px(280), (inner-gap)*2/5)
		rightW = inner - gap - leftW
	}
	side := top != nil && songs > 0 && leftW != inner
	cardH := max(px(220), rowsH)

	y := 0
	heading := func(gtx layout.Context, w int, f func(gtx layout.Context) layout.Dimensions) int {
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(w, px(40))}
		return f(g).Size.Y + px(8)
	}
	leftH, rightH := 0, 0
	if top != nil {
		ui.Offset(gtx, pad, y, func() {
			hh := heading(gtx, leftW, func(gtx layout.Context) layout.Dimensions { return ui.Text(gtx, "Top result", 24, ui.Bold, ui.ColText) })
			ui.Offset(gtx, 0, hh, func() { p.top.layout(gtx, *top, leftW, cardH) })
			leftH = hh + cardH
		})
	}
	if songs > 0 {
		x, sy := pad, y
		if side {
			x = pad + leftW + gap
		} else {
			sy = y + leftH
			if top != nil {
				sy += gap
			}
		}
		ui.Offset(gtx, x, sy, func() {
			if p.songs.Click.Clicked(gtx) {
				ui.Navigate(p.tabPath("tracks"))
			}
			hh := heading(gtx, rightW, func(gtx layout.Context) layout.Dimensions {
				return p.songs.Layout(gtx, "Songs", 24, ui.Bold, ui.ColText, ui.ColNone)
			})
			// Rows carry the table's own 24dp side padding.
			ui.Offset(gtx, -pad, hh, func() {
				g := gtx
				g.Constraints = layout.Constraints{Max: image.Pt(rightW+2*pad, gtx.Constraints.Max.Y)}
				for i, t := range p.topRows.Items {
					ui.Offset(g, 0, i*px(56), func() { p.topTab.Row(g, i, t) })
				}
			})
			rightH = hh + rowsH
		})
	}
	h := max(leftH, rightH)
	if !side {
		h = leftH + rightH
		if top != nil && songs > 0 {
			h += gap
		}
	}
	return layout.Dimensions{Size: image.Pt(w, h+px(32))}
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

func (t *topCard) layout(gtx layout.Context, c spotify.Card, w, h int) layout.Dimensions {
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
	return t.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hov := t.click.Hovered()
		bg := ui.RGB(0x181818)
		if hov {
			bg = ui.RGB(0x282828)
		}
		ui.FillRect(gtx.Ops, image.Rect(0, 0, w, h), ui.Dp(gtx, 8), bg)
		pad, art := ui.Dp(gtx, 20), ui.Dp(gtx, 92)
		round := c.Type == "artist"
		radius := ui.Dp(gtx, 6)
		if round {
			radius = art / 2
		}
		ui.Shadow(gtx.Ops, image.Rect(pad, pad, pad+art, pad+art), radius, ui.Dp(gtx, 12), ui.Dp(gtx, 8), .3)
		icon := "note"
		if round {
			icon = "user"
		}
		ui.Offset(gtx, pad, pad, func() {
			ui.Image{URL: ui.Pick(c.Images, art), Radius: radius, Round: round, Icon: icon}.Layout(gtx, image.Pt(art, art))
		})
		y := pad + art + ui.Dp(gtx, 20)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(w-2*pad, h)}
		ui.Offset(gtx, pad, y, func() { y += ui.Text(g, c.Name, 32, ui.Bold, ui.ColText).Size.Y })
		y += ui.Dp(gtx, 6)
		segs := []ui.Seg{{Text: typeWord(c.Type), Color: ui.ColText}}
		if c.Subtitle != "" && c.Subtitle != "Artist" {
			segs = append(segs, ui.Seg{Text: " • " + c.Subtitle})
		}
		ui.Offset(gtx, pad, y, func() { ui.Inline(g, 14, ui.Regular, ui.ColText2, ui.ColNone, segs...) })
		playing := ui.CtxPlaying(c.URI)
		if v := t.hover.Step(gtx, hov || playing, 300*time.Millisecond); v > 0 {
			bs := ui.Dp(gtx, 48)
			ui.Offset(gtx, w-pad-bs, h-pad-bs+int((1-v)*float32(ui.Dp(gtx, 8))), func() {
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
	rows := p.cardGrid.Count(gtx, len(p.cards), minCard)
	if _, last := p.list.Range(); last >= rows-1 {
		p.more()
	}
	more := len(p.cards) < p.total
	return p.list.Layout(gtx, rows+3, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == 0:
			return bar(gtx)
		case i <= rows:
			return p.cardGrid.Row(gtx, i-1, p.cards, minCard, ui.CardOpts{OnOpen: func(c spotify.Card) { remember(c.URI) }})
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
