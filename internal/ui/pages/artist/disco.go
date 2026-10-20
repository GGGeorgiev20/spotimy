package artist

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

const discoPage = 50

// disco is /artist/<id>/discography/<kind>: every release in a grid, loaded
// a page at a time while scrolling.
type disco struct {
	id, kind string
	artist   ui.Res[*spotify.ArtistPage]
	cards    []spotify.Card
	total    int
	started  bool
	busy     bool
	err      error
	list     ui.List
	rows     ui.CardRows
	name     ui.Link
	filter   ui.Click
}

func newDiscography(id, kind string) *disco {
	d := &disco{id: id, kind: kind}
	uri := "spotify:artist:" + id
	d.artist.Load(func() (*spotify.ArtistPage, error) { return ui.B.Artist(uri) })
	return d
}

// fetch loads the next page of releases.
func (d *disco) fetch() {
	if d.busy {
		return
	}
	d.busy = true
	uri, kind, off := "spotify:artist:"+d.id, d.kind, len(d.cards)
	ui.Async(func() (*spotify.SearchPage, error) { return ui.B.Discography(uri, kind, off, discoPage) }, func(p *spotify.SearchPage, err error) {
		d.busy, d.started = false, true
		if err != nil {
			d.err = err
			return
		}
		d.cards, d.total = append(d.cards, p.Cards...), p.Total
	})
}

func (d *disco) Layout(gtx layout.Context) layout.Dimensions {
	if !d.started && !d.busy {
		d.fetch()
	}
	switch {
	case d.err != nil:
		return ui.Unavailable(gtx, "The discography", d.err)
	case !d.started:
		return ui.Loading(gtx)
	}
	if d.name.Click.Clicked(gtx) {
		ui.Navigate("/artist/" + d.id)
	}
	if d.filter.Clicked(gtx) {
		ui.OpenMenu(d.filterMenu())
	}
	n := d.rows.Count(gtx, len(d.cards), 176)
	dims := d.list.Layout(gtx, n+2, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == 0:
			return d.toolbar(gtx)
		case i <= n:
			return d.rows.Row(gtx, i-1, d.cards, 176, ui.CardOpts{})
		}
		return ui.Gap(0, 24)(gtx)
	})
	if _, last := d.list.Range(); last >= n-2 && len(d.cards) < d.total {
		d.fetch()
	}
	return dims
}

func (d *disco) toolbar(gtx layout.Context) layout.Dimensions {
	return ui.Pad(gtx, 64, 24, 16, 24, func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		h := ui.Dp(gtx, 40)
		name := ""
		if d.artist.Ok() {
			name = d.artist.V.Name
		}
		ui.Offset(gtx, 0, (h-ui.Dp(gtx, 29))/2, func() {
			g := gtx
			g.Constraints.Max.X = w - ui.Dp(gtx, 160)
			d.name.Click.Label = name
			d.name.Layout(g, name, 24, ui.Bold, ui.ColText, ui.ColNone)
		})
		label := kindLabel(d.kind)
		bh, pad := ui.Dp(gtx, 32), ui.Dp(gtx, 8)
		tw := ui.Measure(gtx, label, 14, ui.Regular)
		bw := pad + tw + pad + ui.Dp(gtx, 16) + pad
		ui.Offset(gtx, w-bw, (h-bh)/2, func() {
			d.filter.Label = "Filter releases"
			d.filter.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				c := ui.Alpha(ui.ColText, .7)
				if d.filter.Hovered() {
					c = ui.ColText
				}
				m := op.Record(gtx.Ops)
				td := ui.Text(gtx, label, 14, ui.Regular, c)
				call := m.Stop()
				ui.Offset(gtx, pad, (bh-td.Size.Y)/2, func() { call.Add(gtx.Ops) })
				ui.Offset(gtx, pad+tw+pad, (bh-ui.Dp(gtx, 16))/2, func() { ui.IconPx(gtx, "chevron-down", ui.Dp(gtx, 16), c) })
				return layout.Dimensions{Size: image.Pt(bw, bh)}
			})
		})
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}

func kindLabel(kind string) string {
	switch kind {
	case "albums":
		return "Albums"
	case "singles":
		return "Singles and EPs"
	case "compilations":
		return "Compilations"
	}
	return "All"
}

// filterMenu lists the release kinds this artist has.
func (d *disco) filterMenu() []ui.MenuItem {
	items := []ui.MenuItem{}
	for _, k := range []string{"all", "albums", "singles", "compilations"} {
		if d.artist.Ok() && k != "all" && d.artist.V.Totals[map[string]int{"albums": 0, "singles": 1, "compilations": 2}[k]] == 0 {
			continue
		}
		items = append(items, ui.MenuItem{Label: kindLabel(k), Check: d.kind == k, Action: func() {
			ui.Replace("/artist/" + d.id + "/discography/" + k)
		}})
	}
	return items
}
