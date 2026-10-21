// Package artist is the artist page (/artist/<id>) and its discography
// (/artist/<id>/discography/<kind>).
package artist

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() {
	ui.Register("artist", func(args []string) ui.Page {
		if len(args) >= 3 && args[1] == "discography" {
			return newDiscography(args[0], args[2])
		}
		if len(args) >= 2 && shelfKinds[args[1]] {
			return newShelfPage(args[0], args[1])
		}
		p := &page{uri: "spotify:artist:" + args[0], color: ui.RGB(0x535353)}
		p.res.Load(func() (*spotify.ArtistPage, error) { return ui.B.Artist(p.uri) })
		return p
	})
}

type page struct {
	uri      string
	res      ui.Res[*spotify.ArtistPage]
	ready    bool
	color    color.NRGBA
	list     ui.List
	header   ui.EntityHeader
	actions  ui.ActionBar
	follow   ui.Pill
	table    ui.TrackTable
	tracks   *ui.Paged
	more     bool
	seeMore  ui.Link
	kind     string // discography chip: popular, albums, singles, compilations
	chips    [4]ui.Pill
	disc     ui.Shelf
	shelves  []ui.Shelf
	aboutBtn ui.Click
	pickBtn  ui.Click
	tour     tour
	merch    []ui.Click
	items    []layout.Widget
	headerH  int
}

var chipDefs = [4]struct{ kind, label string }{
	{"popular", "Popular releases"}, {"albums", "Albums"}, {"singles", "Singles and EPs"}, {"compilations", "Compilations"},
}

func (p *page) Chrome() ui.Chrome {
	title := ""
	if p.res.Ok() {
		title = p.res.V.Name
	}
	return ui.Chrome{Title: title, Color: p.color, ContextURI: p.uri, OnPlay: func() { ui.PlayContext(p.uri) }, Threshold: p.headerH, List: &p.list}
}

// count is how many releases the chip's kind has.
func count(a *spotify.ArtistPage, kind string) int {
	switch kind {
	case "popular":
		return len(a.Popular)
	case "albums":
		return a.Totals[0]
	case "singles":
		return a.Totals[1]
	}
	return a.Totals[2]
}

func releases(a *spotify.ArtistPage, kind string) []spotify.Card {
	switch kind {
	case "popular":
		return a.Popular
	case "albums":
		return a.Albums
	case "singles":
		return a.Singles
	}
	return a.Compilations
}

func (p *page) setup(a *spotify.ArtistPage) {
	p.ready = true
	ui.Saved[p.uri] = a.Following
	p.table = ui.TrackTable{Variant: "artist", ContextURI: p.uri}
	p.tracks = &ui.Paged{}
	p.setTracks(a)
	for _, c := range chipDefs {
		if count(a, c.kind) > 0 {
			p.kind = c.kind
			break
		}
	}
	p.shelves = make([]ui.Shelf, len(a.Shelves))
	imgs := a.Header
	if len(imgs) == 0 {
		imgs = a.Avatar
	}
	ui.CoverColor(imgs, func(c color.NRGBA) { p.color = c })
	p.build(a)
}

func (p *page) setTracks(a *spotify.ArtistPage) {
	n := min(len(a.TopTracks), 5)
	if p.more {
		n = min(len(a.TopTracks), 10)
	}
	p.tracks.SetAll(a.TopTracks[:n])
}

// build lists the page's sections; rebuilt when the popular list grows.
func (p *page) build(a *spotify.ArtistPage) {
	p.items = append(p.items[:0], p.headerSection, p.actionBar)
	if len(a.TopTracks) > 0 {
		p.items = append(p.items, p.popularTitle)
		for i := range p.tracks.Items {
			p.items = append(p.items, func(gtx layout.Context) layout.Dimensions {
				return p.table.Row(gtx, i, p.tracks.Items[i])
			})
		}
		p.items = append(p.items, p.seeMoreBtn)
	}
	if a.Pick != nil {
		p.items = append(p.items, p.pickSection)
	}
	if p.kind != "" {
		p.items = append(p.items, p.discography)
	}
	// On Tour, About and Merch come after the Featuring shelf, as in the official page.
	extras := func() {
		if len(a.Concerts) > 0 {
			p.items = append(p.items, p.tourSection)
		}
		if a.Bio != "" {
			p.items = append(p.items, p.aboutSection)
		}
		if len(a.Merch) > 0 {
			p.items = append(p.items, p.merchSection)
		}
	}
	aboutAt := 0
	if len(a.Shelves) > 0 && strings.HasPrefix(a.Shelves[0].Title, "Featuring") {
		aboutAt = 1
	}
	for i := range a.Shelves {
		if i == aboutAt {
			extras()
		}
		p.items = append(p.items, func(gtx layout.Context) layout.Dimensions {
			s := a.Shelves[i]
			return p.shelves[i].Layout(gtx, s.Items, ui.ShelfOpts{Title: s.Title, OnShowAll: func() {
				ui.Navigate("/artist/" + ui.URIID(p.uri) + "/" + s.URI)
			}})
		})
	}
	if aboutAt >= len(a.Shelves) {
		extras()
	}
}

func (p *page) Layout(gtx layout.Context) layout.Dimensions {
	switch {
	case !p.res.Done:
		return ui.Loading(gtx)
	case p.res.Err != nil:
		return ui.Unavailable(gtx, "This artist", p.res.Err)
	}
	a := p.res.V
	if !p.ready {
		p.setup(a)
	}
	if p.seeMore.Click.Clicked(gtx) {
		p.more = !p.more
		p.setTracks(a)
		p.build(a)
	}
	for i := range p.chips {
		if p.chips[i].Click.Clicked(gtx) {
			p.kind = chipDefs[i].kind
		}
	}
	if p.follow.Click.Clicked(gtx) {
		ui.ToggleSaved(p.uri)
	}
	if p.aboutBtn.Clicked(gtx) {
		ui.ShowOverlay(&about{a: a, links: make([]ui.Click, len(a.Links))})
	}
	// The color wash continues behind the action bar.
	top := p.headerH - p.list.ScrollTop()
	if len(a.Header) > 0 {
		washBanner(gtx, p.color, top)
	} else {
		ui.HeaderFade(gtx, p.color, top)
	}
	p.list.Layout(gtx, len(p.items), func(gtx layout.Context, i int) layout.Dimensions { return p.items[i](gtx) })
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

// washBanner is the 173dp tint under a banner header (the banner's color at
// 70% fading into the page).
func washBanner(gtx layout.Context, c color.NRGBA, top int) {
	h := ui.Dp(gtx, 173)
	if top+h > 0 {
		ui.VGradient(gtx.Ops, image.Rect(0, top, gtx.Constraints.Max.X, top+h), ui.Mix(ui.ColSurface, c, .7), ui.ColSurface)
	}
}

func (p *page) headerSection(gtx layout.Context) layout.Dimensions {
	a := p.res.V
	var d layout.Dimensions
	if len(a.Header) > 0 {
		d = banner(gtx, a)
	} else {
		d = p.header.Layout(gtx, ui.HeaderOpts{
			Images: a.Avatar, Title: a.Name, Color: p.color, Round: true, Icon: "user",
			Meta: func(gtx layout.Context) layout.Dimensions { return meta(gtx, a) },
		})
	}
	p.headerH = d.Size.Y
	return d
}

// meta is the "Verified by Spotify" badge and the monthly listeners.
func meta(gtx layout.Context, a *spotify.ArtistPage) layout.Dimensions {
	return ui.Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			if !a.Verified {
				return layout.Dimensions{}
			}
			return verified(gtx)
		},
		func(gtx layout.Context) layout.Dimensions {
			// A 32dp row, 4dp below the badge.
			m := op.Record(gtx.Ops)
			d := ui.Text(gtx, compact(a.MonthlyListeners)+" monthly listeners", 16, ui.Regular, ui.ColText)
			call := m.Stop()
			row := ui.Dp(gtx, 32)
			ui.Offset(gtx, 0, ui.Dp(gtx, 4)+(row-d.Size.Y)/2, func() { call.Add(gtx.Ops) })
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, ui.Dp(gtx, 4)+row)}
		},
	)
}

// banner is the full-width header with the artist's picture behind the name.
func banner(gtx layout.Context, a *spotify.ArtistPage) layout.Dimensions {
	// The web player's banner is 40vh: 40% of the window's height.
	w, h := gtx.Constraints.Max.X, max(ui.Dp(gtx, 240), ui.WindowSize.Y*40/100)
	ui.Image{URL: ui.Pick(a.Header, w), FocusY: .15, Placeholder: ui.RGB(0x535353)}.Layout(gtx, image.Pt(w, h))
	ui.VGradient(gtx.Ops, image.Rect(0, 0, w, h), ui.Alpha(ui.ColBg, 0), ui.Veil(.6))
	pad := ui.ContentPad(gtx)
	gtx.Constraints.Max.X = w - 2*pad
	m := op.Record(gtx.Ops)
	info := ui.Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			size := ui.HeaderTitleSize(gtx, gtx.Constraints.Max.X, a.Name)
			return ui.Label{Text: a.Name, Size: size, Weight: ui.ExtraBold, MaxLines: 2, LineHeight: size}.Layout(gtx)
		},
		func(gtx layout.Context) layout.Dimensions { return meta(gtx, a) },
	)
	call := m.Stop()
	ui.Offset(gtx, pad, h-ui.Dp(gtx, 24)-info.Size.Y, func() { call.Add(gtx.Ops) })
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// verified is the encore verified badge (24dp, light green on the dark
// theme) and "Verified by Spotify", 8dp apart and centered on each other.
func verified(gtx layout.Context) layout.Dimensions {
	s := ui.Dp(gtx, 24)
	ui.IconPx(gtx, "verified", s, ui.RGB(0xc5f0c9))
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints.Max.X -= s + ui.Dp(gtx, 8)
	d := ui.Text(g, "Verified by Spotify", 13, ui.Bold, ui.ColText)
	call := m.Stop()
	ui.Offset(gtx, s+ui.Dp(gtx, 8), (s-d.Size.Y)/2, func() { call.Add(gtx.Ops) })
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, s)}
}

func (p *page) actionBar(gtx layout.Context) layout.Dimensions {
	a := p.res.V
	return p.actions.Layout(gtx, ui.ActionOpts{
		ContextURI: p.uri, Shuffle: true,
		Extra: func(gtx layout.Context) layout.Dimensions {
			txt := "Follow"
			if ui.Saved[p.uri] {
				txt = "Following"
			}
			return p.follow.Layout(gtx, ui.PillStyle{Text: txt, Weight: ui.Bold, Outline: true})
		},
		Menu: func() []ui.MenuItem {
			return ui.CardMenu(spotify.Card{URI: p.uri, Type: "artist", Name: a.Name, Images: a.Avatar}, ui.CardMenuOpts{})
		},
	})
}

func (p *page) popularTitle(gtx layout.Context) layout.Dimensions {
	return ui.PageInset(gtx, 8, 16, func(gtx layout.Context) layout.Dimensions {
		return ui.Text(gtx, "Popular", 24, ui.Bold, ui.ColText)
	})
}

// pickSection is "Artist pick": the cover, the artist's comment in a white
// pill with their picture, then the title and its type.
func (p *page) pickSection(gtx layout.Context) layout.Dimensions {
	a := p.res.V
	pk := a.Pick
	if p.pickBtn.Clicked(gtx) {
		ui.OpenURI(pk.URI)
	}
	return ui.PageInset(gtx, 0, 40, func(gtx layout.Context) layout.Dimensions {
		return ui.Column(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return ui.Pad(gtx, 0, 0, 16, 0, func(gtx layout.Context) layout.Dimensions {
					return ui.Text(gtx, "Artist pick", 24, ui.Bold, ui.ColText)
				})
			},
			func(gtx layout.Context) layout.Dimensions {
				dp := func(v float32) int { return ui.Dp(gtx, v) }
				is, x := dp(88), dp(88+16)
				tw := gtx.Constraints.Max.X - x
				p.pickBtn.Label = pk.Title
				return p.pickBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					ui.Image{URL: ui.Pick(pk.Images, is), Radius: dp(4), Icon: "album"}.Layout(gtx, image.Pt(is, is))
					g := gtx
					g.Constraints = layout.Constraints{Max: image.Pt(max(0, tw), is)}
					y := 0
					if pk.Comment != "" {
						m := op.Record(gtx.Ops)
						cg := g
						cg.Constraints.Max.X = max(0, tw-dp(34+8))
						cd := ui.Text(cg, pk.Comment, 14, ui.Regular, ui.ColBg)
						call := m.Stop()
						ph := dp(28)
						ui.Offset(gtx, x, 0, func() {
							ui.FillRect(gtx.Ops, image.Rect(0, 0, dp(34+8)+cd.Size.X, ph), ph/2, ui.ColText)
							ui.Offset(gtx, dp(2), dp(2), func() {
								ui.Image{URL: ui.Pick(a.Avatar, dp(24)), Round: true, Radius: dp(12), Icon: "artist"}.Layout(gtx, image.Pt(dp(24), dp(24)))
							})
							ui.Offset(gtx, dp(34), (ph-cd.Size.Y)/2, func() { call.Add(gtx.Ops) })
						})
						y = ph + dp(8)
					}
					ui.Offset(gtx, x, y, func() {
						d := ui.Text(g, pk.Title, 16, ui.Bold, ui.ColText)
						ui.Offset(gtx, 0, d.Size.Y+dp(4), func() { ui.Text(g, pk.Subtitle, 14, ui.Regular, ui.ColText2) })
					})
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, is)}
				})
			},
		)
	})
}

func (p *page) seeMoreBtn(gtx layout.Context) layout.Dimensions {
	a := p.res.V
	if len(a.TopTracks) <= 5 {
		return ui.Gap(0, 40)(gtx)
	}
	txt := "See more"
	if p.more {
		txt = "Show less"
	}
	p.seeMore.Click.Label = txt
	bottom := float32(56) // a shelf title follows
	if a.Pick != nil {
		bottom = 41 // the Artist pick heading, which has no top margin
	}
	return ui.Pad(gtx, 16, 0, bottom, ui.ContentPadDp(gtx)+16, func(gtx layout.Context) layout.Dimensions {
		return p.seeMore.Layout(gtx, txt, 14, ui.Bold, ui.Alpha(ui.ColText, .7), ui.ColText)
	})
}

func (p *page) discography(gtx layout.Context) layout.Dimensions {
	a := p.res.V
	kind := p.kind
	if kind == "popular" {
		kind = "all"
	}
	return p.disc.Layout(gtx, releases(a, p.kind), ui.ShelfOpts{
		Title:     "Discography",
		OnShowAll: func() { ui.Navigate("/artist/" + ui.URIID(p.uri) + "/discography/" + kind) },
		Below:     func(gtx layout.Context) layout.Dimensions { return p.chipRow(gtx, a) },
	})
}

func (p *page) chipRow(gtx layout.Context, a *spotify.ArtistPage) layout.Dimensions {
	var kids []layout.FlexChild
	for i, c := range chipDefs {
		if count(a, c.kind) == 0 {
			continue
		}
		kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.chips[i].Layout(gtx, ui.PillStyle{
				Text: c.label, PadX: 12, Selected: p.kind == c.kind,
				Bg: ui.RGB(0x232323), BgHover: ui.ColSurface3,
			})
		}), layout.Rigid(ui.Gap(8, 0)))
	}
	return ui.Pad(gtx, 8, 12, 16, 12, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx, kids...)
	})
}

// compact abbreviates big counts like the header does: 46.4M, 106M.
func compact(n int) string {
	units := []struct {
		div    float64
		suffix string
	}{{1e9, "B"}, {1e6, "M"}, {1e3, "K"}}
	if n < 10000 {
		return ui.FmtNumber(n)
	}
	for _, u := range units {
		if v := float64(n) / u.div; v >= 1 {
			prec := 2
			if v >= 10 {
				prec = 1
			}
			if v >= 100 {
				prec = 0
			}
			f := strconv.FormatFloat(v, 'f', prec, 64)
			if prec > 0 {
				f = strings.TrimSuffix(strings.TrimRight(f, "0"), ".")
			}
			return f + u.suffix
		}
	}
	return fmt.Sprint(n)
}
