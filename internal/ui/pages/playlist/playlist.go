// Package playlist is the playlist page (/playlist/<id>).
package playlist

import (
	"html"
	"image"
	"image/color"
	"regexp"
	"slices"

	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

const pageSize = 100

func init() {
	ui.Register("playlist", func(args []string) ui.Page {
		p := &page{uri: "spotify:playlist:" + args[0], color: ui.RGB(0x535353)}
		p.version = ui.PlaylistVersion[p.uri]
		p.load()
		return p
	})
}

type page struct {
	uri     string
	res     ui.Res[*spotify.Playlist]
	cur     *spotify.Playlist // the playlist setup last ran for
	version int
	color   color.NRGBA
	tracks  *ui.Paged
	table   ui.TrackTable
	list    ui.List
	header  ui.EntityHeader
	actions ui.ActionBar
	owner   ui.Link
	member  ui.Link
	headerH int

	desc    string
	length  string // "about 3 hr", cached per tracks.Gen
	lengthG int
}

func (p *page) load() {
	p.res.Load(func() (*spotify.Playlist, error) { return ui.B.Playlist(p.uri, 0, pageSize) })
}

func (p *page) Chrome() ui.Chrome {
	title := ""
	if p.cur != nil {
		title = p.cur.Name
	}
	return ui.Chrome{Title: title, Color: p.color, ContextURI: p.uri, OnPlay: func() { ui.PlayContext(p.uri) }, Threshold: p.headerH, List: &p.list}
}

var tag = regexp.MustCompile(`<[^>]*>`)

// text drops the HTML links Spotify puts in descriptions.
func text(s string) string { return html.UnescapeString(tag.ReplaceAllString(s, "")) }

// setup runs when a playlist (re)loads.
func (p *page) setup(pl *spotify.Playlist) {
	p.cur = pl
	ui.Saved[p.uri] = pl.Following
	p.desc = text(pl.Description)
	p.tracks = ui.NewPaged(func(off, lim int) (*spotify.TrackPage, error) {
		r, err := ui.B.Playlist(p.uri, off, lim)
		if err != nil {
			return nil, err
		}
		return &r.Tracks, nil
	}, pageSize)
	p.tracks.Seed(pl.Tracks)
	p.lengthG = -1
	p.table = ui.TrackTable{Variant: "playlist", ContextURI: p.uri, PlaylistURI: p.uri, CanEdit: pl.CanEdit}
	if pl.CanEdit {
		p.table.OnMove = p.move
	}
	ui.ExtractedColor(pl.Color, pl.Images, func(c color.NRGBA) { p.color = c })
}

// move applies a drag-reorder at once, then syncs with the server (the
// reload also undoes the move if it failed).
func (p *page) move(from, to int, after bool) {
	items := p.tracks.Items
	if to >= len(items) {
		return
	}
	src, dst := items[from], items[to]
	if src == nil || dst == nil || src.UID == "" || dst.UID == "" {
		return
	}
	at := to
	if after {
		at++
	}
	if from < at {
		at--
	}
	p.tracks.Items = slices.Insert(slices.Delete(items, from, from+1), at, src)
	p.table.Selected = at
	pos := spotify.Position{Before: dst.UID}
	if after {
		pos = spotify.Position{After: dst.UID}
	}
	uid := src.UID
	ui.Async(func() (struct{}, error) { return struct{}{}, ui.B.MovePlaylistItems(p.uri, []string{uid}, pos) }, func(_ struct{}, err error) {
		if err != nil {
			ui.Toast("Couldn't move: " + err.Error())
		}
		ui.PlaylistVersion[p.uri]++
	})
}

const (
	itemHeader = iota
	itemActions
	itemTableHeader
	itemRows
)

func (p *page) Layout(gtx layout.Context) layout.Dimensions {
	if v := ui.PlaylistVersion[p.uri]; v != p.version {
		p.version = v
		p.load()
	}
	switch {
	case !p.res.Done:
		return ui.Loading(gtx)
	case p.res.Err != nil:
		return ui.Unavailable(gtx, "This playlist", p.res.Err)
	}
	pl := p.res.V
	if pl != p.cur {
		p.setup(pl)
	}
	if p.owner.Click.Clicked(gtx) {
		ui.OpenURI(pl.Owner.URI)
	}
	if p.member.Click.Clicked(gtx) && len(pl.Members) == 1 {
		ui.OpenURI(pl.Members[0].URI)
	}
	ui.HeaderFade(gtx, p.color, p.headerH-p.list.ScrollTop())
	if !p.list.Scrolled(0) {
		ui.HeaderFade(gtx, p.color, p.headerH)
	}
	p.list.Sticky = func(gtx layout.Context) layout.Dimensions { return p.table.Header(gtx, true) }
	p.list.StickyIndex, p.list.StickyTop = itemTableHeader, ui.Dp(gtx, 64)

	n := p.tracks.Total
	d := p.list.Layout(gtx, itemRows+n+1, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == itemHeader:
			o := ui.HeaderOpts{
				Images: pl.Images, Type: "Playlist", Title: pl.Name, Color: p.color, Icon: "note",
				Description: p.desc,
				Meta:        func(gtx layout.Context) layout.Dimensions { return p.meta(gtx, pl) },
			}
			if pl.IsOwner {
				o.OnEdit = func() { ui.EditPlaylist(p.uri, pl.Name, p.desc, nil) }
			}
			d := p.header.Layout(gtx, o)
			p.headerH = d.Size.Y
			return d
		case i == itemActions:
			a := ui.ActionOpts{
				ContextURI: p.uri, Shuffle: true,
				Menu: func() []ui.MenuItem {
					return ui.CardMenu(spotify.Card{URI: p.uri, Type: "playlist", Name: pl.Name, Images: pl.Images}, ui.CardMenuOpts{Own: pl.IsOwner, Description: p.desc})
				},
			}
			if !pl.IsOwner {
				a.SaveURI = p.uri
			}
			return p.actions.Layout(gtx, a)
		case i == itemTableHeader:
			return p.table.Header(gtx, false)
		case i < itemRows+n:
			return p.table.Row(gtx, i-itemRows, p.tracks.Items[i-itemRows])
		}
		return ui.Gap(0, 32)(gtx)
	})
	first, last := p.list.Range()
	p.tracks.Ensure(first-itemRows-5, last-itemRows+5)
	return d
}

func (p *page) meta(gtx layout.Context, pl *spotify.Playlist) layout.Dimensions {
	dim := ui.Alpha(ui.ColText, .7)
	var segs []ui.Seg
	if len(pl.Owner.Images) > 0 {
		segs = append(segs, ui.Seg{Widget: func(gtx layout.Context) layout.Dimensions {
			s := ui.Dp(gtx, 24)
			ui.Image{URL: ui.Pick(pl.Owner.Images, s), Round: true}.Layout(gtx, image.Pt(s, s))
			return layout.Dimensions{Size: image.Pt(s+ui.Dp(gtx, 4), s), Baseline: ui.Dp(gtx, 6)}
		}})
	}
	segs = append(segs, ui.Seg{Text: pl.Owner.Name, Link: &p.owner, Weight: ui.Bold})
	switch len(pl.Members) {
	case 0:
	case 1:
		segs = append(segs, ui.Seg{Text: " and ", Color: dim}, ui.Seg{Text: pl.Members[0].Name, Link: &p.member, Weight: ui.Bold})
	default:
		segs = append(segs, ui.Seg{Text: " and " + ui.FmtNumber(len(pl.Members)) + " others", Color: dim})
	}
	if pl.Followers > 0 {
		saves := " saves"
		if pl.Followers == 1 {
			saves = " save"
		}
		segs = append(segs, ui.Seg{Text: " • " + ui.FmtNumber(pl.Followers) + saves, Color: dim})
	}
	if p.lengthG != p.tracks.Gen {
		p.lengthG = p.tracks.Gen
		p.length = ui.FmtListLength(p.tracks.Loaded(), p.tracks.Total)
	}
	songs := " songs"
	if p.tracks.Total == 1 {
		songs = " song"
	}
	s := " • " + ui.FmtNumber(p.tracks.Total) + songs
	if p.length != "" {
		s += ", " + p.length
	}
	segs = append(segs, ui.Seg{Text: s, Color: dim})
	return ui.Inline(gtx, 14, ui.Regular, ui.ColText, ui.ColNone, segs...)
}
