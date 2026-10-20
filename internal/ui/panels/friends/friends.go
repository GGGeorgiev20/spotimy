// Package friends is the Friend Activity panel (Prefs.RightPanel "friends").
package friends

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() { ui.RightPanels["friends"] = p.layout }

const refreshEvery = 2 * time.Minute

type row struct {
	avatar                   ui.Click
	name, track, artist, ctx ui.Link
}

var p panel

type panel struct {
	header ui.PanelHeader
	res    ui.Res[[]spotify.Friend]
	loaded time.Time
	list   ui.List
	rows   []row
}

func (p *panel) layout(gtx layout.Context) layout.Dimensions {
	// Reload on open and every two minutes while open. The frame request also
	// keeps the "3 min" labels fresh.
	if p.loaded.IsZero() || gtx.Now.Sub(p.loaded) >= refreshEvery {
		p.loaded = gtx.Now
		p.res.Load(ui.B.Friends)
	}
	gtx.Execute(op.InvalidateCmd{At: p.loaded.Add(refreshEvery)})

	return ui.Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return p.header.Layout(gtx, "Close Friend Activity", func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, "Friend Activity", 16, ui.Bold, ui.ColText)
			}, nil)
		},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			return p.body(gtx)
		},
	)
}

func (p *panel) body(gtx layout.Context) layout.Dimensions {
	msg := func(s string) layout.Dimensions {
		return ui.Pad(gtx, 0, 16, 0, 16, func(gtx layout.Context) layout.Dimensions {
			return ui.Label{Text: s, Size: 14, Color: ui.ColText2, MaxLines: 3}.Layout(gtx)
		})
	}
	switch {
	case !p.res.Done:
		return ui.Loading(gtx)
	case p.res.Err != nil && len(p.res.V) == 0:
		ui.Log.Warn("friend activity", "err", p.res.Err)
		return msg("Couldn't load Friend Activity.")
	case len(p.res.V) == 0:
		return msg("Follow friends to see what they're listening to.")
	}
	fs := p.res.V
	if len(p.rows) < len(fs) {
		p.rows = append(p.rows, make([]row, len(fs)-len(p.rows))...)
	}
	return p.list.Layout(gtx, len(fs), func(gtx layout.Context, i int) layout.Dimensions {
		return p.rows[i].layout(gtx, fs[i])
	})
}

func (r *row) layout(gtx layout.Context, f spotify.Friend) layout.Dimensions {
	if r.avatar.Clicked(gtx) || r.name.Click.Clicked(gtx) {
		ui.OpenURI(f.User.URI)
	}
	if r.track.Click.Clicked(gtx) {
		ui.OpenURI(f.Album.URI)
	}
	if r.artist.Click.Clicked(gtx) {
		ui.OpenURI(f.Artist.URI)
	}
	if r.ctx.Click.Clicked(gtx) {
		ui.OpenURI(f.Context.URI)
	}

	w := gtx.Constraints.Max.X
	padX, padY, av, gap := ui.Dp(gtx, 16), ui.Dp(gtx, 12), ui.Dp(gtx, 48), ui.Dp(gtx, 12)
	tx := padX + av + gap
	tw := max(0, w-tx-padX)

	// Avatar with the "playing now" dot.
	r.avatar.Label = f.User.Name
	ui.Offset(gtx, padX, padY, func() {
		r.avatar.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			ui.Image{URL: ui.Pick(f.User.Images, av), Round: true, Icon: "user"}.Layout(gtx, image.Pt(av, av))
			if f.Playing {
				d, b := ui.Dp(gtx, 14), ui.Dp(gtx, 2)
				ui.Offset(gtx, av-d, av-d, func() {
					ui.FillCircle(gtx.Ops, d, ui.ColSurface)
					ui.Offset(gtx, b, b, func() { ui.FillCircle(gtx.Ops, d-2*b, ui.ColAccent) })
				})
			}
			return layout.Dimensions{Size: image.Pt(av, av)}
		})
	})

	y := padY
	line := func(wd layout.Widget) {
		var d layout.Dimensions
		ui.Offset(gtx, tx, y, func() {
			g := gtx
			g.Constraints = layout.Constraints{Max: image.Pt(tw, gtx.Constraints.Max.Y)}
			d = wd(g)
		})
		y += d.Size.Y + ui.Dp(gtx, 2)
	}

	// Name and time (or the equalizer while they're playing).
	var when layout.Dimensions
	ui.Offset(gtx, tx, y, func() {
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(tw, gtx.Constraints.Max.Y)}
		when = r.when(g, f, tw)
	})
	line(func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = max(0, tw-when.Size.X-ui.Dp(gtx, 8))
		return r.name.Layout(gtx, f.User.Name, 14, ui.Bold, ui.ColText, ui.ColNone)
	})

	// "Track • Artist": the artist keeps up to 45% of the line.
	line(func(gtx layout.Context) layout.Dimensions {
		sep := ui.Measure(gtx, " • ", 14, ui.Regular)
		artistW := min(ui.Measure(gtx, f.Artist.Name, 14, ui.Regular), tw*45/100)
		return ui.Inline(gtx, 14, ui.Regular, ui.ColText2, ui.ColText,
			ui.Seg{Widget: func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, tw-artistW-sep)
				return r.track.Layout(gtx, f.Track.Name, 14, ui.Regular, ui.Alpha(ui.ColText, .85), ui.ColText)
			}},
			ui.Seg{Text: " • "},
			ui.Seg{Text: f.Artist.Name, Link: &r.artist},
		)
	})

	// Context: playlist, album or artist.
	if f.Context.Name != "" {
		line(func(gtx layout.Context) layout.Dimensions {
			is := ui.Dp(gtx, 16)
			ui.Offset(gtx, 0, (ui.Dp(gtx, 17)-is)/2, func() { ui.IconPx(gtx, ctxIcon(f.Context.Type), is, ui.ColText2) })
			ui.Offset(gtx, is+ui.Dp(gtx, 6), 0, func() {
				gtx.Constraints.Max.X = max(0, gtx.Constraints.Max.X-is-ui.Dp(gtx, 6))
				r.ctx.Layout(gtx, f.Context.Name, 14, ui.Regular, ui.ColText2, ui.ColText)
			})
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, ui.Dp(gtx, 17))}
		})
	}
	return layout.Dimensions{Size: image.Pt(w, max(y+padY-ui.Dp(gtx, 2), 2*padY+av))}
}

// when draws the right-aligned time since they played it, or the equalizer.
func (r *row) when(gtx layout.Context, f spotify.Friend, tw int) layout.Dimensions {
	if f.Playing {
		s := ui.Dp(gtx, 14)
		ui.Offset(gtx, tw-s, ui.Dp(gtx, 2), func() { ui.Equalizer(gtx, s) })
		return layout.Dimensions{Size: image.Pt(s, s)}
	}
	t := ui.FmtAgo(f.Timestamp, gtx.Now)
	tw2 := ui.Measure(gtx, t, 12, ui.Regular)
	ui.Offset(gtx, tw-tw2, ui.Dp(gtx, 2), func() { ui.Text(gtx, t, 12, ui.Regular, ui.ColText2) })
	return layout.Dimensions{Size: image.Pt(tw2, ui.Dp(gtx, 14))}
}

func ctxIcon(typ string) string {
	switch typ {
	case "album", "artist":
		return typ
	}
	return "playlist"
}
