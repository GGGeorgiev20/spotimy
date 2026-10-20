// Package collection is Liked Songs (/collection/tracks) and Your Episodes
// (/collection/episodes).
package collection

import (
	"slices"

	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() {
	ui.Register("collection", func(args []string) ui.Page {
		if len(args) > 0 && args[0] == "episodes" {
			p := &episodes{}
			p.res.Load(yourEpisodes)
			return p
		}
		return newLiked()
	})
}

// yourEpisodes drops saved episodes Spotify no longer resolves (they come back blank).
func yourEpisodes() ([]spotify.Episode, error) {
	eps, err := ui.B.YourEpisodes()
	return slices.DeleteFunc(eps, func(e spotify.Episode) bool { return e.Name == "" }), err
}

var (
	likedColor    = ui.RGB(0x5038a0)
	episodesColor = ui.RGB(0x006450)
)

const pageSize = 100

// likedArt is the purple heart Spotify uses for Liked Songs.
var likedArt = []spotify.Image{{URL: "https://misc.scdn.co/liked-songs/liked-songs-300.png", Width: 300, Height: 300}}

// ---- Liked Songs ----

type liked struct {
	tracks  *ui.Paged
	table   ui.TrackTable
	list    ui.List
	header  ui.EntityHeader
	actions ui.ActionBar
	headerH int

	ctl  ui.ListControls // search and sort
	view ui.TrackView

	length  string
	lengthG int
	loading bool
}

func newLiked() *liked {
	p := &liked{lengthG: -1, loading: true}
	p.ctl = ui.ListControls{Hint: "Search in playlist", Default: ui.SortAdded, Options: []ui.SortOpt{
		{Key: ui.SortTitle, Label: "Title"}, {Key: ui.SortArtist, Label: "Artist"},
		{Key: ui.SortAlbum, Label: "Album"}, {Key: ui.SortAdded, Label: "Date added"},
	}}
	p.table = ui.TrackTable{Variant: "playlist"}
	p.tracks = ui.NewPaged(func(off, lim int) (*spotify.TrackPage, error) { return ui.B.LikedSongs(off, lim) }, pageSize)
	ui.Async(func() (*spotify.TrackPage, error) { return ui.B.LikedSongs(0, pageSize) }, func(tp *spotify.TrackPage, err error) {
		p.loading = false
		if err != nil {
			p.tracks.Err = err
			return
		}
		p.tracks.Seed(*tp)
		for _, t := range tp.Items {
			ui.Saved[ui.LikeKey(t)] = true
		}
	})
	return p
}

// ctx is how the player knows Liked Songs; it needs the user's name.
func ctx() string {
	if ui.Me == nil {
		return ""
	}
	return "spotify:user:" + ui.Me.Username + ":collection"
}

func (p *liked) Chrome() ui.Chrome {
	return ui.Chrome{Title: "Liked Songs", Color: likedColor, ContextURI: ctx(), OnPlay: func() { ui.PlayContext(ctx()) }, Threshold: p.headerH, List: &p.list}
}

const (
	itemHeader = iota
	itemActions
	itemTableHeader
	itemRows
)

func (p *liked) Layout(gtx layout.Context) layout.Dimensions {
	switch {
	case p.tracks.Err != nil:
		return ui.Unavailable(gtx, "Liked Songs", p.tracks.Err)
	case p.loading:
		return ui.Loading(gtx)
	}
	c := ctx()
	p.table.ContextURI = c
	ui.HeaderFade(gtx, likedColor, p.headerH-p.list.ScrollTop())
	if !p.list.Scrolled(0) {
		ui.HeaderFade(gtx, likedColor, p.headerH)
	}
	p.list.Sticky = func(gtx layout.Context) layout.Dimensions { return p.table.Header(gtx, true) }
	p.list.StickyIndex, p.list.StickyTop = itemTableHeader, ui.Dp(gtx, 64)

	reordered := p.view.Update(p.tracks, &p.ctl, &p.table)
	n := p.view.Len(p.tracks)
	d := p.list.Layout(gtx, itemRows+n+1, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == itemHeader:
			d := p.header.Layout(gtx, ui.HeaderOpts{
				Images: likedArt, Type: "Playlist", Title: "Liked Songs", Color: likedColor, Icon: "heart-filled",
				Meta: p.meta,
			})
			p.headerH = d.Size.Y
			return d
		case i == itemActions:
			d := p.actions.Layout(gtx, ui.ActionOpts{ContextURI: c, Shuffle: true})
			if p.tracks.Total > 0 {
				p.ctl.Layout(gtx, d.Size)
			}
			return d
		case i == itemTableHeader:
			return p.table.Header(gtx, false)
		case i < itemRows+n:
			return p.table.Row(gtx, i-itemRows, p.view.At(p.tracks, i-itemRows))
		case n == 0 && reordered && i == itemRows:
			return ui.NoMatches(gtx, p.ctl.Query())
		}
		return ui.Gap(0, 32)(gtx)
	})
	if !reordered {
		first, last := p.list.Range()
		p.tracks.Ensure(first-itemRows-5, last-itemRows+5)
	}
	return d
}

func (p *liked) meta(gtx layout.Context) layout.Dimensions {
	dim := ui.Alpha(ui.ColText, .7)
	var segs []ui.Seg
	if ui.Me != nil {
		segs = append(segs, ui.Seg{Text: ui.Me.Name, Weight: ui.Bold}, ui.Seg{Text: " • ", Color: dim})
	}
	if p.lengthG != p.tracks.Gen {
		p.lengthG = p.tracks.Gen
		p.length = ui.FmtListLength(p.tracks.Loaded(), p.tracks.Total)
	}
	songs := " songs"
	if p.tracks.Total == 1 {
		songs = " song"
	}
	s := ui.FmtNumber(p.tracks.Total) + songs
	if p.length != "" {
		s += ", " + p.length
	}
	segs = append(segs, ui.Seg{Text: s, Color: dim})
	return ui.Inline(gtx, 14, ui.Regular, ui.ColText, ui.ColNone, segs...)
}

// ---- Your Episodes ----

type episodes struct {
	res     ui.Res[[]spotify.Episode]
	list    ui.List
	header  ui.EntityHeader
	rows    []ui.EpisodeRow
	headerH int
}

func (p *episodes) Chrome() ui.Chrome {
	return ui.Chrome{Title: "Your Episodes", Color: episodesColor, Threshold: p.headerH, List: &p.list}
}

func (p *episodes) Layout(gtx layout.Context) layout.Dimensions {
	switch {
	case !p.res.Done:
		return ui.Loading(gtx)
	case p.res.Err != nil:
		return ui.Unavailable(gtx, "Your Episodes", p.res.Err)
	}
	eps := p.res.V
	if len(p.rows) != len(eps) {
		p.rows = make([]ui.EpisodeRow, len(eps))
	}
	ui.HeaderFade(gtx, episodesColor, p.headerH-p.list.ScrollTop())
	if !p.list.Scrolled(0) {
		ui.HeaderFade(gtx, episodesColor, p.headerH)
	}
	return p.list.Layout(gtx, len(eps)+3, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == 0:
			d := p.header.Layout(gtx, ui.HeaderOpts{
				Type: "Playlist", Title: "Your Episodes", Color: episodesColor, Icon: "note",
				Meta: func(gtx layout.Context) layout.Dimensions {
					s := ui.FmtNumber(len(eps)) + " episodes"
					if len(eps) == 1 {
						s = "1 episode"
					}
					return ui.Text(gtx, s, 14, ui.Regular, ui.ColText)
				},
			})
			p.headerH = d.Size.Y
			return d
		case i == 1:
			return ui.Gap(0, 24)(gtx)
		case i < len(eps)+2:
			return ui.Pad(gtx, 0, 24, 0, 24, func(gtx layout.Context) layout.Dimensions {
				return p.rows[i-2].Layout(gtx, eps[i-2], "")
			})
		}
		if len(eps) == 0 {
			return ui.Pad(gtx, 0, 24, 0, 24, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, "Episodes you save will appear here.", 16, ui.Regular, ui.ColText2)
			})
		}
		return ui.Gap(0, 32)(gtx)
	})
}
