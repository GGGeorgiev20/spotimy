package ui

import (
	"image"
	"net/url"
	"strings"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// The dropdown that opens under the top bar's search box while it's focused:
// recent searches while it's empty, and as you type, query completions and
// matching artists, songs and so on, like the desktop client. Enter (or a
// completion) opens the full search page.
var drop struct {
	res  Res[[]spotify.Card] // recent searches
	sugg Res[[]spotify.Suggestion]
	q    string // the query sugg was asked for
	tm   *time.Timer

	rows  []dropRow
	clear Pill
	list  List
	sel   int // keyboard-selected row of the suggestions, -1 for none
	shown int // the selection last scrolled into view
	// rect is where the dropdown was drawn (window px); a press inside it
	// keeps the search box focused so the click lands.
	rect image.Rectangle
	tag  int
}

type dropRow struct {
	click  Click
	remove IconButton
	follow Pill
	save   IconButton
}

const dropMax = 10

func init() { drop.sel = -1 }

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

// searchTyped asks for suggestions 150ms after typing stops.
func searchTyped() {
	drop.sel = -1
	if drop.tm != nil {
		drop.tm.Stop()
	}
	drop.tm = time.AfterFunc(150*time.Millisecond, func() {
		Post(func() {
			q := strings.TrimSpace(SearchQuery.Text())
			if q == "" || q == drop.q {
				return
			}
			drop.q = q
			drop.list.ScrollToTop()
			drop.sugg.Load(func() ([]spotify.Suggestion, error) { return B.SearchSuggestions(q) })
		})
	})
}

// submitSearch opens the search page for q and closes the dropdown.
func submitSearch(gtx layout.Context, q string) {
	q = strings.TrimSpace(q)
	if q != strings.TrimSpace(SearchQuery.Text()) {
		SearchQuery.SetText(q)
	}
	SearchQuery.Blur(gtx)
	path := "/search"
	if q != "" {
		path += "/" + url.PathEscape(q)
	}
	switch {
	case Current().Path == path:
	case strings.HasPrefix(Current().Path, "/search"):
		Replace(path)
	default:
		Navigate(path)
	}
}

// suggestions is what the dropdown lists for the typed text.
func suggestions() []spotify.Suggestion {
	if strings.TrimSpace(SearchQuery.Text()) == "" || drop.q == "" {
		return nil
	}
	return drop.sugg.V
}

// searchKey handles ↑, ↓ and Enter in the search box; Enter without a
// selection opens the search page.
func searchKey(gtx layout.Context, name string) {
	n := len(suggestions())
	switch name {
	case "↑":
		if n > 0 {
			drop.sel = (max(drop.sel, 0) + n - 1) % n
		}
	case "↓":
		if n > 0 {
			drop.sel = (drop.sel + 1) % n
		}
	default: // Enter
		if s := suggestions(); drop.sel >= 0 && drop.sel < len(s) {
			openSuggestion(gtx, s[drop.sel])
			return
		}
		submitSearch(gtx, SearchQuery.Text())
	}
}

func openSuggestion(gtx layout.Context, s spotify.Suggestion) {
	if s.Card == nil {
		submitSearch(gtx, s.Query)
		return
	}
	SearchQuery.Blur(gtx)
	RememberSearch(s.Card.URI)
	if s.Card.Type == "track" {
		Play(s.Card.URI, "") // there's no song page: play it, like the search page's rows
		return
	}
	OpenURI(s.Card.URI)
}

// dropItem is one row of the dropdown and its height in dp.
type dropItem struct {
	h float32
	w layout.Widget
}

// layoutSearchDropdown draws the dropdown under box (the search box's window
// rect) when it should be open.
func layoutSearchDropdown(gtx layout.Context, box image.Rectangle) {
	drop.rect = image.Rectangle{}
	if !searchFocused {
		return
	}
	var items []dropItem
	if strings.TrimSpace(SearchQuery.Text()) == "" {
		items = recentItems(gtx)
	} else {
		items = suggestionItems(gtx)
	}
	if len(items) == 0 {
		return
	}

	w, pad := box.Dx(), Dp(gtx, 8)
	maxH := float32(398)
	if float32(WindowSize.Y)/gtx.Metric.PxPerDp >= 1200 {
		maxH = 590
	}
	var contentH float32
	for _, it := range items {
		contentH += it.h
	}
	h := Dp(gtx, min(maxH, contentH+16))
	r := image.Rect(box.Min.X, box.Max.Y+Dp(gtx, 4), box.Max.X, box.Max.Y+Dp(gtx, 4)+h)
	drop.rect = r
	Offset(gtx, r.Min.X, r.Min.Y, func() {
		rr := image.Rect(0, 0, w, h)
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
		if drop.sel != drop.shown && drop.sel >= 0 {
			drop.shown = drop.sel
			scrollIntoView(gtx, items, drop.sel+1, h-2*pad) // +1: the key hints
		}
		Offset(gtx, pad, pad, func() {
			g := gtx
			g.Constraints = layout.Exact(image.Pt(w-2*pad, h-2*pad))
			drop.list.Layout(g, len(items), func(gtx layout.Context, i int) layout.Dimensions {
				gtx.Constraints.Max.Y = Dp(gtx, items[i].h)
				items[i].w(gtx)
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, Dp(gtx, items[i].h))}
			})
		})
	})
}

// scrollIntoView scrolls the dropdown's list just enough to show item i.
func scrollIntoView(gtx layout.Context, items []dropItem, i, viewH int) {
	if i == 1 { // the first row: show the key hints above it too
		drop.list.ScrollToTop()
		return
	}
	top := 0
	for _, it := range items[:i] {
		top += Dp(gtx, it.h)
	}
	cur, ih := drop.list.ScrollTop(), Dp(gtx, items[i].h)
	switch {
	case top < cur:
		drop.list.Position.Offset -= cur - top
	case top+ih > cur+viewH:
		drop.list.Position.Offset += top + ih - cur - viewH
	}
	drop.list.Position.BeforeEnd = true
}

func (r *dropRow) hovered() bool {
	return r.click.Hovered() || r.remove.Click.Hovered() || r.follow.Click.Hovered() || r.save.Click.Hovered()
}

func dropRows(n int) {
	if len(drop.rows) < n {
		drop.rows = append(drop.rows, make([]dropRow, n-len(drop.rows))...)
	}
}

var dropRowBg = Mix(ColSurface3, ColText, .1)

// recentItems is "Recent searches", its rows and "Clear recent searches".
func recentItems(gtx layout.Context) []dropItem {
	drop.res.Once(func() ([]spotify.Card, error) { return B.RecentSearches() })
	cards := drop.res.V
	if len(cards) > dropMax {
		cards = cards[:dropMax]
	}
	if len(cards) == 0 {
		return nil
	}
	dropRows(len(cards))
	if drop.clear.Click.Clicked(gtx) {
		uris := make([]string, len(drop.res.V))
		for i, c := range drop.res.V {
			uris[i] = c.URI
		}
		forgetSearches(uris)
		return nil
	}
	items := []dropItem{{40, func(gtx layout.Context) layout.Dimensions {
		return Pad(gtx, 8, 8, 4, 8, func(gtx layout.Context) layout.Dimensions {
			return Text(gtx, "Recent searches", 16, Bold, ColText)
		})
	}}}
	for i, c := range cards {
		r := &drop.rows[i]
		if r.remove.Click.Clicked(gtx) {
			forgetSearches([]string{c.URI})
			return nil
		}
		if r.click.Clicked(gtx) {
			openSuggestion(gtx, spotify.Suggestion{Card: &c})
			return nil
		}
		items = append(items, dropItem{64, func(gtx layout.Context) layout.Dimensions {
			return entityRow(gtx, r, c, false, func(gtx layout.Context, rw, rowH int) {
				Offset(gtx, rw-Dp(gtx, 4+32), (rowH-Dp(gtx, 32))/2, func() {
					r.remove.Layout(gtx, IconStyle{Icon: "close", Size: 16, Box: 32, Label: "Remove " + c.Name})
				})
			})
		}})
	}
	items = append(items, dropItem{48, func(gtx layout.Context) layout.Dimensions {
		Offset(gtx, Dp(gtx, 8), Dp(gtx, 8), func() {
			drop.clear.Layout(gtx, PillStyle{Text: "Clear recent searches", Size: 13, Weight: Bold, Height: 32, PadX: 15, Outline: true})
		})
		return layout.Dimensions{}
	}})
	return items
}

// suggestionItems is the key hints, the query completions and the entities.
func suggestionItems(gtx layout.Context) []dropItem {
	sugg := suggestions()
	if len(sugg) == 0 {
		return nil
	}
	dropRows(len(sugg))
	var uris []string
	for _, s := range sugg {
		if s.Card != nil && (s.Card.Type == "track" || s.Card.Type == "artist") {
			if _, ok := Saved[s.Card.URI]; !ok {
				uris = append(uris, s.Card.URI)
			}
		}
	}
	if len(uris) > 0 {
		LoadSaved(uris)
	}
	items := []dropItem{{24, keyHints}}
	typed := strings.TrimSpace(SearchQuery.Text())
	for i, s := range sugg {
		r := &drop.rows[i]
		if r.click.Clicked(gtx) {
			openSuggestion(gtx, s)
			return nil
		}
		sel := i == drop.sel
		if s.Card == nil {
			items = append(items, dropItem{56, func(gtx layout.Context) layout.Dimensions {
				return queryRow(gtx, r, s.Query, typed, sel)
			}})
			continue
		}
		c := *s.Card
		if r.follow.Click.Clicked(gtx) {
			ToggleFollow(c.URI)
		}
		if r.save.Click.Clicked(gtx) {
			ToggleSaved(c.URI)
		}
		items = append(items, dropItem{64, func(gtx layout.Context) layout.Dimensions {
			return entityRow(gtx, r, c, sel, func(gtx layout.Context, rw, rowH int) {
				switch c.Type {
				case "artist":
					txt := "Follow"
					if Saved[c.URI] {
						txt = "Following"
					}
					m := op.Record(gtx.Ops)
					d := r.follow.Layout(gtx, PillStyle{Text: txt, Weight: Bold, Height: 32, PadX: 15, Outline: true})
					call := m.Stop()
					Offset(gtx, rw-Dp(gtx, 12)-d.Size.X, (rowH-d.Size.Y)/2, func() { call.Add(gtx.Ops) })
				case "track":
					if !Saved[c.URI] && !r.hovered() && !sel {
						return
					}
					st := IconStyle{Icon: "plus-circle", Size: 16, Box: 32, Label: "Save to Your Library"}
					if Saved[c.URI] {
						st = IconStyle{Icon: "check-circle", Size: 16, Box: 32, Color: ColAccent, Hover: ColAccent, Label: "Remove from Your Library"}
					}
					Offset(gtx, rw-Dp(gtx, 4+32), (rowH-Dp(gtx, 32))/2, func() { r.save.Layout(gtx, st) })
				}
			})
		}})
	}
	return items
}

// keyHints is the "↑ ↓ Navigate · Enter Search" header.
func keyHints(gtx layout.Context) layout.Dimensions {
	w := gtx.Constraints.Max.X
	cap := func(x int, wd layout.Widget) int {
		m := op.Record(gtx.Ops)
		d := wd(gtx)
		call := m.Stop()
		cw, ch := max(Dp(gtx, 20), d.Size.X+Dp(gtx, 8)), Dp(gtx, 20)
		Offset(gtx, x, 0, func() {
			strokeRRect(gtx.Ops, image.Rect(0, 0, cw, ch), Dp(gtx, 4), max(1, Dp(gtx, 1)), ColText2)
			Offset(gtx, (cw-d.Size.X)/2, (ch-d.Size.Y)/2, func() { call.Add(gtx.Ops) })
		})
		return cw
	}
	label := func(x int, s string) int {
		m := op.Record(gtx.Ops)
		d := Text(gtx, s, 12, Regular, ColText2)
		call := m.Stop()
		Offset(gtx, x, (Dp(gtx, 20)-d.Size.Y)/2, func() { call.Add(gtx.Ops) })
		return d.Size.X
	}
	arrow := func(name string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			s := Dp(gtx, 12)
			IconPx(gtx, name, s, ColText2)
			return layout.Dimensions{Size: image.Pt(s, s)}
		}
	}
	x := Dp(gtx, 4)
	x += cap(x, arrow("arrow-up")) + Dp(gtx, 8)
	x += cap(x, arrow("arrow-down")) + Dp(gtx, 8)
	label(x, "Navigate")
	sw := Measure(gtx, "Search", 12, Regular)
	enter := func(gtx layout.Context) layout.Dimensions { return Text(gtx, "Enter", 12, Regular, ColText2) }
	ew := Measure(gtx, "Enter", 12, Regular) + Dp(gtx, 8)
	x = w - Dp(gtx, 4) - sw - Dp(gtx, 8) - ew
	cap(x, enter)
	label(w-Dp(gtx, 4)-sw, "Search")
	return layout.Dimensions{Size: image.Pt(w, Dp(gtx, 24))}
}

// queryRow is a completion: a search icon and the text, the typed part bold.
func queryRow(gtx layout.Context, r *dropRow, text, typed string, sel bool) layout.Dimensions {
	rw, rowH := gtx.Constraints.Max.X, Dp(gtx, 56)
	r.click.Label = text
	return r.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if r.click.Hovered() || sel {
			FillRect(gtx.Ops, image.Rect(0, 0, rw, rowH), Dp(gtx, 6), dropRowBg)
		}
		is := Dp(gtx, 24)
		Offset(gtx, Dp(gtx, 8+12), (rowH-is)/2, func() { IconPx(gtx, "search", is, ColText2) })
		bold, rest := "", text
		if len(typed) <= len(text) && strings.EqualFold(text[:len(typed)], typed) {
			bold, rest = text[:len(typed)], text[len(typed):]
		}
		tx := Dp(gtx, 8+48+12)
		m := op.Record(gtx.Ops)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(max(0, rw-tx-Dp(gtx, 4)), rowH)}
		d := Inline(g, 16, Regular, ColText, ColText, Seg{Text: bold, Weight: Bold}, Seg{Text: rest})
		call := m.Stop()
		Offset(gtx, tx, (rowH-d.Size.Y)/2, func() { call.Add(gtx.Ops) })
		return layout.Dimensions{Size: image.Pt(rw, rowH)}
	})
}

// entityRow is a 64dp row: picture, name (with the verified badge) and the
// "Type • Who" line; trailing draws the row's buttons on the right.
func entityRow(gtx layout.Context, r *dropRow, c spotify.Card, sel bool, trailing func(gtx layout.Context, rw, rowH int)) layout.Dimensions {
	rw, rowH := gtx.Constraints.Max.X, Dp(gtx, 64)
	r.click.Label = c.Name
	return r.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if r.hovered() || sel {
			FillRect(gtx.Ops, image.Rect(0, 0, rw, rowH), Dp(gtx, 6), dropRowBg)
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
		if c.Type == "artist" {
			tw = rw - tx - Dp(gtx, 12+100)
		}
		detail := c.Detail
		if detail == "" {
			detail = c.Subtitle
		}
		segs := []Seg{{Text: c.Name}}
		if c.Verified {
			segs = append(segs, Seg{Widget: func(gtx layout.Context) layout.Dimensions {
				// Sits on the baseline, centered on the 22dp line.
				s, gap := Dp(gtx, 12), Dp(gtx, 8)
				Offset(gtx, gap, 0, func() { IconPx(gtx, "verified", s, RGB(0xc5f0c9)) })
				return layout.Dimensions{Size: image.Pt(gap+s, s)}
			}})
		}
		m := op.Record(gtx.Ops)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(max(0, tw), rowH)}
		nd := Inline(g, 16, Regular, ColText, ColText, segs...)
		var dd layout.Dimensions
		Offset(gtx, 0, nd.Size.Y, func() { dd = Text(g, detail, 14, Regular, ColText2) })
		call := m.Stop()
		Offset(gtx, tx, (rowH-nd.Size.Y-dd.Size.Y)/2, func() { call.Add(gtx.Ops) })
		trailing(gtx, rw, rowH)
		return layout.Dimensions{Size: image.Pt(rw, rowH)}
	})
}

// inSearchDropdown reports whether p (window px) is on the open dropdown.
func inSearchDropdown(p image.Point) bool { return p.In(drop.rect) }
