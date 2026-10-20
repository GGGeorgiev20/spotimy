// Package profile is a user's profile (/user/<id>) and its followers and
// following lists (/user/<id>/followers, /user/<id>/following).
package profile

import (
	"image/color"
	"net/url"

	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() {
	ui.Register("user", func(args []string) ui.Page {
		id, err := url.PathUnescape(args[0])
		if err != nil {
			id = args[0]
		}
		if len(args) > 1 && (args[1] == "followers" || args[1] == "following") {
			return newUsers(id, args[1])
		}
		p := &page{id: id, color: ui.RGB(0x535353)}
		p.res.Load(func() (*spotify.Profile, error) { return ui.B.Profile(id) })
		return p
	})
}

type page struct {
	id        string
	res       ui.Res[*spotify.Profile]
	ready     bool
	color     color.NRGBA
	list      ui.List
	header    ui.EntityHeader
	more      ui.IconButton
	followers ui.Link
	following ui.Link
	artists   ui.Shelf
	playlists ui.Shelf
	fans      ui.Shelf
	follows   ui.Shelf
	table     ui.TrackTable
	tracks    *ui.Paged
	items     []layout.Widget
	headerH   int
}

func (p *page) Chrome() ui.Chrome {
	title := ""
	if p.res.Ok() {
		title = p.res.V.Name
	}
	return ui.Chrome{Title: title, Color: p.color, Threshold: p.headerH, List: &p.list}
}

func (p *page) setup(pr *spotify.Profile) {
	p.ready = true
	p.table = ui.TrackTable{Variant: "search"}
	p.tracks = &ui.Paged{}
	p.tracks.SetAll(pr.TopTracks)
	if len(pr.Images) > 0 {
		ui.CoverColor(pr.Images, func(c color.NRGBA) { p.color = c })
	}
	p.items = []layout.Widget{p.headerSection, p.actionRow}
	if len(pr.TopArtists) > 0 {
		p.items = append(p.items, func(gtx layout.Context) layout.Dimensions {
			return p.artists.Layout(gtx, pr.TopArtists, ui.ShelfOpts{Title: "Top artists this month", Note: "Only visible to you"})
		})
	}
	if len(pr.TopTracks) > 0 {
		p.items = append(p.items, p.tracksTitle)
		for i := range pr.TopTracks {
			p.items = append(p.items, func(gtx layout.Context) layout.Dimensions { return p.table.Row(gtx, i, p.tracks.Items[i]) })
		}
		p.items = append(p.items, ui.Gap(0, 32))
	}
	shelf := func(s *ui.Shelf, title string, cards []spotify.Card, showAll func()) {
		if len(cards) == 0 {
			return
		}
		p.items = append(p.items, func(gtx layout.Context) layout.Dimensions {
			return s.Layout(gtx, cards, ui.ShelfOpts{Title: title, OnShowAll: showAll})
		})
	}
	shelf(&p.playlists, "Public Playlists", pr.Playlists, nil)
	shelf(&p.fans, "Followers", pr.FollowerList, func() { ui.Navigate("/user/" + url.PathEscape(p.id) + "/followers") })
	shelf(&p.follows, "Following", pr.FollowingList, func() { ui.Navigate("/user/" + url.PathEscape(p.id) + "/following") })
	p.items = append(p.items, ui.Gap(0, 24))
}

func (p *page) Layout(gtx layout.Context) layout.Dimensions {
	switch {
	case !p.res.Done:
		return ui.Loading(gtx)
	case p.res.Err != nil:
		return ui.Unavailable(gtx, "This profile", p.res.Err)
	}
	pr := p.res.V
	if !p.ready {
		p.setup(pr)
	}
	if p.followers.Click.Clicked(gtx) {
		ui.Navigate("/user/" + url.PathEscape(p.id) + "/followers")
	}
	if p.following.Click.Clicked(gtx) {
		ui.Navigate("/user/" + url.PathEscape(p.id) + "/following")
	}
	if p.more.Click.Clicked(gtx) {
		ui.OpenMenu(ui.CardMenu(spotify.Card{URI: pr.URI, Type: "user", Name: pr.Name}, ui.CardMenuOpts{}))
	}
	ui.HeaderFade(gtx, p.color, p.headerH-p.list.ScrollTop())
	return p.list.Layout(gtx, len(p.items), func(gtx layout.Context, i int) layout.Dimensions { return p.items[i](gtx) })
}

func (p *page) headerSection(gtx layout.Context) layout.Dimensions {
	pr := p.res.V
	d := p.header.Layout(gtx, ui.HeaderOpts{
		Images: pr.Images, Type: "Profile", Title: pr.Name, Color: p.color, Round: true, Icon: "user",
		Meta: func(gtx layout.Context) layout.Dimensions { return p.meta(gtx, pr) },
	})
	p.headerH = d.Size.Y
	return d
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return ui.FmtNumber(n) + " " + word + "s"
}

// meta is "N Public Playlists • N Followers • N Following"; the last two open the lists.
func (p *page) meta(gtx layout.Context, pr *spotify.Profile) layout.Dimensions {
	dim := ui.Alpha(ui.ColText, .7)
	var segs []ui.Seg
	if pr.PublicPlaylists > 0 {
		segs = append(segs, ui.Seg{Text: plural(pr.PublicPlaylists, "Public Playlist") + " • ", Color: dim})
	}
	p.followers.Click.Label, p.following.Click.Label = "Followers", "Following"
	segs = append(segs,
		ui.Seg{Text: plural(pr.Followers, "Follower"), Link: &p.followers},
		ui.Seg{Text: " • ", Color: dim},
		ui.Seg{Text: ui.FmtNumber(pr.Following) + " Following", Link: &p.following},
	)
	return ui.Inline(gtx, 14, ui.Regular, ui.ColText, ui.ColNone, segs...)
}

// actionRow keeps the header's "…" button where the official page has it.
func (p *page) actionRow(gtx layout.Context) layout.Dimensions {
	return ui.PageInset(gtx, 17, 17, func(gtx layout.Context) layout.Dimensions {
		d := p.more.Layout(gtx, ui.IconStyle{Icon: "more", Size: 32, Box: 32, Label: "More options"})
		d.Size.Y = ui.Dp(gtx, 56)
		return d
	})
}

func (p *page) tracksTitle(gtx layout.Context) layout.Dimensions {
	return ui.PageInset(gtx, 0, 16, func(gtx layout.Context) layout.Dimensions {
		return ui.Column(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, "Top tracks this month", 24, ui.Bold, ui.ColText)
			},
			func(gtx layout.Context) layout.Dimensions {
				return ui.Pad(gtx, 2, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
					return ui.Text(gtx, "Only visible to you", 14, ui.Regular, ui.ColText2)
				})
			},
		)
	})
}
