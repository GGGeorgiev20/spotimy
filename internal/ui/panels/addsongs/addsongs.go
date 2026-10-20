// Package addsongs is the "Add to playlist" right panel that a playlist's
// "Find more" opens: a search box over suggested songs.
package addsongs

import (
	"image"
	"strings"

	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

const panelName = "addsongs"

func init() { ui.RightPanels[panelName] = layoutPanel }

var p struct {
	target spotify.Card // the playlist songs are added to
	sugg   ui.Res[[]spotify.Track]
	hits   ui.Res[[]spotify.Track]
	query  string
	input  ui.Input
	chip   ui.Pill
	head   ui.PanelHeader
	list   ui.List
	rows   map[string]*row
	added  map[string]bool
	skip   []string // the playlist's songs, never suggested
}

type row struct {
	click, play, add ui.Click
	artists          []ui.Link
}

// Open shows the panel for playlist target. have are its songs, which are
// left out of the suggestions.
func Open(target spotify.Card, have []string) {
	if p.target.URI != target.URI {
		p.target, p.query, p.skip = target, "", have
		p.sugg, p.hits = ui.Res[[]spotify.Track]{}, ui.Res[[]spotify.Track]{}
		p.added, p.rows = map[string]bool{}, map[string]*row{}
		p.input.SetText("")
	}
	ui.Prefs.RightPanel = panelName
}

func layoutPanel(gtx layout.Context) layout.Dimensions {
	p.sugg.Once(func() ([]spotify.Track, error) { return ui.B.PlaylistRecommendations(p.target.URI, p.skip, 25) })
	if changed, _ := p.input.Poll(gtx); changed {
		if q := strings.TrimSpace(p.input.Text()); q != p.query {
			p.query = q
			if q != "" {
				p.hits.Load(func() ([]spotify.Track, error) {
					r, err := ui.B.SearchType(q, "tracks", 0, 30)
					if err != nil {
						return nil, err
					}
					return r.Tracks, nil
				})
			}
		}
	}
	tracks, loaded, label := p.sugg.V, p.sugg.Done, "Tracks matching "+p.target.Name
	if p.query != "" {
		tracks, loaded, label = p.hits.V, p.hits.Done, ""
	}
	px := func(v float32) int { return ui.Dp(gtx, v) }

	head := func(gtx layout.Context) layout.Dimensions {
		return p.head.Layout(gtx, "Close", func(gtx layout.Context) layout.Dimensions {
			return ui.Text(gtx, "Add to playlist", 16, ui.Bold, ui.ColText)
		}, nil)
	}
	search := func(gtx layout.Context) layout.Dimensions {
		return ui.Pad(gtx, 0, 16, 16, 16, func(gtx layout.Context) layout.Dimensions {
			bw, bh := gtx.Constraints.Max.X, px(48)
			ui.FillRect(gtx.Ops, image.Rect(0, 0, bw, bh), bh/2, ui.RGB(0x1f1f1f))
			is := px(16)
			ui.Offset(gtx, px(12), (bh-is)/2, func() { ui.IconPx(gtx, "search", is, ui.ColText2) })
			ui.Offset(gtx, px(40), (bh-px(22))/2, func() {
				g := gtx
				g.Constraints = layout.Constraints{Min: image.Pt(bw-px(52), 0), Max: image.Pt(bw-px(52), px(22))}
				p.input.Layout(g, ui.InputStyle{Hint: "Search", Size: 16})
			})
			return layout.Dimensions{Size: image.Pt(bw, bh)}
		})
	}
	chips := func(gtx layout.Context) layout.Dimensions {
		return ui.Pad(gtx, 0, 16, 0, 16, func(gtx layout.Context) layout.Dimensions {
			return p.chip.Layout(gtx, ui.PillStyle{Text: "Suggested songs", PadX: 12, Selected: true})
		})
	}
	text := func(s string, top float32) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return ui.Pad(gtx, top, 16, 8, 16, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, s, 14, ui.Regular, ui.ColText2)
			})
		}
	}
	var items []layout.Widget
	items = append(items, head, search)
	if p.query == "" {
		items = append(items, chips, text(label, 8))
	}
	switch {
	case !loaded:
		items = append(items, ui.Loading)
	case len(tracks) == 0 && p.query != "":
		items = append(items, text(`No results found for "`+p.query+`"`, 16))
	}
	for i := range tracks {
		t := tracks[i]
		items = append(items, func(gtx layout.Context) layout.Dimensions { return trackRow(gtx, t) })
	}
	items = append(items, ui.Gap(0, 16))
	return p.list.Layout(gtx, len(items), func(gtx layout.Context, i int) layout.Dimensions { return items[i](gtx) })
}

// trackRow is one 64dp suggestion: cover, title, artists and the Add button.
func trackRow(gtx layout.Context, t spotify.Track) layout.Dimensions {
	r := p.rows[t.URI]
	if r == nil {
		r = &row{}
		p.rows[t.URI] = r
	}
	if len(r.artists) < len(t.Artists) {
		r.artists = make([]ui.Link, len(t.Artists))
	}
	for k := range t.Artists {
		if r.artists[k].Click.Clicked(gtx) {
			ui.OpenURI(t.Artists[k].URI)
		}
	}
	if r.add.Clicked(gtx) && !p.added[t.URI] {
		p.added[t.URI] = true
		ui.AddToPlaylist(p.target, []string{t.URI})
	}
	if r.play.Clicked(gtx) {
		ui.Play(t.URI, "")
	}
	if _, ok := r.click.RightClicked(gtx); ok {
		ui.OpenMenu(ui.TrackMenu(t, ui.TrackMenuOpts{}))
	}
	if r.click.DoubleClicked(gtx) {
		ui.Play(t.URI, "")
	}
	w, h := gtx.Constraints.Max.X, ui.Dp(gtx, 64)
	px := func(v float32) int { return ui.Dp(gtx, v) }
	r.click.Label, r.click.Arrow = t.Name, true
	r.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hov := r.click.Hovered()
		if hov {
			ui.FillRect(gtx.Ops, image.Rect(px(8), 0, w-px(8), h), px(6), ui.Alpha(ui.ColText, .1))
		}
		cover := px(48)
		ui.Offset(gtx, px(16), (h-cover)/2, func() {
			var imgs []spotify.Image
			if t.Album != nil {
				imgs = t.Album.Images
			}
			ui.Image{URL: ui.Pick(imgs, cover), Radius: px(4), Icon: "note"}.Layout(gtx, image.Pt(cover, cover))
			if hov {
				ui.FillRect(gtx.Ops, image.Rect(0, 0, cover, cover), px(4), ui.Alpha(ui.ColBg, .4))
				r.play.Label = "Play " + t.Name
				ui.Offset(gtx, (cover-px(32))/2, (cover-px(32))/2, func() {
					r.play.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						ui.Offset(gtx, px(8), px(8), func() { ui.IconPx(gtx, "play", px(16), ui.ColText) })
						return layout.Dimensions{Size: image.Pt(px(32), px(32))}
					})
				})
			}
		})
		tx := px(16 + 48 + 12)
		tw := w - tx - px(16+32+4)
		ui.Offset(gtx, tx, 0, func() {
			g := gtx
			g.Constraints = layout.Constraints{Max: image.Pt(tw, h)}
			ui.Offset(gtx, 0, px(10), func() { ui.Text(g, t.Name, 16, ui.Regular, ui.ColText) })
			var segs []ui.Seg
			if t.Explicit {
				segs = append(segs, ui.Seg{Widget: ui.ExplicitBadge})
			}
			for k, a := range t.Artists {
				if k > 0 {
					segs = append(segs, ui.Seg{Text: ", "})
				}
				segs = append(segs, ui.Seg{Text: a.Name, Link: &r.artists[k]})
			}
			ui.Offset(gtx, 0, px(34), func() { ui.Inline(g, 14, ui.Regular, ui.ColText2, ui.ColText, segs...) })
		})
		ui.Offset(gtx, w-px(16+32), (h-px(32))/2, func() {
			r.add.Label = "Add to Playlist"
			if p.added[t.URI] {
				r.add.Label = "Added"
			}
			r.add.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				icon, c := "plus-circle", ui.ColText2
				switch {
				case p.added[t.URI]:
					icon, c = "check-circle", ui.ColAccent
				case r.add.Hovered():
					c = ui.ColText
				}
				ui.Offset(gtx, px(8), px(8), func() { ui.IconPx(gtx, icon, px(16), c) })
				return layout.Dimensions{Size: image.Pt(px(32), px(32))}
			})
		})
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
	return layout.Dimensions{Size: image.Pt(w, h)}
}
