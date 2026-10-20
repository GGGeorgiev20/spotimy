// Package show is the podcast page (/show/<id>).
package show

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

const pageSize = 50

func init() {
	ui.Register("show", func(args []string) ui.Page {
		p := &page{uri: "spotify:show:" + args[0], color: ui.RGB(0x535353)}
		p.res.Load(func() (*spotify.Show, error) { return ui.B.Show(p.uri) })
		return p
	})
}

type page struct {
	uri     string
	res     ui.Res[*spotify.Show]
	ready   bool
	color   color.NRGBA
	eps     []spotify.Episode
	rows    []ui.EpisodeRow
	busy    bool // an older page of episodes is loading
	done    bool // ...or failed, so don't retry every frame
	list    ui.List
	header  ui.EntityHeader
	actions ui.ActionBar
	headerH int
	aboveH  int // header + action bar: where the side column starts
}

func (p *page) Chrome() ui.Chrome {
	title := ""
	if p.res.Ok() {
		title = p.res.V.Name
	}
	return ui.Chrome{Title: title, Color: p.color, ContextURI: p.uri, OnPlay: func() { ui.PlayContext(p.uri) }, Threshold: p.headerH, List: &p.list}
}

func (p *page) setup(s *spotify.Show) {
	p.ready = true
	ui.Saved[p.uri] = s.Saved
	p.eps = s.Episodes
	p.rows = make([]ui.EpisodeRow, len(p.eps))
	ui.CoverColor(s.Images, func(c color.NRGBA) { p.color = c })
}

// more loads the next page of older episodes.
func (p *page) more(total int) {
	if p.busy || p.done || len(p.eps) >= total {
		return
	}
	p.busy = true
	ui.Async(func() ([]spotify.Episode, error) { return ui.B.ShowEpisodes(p.uri, len(p.eps), pageSize) }, func(es []spotify.Episode, err error) {
		p.busy = false
		if err != nil || len(es) == 0 {
			p.done = true
			return
		}
		p.eps = append(p.eps, es...)
		p.rows = append(p.rows, make([]ui.EpisodeRow, len(es))...)
	})
}

func about(gtx layout.Context, s *spotify.Show) layout.Dimensions {
	return ui.Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return ui.Pad(gtx, 0, 0, 16, 0, func(gtx layout.Context) layout.Dimensions { return ui.Text(gtx, "About", 24, ui.Bold, ui.ColText) })
		},
		func(gtx layout.Context) layout.Dimensions {
			return ui.Label{Text: s.Description, Size: 14, Color: ui.ColText2, MaxLines: -1}.Layout(gtx)
		},
	)
}

func (p *page) Layout(gtx layout.Context) layout.Dimensions {
	switch {
	case !p.res.Done:
		return ui.Loading(gtx)
	case p.res.Err != nil:
		return ui.Unavailable(gtx, "This show", p.res.Err)
	}
	s := p.res.V
	if !p.ready {
		p.setup(s)
	}
	w := gtx.Constraints.Max.X
	pad, gap := ui.ContentPad(gtx), ui.Dp(gtx, 32)
	// Wide: episodes left (2fr), About right (1fr); narrow: About comes first.
	col := (w - 2*pad - gap) / 3
	wide := w > ui.Dp(gtx, 900) && col >= ui.Dp(gtx, 220)
	epsW := w - 2*pad
	heading, rows := 3, 4 // item indexes; the narrow layout has About at 2
	if wide {
		epsW = 2 * col
		heading, rows = 2, 3
	}

	ui.HeaderFade(gtx, p.color, p.headerH-p.list.ScrollTop())
	if !p.list.Scrolled(0) {
		ui.HeaderFade(gtx, p.color, p.headerH)
	}
	n := len(p.eps)
	d := p.list.Layout(gtx, rows+n+1, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == 0:
			d := p.header.Layout(gtx, ui.HeaderOpts{
				Images: s.Images, Type: "Podcast", Title: s.Name, Color: p.color, Icon: "note",
				Meta: func(gtx layout.Context) layout.Dimensions {
					return ui.Text(gtx, s.Publisher, 16, ui.Bold, ui.ColText)
				},
			})
			p.headerH = d.Size.Y
			return d
		case i == 1:
			d := p.actions.Layout(gtx, ui.ActionOpts{
				ContextURI: p.uri, SaveURI: p.uri,
				Menu: func() []ui.MenuItem {
					return ui.CardMenu(spotify.Card{URI: p.uri, Type: "show", Name: s.Name, Images: s.Images}, ui.CardMenuOpts{})
				},
			})
			p.aboveH = p.headerH + d.Size.Y
			return d
		case i == heading:
			return ui.PageInset(gtx, 0, 16, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, "All Episodes", 24, ui.Bold, ui.ColText)
			})
		case i < heading:
			return ui.PageInset(gtx, 0, 24, func(gtx layout.Context) layout.Dimensions { return about(gtx, s) })
		case i < rows+n:
			g := gtx
			g.Constraints.Max.X, g.Constraints.Min.X = epsW, epsW
			var d layout.Dimensions
			ui.Offset(gtx, pad, 0, func() { d = p.rows[i-rows].Layout(g, p.eps[i-rows], p.uri) })
			return layout.Dimensions{Size: image.Pt(w, d.Size.Y)}
		}
		return ui.Gap(0, 32)(gtx)
	})
	if wide {
		// The About column scrolls with the list but isn't one of its items,
		// so the (long) episode list stays virtualized.
		if top := p.aboveH - p.list.ScrollTop(); top < d.Size.Y {
			defer clip.Rect{Max: d.Size}.Push(gtx.Ops).Pop()
			ui.Offset(gtx, pad+epsW+gap, top, func() {
				g := gtx
				g.Constraints = layout.Constraints{Max: image.Pt(col, 1<<20)}
				about(g, s)
			})
		}
	}
	if _, last := p.list.Range(); last >= rows+n-5 {
		p.more(s.Total)
	}
	return d
}
