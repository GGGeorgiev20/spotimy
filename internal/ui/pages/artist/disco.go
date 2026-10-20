package artist

import (
	"fmt"
	"image"
	"sort"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

const discoPage = 50

// The sort and view are remembered while the app runs, like the web player's.
var (
	discoByName   bool // false: Release date
	discoGridView bool // false: List
)

// disco is /artist/<id>/discography/<kind>: every release as a list (each with
// its tracks) or a grid of covers, loaded a page at a time while scrolling.
type disco struct {
	id, kind string
	artist   ui.Res[*spotify.ArtistPage]
	cards    []spotify.Card // grid view
	blocks   []*block       // list view: one per release
	starts   []int          // list view: index of each block's first list item
	total    int
	started  bool
	busy     bool
	gen      int // bumped when the sort changes, dropping answers to the old order
	err      error
	list     ui.List
	rows     ui.CardRows
	name     ui.Link
	filter   ui.Click
	sortBtn  ui.Click
}

// block is a release in the list view: its header and track table.
type block struct {
	r      spotify.Release
	res    ui.Res[*spotify.Album]
	tracks *ui.Paged
	table  ui.TrackTable
	title  ui.Link
	play   ui.PlayButton
	save   ui.IconButton
	more   ui.IconButton
	cover  ui.Click
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
	uri, kind, off, byName, gen := "spotify:artist:"+d.id, d.kind, len(d.blocks), discoByName, d.gen
	ui.Async(func() (*spotify.ReleasePage, error) { return ui.B.Discography(uri, kind, byName, off, discoPage) }, func(p *spotify.ReleasePage, err error) {
		if gen != d.gen {
			return
		}
		d.busy, d.started = false, true
		if err != nil {
			d.err = err
			return
		}
		for _, r := range p.Releases {
			d.cards = append(d.cards, r.Card)
			d.blocks = append(d.blocks, &block{r: r, table: ui.TrackTable{Variant: "album", ContextURI: r.URI}, tracks: &ui.Paged{}})
		}
		d.total = p.Total
	})
}

// resort starts over with the sort order in discoByName.
func (d *disco) resort() {
	d.gen++
	d.cards, d.blocks, d.total = nil, nil, 0
	d.started, d.busy = false, false
	d.list = ui.List{}
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
	if d.sortBtn.Clicked(gtx) {
		ui.OpenMenu(d.sortMenu())
	}
	var dims layout.Dimensions
	var last, n int
	if discoGridView {
		n = d.rows.Count(gtx, len(d.cards), 176)
		dims = d.list.Layout(gtx, n+2, func(gtx layout.Context, i int) layout.Dimensions {
			switch {
			case i == 0:
				return d.toolbar(gtx, 16)
			case i <= n:
				return d.rows.Row(gtx, i-1, d.cards, 176, ui.CardOpts{})
			}
			return ui.Gap(0, 24)(gtx)
		})
		_, last = d.list.Range()
	} else {
		n = d.layoutStarts()
		dims = d.list.Layout(gtx, n+1, func(gtx layout.Context, i int) layout.Dimensions {
			if i == 0 {
				return d.toolbar(gtx, 64)
			}
			return d.item(gtx, i)
		})
		var first int
		first, last = d.list.Range()
		d.loadVisible(first, last)
	}
	if last >= n-2 && len(d.blocks) < d.total {
		d.fetch()
	}
	return dims
}

func (d *disco) toolbar(gtx layout.Context, bottom float32) layout.Dimensions {
	return ui.PageInset(gtx, 64, bottom, func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		h := ui.Dp(gtx, 40)
		name := ""
		if d.artist.Ok() {
			name = d.artist.V.Name
		}
		sortLabel, viewIcon := "Release date", "list"
		if discoByName {
			sortLabel = "Name"
		}
		if discoGridView {
			viewIcon = "grid"
		}
		// Right to left: the sort and view button, then the filter.
		bh, pad, ic := ui.Dp(gtx, 32), ui.Dp(gtx, 8), ui.Dp(gtx, 16)
		sw := ui.Dp(gtx, 16) + ui.Measure(gtx, sortLabel, 14, ui.Regular) + ui.Dp(gtx, 8) + ic + ui.Dp(gtx, 12)
		d.sortBtn.Label = "Sort and view"
		ui.Offset(gtx, w-sw, (h-bh)/2, func() {
			d.sortBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				c := ui.ColText2
				if d.sortBtn.Hovered() {
					c = ui.ColText
				}
				m := op.Record(gtx.Ops)
				td := ui.Text(gtx, sortLabel, 14, ui.Regular, c)
				call := m.Stop()
				ui.Offset(gtx, ui.Dp(gtx, 16), (bh-td.Size.Y)/2, func() { call.Add(gtx.Ops) })
				ui.Offset(gtx, sw-ui.Dp(gtx, 12)-ic, (bh-ic)/2, func() { ui.IconPx(gtx, viewIcon, ic, c) })
				return layout.Dimensions{Size: image.Pt(sw, bh)}
			})
		})
		label := kindLabel(d.kind)
		tw := ui.Measure(gtx, label, 14, ui.Regular)
		bw := pad + tw + pad + ic + pad
		ui.Offset(gtx, w-sw-bw, (h-bh)/2, func() {
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
				ui.Offset(gtx, pad+tw+pad, (bh-ic)/2, func() { ui.IconPx(gtx, "chevron-down", ic, c) })
				return layout.Dimensions{Size: image.Pt(bw, bh)}
			})
		})
		ui.Offset(gtx, 0, (h-ui.Dp(gtx, 29))/2, func() {
			g := gtx
			g.Constraints.Max.X = max(0, w-sw-bw-ui.Dp(gtx, 16))
			d.name.Click.Label = name
			d.name.Layout(g, name, 24, ui.Bold, ui.ColText, ui.ColNone)
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

func menuLabel(text string) ui.MenuItem {
	return ui.MenuItem{Widget: func(gtx layout.Context) layout.Dimensions {
		return ui.Pad(gtx, 12, 12, 8, 12, func(gtx layout.Context) layout.Dimensions {
			return ui.Text(gtx, text, 14, ui.SemiBold, ui.ColText2)
		})
	}}
}

// sortMenu is "Sort by" Release date / Name and "View as" List / Grid.
func (d *disco) sortMenu() []ui.MenuItem {
	items := []ui.MenuItem{menuLabel("Sort by")}
	for _, byName := range []bool{false, true} {
		label := "Release date"
		if byName {
			label = "Name"
		}
		on := byName == discoByName
		items = append(items, ui.MenuItem{Label: label, Check: on, Accent: on, Action: func() {
			if byName != discoByName {
				discoByName = byName
				d.resort()
			}
		}})
	}
	items = append(items, menuLabel("View as"))
	for _, grid := range []bool{false, true} {
		label, icon := "List", "list"
		if grid {
			label, icon = "Grid", "grid"
		}
		on := grid == discoGridView
		items = append(items, ui.MenuItem{Label: label, Icon: icon, Check: on, Accent: on, Action: func() {
			if grid != discoGridView {
				discoGridView = grid
				d.list = ui.List{}
			}
		}})
	}
	return items
}

// Each release is a header, the table's column header, one item per track and
// a gap. A block's track count is known from the discography before its album
// loads, so the list's length doesn't jump.
const (
	blockHeader = iota
	blockColumns
	blockRows
)

func (b *block) rowCount() int {
	if b.res.Ok() {
		return len(b.res.V.Tracks)
	}
	return b.r.Tracks
}

// layoutStarts fills d.starts and returns the number of list items after the toolbar.
func (d *disco) layoutStarts() int {
	d.starts = d.starts[:0]
	n := 0
	for _, b := range d.blocks {
		d.starts = append(d.starts, n)
		n += blockRows + b.rowCount() + 1
	}
	return n
}

// item draws list item i (1-based, after the toolbar).
func (d *disco) item(gtx layout.Context, i int) layout.Dimensions {
	i--
	k := sort.SearchInts(d.starts, i+1) - 1
	b, j := d.blocks[k], i-d.starts[k]
	switch {
	case j == blockHeader:
		return b.header(gtx)
	case j == blockColumns:
		return b.table.Header(gtx, false)
	case j < blockRows+b.rowCount():
		var t *spotify.Track // nil: still loading
		if j-blockRows < len(b.tracks.Items) {
			t = b.tracks.Items[j-blockRows]
		}
		return b.table.Row(gtx, j-blockRows, t)
	}
	return ui.Gap(0, 64)(gtx)
}

// loadVisible fetches the tracks of the releases on screen (and a few rows ahead).
func (d *disco) loadVisible(first, last int) {
	for k, b := range d.blocks {
		end := d.starts[k] + blockRows + b.rowCount() + 1
		if d.starts[k] <= last+8 && end >= first {
			uri := b.r.URI
			b.res.Once(func() (*spotify.Album, error) { return ui.B.Album(uri) })
		}
		if b.res.Ok() && b.tracks.Items == nil {
			b.tracks.SetAll(b.res.V.Tracks)
		}
	}
}

// header is the release's cover, title, "Album • 2025 • 12 songs" and play, save and more.
func (b *block) header(gtx layout.Context) layout.Dimensions {
	r := b.r
	if b.title.Click.Clicked(gtx) || b.cover.Clicked(gtx) {
		ui.Navigate("/album/" + ui.URIID(r.URI))
	}
	if b.play.Click.Clicked(gtx) {
		ui.PlayContext(r.URI)
	}
	if b.save.Click.Clicked(gtx) {
		ui.ToggleSaved(r.URI)
	}
	if b.more.Click.Clicked(gtx) {
		ui.OpenMenu(ui.CardMenu(r.Card, ui.CardMenuOpts{}))
	}
	ui.LoadSaved([]string{r.URI})
	w, h := gtx.Constraints.Max.X, ui.Dp(gtx, 168)
	cover, left := ui.Dp(gtx, 136), ui.Dp(gtx, 24+32)
	ui.Offset(gtx, left, 0, func() {
		b.cover.Label = r.Name
		b.cover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.Image{URL: ui.Pick(r.Images, cover), Radius: ui.Dp(gtx, 4), Icon: "album"}.Layout(gtx, image.Pt(cover, cover))
		})
	})
	tx := left + cover + ui.Dp(gtx, 24)
	g := gtx
	g.Constraints.Max.X = max(0, w-tx-ui.Dp(gtx, 24))
	ui.Offset(gtx, tx, 0, func() {
		ui.Column(g,
			func(gtx layout.Context) layout.Dimensions {
				b.title.Click.Label = r.Name
				gtx.Constraints.Min = image.Point{} // only the text is a link
				return b.title.Layout(gtx, r.Name, 32, ui.Bold, ui.ColText, ui.ColNone)
			},
			ui.Gap(0, 8),
			func(gtx layout.Context) layout.Dimensions {
				songs := "songs"
				if b.rowCount() == 1 {
					songs = "song"
				}
				return ui.Text(gtx, fmt.Sprintf("%s • %s • %s %s", r.Kind, r.Year, ui.FmtNumber(b.rowCount()), songs), 14, ui.Regular, ui.Alpha(ui.ColText, .7))
			},
			ui.Gap(0, 20),
			func(gtx layout.Context) layout.Dimensions { return b.actions(gtx) },
		)
	})
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// actions is the row of 32dp play and 24dp save and "…" buttons.
func (b *block) actions(gtx layout.Context) layout.Dimensions {
	saved := ui.Saved[b.r.URI]
	icon, tip := "plus-circle", "Save to Your Library"
	if saved {
		icon, tip = "check-circle", "Remove from Your Library"
	}
	d := layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.play.Layout(gtx, 32, ui.CtxPlaying(b.r.URI)) }),
		layout.Rigid(ui.Gap(16, 0)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return b.save.Layout(gtx, ui.IconStyle{Icon: icon, Size: 24, Box: 24, Active: saved, Label: tip})
		}),
		layout.Rigid(ui.Gap(16, 0)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return b.more.Layout(gtx, ui.IconStyle{Icon: "more", Size: 24, Box: 24, Label: "More options"})
		}),
	)
	d.Size.Y = ui.Dp(gtx, 48)
	return d
}
