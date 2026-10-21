package playlist

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui/panels/addsongs"
)

const recCount = 10

// recs is what the web player shows under a playlist you can edit: "Find
// more" and ten recommended songs to add.
type recs struct {
	res     ui.Res[[]spotify.Track]
	added   map[string]bool // taken out of the list once added
	find    ui.Link
	refresh ui.Click
	rows    map[string]*recRow
}

type recRow struct {
	row, play ui.Click
	add       ui.Pill
	artists   []ui.Link
	album     ui.Link
}

// shown is the recommendations still on offer.
func (r *recs) shown() []spotify.Track {
	var out []spotify.Track
	for _, t := range r.res.V {
		if !r.added[t.URI] {
			out = append(out, t)
		}
	}
	return out
}

func (r *recs) fetch(uri string, skip []string) {
	r.added = map[string]bool{}
	r.res.Load(func() ([]spotify.Track, error) { return ui.B.PlaylistRecommendations(uri, skip, recCount) })
}

// haveURIs are the songs loaded so far: never recommended.
func (p *page) haveURIs() []string {
	var uris []string
	for _, t := range p.tracks.Loaded() {
		uris = append(uris, t.URI)
	}
	return uris
}

func (p *page) card(pl *spotify.Playlist) spotify.Card {
	return spotify.Card{URI: p.uri, Type: "playlist", Name: pl.Name, Images: pl.Images}
}

// layoutFind is the "Find more" link under the last row; it opens the
// "Add to playlist" panel.
func (p *page) layoutFind(gtx layout.Context, pl *spotify.Playlist) layout.Dimensions {
	r := &p.recs
	if r.find.Click.Clicked(gtx) {
		addsongs.Open(p.card(pl), p.haveURIs())
	}
	w := gtx.Constraints.Max.X
	r.find.Click.Label = "Find more songs or episodes for your playlist"
	ui.Offset(gtx, w-ui.Dp(gtx, 24+16)-ui.Measure(gtx, "Find more", 14, ui.Bold), ui.Dp(gtx, 32), func() {
		r.find.Layout(gtx, "Find more", 14, ui.Bold, ui.ColText, ui.ColNone)
	})
	return layout.Dimensions{Size: image.Pt(w, ui.Dp(gtx, 32+20))}
}

// layoutRecs is the "Recommended" block.
func (p *page) layoutRecs(gtx layout.Context, pl *spotify.Playlist) layout.Dimensions {
	r := &p.recs
	px := func(v float32) int { return ui.Dp(gtx, v) }
	w := gtx.Constraints.Max.X
	cw := w - 2*px(24)
	tracks := r.shown()

	if r.refresh.Clicked(gtx) {
		skip := p.haveURIs()
		for _, t := range r.res.V {
			skip = append(skip, t.URI)
		}
		r.fetch(p.uri, skip)
	}
	sub := "Based on what's in this playlist"
	if p.tracks.Total == 0 {
		sub = "Based on your listening"
	}
	y := px(30)
	ui.Offset(gtx, px(24), y, func() {
		ui.Text(gtx, "Recommended", 24, ui.Bold, ui.ColText)
		ui.Offset(gtx, 0, px(34), func() { ui.Text(gtx, sub, 14, ui.Regular, ui.ColText2) })
	})
	y += px(54 + 24)
	for i, t := range tracks {
		ui.Offset(gtx, px(24), y+px(1)+i*px(56), func() {
			g := gtx
			g.Constraints = layout.Exact(image.Pt(cw, px(56)))
			r.layoutRow(g, pl, t, p.uri)
		})
	}
	y += px(10*56 + 2 + 24)
	ui.Offset(gtx, w-px(24)-px(122), y, func() {
		r.refresh.Label = "Refresh"
		r.refresh.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			c := ui.ColText2
			if r.refresh.Hovered() {
				c = ui.ColText
			}
			h := px(48)
			g := gtx
			g.Constraints = layout.Exact(image.Pt(px(122), h))
			layout.Center.Layout(g, func(gtx layout.Context) layout.Dimensions { return ui.Text(gtx, "Refresh", 16, ui.Bold, c) })
			return layout.Dimensions{Size: image.Pt(px(122), h)}
		})
	})
	return layout.Dimensions{Size: image.Pt(w, y+px(48))}
}

// layoutRow is one recommended song: cover, title, artists, album and "Add".
func (r *recs) layoutRow(gtx layout.Context, pl *spotify.Playlist, t spotify.Track, uri string) {
	px := func(v float32) int { return ui.Dp(gtx, v) }
	if r.rows == nil {
		r.rows = map[string]*recRow{}
	}
	rr := r.rows[t.URI]
	if rr == nil {
		rr = &recRow{}
		r.rows[t.URI] = rr
	}
	if len(rr.artists) < len(t.Artists) {
		rr.artists = make([]ui.Link, len(t.Artists))
	}
	if rr.add.Click.Clicked(gtx) {
		r.added[t.URI] = true
		ui.AddToPlaylist(spotify.Card{URI: uri, Type: "playlist", Name: pl.Name, Images: pl.Images}, []string{t.URI})
	}
	if rr.play.Clicked(gtx) || rr.row.DoubleClicked(gtx) {
		ui.Play(t.URI, "")
	}
	for k := range t.Artists {
		if rr.artists[k].Click.Clicked(gtx) {
			ui.OpenURI(t.Artists[k].URI)
		}
	}
	if rr.album.Click.Clicked(gtx) && t.Album != nil {
		ui.OpenURI(t.Album.URI)
	}
	if _, ok := rr.row.RightClicked(gtx); ok {
		ui.OpenMenu(ui.TrackMenu(t, ui.TrackMenuOpts{}))
	}
	w, h := gtx.Constraints.Max.X, px(56)
	rr.row.Label, rr.row.Arrow = t.Name, true
	rr.row.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hov := rr.row.Hovered()
		if hov {
			ui.FillRect(gtx.Ops, image.Rect(0, 0, w, h), px(4), ui.ColRowHover)
		}
		gap, last := px(16), px(120)
		inner := w - 2*px(16)
		// Columns: title 2fr, album 1fr, one empty 1fr, then the Add button.
		fr := float64(inner-last-3*gap) / 4
		titleW, albumW := int(fr*2), int(fr)
		if w+2*px(24) <= px(640) {
			titleW, albumW = inner-last-gap, 0
		}
		cover := px(40)
		cx := px(8)
		ui.Offset(gtx, cx, (h-cover)/2, func() {
			var imgs []spotify.Image
			if t.Album != nil {
				imgs = t.Album.Images
			}
			ui.Image{URL: ui.Pick(imgs, cover), Radius: px(4), Icon: "note"}.Layout(gtx, image.Pt(cover, cover))
			if hov {
				ui.FillRect(gtx.Ops, image.Rect(0, 0, cover, cover), px(4), ui.Alpha(ui.ColBg, .5))
				rr.play.Label = "Play " + t.Name
				rr.play.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					ui.Offset(gtx, (cover-px(16))/2, (cover-px(16))/2, func() { ui.IconPx(gtx, "play", px(16), ui.ColText) })
					return layout.Dimensions{Size: image.Pt(cover, cover)}
				})
			}
		})
		nameCol := ui.ColText
		if ui.IsCurrentTrack(t, "") {
			nameCol = ui.ColAccent
		}
		sub := ui.ColText2
		if hov {
			sub = ui.ColText
		}
		tx := px(8 + 40 + 12)
		m := op.Record(gtx.Ops)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(max(0, px(16)+titleW-tx), h)}
		nd := ui.Text(g, t.Name, 16, ui.Regular, nameCol)
		var segs []ui.Seg
		if t.Explicit {
			segs = append(segs, ui.Seg{Widget: ui.ExplicitBadge})
		}
		for k, a := range t.Artists {
			if k > 0 {
				segs = append(segs, ui.Seg{Text: ", "})
			}
			segs = append(segs, ui.Seg{Text: a.Name, Link: &rr.artists[k]})
		}
		var ad layout.Dimensions
		ui.Offset(gtx, 0, nd.Size.Y+px(2), func() { ad = ui.Inline(g, 14, ui.Regular, sub, ui.ColText, segs...) })
		call := m.Stop()
		ui.Offset(gtx, tx, (h-nd.Size.Y-px(2)-ad.Size.Y)/2, func() { call.Add(gtx.Ops) })
		if albumW > 0 && t.Album != nil {
			ui.Offset(gtx, px(16)+titleW+gap, 0, func() {
				g := gtx
				g.Constraints = layout.Exact(image.Pt(albumW, h))
				layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = image.Point{}
					return rr.album.Layout(gtx, t.Album.Name, 14, ui.Regular, ui.ColText2, ui.ColText)
				})
			})
		}
		ui.Offset(gtx, w-px(16)-px(61), (h-px(32))/2, func() {
			rr.add.Layout(gtx, ui.PillStyle{Text: "Add", Weight: ui.Bold, Outline: true})
		})
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}
