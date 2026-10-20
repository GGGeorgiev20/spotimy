package artist

import (
	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

// shelfKinds are the "Show all" pages of the artist's shelves
// (/artist/<id>/<kind>).
var shelfKinds = map[string]bool{"featuring": true, "discovered-on": true, "playlists": true, "related": true, "appears-on": true}

// shelfPage is one shelf in full: its title and a grid of cards.
type shelfPage struct {
	res  ui.Res[*spotify.Shelf]
	list ui.List
	grid ui.CardRows
}

func newShelfPage(id, kind string) *shelfPage {
	p := &shelfPage{}
	uri := "spotify:artist:" + id
	p.res.Load(func() (*spotify.Shelf, error) { return ui.B.ArtistShelf(uri, kind) })
	return p
}

func (p *shelfPage) Layout(gtx layout.Context) layout.Dimensions {
	switch {
	case !p.res.Done:
		return ui.Loading(gtx)
	case p.res.Err != nil:
		return ui.Unavailable(gtx, "This page", p.res.Err)
	}
	s := p.res.V
	rows := p.grid.Count(gtx, len(s.Items), 176)
	return p.list.Layout(gtx, rows+2, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == 0:
			// A shelf's title row: 48dp high, the title at its bottom.
			return ui.Pad(gtx, 19, 24, 8, 24, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, s.Title, 24, ui.Bold, ui.ColText)
			})
		case i <= rows:
			return p.grid.Row(gtx, i-1, s.Items, 176, ui.CardOpts{})
		}
		return ui.Gap(0, 24)(gtx)
	})
}
