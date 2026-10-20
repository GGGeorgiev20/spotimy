// Package episode is the podcast episode page (/episode/<id>).
package episode

import (
	"image/color"

	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() {
	ui.Register("episode", func(args []string) ui.Page {
		p := &page{uri: "spotify:episode:" + args[0], color: ui.RGB(0x535353)}
		ui.LoadSaved([]string{p.uri})
		p.res.Load(func() (*spotify.Episode, error) { return ui.B.Episode(p.uri) })
		return p
	})
}

type page struct {
	uri     string
	res     ui.Res[*spotify.Episode]
	ready   bool
	color   color.NRGBA
	list    ui.List
	header  ui.EntityHeader
	actions ui.ActionBar
	show    ui.Link
	headerH int
}

func (p *page) playing() bool { return ui.IsCurrent(p.uri, "") && ui.Playing() }

func (p *page) play() {
	if ui.IsCurrent(p.uri, "") {
		ui.PlayPause()
	} else {
		ui.Play(p.uri, "")
	}
}

func (p *page) Chrome() ui.Chrome {
	title := ""
	if p.res.Ok() {
		title = p.res.V.Name
	}
	return ui.Chrome{Title: title, Color: p.color, ContextURI: p.uri, OnPlay: p.play, Threshold: p.headerH, List: &p.list}
}

func (p *page) Layout(gtx layout.Context) layout.Dimensions {
	switch {
	case !p.res.Done:
		return ui.Loading(gtx)
	case p.res.Err != nil:
		return ui.Unavailable(gtx, "This episode", p.res.Err)
	}
	ep := p.res.V
	if !p.ready {
		p.ready = true
		ui.CoverColor(ep.Images, func(c color.NRGBA) { p.color = c })
	}
	if p.show.Click.Clicked(gtx) {
		ui.OpenURI(ep.ShowURI)
	}
	ui.HeaderFade(gtx, p.color, p.headerH-p.list.ScrollTop())
	if !p.list.Scrolled(0) {
		ui.HeaderFade(gtx, p.color, p.headerH)
	}
	return p.list.Layout(gtx, 3, func(gtx layout.Context, i int) layout.Dimensions {
		switch i {
		case 0:
			d := p.header.Layout(gtx, ui.HeaderOpts{
				Images: ep.Images, Type: "Podcast Episode", Title: ep.Name, Color: p.color, Icon: "note",
				Meta: func(gtx layout.Context) layout.Dimensions {
					return p.show.Layout(gtx, ep.ShowName, 24, ui.Bold, ui.ColText, ui.ColNone)
				},
			})
			p.headerH = d.Size.Y
			return d
		case 1:
			playing := p.playing()
			return p.actions.Layout(gtx, ui.ActionOpts{
				ContextURI: p.uri, OnPlay: p.play, Playing: &playing, SaveURI: p.uri,
				Above: func(gtx layout.Context) layout.Dimensions { return meta(gtx, ep) },
			})
		}
		return ui.PageInset(gtx, 0, 32, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, ui.Dp(gtx, 800))
			return ui.Column(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return ui.Pad(gtx, 0, 0, 12, 0, func(gtx layout.Context) layout.Dimensions {
						return ui.Text(gtx, "Episode Description", 24, ui.Bold, ui.ColText)
					})
				},
				func(gtx layout.Context) layout.Dimensions {
					return ui.Label{Text: ep.Description, Size: 16, Color: ui.ColText2, MaxLines: -1}.Layout(gtx)
				},
			)
		})
	})
}

func meta(gtx layout.Context, ep *spotify.Episode) layout.Dimensions {
	s := ui.FmtEpisodeDate(ep.ReleaseDate, gtx.Now)
	if s != "" {
		s += " • "
	}
	s += ui.FmtEpisodeLen(ep.DurationMs)
	if ep.State == "IN_PROGRESS" {
		s += " • " + ui.FmtEpisodeLen(ep.DurationMs-ep.PlayedMs) + " left"
	}
	return ui.Label{Text: s, Size: 14, Color: ui.ColText2}.Layout(gtx)
}
