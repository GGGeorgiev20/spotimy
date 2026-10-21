package ui

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// The recent-searches dropdown that opens under the top bar's search box
// while it's focused and empty, like the web player's (the /search page itself
// only browses categories).
var drop struct {
	res   Res[[]spotify.Card]
	rows  []dropRow
	clear Pill
	// rect is where the dropdown was drawn (window px); a press inside it
	// keeps the search box focused so the click lands.
	rect image.Rectangle
	tag  int
}

type dropRow struct {
	click  Click
	remove IconButton
}

const dropMax = 10

// RememberSearch saves an opened or played result as a recent search.
func RememberSearch(uri string) {
	Async(func() (struct{}, error) { return struct{}{}, B.SaveRecentSearch(uri) }, func(struct{}, error) {
		drop.res = Res[[]spotify.Card]{} // reload next time the dropdown opens
	})
}

func forgetSearches(uris []string) {
	Go("Couldn't remove the recent search", func() error { return B.RemoveRecentSearches(uris) }, func() {
		gone := map[string]bool{}
		for _, u := range uris {
			gone[u] = true
		}
		keep := drop.res.V[:0:0]
		for _, c := range drop.res.V {
			if !gone[c.URI] {
				keep = append(keep, c)
			}
		}
		drop.res.V = keep
	})
}

// layoutSearchDropdown draws the dropdown under box (the search box's window
// rect) when it should be open.
func layoutSearchDropdown(gtx layout.Context, box image.Rectangle) {
	drop.rect = image.Rectangle{}
	if !searchFocused || SearchQuery.Len() > 0 {
		return
	}
	drop.res.Once(func() ([]spotify.Card, error) { return B.RecentSearches() })
	items := drop.res.V
	if len(items) > dropMax {
		items = items[:dropMax]
	}
	if len(items) == 0 {
		return
	}
	if len(drop.rows) < len(items) {
		drop.rows = make([]dropRow, len(items))
	}
	for i, c := range items {
		r := &drop.rows[i]
		if r.remove.Click.Clicked(gtx) {
			forgetSearches([]string{c.URI})
			return
		}
		if r.click.Clicked(gtx) {
			SearchQuery.Blur(gtx)
			RememberSearch(c.URI)
			OpenURI(c.URI)
			return
		}
	}
	if drop.clear.Click.Clicked(gtx) {
		uris := make([]string, len(drop.res.V))
		for i, c := range drop.res.V {
			uris[i] = c.URI
		}
		forgetSearches(uris)
		return
	}

	w := box.Dx()
	pad, rowH := Dp(gtx, 8), Dp(gtx, 64)
	m := op.Record(gtx.Ops)
	y := pad
	Offset(gtx, pad, y, func() {
		Pad(gtx, 8, 8, 4, 8, func(gtx layout.Context) layout.Dimensions {
			d := Text(gtx, "Recent searches", 16, Bold, ColText)
			y += d.Size.Y + Dp(gtx, 12)
			return d
		})
	})
	rowBg := Mix(ColSurface3, ColText, .1)
	for i, c := range items {
		r := &drop.rows[i]
		r.click.Label = c.Name
		Offset(gtx, pad, y, func() {
			rw := w - 2*pad
			r.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if r.click.Hovered() || r.remove.Click.Hovered() {
					FillRect(gtx.Ops, image.Rect(0, 0, rw, rowH), Dp(gtx, 6), rowBg)
				}
				is := Dp(gtx, 48)
				Offset(gtx, Dp(gtx, 8), (rowH-is)/2, func() {
					round := c.Type == "artist" || c.Type == "user"
					radius := Dp(gtx, 4)
					if round {
						radius = is / 2
					}
					Image{URL: Pick(c.Images, is), Radius: radius, Round: round, Icon: cardIcon(c.Type)}.Layout(gtx, image.Pt(is, is))
				})
				tx := Dp(gtx, 8+48+12)
				tw := rw - tx - Dp(gtx, 4+32+12)
				detail := c.Detail
				if detail == "" {
					detail = c.Subtitle
				}
				m := op.Record(gtx.Ops)
				g := gtx
				g.Constraints = layout.Constraints{Max: image.Pt(max(0, tw), rowH)}
				nd := Text(g, c.Name, 16, Regular, ColText)
				var dd layout.Dimensions
				Offset(gtx, 0, nd.Size.Y, func() { dd = Text(g, detail, 13, Regular, ColText2) })
				call := m.Stop()
				Offset(gtx, tx, (rowH-nd.Size.Y-dd.Size.Y)/2, func() { call.Add(gtx.Ops) })
				Offset(gtx, rw-Dp(gtx, 4+32), (rowH-Dp(gtx, 32))/2, func() {
					r.remove.Layout(gtx, IconStyle{Icon: "close", Size: 16, Box: 32, Label: "Remove " + c.Name})
				})
				return layout.Dimensions{Size: image.Pt(rw, rowH)}
			})
		})
		y += rowH
	}
	Offset(gtx, pad+Dp(gtx, 8), y+Dp(gtx, 8), func() {
		d := drop.clear.Layout(gtx, PillStyle{Text: "Clear recent searches", Size: 13, Weight: Bold, Height: 32, PadX: 15, Outline: true})
		y += d.Size.Y + 2*Dp(gtx, 8)
	})
	y += pad
	call := m.Stop()

	r := image.Rect(box.Min.X, box.Max.Y+Dp(gtx, 4), box.Max.X, box.Max.Y+Dp(gtx, 4)+y)
	drop.rect = r
	Offset(gtx, r.Min.X, r.Min.Y, func() {
		rr := image.Rect(0, 0, w, y)
		popupShadow(gtx, rr)
		FillRect(gtx.Ops, rr, Dp(gtx, 8), ColSurface3)
		// Swallow presses on the background so they don't reach the page.
		st := clip.Rect(rr).Push(gtx.Ops)
		event.Op(gtx.Ops, &drop.tag)
		for {
			if _, ok := gtx.Event(pointer.Filter{Target: &drop.tag, Kinds: pointer.Press}); !ok {
				break
			}
		}
		st.Pop()
		call.Add(gtx.Ops)
	})
}

// inSearchDropdown reports whether p (window px) is on the open dropdown.
func inSearchDropdown(p image.Point) bool { return p.In(drop.rect) }
