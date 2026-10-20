// Package album is the album page (/album/<id>).
package album

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() {
	ui.Register("album", func(args []string) ui.Page {
		p := &page{uri: "spotify:album:" + args[0], color: ui.RGB(0x535353)}
		p.res.Load(func() (*spotify.Album, error) { return ui.B.Album(p.uri) })
		return p
	})
}

type page struct {
	uri     string
	res     ui.Res[*spotify.Album]
	ready   bool
	color   color.NRGBA
	tracks  *ui.Paged
	table   ui.TrackTable
	list    ui.List
	header  ui.EntityHeader
	actions ui.ActionBar
	artists []ui.Link
	more    ui.Shelf
	headerH int
}

func (p *page) Chrome() ui.Chrome {
	title := ""
	if p.res.Ok() {
		title = p.res.V.Name
	}
	return ui.Chrome{Title: title, Color: p.color, ContextURI: p.uri, OnPlay: func() { ui.PlayContext(p.uri) }, Threshold: p.headerH, List: &p.list}
}

// setup runs once when the album arrives.
func (p *page) setup(a *spotify.Album) {
	p.ready = true
	ui.Saved[p.uri] = a.Saved
	p.tracks = &ui.Paged{}
	p.tracks.SetAll(a.Tracks)
	p.table = ui.TrackTable{Variant: "album", ContextURI: p.uri}
	p.artists = make([]ui.Link, len(a.Artists))
	ui.ExtractedColor(a.Color, a.Images, func(c color.NRGBA) { p.color = c })
}

const (
	itemHeader = iota
	itemActions
	itemTableHeader
	itemRows
)

func (p *page) Layout(gtx layout.Context) layout.Dimensions {
	switch {
	case !p.res.Done:
		return ui.Loading(gtx)
	case p.res.Err != nil:
		return ui.Unavailable(gtx, "This album", p.res.Err)
	}
	a := p.res.V
	if !p.ready {
		p.setup(a)
	}
	for i := range p.artists {
		if p.artists[i].Click.Clicked(gtx) {
			ui.OpenURI(a.Artists[i].URI)
		}
	}
	ui.HeaderFade(gtx, p.color, p.headerH-p.list.ScrollTop())
	if !p.list.Scrolled(0) {
		ui.HeaderFade(gtx, p.color, p.headerH)
	}
	p.list.Sticky = func(gtx layout.Context) layout.Dimensions { return p.table.Header(gtx, true) }
	p.list.StickyIndex, p.list.StickyTop = itemTableHeader, ui.Dp(gtx, 64)

	n := len(a.Tracks)
	return p.list.Layout(gtx, itemRows+n+2, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == itemHeader:
			d := p.header.Layout(gtx, ui.HeaderOpts{
				Images: a.Images, Type: typeLabel(a.Type), Title: a.Name, Color: p.color, Icon: "album",
				Meta: func(gtx layout.Context) layout.Dimensions { return p.meta(gtx, a) },
			})
			p.headerH = d.Size.Y
			return d
		case i == itemActions:
			return p.actions.Layout(gtx, ui.ActionOpts{
				ContextURI: p.uri, Shuffle: true, SaveURI: p.uri,
				Menu: func() []ui.MenuItem {
					return ui.CardMenu(spotify.Card{URI: p.uri, Type: "album", Name: a.Name, Images: a.Images}, ui.CardMenuOpts{})
				},
			})
		case i == itemTableHeader:
			return p.table.Header(gtx, false)
		case i < itemRows+n:
			return p.table.Row(gtx, i-itemRows, p.tracks.Items[i-itemRows])
		case i == itemRows+n:
			return p.footer(gtx, a)
		default:
			return p.moreBy(gtx, a)
		}
	})
}

func typeLabel(t string) string {
	switch t {
	case "SINGLE":
		return "Single"
	case "EP":
		return "EP"
	case "COMPILATION":
		return "Compilation"
	}
	return "Album"
}

func (p *page) meta(gtx layout.Context, a *spotify.Album) layout.Dimensions {
	dim := ui.Alpha(ui.ColText, .7)
	var segs []ui.Seg
	if len(a.Artists) > 0 && len(a.Artists[0].Images) > 0 {
		segs = append(segs, ui.Seg{Widget: func(gtx layout.Context) layout.Dimensions {
			s := ui.Dp(gtx, 24)
			ui.Image{URL: ui.Pick(a.Artists[0].Images, s), Round: true}.Layout(gtx, image.Pt(s, s))
			return layout.Dimensions{Size: image.Pt(s+ui.Dp(gtx, 4), s), Baseline: ui.Dp(gtx, 6)}
		}})
	}
	for i, ar := range a.Artists {
		if i > 0 {
			segs = append(segs, ui.Seg{Text: " • ", Color: dim})
		}
		segs = append(segs, ui.Seg{Text: ar.Name, Link: &p.artists[i], Weight: ui.Bold})
	}
	if len(a.Date) >= 4 {
		segs = append(segs, ui.Seg{Text: " • " + a.Date[:4], Color: dim})
	}
	var total int64
	for _, t := range a.Tracks {
		total += int64(t.DurationMs)
	}
	songs := "songs"
	if a.Total == 1 {
		songs = "song"
	}
	segs = append(segs, ui.Seg{Text: " • " + ui.FmtNumber(a.Total) + " " + songs + ", " + ui.FmtTotal(total), Color: dim})
	return ui.Inline(gtx, 14, ui.Regular, ui.ColText, ui.ColNone, segs...)
}

func (p *page) footer(gtx layout.Context, a *spotify.Album) layout.Dimensions {
	return ui.PageInset(gtx, 32, 0, func(gtx layout.Context) layout.Dimensions {
		rows := []layout.Widget{func(gtx layout.Context) layout.Dimensions {
			released := a.Date
			for _, f := range []string{time.RFC3339, "2006-01-02"} {
				if t, err := time.Parse(f, a.Date); err == nil {
					released = t.Format("January 2, 2006")
					break
				}
			}
			return ui.Pad(gtx, 0, 0, 4, 0, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, released, 14, ui.Regular, ui.ColText2)
			})
		}}
		for _, c := range a.Copyright {
			rows = append(rows, func(gtx layout.Context) layout.Dimensions {
				return ui.Label{Text: c, Size: 11, Color: ui.ColText2, MaxLines: 2}.Layout(gtx)
			})
		}
		return ui.Column(gtx, rows...)
	})
}

func (p *page) moreBy(gtx layout.Context, a *spotify.Album) layout.Dimensions {
	if len(a.More) == 0 || len(a.Artists) == 0 {
		return ui.Gap(0, 32)(gtx)
	}
	artist := a.Artists[0]
	return ui.Pad(gtx, 32, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
		return p.more.Layout(gtx, a.More, ui.ShelfOpts{
			Title: "More by " + artist.Name,
			OnShowAll: func() {
				ui.Navigate("/artist/" + ui.URIID(artist.URI) + "/discography/all")
			},
			ShowAllText: "See discography",
		})
	})
}
