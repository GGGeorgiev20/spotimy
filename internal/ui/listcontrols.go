package ui

import (
	"cmp"
	"image"
	"slices"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// SortOpt is one entry of a track list's sort menu.
type SortOpt struct{ Key, Label string }

// Sort keys of the track list menu. "custom" is the list's own order.
const (
	SortCustom   = "custom"
	SortTitle    = "title"
	SortArtist   = "artist"
	SortAlbum    = "album"
	SortAdded    = "added"
	SortDuration = "duration"
)

// ViewOrder returns the indices of the rows of tracks that match query
// (title, artist or album contains it), ordered by key. Rows that aren't
// loaded yet (nil) are left out. desc reverses the key's natural order:
// A→Z, shortest first, newest first for dates.
func ViewOrder(tracks []*spotify.Track, query, key string, desc bool) []int {
	q := strings.ToLower(strings.TrimSpace(query))
	var idx []int
	for i, t := range tracks {
		if t != nil && (q == "" || trackMatches(*t, q)) {
			idx = append(idx, i)
		}
	}
	text := func(f func(t *spotify.Track) string) func(a, b int) int {
		return func(a, b int) int {
			return cmp.Compare(strings.ToLower(f(tracks[a])), strings.ToLower(f(tracks[b])))
		}
	}
	var by func(a, b int) int
	switch key {
	case SortTitle:
		by = text(func(t *spotify.Track) string { return t.Name })
	case SortArtist:
		by = text(func(t *spotify.Track) string { return artistNames(t) })
	case SortAlbum:
		by = text(func(t *spotify.Track) string {
			if t.Album == nil {
				return ""
			}
			return t.Album.Name
		})
	case SortAdded:
		by = func(a, b int) int { return -cmp.Compare(tracks[a].AddedAt, tracks[b].AddedAt) }
	case SortDuration:
		by = func(a, b int) int { return cmp.Compare(tracks[a].DurationMs, tracks[b].DurationMs) }
	}
	if by != nil {
		slices.SortStableFunc(idx, func(a, b int) int {
			if desc {
				return -by(a, b)
			}
			return by(a, b)
		})
	}
	return idx
}

func artistNames(t *spotify.Track) string {
	names := make([]string, len(t.Artists))
	for i, a := range t.Artists {
		names[i] = a.Name
	}
	return strings.Join(names, ", ")
}

func trackMatches(t spotify.Track, q string) bool {
	if strings.Contains(strings.ToLower(t.Name), q) || strings.Contains(strings.ToLower(artistNames(&t)), q) {
		return true
	}
	return t.Album != nil && strings.Contains(strings.ToLower(t.Album.Name), q)
}

// TrackView is what a ListControls makes of a Paged list: its rows searched
// and sorted. Until the controls are used the list is shown as it comes.
type TrackView struct {
	order    []int
	src      *Paged
	ver, gen int
	on       bool
}

// Update refreshes the view and reports whether it is reordered. Searching
// and sorting need every row, so they load the rest of the list.
func (v *TrackView) Update(p *Paged, ctl *ListControls, tab *TrackTable) bool {
	if !ctl.Reordered() {
		if v.on {
			v.on, v.order = false, nil
			tab.Reset()
		}
		return false
	}
	if !v.on || v.src != p || v.ver != ctl.Version {
		tab.Reset()
	}
	if !v.on || v.src != p || v.ver != ctl.Version || v.gen != p.Gen {
		v.order = ViewOrder(p.Items, ctl.Query(), ctl.Sort, ctl.Desc)
		v.on, v.src, v.ver, v.gen = true, p, ctl.Version, p.Gen
	}
	p.Ensure(0, p.Total)
	return true
}

// Len is the number of rows shown.
func (v *TrackView) Len(p *Paged) int {
	if v.on {
		return len(v.order)
	}
	return p.Total
}

// At is row k as shown (nil while it loads).
func (v *TrackView) At(p *Paged, k int) *spotify.Track {
	if v.on {
		return p.Items[v.order[k]]
	}
	return p.Items[k]
}

// ListControls is the search field and sort menu at the right end of a
// track list's action bar. Pages compare Version to see when to rebuild.
type ListControls struct {
	Options []SortOpt
	Default string // the key that leaves the list as it comes
	Hint    string // search placeholder
	Sort    string
	Desc    bool
	Version int

	search  Input
	query   string
	open    bool
	focus   bool  // grab the keyboard once the field is up
	had     bool  // the field has had the keyboard since it opened
	btn     Click // the collapsed search button
	clear   Click
	sortBtn Click
	rows    []Click
}

// Query is the text typed in the search field.
func (l *ListControls) Query() string { return strings.TrimSpace(l.query) }

// Reordered reports whether the list is searched or sorted away from its
// own order.
func (l *ListControls) Reordered() bool {
	return l.Query() != "" || (l.Sort != "" && l.Sort != l.Default) || l.Desc
}

func (l *ListControls) label() string {
	for _, o := range l.Options {
		if o.Key == l.Sort {
			return o.Label
		}
	}
	return l.Options[0].Label
}

func (l *ListControls) setQuery(q string) {
	if q != l.query {
		l.query = q
		l.Version++
	}
}

func (l *ListControls) pick(k string) {
	switch {
	case k == SortCustom:
		l.Sort, l.Desc = k, false
	case k == l.Sort:
		l.Desc = !l.Desc
	default:
		l.Sort, l.Desc = k, false
	}
	l.Version++
}

func (l *ListControls) menu() []MenuItem {
	items := []MenuItem{{Widget: func(gtx layout.Context) layout.Dimensions {
		return Pad(gtx, 12, 8, 12, 12, func(gtx layout.Context) layout.Dimensions {
			return Text(gtx, "Sort by", 12, Bold, Alpha(ColText, .7))
		})
	}}}
	if len(l.rows) < len(l.Options) {
		l.rows = append(l.rows, make([]Click, len(l.Options)-len(l.rows))...)
	}
	for i, o := range l.Options {
		c := &l.rows[i]
		items = append(items, MenuItem{Widget: func(gtx layout.Context) layout.Dimensions {
			if c.Clicked(gtx) {
				l.pick(o.Key)
				CloseMenu()
			}
			w, h := gtx.Constraints.Max.X, Dp(gtx, 40)
			c.Label = o.Label
			return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				sel := o.Key == l.Sort
				if c.Hovered() {
					FillRect(gtx.Ops, image.Rect(0, 0, w, h), Dp(gtx, 2), Alpha(ColText, .1))
				}
				fg := Alpha(ColText, .9)
				if sel {
					fg = ColAccent
				}
				Offset(gtx, Dp(gtx, 12), 0, func() {
					g := gtx
					g.Constraints = layout.Exact(image.Pt(w-Dp(gtx, 12+8+16+8), h))
					layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions { return Text(gtx, o.Label, 14, Regular, fg) })
				})
				if sel && o.Key != SortCustom {
					arrow := "arrow-up"
					// The arrow shows the order of the key's natural direction.
					if l.Desc != (o.Key == SortAdded) {
						arrow = "arrow-down"
					}
					is := Dp(gtx, 16)
					Offset(gtx, w-Dp(gtx, 8)-is, (h-is)/2, func() { IconPx(gtx, arrow, is, ColAccent) })
				}
				return layout.Dimensions{Size: image.Pt(w, h)}
			})
		}})
	}
	return items
}

// Layout draws the controls over the right end of an action bar of the given
// size (they sit on the play button's row).
func (l *ListControls) Layout(gtx layout.Context, bar image.Point) {
	if l.Sort == "" {
		l.Sort = l.Default
	}
	if l.btn.Clicked(gtx) {
		l.open, l.focus = true, true
	}
	if l.clear.Clicked(gtx) {
		l.search.SetText("")
		l.setQuery("")
		l.open = false
	}
	if l.sortBtn.Clicked(gtx) {
		OpenMenu(l.menu())
	}
	if changed, _ := l.search.Poll(gtx); changed {
		l.setQuery(l.search.Text())
	}
	for {
		e, ok := gtx.Event(key.Filter{Focus: &l.search.Editor, Name: key.NameEscape})
		if !ok {
			break
		}
		if ke, ok := e.(key.Event); ok && ke.State == key.Press {
			l.search.SetText("")
			l.setQuery("")
			l.open = false
			l.search.Blur(gtx)
		}
	}
	// Like the official client, the field folds back up when it loses focus empty.
	switch focused := gtx.Focused(&l.search.Editor); {
	case focused:
		l.had = true
	case l.open && l.had && l.query == "":
		l.open, l.had = false, false
	}

	h, gap := Dp(gtx, 32), Dp(gtx, 8)
	y := Dp(gtx, 24) + (Dp(gtx, 56)-h)/2
	label := l.label()
	sw := Dp(gtx, 8) + Measure(gtx, label, 14, Regular) + gap + Dp(gtx, 16) + Dp(gtx, 8)
	sx := bar.X - Dp(gtx, 24) - sw
	Offset(gtx, sx, y, func() {
		l.sortBtn.Label = "Sort by"
		l.sortBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			c := Alpha(ColText, .7)
			if l.Sort != l.Default || l.Desc {
				c = ColAccent
			} else if l.sortBtn.Hovered() {
				c = ColText
			}
			Offset(gtx, Dp(gtx, 8), 0, func() {
				g := gtx
				g.Constraints = layout.Exact(image.Pt(sw, h))
				layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions { return Text(gtx, label, 14, Regular, c) })
			})
			is := Dp(gtx, 16)
			Offset(gtx, sw-Dp(gtx, 8)-is, (h-is)/2, func() { IconPx(gtx, "list", is, c) })
			return layout.Dimensions{Size: image.Pt(sw, h)}
		})
	})

	if !l.open {
		Offset(gtx, sx-gap-h, y, func() {
			l.btn.Label = "Search in playlist"
			l.btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				c := Alpha(ColText, .7)
				if l.btn.Hovered() {
					c = ColText
				}
				is := Dp(gtx, 16)
				Offset(gtx, (h-is)/2, (h-is)/2, func() { IconPx(gtx, "search", is, c) })
				return layout.Dimensions{Size: image.Pt(h, h)}
			})
		})
		return
	}
	fw := min(Dp(gtx, 224), sx-gap-Dp(gtx, 24+56+24+24))
	fx := sx - gap - fw
	Offset(gtx, fx, y, func() {
		focused := l.search.Focused()
		bg := Alpha(ColText, .1)
		if focused {
			bg = Alpha(ColText, .15)
		}
		FillRect(gtx.Ops, image.Rect(0, 0, fw, h), Dp(gtx, 4), bg)
		is := Dp(gtx, 16)
		Offset(gtx, Dp(gtx, 8), (h-is)/2, func() { IconPx(gtx, "search", is, ColText2) })
		tx, right := Dp(gtx, 32), Dp(gtx, 8)
		if l.query != "" {
			right = h
			Offset(gtx, fw-h, 0, func() {
				l.clear.Label = "Clear search"
				l.clear.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					c := ColText2
					if l.clear.Hovered() {
						c = ColText
					}
					Offset(gtx, (h-is)/2, (h-is)/2, func() { IconPx(gtx, "close", is, c) })
					return layout.Dimensions{Size: image.Pt(h, h)}
				})
			})
		}
		Offset(gtx, tx, Dp(gtx, 6), func() {
			g := gtx
			w := fw - tx - right
			g.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, h-Dp(gtx, 12))}
			l.search.Layout(g, InputStyle{Hint: l.Hint})
		})
	})
	if l.focus {
		l.focus = false
		l.search.Focus(gtx)
	}
}

// NoMatches is the empty state of a searched track list.
func NoMatches(gtx layout.Context, query string) layout.Dimensions {
	return Pad(gtx, 64, 24, 64, 24, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Label{Text: `No results found for "` + query + `"`, Size: 24, Weight: Bold, Align: text.Middle, MaxLines: 2}.Layout(gtx)
			}),
			layout.Rigid(Gap(0, 8)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Label{Text: "Please make sure your words are spelled correctly, or use fewer or different keywords.", Color: ColText2, Align: text.Middle, MaxLines: 3, Size: 16}.Layout(gtx)
			}),
		)
	})
}
