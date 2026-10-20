package artist

import (
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

// tour is the "On Tour" section: upcoming concerts in a grid. The pages for a
// concert and for all of them live on the web player, so those open there.
type tour struct {
	title, all ui.Link
	rows       []ui.Click
}

// concertCols is how many concerts fit side by side at the content width w.
func concertCols(w int) int {
	switch {
	case w >= 540:
		return 3
	case w >= 360:
		return 2
	}
	return 1
}

func (p *page) tourSection(gtx layout.Context) layout.Dimensions {
	a, t := p.res.V, &p.tour
	url := "https://open.spotify.com/artist/" + ui.URIID(a.URI) + "/concerts"
	if t.title.Click.Clicked(gtx) || t.all.Click.Clicked(gtx) {
		backend.OpenURL(url)
	}
	if len(t.rows) < len(a.Concerts) {
		t.rows = make([]ui.Click, len(a.Concerts))
	}
	for i := range a.Concerts {
		if t.rows[i].Clicked(gtx) {
			backend.OpenURL("https://open.spotify.com/concert/" + ui.URIID(a.Concerts[i].URI))
		}
	}
	return ui.Pad(gtx, 0, 24, 40, 24, func(gtx layout.Context) layout.Dimensions {
		return ui.Column(gtx,
			func(gtx layout.Context) layout.Dimensions { return t.header(gtx, a) },
			ui.Gap(0, 8),
			func(gtx layout.Context) layout.Dimensions { return t.grid(gtx, a.Concerts) },
		)
	})
}

// header is the 48dp row: the title at its bottom and the link to every concert.
func (t *tour) header(gtx layout.Context, a *spotify.ArtistPage) layout.Dimensions {
	w, h := gtx.Constraints.Max.X, ui.Dp(gtx, 48)
	gtx.Constraints.Min = image.Point{}
	all := fmt.Sprintf("View all upcoming concerts (%d)", a.ConcertTotal)
	allW := ui.Measure(gtx, all, 14, ui.Bold)
	ui.Offset(gtx, 0, h-ui.Dp(gtx, 29), func() {
		g := gtx
		g.Constraints.Max.X = max(0, w-allW-ui.Dp(gtx, 16))
		t.title.Click.Label = "On Tour"
		t.title.Layout(g, "On Tour", 24, ui.Bold, ui.ColText, ui.ColNone)
	})
	ui.Offset(gtx, w-allW, h-ui.Dp(gtx, 21), func() {
		t.all.Click.Label = all
		t.all.Layout(gtx, all, 14, ui.Bold, ui.ColText2, ui.ColText)
	})
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func (t *tour) grid(gtx layout.Context, cs []spotify.Concert) layout.Dimensions {
	w, gap := gtx.Constraints.Max.X, ui.Dp(gtx, 8)
	cols := concertCols(int(float32(w) / gtx.Metric.PxPerDp))
	cw := (w - (cols-1)*gap) / cols
	rowH := ui.Dp(gtx, 82)
	for i, c := range cs {
		x, y := i%cols*(cw+gap), i/cols*(rowH+gap)
		ui.Offset(gtx, x, y, func() {
			g := gtx
			g.Constraints = layout.Exact(image.Pt(cw, rowH))
			t.concert(g, &t.rows[i], c)
		})
	}
	n := (len(cs) + cols - 1) / cols
	return layout.Dimensions{Size: image.Pt(w, max(0, n*(rowH+gap)-gap))}
}

// concert is one cell. Its hover box reaches 8dp left of the text, as on the web.
func (t *tour) concert(gtx layout.Context, click *ui.Click, c spotify.Concert) {
	pad, w, h := ui.Dp(gtx, 8), gtx.Constraints.Max.X, gtx.Constraints.Max.Y
	click.Label = c.City + ", " + c.Title
	ui.Offset(gtx, -pad, 0, func() {
		click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			bw := w + pad
			if click.Hovered() {
				ui.FillRect(gtx.Ops, image.Rect(0, 0, bw, h), ui.Dp(gtx, 4), ui.Alpha(ui.ColText, .1))
			}
			ui.Offset(gtx, pad, ui.Dp(gtx, 9), func() { dateTile(gtx, c) })
			tx := pad + ui.Dp(gtx, 80)
			g := gtx
			g.Constraints.Max.X = max(0, w-ui.Dp(gtx, 80))
			ui.Offset(gtx, tx, pad, func() {
				ui.Column(g,
					func(gtx layout.Context) layout.Dimensions { return ui.Text(gtx, c.City, 16, ui.Bold, ui.ColText) },
					ui.Gap(0, 4),
					func(gtx layout.Context) layout.Dimensions { return ui.Text(gtx, c.Title, 14, ui.Regular, ui.ColText2) },
					func(gtx layout.Context) layout.Dimensions {
						return ui.Text(gtx, c.Start.Format("Mon 3:04 PM")+" • "+c.Venue, 14, ui.Regular, ui.ColText2)
					},
				)
			})
			return layout.Dimensions{Size: image.Pt(bw, h)}
		})
	})
}

// dateTile is the 64dp tile with the month over the day.
func dateTile(gtx layout.Context, c spotify.Concert) {
	s := ui.Dp(gtx, 64)
	ui.FillRect(gtx.Ops, image.Rect(0, 0, s, s), ui.Dp(gtx, 4), ui.Alpha(ui.ColText, .1))
	line := func(txt string, size float32, y int) {
		m := op.Record(gtx.Ops)
		d := ui.Text(gtx, txt, size, ui.Bold, ui.ColText)
		call := m.Stop()
		ui.Offset(gtx, (s-d.Size.X)/2, y, func() { call.Add(gtx.Ops) })
	}
	line(c.Start.Format("Jan"), 14, ui.Dp(gtx, 7))
	line(fmt.Sprint(c.Start.Day()), 24, ui.Dp(gtx, 29))
}

// merchSection is the "Merch" heading and the shop's items.
func (p *page) merchSection(gtx layout.Context) layout.Dimensions {
	a := p.res.V
	if len(p.merch) < len(a.Merch) {
		p.merch = make([]ui.Click, len(a.Merch))
	}
	for i := range a.Merch {
		if p.merch[i].Clicked(gtx) {
			backend.OpenURL(a.Merch[i].URL)
		}
	}
	rows := []layout.Widget{func(gtx layout.Context) layout.Dimensions {
		return ui.Pad(gtx, 0, 0, 16, 0, func(gtx layout.Context) layout.Dimensions {
			return ui.Text(gtx, "Merch", 24, ui.Bold, ui.ColText)
		})
	}}
	for i, m := range a.Merch {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return merchRow(gtx, &p.merch[i], m) })
	}
	return ui.Pad(gtx, 0, 24, sectionGap(true), 24, func(gtx layout.Context) layout.Dimensions { return ui.Column(gtx, rows...) })
}

// sectionGap is the space under About or Merch: a shelf below starts with a
// 19dp margin above its title that ours lacks, while another section follows
// directly.
func sectionGap(beforeShelf bool) float32 {
	if beforeShelf {
		return 19
	}
	return 0
}

// merchRow is a 64dp row: picture, name and description, and a link-out icon.
func merchRow(gtx layout.Context, click *ui.Click, m spotify.Merch) layout.Dimensions {
	pad, w, h := ui.Dp(gtx, 8), gtx.Constraints.Max.X, ui.Dp(gtx, 64)
	click.Label = m.Name
	ui.Offset(gtx, -pad, 0, func() {
		click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			bw := w + 2*pad
			if click.Hovered() {
				ui.FillRect(gtx.Ops, image.Rect(0, 0, bw, h), ui.Dp(gtx, 4), ui.Alpha(ui.ColText, .1))
			}
			is := ui.Dp(gtx, 48)
			ui.Offset(gtx, pad, pad, func() {
				ui.Image{URL: ui.Pick(m.Images, is), Radius: ui.Dp(gtx, 4)}.Layout(gtx, image.Pt(is, is))
			})
			ix := ui.Dp(gtx, 16)
			g := gtx
			g.Constraints.Max.X = max(0, bw-pad-is-2*ix-ui.Dp(gtx, 16)-pad)
			ui.Offset(gtx, pad+is+ix, ui.Dp(gtx, 11), func() {
				ui.Column(g,
					func(gtx layout.Context) layout.Dimensions { return ui.Text(gtx, m.Name, 16, ui.Bold, ui.ColText) },
					func(gtx layout.Context) layout.Dimensions {
						return ui.Text(gtx, m.Description, 14, ui.Regular, ui.Alpha(ui.ColText, .7))
					},
				)
			})
			ui.Offset(gtx, bw-pad-ui.Dp(gtx, 16), (h-ui.Dp(gtx, 16))/2, func() {
				ui.IconPx(gtx, "external", ui.Dp(gtx, 16), ui.Alpha(ui.ColText, .5))
			})
			return layout.Dimensions{Size: image.Pt(bw, h)}
		})
	})
	return layout.Dimensions{Size: image.Pt(w, h)}
}
