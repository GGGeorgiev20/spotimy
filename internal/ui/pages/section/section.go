// Package section is the "Show all" page of a home shelf (/section/<uri>).
package section

import (
	"net/url"
	"strings"

	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() {
	ui.Register("section", func(args []string) ui.Page {
		id, _ := url.PathUnescape(args[0])
		if !strings.HasPrefix(id, "spotify:") {
			id = "spotify:section:" + id
		}
		p := &page{}
		p.res.Load(func() (*spotify.Shelf, error) { return ui.B.HomeSection(id) })
		return p
	})
}

type page struct {
	res  ui.Res[*spotify.Shelf]
	list ui.List
	grid ui.CardRows
	head ui.Shelf
}

const minCard = 176

func (p *page) Layout(gtx layout.Context) layout.Dimensions {
	switch {
	case !p.res.Done:
		return ui.Loading(gtx)
	case p.res.Err != nil:
		return ui.Unavailable(gtx, "This section", p.res.Err)
	}
	s := p.res.V
	rows := p.grid.Count(gtx, len(s.Items), minCard)
	return p.list.Layout(gtx, rows+2, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == 0:
			return ui.PageInset(gtx, 64, 32, func(gtx layout.Context) layout.Dimensions {
				return p.head.HeaderRow(gtx, ui.ShelfOpts{Title: s.Title, Label: s.Label})
			})
		case i <= rows:
			return p.grid.Row(gtx, i-1, s.Items, minCard, ui.CardOpts{})
		}
		return ui.Gap(0, 24)(gtx)
	})
}
