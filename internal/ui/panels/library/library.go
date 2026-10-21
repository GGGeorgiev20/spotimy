// Package library is the left "Your Library" panel: filter chips, search,
// sort and view menu, and the virtualized list of the user's library.
package library

import (
	"image"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

const (
	pageSize  = 50
	maxW      = 600 // the widest the shell lets the panel get ("expanded")
	typedWait = 250 * time.Millisecond
	tipWait   = 375 * time.Millisecond
	headerH   = 59 // 16 above, 35 buttons, 8 below
)

func init() { ui.LibraryPanel = layoutPanel }

// page is one query's items, loaded page by page as they scroll into view.
type page struct {
	items []*spotify.LibraryItem // nil = not loaded yet
	query spotify.LibraryQuery
	asked map[int]bool // page numbers requested
}

type row struct {
	click, play ui.Click
	drop        ui.DropTarget // songs dragged onto Liked Songs or a playlist
}

// libraryGroup names the library's drop targets.
const libraryGroup = "library"

// takesSongs reports whether songs dropped on it can go in: Liked Songs and
// the playlists the user can edit.
func takesSongs(it *spotify.LibraryItem) bool {
	switch it.Type {
	case "collection":
		return it.URI == "spotify:collection:tracks"
	case "playlist":
		return ui.CanAddTo(it.URI)
	}
	return false
}

// dropSongs adds dropped songs to it.
func dropSongs(it *spotify.LibraryItem, pl *ui.DragPayload) {
	if it.Type == "collection" {
		ui.SaveAll(pl.Tracks)
		return
	}
	uris := make([]string, len(pl.Tracks))
	for i, t := range pl.Tracks {
		uris[i] = t.URI
	}
	ui.AddToPlaylist(it.Card, uris)
}

var st struct {
	filter  string
	folders []spotify.LibraryItem // open folder trail
	raw     string                // search text as typed
	text    string                // search text sent to the backend (debounced)
	typedAt time.Time

	search    ui.Input
	searching bool
	focus     bool // grab keyboard focus on the next frame
	seen      bool // the field has had focus since it opened

	key  string
	ver  int
	gen  int
	res  *page
	err  error
	list ui.List

	filters, sorts []string
	rows           []row
	cards          []ui.Card
	chips          []ui.Pill
	clearChip      ui.Click
	back           ui.IconButton
	clearText      ui.IconButton
	searchBtn      ui.Click
	sortBtn        ui.Click
	title          ui.Click
	titleHover     ui.Hover
	create, expand ui.Click
	createPill     ui.Pill
	viewBtns       [3]ui.Click
	restoreW       int

	listTop  int
	tipFor   *ui.Click
	tipSince time.Time
}

func query() spotify.LibraryQuery {
	q := spotify.LibraryQuery{Sort: ui.Prefs.LibSort, TextFilter: st.text}
	if st.filter != "" {
		q.Filters = []string{st.filter}
	}
	if n := len(st.folders); n > 0 {
		q.FolderURI = st.folders[n-1].URI
	}
	return q
}

// sync reloads the list when the query or the library changed.
func sync(gtx layout.Context) {
	if st.raw != st.text {
		if at := st.typedAt.Add(typedWait); st.raw != "" && gtx.Now.Before(at) {
			gtx.Execute(op.InvalidateCmd{At: at})
		} else {
			st.text = st.raw
		}
	}
	q := query()
	key := q.Sort + "\x00" + q.TextFilter + "\x00" + q.FolderURI + "\x00" + st.filter
	if key == st.key && ui.LibraryVersion == st.ver {
		return
	}
	if key != st.key {
		st.list.ScrollToTop()
	}
	st.key, st.ver = key, ui.LibraryVersion
	st.gen++
	gen := st.gen
	q.Limit = pageSize
	ui.Async(func() (*spotify.Library, error) { return ui.B.Library(q) }, func(l *spotify.Library, err error) {
		if gen != st.gen {
			return
		}
		if st.err = err; err != nil {
			return
		}
		p := &page{items: make([]*spotify.LibraryItem, l.Total), query: q, asked: map[int]bool{0: true}}
		p.put(0, l.Items)
		st.res = p
		if len(q.Filters) == 0 && q.TextFilter == "" && q.FolderURI == "" {
			st.filters = l.Filters
		}
		if len(l.Sorts) > 0 {
			st.sorts = l.Sorts
		}
		st.rows = make([]row, l.Total)
		st.cards = make([]ui.Card, l.Total)
	})
}

func (p *page) put(off int, items []spotify.LibraryItem) {
	for i := range items {
		if off+i < len(p.items) {
			p.items[off+i] = &items[i]
		}
	}
}

// ensure loads the pages covering items [start, end).
func (p *page) ensure(start, end int) {
	end = min(end, len(p.items))
	for pg := max(0, start) / pageSize; pg*pageSize < end; pg++ {
		if p.asked[pg] {
			continue
		}
		p.asked[pg] = true
		q := p.query
		q.Offset, q.Limit = pg*pageSize, pageSize
		ui.Async(func() (*spotify.Library, error) { return ui.B.Library(q) }, func(l *spotify.Library, err error) {
			if err != nil {
				delete(p.asked, pg)
				return
			}
			p.put(q.Offset, l.Items)
		})
	}
}

// gridCols is how many 160dp cards fit in the grid view of a w-wide panel.
func gridCols(w, dp int) int { return max(1, (w-16*dp)/(160*dp)) }

// subtitle is the second line of a row: "Playlist • Owner".
func subtitle(it *spotify.LibraryItem) string {
	switch {
	case it.Type == "collection" && it.Count > 0:
		return "Playlist • " + ui.FmtNumber(it.Count) + " songs"
	case it.Type == "folder":
		return "Folder • " + it.Subtitle
	}
	return it.Subtitle
}

// compactSub is what follows the name in the compact view: the creator.
func compactSub(it *spotify.LibraryItem) string {
	s := subtitle(it)
	if _, after, ok := strings.Cut(s, " • "); ok {
		return after
	}
	return s
}

// ---- Layout ----

// origin converts panel coordinates to window coordinates: the shell puts the
// panel 8dp from the left and below the 64dp top bar.
func origin(gtx layout.Context, x, y int) image.Point {
	return image.Pt(ui.Dp(gtx, 8)+x, ui.Dp(gtx, 64)+y)
}

func place(gtx layout.Context, x, y, w, h int, f func(layout.Context) layout.Dimensions) layout.Dimensions {
	var d layout.Dimensions
	ui.Offset(gtx, x, y, func() {
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(w, h)}
		d = f(g)
	})
	return d
}

func layoutPanel(gtx layout.Context, collapsed bool) layout.Dimensions {
	size := gtx.Constraints.Max
	collapsed = collapsed || size.X < ui.Dp(gtx, 240) // the shell squeezed it: show the rail
	sync(gtx)
	y := 0
	if collapsed {
		y = railHeader(gtx, size.X)
	} else {
		y = controls(gtx, size.X, header(gtx, size.X))
	}
	st.listTop, st.list.NoBar = y, collapsed
	place(gtx, 0, y, size.X, size.Y-y, func(gtx layout.Context) layout.Dimensions {
		return layoutList(gtx, collapsed)
	})
	return layout.Dimensions{Size: size}
}

// roundButton is a 35dp circle with an icon: filled (Create) or bare (Expand).
func roundButton(gtx layout.Context, c *ui.Click, icon string, filled bool, tip string) layout.Dimensions {
	c.Label = tip
	d := c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		s := ui.Dp(gtx, 35)
		hov := c.Hovered()
		if filled {
			bg := ui.ColSurface2
			if hov {
				bg = ui.ColSurface3
			}
			ui.FillRect(gtx.Ops, image.Rect(0, 0, s, s), s/2, bg)
		}
		col := ui.ColText2
		if hov {
			col = ui.ColText
		}
		is := ui.Dp(gtx, 16)
		ui.Offset(gtx, (s-is)/2, (s-is)/2, func() { ui.IconPx(gtx, icon, is, col) })
		return layout.Dimensions{Size: image.Pt(s, s)}
	})
	ui.Tip(gtx, c, d, tip, true)
	return d
}

func createMenu(gtx layout.Context, x, y int) {
	ui.OpenMenuAt(origin(gtx, x, y), []ui.MenuItem{{Label: "Playlist", Icon: "playlist", Action: func() { ui.CreatePlaylist(nil) }}})
}

// railTip shows the collapsed rail's tooltip to the right of the rail.
func railTip(gtx layout.Context, c *ui.Click, title, sub string, y int) {
	if !c.Hovered() {
		if st.tipFor == c {
			st.tipFor = nil
		}
		return
	}
	if st.tipFor != c {
		st.tipFor, st.tipSince = c, gtx.Now
	}
	if at := st.tipSince.Add(tipWait); gtx.Now.Before(at) {
		gtx.Execute(op.InvalidateCmd{At: at})
		return
	}
	ui.ShowTip(title, sub, origin(gtx, ui.Dp(gtx, 72+4), y))
}

func railHeader(gtx layout.Context, w int) int {
	if st.title.Clicked(gtx) {
		ui.Prefs.LibCollapsed = false
	}
	if st.create.Clicked(gtx) {
		createMenu(gtx, (w-ui.Dp(gtx, 35))/2, ui.Dp(gtx, 64+35+4))
	}
	place(gtx, (w-ui.Dp(gtx, 40))/2, ui.Dp(gtx, 16), w, ui.Dp(gtx, 40), func(gtx layout.Context) layout.Dimensions {
		st.title.Label = "Expand Your Library"
		return st.title.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			s, is := ui.Dp(gtx, 40), ui.Dp(gtx, 24)
			col := ui.ColText2
			if st.title.Hovered() {
				col = ui.ColText
			}
			ui.Offset(gtx, (s-is)/2, (s-is)/2, func() { ui.IconPx(gtx, "library", is, col) })
			return layout.Dimensions{Size: image.Pt(s, s)}
		})
	})
	railTip(gtx, &st.title, "Expand Your Library", "", ui.Dp(gtx, 36))
	place(gtx, (w-ui.Dp(gtx, 35))/2, ui.Dp(gtx, 64), w, ui.Dp(gtx, 35), func(gtx layout.Context) layout.Dimensions {
		return roundButton(gtx, &st.create, "plus", true, "Create")
	})
	return ui.Dp(gtx, 64+35+8)
}

func header(gtx layout.Context, w int) int {
	pad, top := ui.Dp(gtx, 16), ui.Dp(gtx, 16)
	if st.title.Clicked(gtx) {
		ui.Prefs.LibCollapsed = true
	}
	if st.expand.Clicked(gtx) {
		if ui.Prefs.LibWidth >= maxW {
			ui.Prefs.LibWidth = 340
			if st.restoreW > 0 {
				ui.Prefs.LibWidth = min(maxW-1, st.restoreW)
			}
		} else {
			st.restoreW, ui.Prefs.LibWidth = ui.Prefs.LibWidth, maxW
		}
	}
	bw := ui.Dp(gtx, 35)
	expanded := ui.Prefs.LibWidth >= maxW
	exTip, exIcon := "Expand Your Library", "maximize"
	if expanded {
		exTip = "Minimize Your Library"
	}
	ex := w - pad - bw
	place(gtx, ex, top, bw, bw, func(gtx layout.Context) layout.Dimensions {
		return roundButton(gtx, &st.expand, exIcon, false, exTip)
	})

	// Create: icon only while narrow, "+ Create" once there is room.
	cx := ex - ui.Dp(gtx, 8)
	wide := w >= ui.Dp(gtx, 400)
	if st.create.Clicked(gtx) || st.createPill.Click.Clicked(gtx) {
		createMenu(gtx, cx-bw, top+bw+ui.Dp(gtx, 4))
	}
	if wide {
		place(gtx, 0, top, cx, bw, func(gtx layout.Context) layout.Dimensions {
			pw := ui.Measure(gtx, "Create", 14, ui.Bold) + ui.Dp(gtx, 32+24)
			ui.Offset(gtx, cx-pw, 0, func() {
				g := gtx
				g.Constraints.Max.X = pw
				st.createPill.Layout(g, ui.PillStyle{Text: "Create", Icon: "plus", Weight: ui.Bold, Height: 35, Bg: ui.ColSurface2, BgHover: ui.ColSurface3, Fg: ui.ColText2})
			})
			return layout.Dimensions{}
		})
	} else {
		place(gtx, cx-bw, top, bw, bw, func(gtx layout.Context) layout.Dimensions {
			return roundButton(gtx, &st.create, "plus", true, "Create")
		})
	}

	// Title: the collapse icon slides in from the left on hover.
	const label = "Your Library"
	v := st.titleHover.Step(gtx, st.title.Hovered(), 200*time.Millisecond)
	base := -ui.Dp(gtx, 8) + int(float32(ui.Dp(gtx, 24))*v)
	tx := base + ui.Dp(gtx, 28)
	h := ui.Dp(gtx, 30)
	place(gtx, 0, top+(bw-h)/2, w, h, func(gtx layout.Context) layout.Dimensions {
		st.title.Label = "Collapse Your Library"
		d := st.title.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			is := ui.Dp(gtx, 16)
			ui.Offset(gtx, base, (h-is)/2, func() { ui.IconPx(gtx, "collapse", is, ui.Alpha(ui.ColText2, v)) })
			ui.Offset(gtx, tx, 0, func() {
				g := gtx
				g.Constraints = layout.Constraints{Min: image.Pt(0, h), Max: image.Pt(max(0, cx-bw-tx), h)}
				layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions {
					return ui.Text(gtx, label, 16, ui.Bold, ui.ColText)
				})
			})
			return layout.Dimensions{Size: image.Pt(tx+ui.Measure(gtx, label, 16, ui.Bold), h)}
		})
		ui.Tip(gtx, &st.title, d, "Collapse Your Library", true)
		return d
	})
	return ui.Dp(gtx, headerH)
}

// controls draws the filter chips (or a folder's back bar) and the search/sort
// row, and returns where the list starts.
func controls(gtx layout.Context, w, y int) int {
	dp8 := ui.Dp(gtx, 8)
	if len(st.folders) > 0 || len(st.filters) > 0 {
		place(gtx, 0, y+dp8, w, ui.Dp(gtx, 32), func(gtx layout.Context) layout.Dimensions { return chipRow(gtx, w) })
		y += ui.Dp(gtx, 40)
	}
	y += dp8 + ui.Dp(gtx, 2)
	place(gtx, 0, y, w, ui.Dp(gtx, 32), func(gtx layout.Context) layout.Dimensions { return tools(gtx, w, y) })
	return y + ui.Dp(gtx, 32)
}

func chipRow(gtx layout.Context, w int) layout.Dimensions {
	x := ui.Dp(gtx, 16)
	h := ui.Dp(gtx, 32)
	if n := len(st.folders); n > 0 {
		if st.back.Click.Clicked(gtx) {
			st.folders = st.folders[:n-1]
		}
		ui.Offset(gtx, ui.Dp(gtx, 8), 0, func() {
			st.back.Layout(gtx, ui.IconStyle{Icon: "chevron-left", Size: 16, Label: "Go back"})
		})
		ui.Offset(gtx, ui.Dp(gtx, 48), 0, func() {
			g := gtx
			g.Constraints = layout.Constraints{Min: image.Pt(0, h), Max: image.Pt(w-ui.Dp(gtx, 64), h)}
			layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, st.folders[n-1].Name, 16, ui.Bold, ui.ColText)
			})
		})
		return layout.Dimensions{Size: image.Pt(w, h)}
	}
	if len(st.chips) < len(st.filters) {
		st.chips = append(st.chips, make([]ui.Pill, len(st.filters)-len(st.chips))...)
	}
	if st.filter != "" {
		if st.clearChip.Clicked(gtx) {
			st.filter = ""
		}
		st.clearChip.Label = "Clear filters"
		ui.Offset(gtx, x, 0, func() {
			st.clearChip.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				bg := ui.ColSurface3
				if st.clearChip.Hovered() {
					bg = ui.ColHighlight
				}
				ui.FillRect(gtx.Ops, image.Rect(0, 0, h, h), h/2, bg)
				is := ui.Dp(gtx, 16)
				ui.Offset(gtx, (h-is)/2, (h-is)/2, func() { ui.IconPx(gtx, "close", is, ui.ColText) })
				return layout.Dimensions{Size: image.Pt(h, h)}
			})
		})
		x += h + ui.Dp(gtx, 8)
	}
	for i, f := range st.filters {
		if st.filter != "" && st.filter != f || x >= w {
			continue
		}
		c := &st.chips[i]
		if c.Click.Clicked(gtx) {
			if st.filter == f {
				st.filter = ""
			} else {
				st.filter = f
			}
		}
		d := place(gtx, x, 0, w-x, h, func(gtx layout.Context) layout.Dimensions {
			return c.Layout(gtx, ui.PillStyle{Text: f, PadX: 12, Bg: ui.ColSurface3, BgHover: ui.ColHighlight, Selected: st.filter == f})
		})
		x += d.Size.X + ui.Dp(gtx, 8)
	}
	return layout.Dimensions{Size: image.Pt(w, h)}
}

var views = []struct{ id, label, icon string }{{"compact", "Compact", "compact"}, {"list", "List", "list"}, {"grid", "Grid", "grid"}}

func view() (id, icon string) {
	for _, v := range views {
		if v.id == ui.Prefs.LibView {
			return v.id, v.icon
		}
	}
	return "list", "list"
}

// menuLabel is a section heading of the sort menu.
func menuLabel(text string) ui.MenuItem {
	return ui.MenuItem{Widget: func(gtx layout.Context) layout.Dimensions {
		return ui.Pad(gtx, 12, 8, 12, 12, func(gtx layout.Context) layout.Dimensions {
			return ui.Label{Text: text, Size: 12, Weight: ui.Bold, Color: ui.Alpha(ui.ColText, .7), LineHeight: 17}.Layout(gtx)
		})
	}}
}

// viewSwitch is the segmented control that picks the list view.
func viewSwitch(gtx layout.Context) layout.Dimensions {
	dp := func(v float32) int { return ui.Dp(gtx, v) }
	w, h := gtx.Constraints.Max.X, dp(40)
	ui.FillRect(gtx.Ops, image.Rect(0, 0, w, h), dp(8), ui.ColSurface2)
	cur, _ := view()
	for i, v := range views {
		c := &st.viewBtns[i]
		if c.Clicked(gtx) {
			ui.Prefs.LibView = v.id
			ui.CloseMenu()
		}
		ui.Offset(gtx, dp(4)+i*dp(48), dp(4), func() {
			c.Label = v.label
			c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				col := ui.ColText2
				switch {
				case v.id == cur:
					col = ui.ColAccent
				case c.Hovered():
					col = ui.ColText
				}
				ui.Offset(gtx, dp(16), dp(8), func() { ui.IconPx(gtx, v.icon, dp(16), col) })
				return layout.Dimensions{Size: image.Pt(dp(48), dp(32))}
			})
		})
	}
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func sortMenu() []ui.MenuItem {
	items := []ui.MenuItem{menuLabel("Sort by")}
	sorts := st.sorts
	if len(sorts) == 0 {
		sorts = []string{"Recents", "Recently Added", "Alphabetical", "Creator"}
	}
	for _, s := range sorts {
		items = append(items, ui.MenuItem{Label: s, Check: s == ui.Prefs.LibSort, Accent: s == ui.Prefs.LibSort, Action: func() { ui.Prefs.LibSort = s }})
	}
	return append(items,
		ui.MenuItem{Widget: func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: image.Pt(0, ui.Dp(gtx, 8))} }},
		menuLabel("View as"),
		ui.MenuItem{Widget: viewSwitch})
}

func tools(gtx layout.Context, w, y int) layout.Dimensions {
	h := ui.Dp(gtx, 32)
	// Sort button, right-aligned.
	sort := ui.Prefs.LibSort
	_, icon := view()
	bw := ui.Measure(gtx, sort, 14, ui.Regular) + ui.Dp(gtx, 8+16)
	bx := w - ui.Dp(gtx, 24) - bw
	if st.sortBtn.Clicked(gtx) {
		ui.OpenMenuWide(origin(gtx, bx+bw-ui.Dp(gtx, 208), y+h+ui.Dp(gtx, 8)), 208, sortMenu())
	}
	place(gtx, bx, 0, bw, h, func(gtx layout.Context) layout.Dimensions {
		st.sortBtn.Label = sort + ", " + icon + " view"
		return st.sortBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			col := ui.ColText2
			if st.sortBtn.Hovered() {
				col = ui.ColText
			}
			g := gtx
			g.Constraints = layout.Constraints{Max: image.Pt(bw, h)}
			m := op.Record(gtx.Ops)
			td := ui.Text(g, sort, 14, ui.Regular, col)
			call := m.Stop()
			ui.Offset(gtx, 0, (h-td.Size.Y)/2, func() { call.Add(gtx.Ops) })
			is := ui.Dp(gtx, 16)
			ui.Offset(gtx, bw-is, (h-is)/2, func() { ui.IconPx(gtx, icon, is, col) })
			return layout.Dimensions{Size: image.Pt(bw, h)}
		})
	})

	// Search: a magnifier that opens into a field.
	x0 := ui.Dp(gtx, 16)
	if st.searchBtn.Clicked(gtx) {
		st.searching, st.focus = true, true
	}
	if !st.searching && st.raw == "" {
		place(gtx, x0, 0, h, h, func(gtx layout.Context) layout.Dimensions {
			st.searchBtn.Label = "Search in Your Library"
			return st.searchBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				col := ui.Alpha(ui.ColText, .7)
				if st.searchBtn.Hovered() {
					col = ui.ColText
				}
				is := ui.Dp(gtx, 16)
				ui.Offset(gtx, (h-is)/2, (h-is)/2, func() { ui.IconPx(gtx, "search", is, col) })
				return layout.Dimensions{Size: image.Pt(h, h)}
			})
		})
		return layout.Dimensions{}
	}
	return searchField(gtx, x0, h, min(ui.Dp(gtx, 200), bx-x0-ui.Dp(gtx, 8)))
}

func searchField(gtx layout.Context, x0, h, bw int) layout.Dimensions {
	if changed, _ := st.search.Poll(gtx); changed {
		st.raw, st.typedAt = st.search.Text(), gtx.Now
	}
	if st.focus {
		st.search.Focus(gtx)
		st.focus = false
	}
	switch {
	case st.search.Focused():
		st.seen = true
	case st.seen:
		st.seen, st.searching = false, false
	}
	if st.clearText.Click.Clicked(gtx) {
		st.search.SetText("")
		st.raw, st.text = "", ""
		st.searching = false
	}
	ui.Offset(gtx, x0, 0, func() {
		ui.FillRect(gtx.Ops, image.Rect(0, 0, bw, h), ui.Dp(gtx, 4), ui.ColSurface3)
		is := ui.Dp(gtx, 16)
		ui.Offset(gtx, ui.Dp(gtx, 8), (h-is)/2, func() { ui.IconPx(gtx, "search", is, ui.ColText2) })
		clearW := 0
		if st.raw != "" {
			clearW = h
			ui.Offset(gtx, bw-h, 0, func() {
				st.clearText.Layout(gtx, ui.IconStyle{Icon: "close", Size: 16, Box: 32, Label: "Clear search"})
			})
		}
		ix := ui.Dp(gtx, 32)
		ui.Offset(gtx, ix, ui.Dp(gtx, 7), func() {
			g := gtx
			g.Constraints = layout.Constraints{Min: image.Pt(bw-ix-clearW, 0), Max: image.Pt(bw-ix-clearW, h)}
			st.search.Layout(g, ui.InputStyle{Hint: "Search in Your Library"})
		})
	})
	return layout.Dimensions{}
}

// ---- List ----

func layoutList(gtx layout.Context, collapsed bool) layout.Dimensions {
	size := gtx.Constraints.Max
	p := st.res
	switch {
	case p == nil && st.err != nil:
		return ui.Pad(gtx, 16, 16, 0, 16, func(gtx layout.Context) layout.Dimensions {
			return ui.Label{Text: "Couldn't load Your Library.", Color: ui.ColText2, MaxLines: 2}.Layout(gtx)
		})
	case p == nil:
		return ui.Loading(gtx)
	case len(p.items) == 0 && st.text != "":
		return noResults(gtx)
	}
	mode, _ := view()
	if collapsed {
		mode = "list"
	}
	cols := 1
	if mode == "grid" {
		cols = gridCols(size.X, ui.Dp(gtx, 1))
	}
	n := (len(p.items) + cols - 1) / cols
	d := st.list.Layout(gtx, n, func(gtx layout.Context, i int) layout.Dimensions {
		top, bottom := 0, 0
		if i == 0 {
			top = ui.Dp(gtx, 8)
		}
		if i == n-1 {
			bottom = ui.Dp(gtx, 8)
		}
		var d layout.Dimensions
		ui.Offset(gtx, 0, top, func() {
			if cols > 1 {
				d = gridRow(gtx, p, i, cols)
			} else {
				d = listRow(gtx, p.items[i], i, collapsed, mode == "compact")
			}
		})
		d.Size.Y += top + bottom
		return d
	})
	first, last := st.list.Range()
	p.ensure(first*cols, (last+1)*cols+pageSize)
	return d
}

func noResults(gtx layout.Context) layout.Dimensions {
	return ui.Pad(gtx, 32, 24, 0, 24, func(gtx layout.Context) layout.Dimensions {
		return ui.Column(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return ui.Label{Text: `Couldn't find "` + st.text + `"`, Size: 16, Weight: ui.Bold, MaxLines: 2}.Layout(gtx)
			},
			func(gtx layout.Context) layout.Dimensions {
				return ui.Pad(gtx, 8, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
					return ui.Label{Text: "Try searching again using a different spelling or keyword.", Color: ui.ColText2, MaxLines: 3}.Layout(gtx)
				})
			},
		)
	})
}

func open(it *spotify.LibraryItem) {
	if it.Type == "folder" {
		st.folders = append(st.folders, *it)
		return
	}
	ui.OpenURI(it.URI)
}

func menuFor(it *spotify.LibraryItem) []ui.MenuItem {
	own := it.Type == "playlist" && ui.Me != nil && it.Owner == ui.Me.Name
	return ui.CardMenu(it.Card, ui.CardMenuOpts{Library: true, Pinned: it.Pinned, Own: own})
}

func iconFor(typ string) string {
	switch typ {
	case "artist":
		return "user"
	case "album":
		return "album"
	case "folder":
		return "library"
	}
	return "note"
}

// thumb draws the 48dp cover; Liked Songs gets its gradient and heart.
func thumb(gtx layout.Context, it *spotify.LibraryItem, px int) {
	radius := ui.Dp(gtx, 4)
	if it.Type == "collection" && len(it.Images) == 0 {
		defer clip.UniformRRect(image.Rect(0, 0, px, px), radius).Push(gtx.Ops).Pop()
		paint.LinearGradientOp{Stop1: f32.Pt(0, 0), Color1: ui.RGB(0x450af5), Stop2: f32.Pt(float32(px), float32(px)), Color2: ui.RGB(0xc4efd9)}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		is := px * 2 / 5
		ui.Offset(gtx, (px-is)/2, (px-is)/2, func() { ui.IconPx(gtx, "heart-filled", is, ui.ColText) })
		return
	}
	ui.Image{URL: ui.Pick(it.Images, px), Radius: radius, Round: it.Type == "artist", Icon: iconFor(it.Type)}.Layout(gtx, image.Pt(px, px))
}

func playIcon(gtx layout.Context, it *spotify.LibraryItem, px int, round bool) {
	r := ui.Dp(gtx, 4)
	if round {
		r = px / 2
	}
	ui.FillRect(gtx.Ops, image.Rect(0, 0, px, px), r, ui.Alpha(ui.ColBg, .5))
	name := "play"
	if ui.CtxPlaying(it.URI) {
		name = "pause"
	}
	is := ui.Dp(gtx, 24)
	ui.Offset(gtx, (px-is)/2, (px-is)/2, func() { ui.IconPx(gtx, name, is, ui.ColText) })
}

// listRow is one row of the list, compact and rail views.
func listRow(gtx layout.Context, it *spotify.LibraryItem, i int, collapsed, compact bool) layout.Dimensions {
	w := gtx.Constraints.Max.X
	h := ui.Dp(gtx, 64)
	if compact && !collapsed {
		h = ui.Dp(gtx, 32)
	}
	if it == nil {
		return layout.Dimensions{Size: image.Pt(w, h)}
	}
	r := &st.rows[i]
	folder := it.Type == "folder"
	if r.play.Clicked(gtx) {
		ui.PlayContext(it.URI)
	}
	if r.click.Clicked(gtx) {
		open(it)
	}
	if r.click.DoubleClicked(gtx) && !folder {
		ui.Play(it.URI, "")
	}
	if _, ok := r.click.RightClicked(gtx); ok {
		if items := menuFor(it); len(items) > 0 {
			ui.OpenMenu(items)
		}
	}
	px := ui.Dp(gtx, 8)
	if collapsed {
		px = ui.Dp(gtx, 4)
	}
	rw := w - 2*px
	takes := takesSongs(it)
	spot, ok := ui.DropHover(libraryGroup)
	dropOver := takes && ok && spot == i
	ui.Offset(gtx, px, 0, func() {
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(rw, h)}
		r.click.Label = it.Name
		r.click.Layout(g, func(gtx layout.Context) layout.Dimensions {
			hov := r.click.Hovered()
			switch {
			case dropOver: // a drag over it: outlined in green
				rad := ui.Dp(gtx, 6)
				b := max(1, ui.Dp(gtx, 2))
				ui.FillRect(gtx.Ops, image.Rect(0, 0, rw, h), rad, ui.ColAccent)
				ui.FillRect(gtx.Ops, image.Rect(b, b, rw-b, h-b), rad-b, ui.ColSurface2)
			case !folder && ui.Current().Path == ui.URIToPath(it.URI): // the open page is tinted
				a := float32(.1)
				if hov {
					a = .14
				}
				ui.FillRect(gtx.Ops, image.Rect(0, 0, rw, h), ui.Dp(gtx, 6), ui.Alpha(ui.ColText, a))
			case hov:
				ui.FillRect(gtx.Ops, image.Rect(0, 0, rw, h), ui.Dp(gtx, 6), ui.ColSurface2)
			}
			pad := ui.Dp(gtx, 8)
			tx := pad
			if !compact || collapsed {
				ts := ui.Dp(gtx, 48)
				tx = pad + ts + ui.Dp(gtx, 12)
				x := pad
				if collapsed {
					x = (rw - ts) / 2
				}
				ui.Offset(gtx, x, (h-ts)/2, func() {
					if folder {
						thumb(gtx, it, ts)
						return
					}
					r.play.Label = "Play " + it.Name
					r.play.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						thumb(gtx, it, ts)
						if hov {
							playIcon(gtx, it, ts, it.Type == "artist")
						}
						return layout.Dimensions{Size: image.Pt(ts, ts)}
					})
				})
			}
			if collapsed {
				return layout.Dimensions{Size: image.Pt(rw, h)}
			}
			right := rw - pad
			if ui.IsContext(it.URI) && ui.Playing() {
				is := ui.Dp(gtx, 16)
				ui.Offset(gtx, right-is, (h-is)/2, func() { ui.IconPx(gtx, "volume", is, ui.ColAccent) })
				right -= is + pad
			}
			g := gtx
			g.Constraints = layout.Constraints{Max: image.Pt(max(0, right-tx), h)}
			nameCol := ui.ColText
			if ui.IsContext(it.URI) {
				nameCol = ui.ColAccent
			}
			if compact {
				m := op.Record(gtx.Ops)
				d := ui.Inline(g, 14, ui.Regular, ui.ColText2, ui.ColNone,
					ui.Seg{Text: it.Name, Color: nameCol}, ui.Seg{Text: " • " + compactSub(it)})
				call := m.Stop()
				ui.Offset(gtx, tx, (h-d.Size.Y)/2, func() { call.Add(gtx.Ops) })
				return layout.Dimensions{Size: image.Pt(rw, h)}
			}
			m := op.Record(gtx.Ops)
			nd := ui.Text(g, it.Name, 16, ui.Regular, nameCol)
			name := m.Stop()
			m = op.Record(gtx.Ops)
			sd := subLine(g, it)
			sub := m.Stop()
			y := (h - (nd.Size.Y + ui.Dp(gtx, 2) + sd.Size.Y)) / 2
			ui.Offset(gtx, tx, y, func() { name.Add(gtx.Ops) })
			ui.Offset(gtx, tx, y+nd.Size.Y+ui.Dp(gtx, 2), func() { sub.Add(gtx.Ops) })
			return layout.Dimensions{Size: image.Pt(rw, h)}
		})
		if takes {
			r.drop.Group = libraryGroup
			if pl, _ := r.drop.Add(gtx, image.Pt(rw, h), i); pl != nil {
				dropSongs(it, pl)
			}
		}
		if collapsed {
			// Rail rows are all 64dp, after the list's 8dp top padding.
			railTip(gtx, &r.click, it.Name, subtitle(it), st.listTop+ui.Dp(gtx, 8)+i*h+h/2-st.list.ScrollTop())
		}
	})
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// subLine is "Playlist • Owner", led by the green pin for pinned items.
func subLine(gtx layout.Context, it *spotify.LibraryItem) layout.Dimensions {
	var segs []ui.Seg
	if it.Pinned {
		segs = append(segs, ui.Seg{Widget: func(gtx layout.Context) layout.Dimensions {
			s := ui.Dp(gtx, 12)
			ui.IconPx(gtx, "pin", s, ui.ColAccent)
			return layout.Dimensions{Size: image.Pt(s+ui.Dp(gtx, 4), s)}
		}})
	}
	segs = append(segs, ui.Seg{Text: subtitle(it)})
	return ui.Inline(gtx, 14, ui.Regular, ui.ColText2, ui.ColNone, segs...)
}

// gridRow is one row of cards in the grid view.
func gridRow(gtx layout.Context, p *page, ri, cols int) layout.Dimensions {
	w := gtx.Constraints.Max.X
	pad := ui.Dp(gtx, 8)
	cw := (w - 2*pad) / cols
	h := 0
	for c := 0; c < cols && ri*cols+c < len(p.items); c++ {
		i := ri*cols + c
		it := p.items[i]
		if it == nil {
			continue
		}
		ui.Offset(gtx, pad+c*cw, 0, func() {
			g := gtx
			g.Constraints = layout.Constraints{Min: image.Pt(cw, 0), Max: image.Pt(cw, 1<<20)}
			o := ui.CardOpts{Menu: func() []ui.MenuItem { return menuFor(it) }}
			if it.Type == "folder" {
				o.NoNavigate, o.OnOpen = true, func(spotify.Card) { open(it) }
			}
			c := it.Card
			c.Subtitle = subtitle(it)
			d := st.cards[i].Layout(g, c, o)
			h = max(h, d.Size.Y)
		})
	}
	return layout.Dimensions{Size: image.Pt(w, max(h, cw))}
}
