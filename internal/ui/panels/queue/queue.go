// Package queue is the Queue panel (Prefs.RightPanel "queue"): the playing
// track, what the user queued and what's next from the context.
package queue

import (
	"image"
	"math"
	"slices"
	"strconv"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() { ui.RightPanels["queue"] = p.layout }

var p panel

type panel struct {
	header  ui.PanelHeader
	list    ui.List
	res     ui.Res[*backend.Queue]
	version int // Player.Version+1 the queue was loaded for; 0 = never
	ctx     ui.ContextName

	rows           []row
	clear, ctxLink ui.Link
	find           ui.Pill

	// drop takes tracks dragged onto the empty panel; heads onto the
	// headings and the space below the last row (the rows have their own).
	drop  ui.DropTarget
	heads []ui.DropTarget

	loggedErr error // the load error last logged

	// playing is the track last seen playing: when another starts, it and
	// what was skipped to reach it leave the list at once, rather than when
	// the queue reloads (which waits for the track to settle).
	playing string

	// A move slides the rows from where they were: slides holds each moved
	// row's starting offset (px), eased to 0 from slideAt. rowPx and headPx
	// are the row and heading heights last laid out.
	slides        map[string]int
	slideAt       time.Time
	rowPx, headPx int

	// The "Recently played" tab.
	recent           bool
	tabQueue, tabRec ui.Click
	recentRes        ui.Res[[]spotify.Track]
	recentVersion    int // the HistoryVersion recentRes was loaded for
	recentRows       []row
	recentList       ui.List
}

const rowDp = 56

const (
	itemNowHead = iota
	itemNow
	itemQueuedHead
	itemNextHead
	itemTrack
)

type item struct {
	kind int
	i    int // itemTrack: index into the queue
}

func (p *panel) layout(gtx layout.Context) layout.Dimensions {
	if p.version != ui.Player.Version+1 && (p.version == 0 || ui.Settled(gtx)) {
		p.version = ui.Player.Version + 1
		p.res.Load(ui.B.Queue)
	}
	if p.tabQueue.Clicked(gtx) {
		p.recent = false
	}
	if p.tabRec.Clicked(gtx) {
		p.recent = true
		p.recentVersion = -1
	}
	if p.recent && p.recentVersion != ui.HistoryVersion {
		p.recentVersion = ui.HistoryVersion
		h := ui.RecentlyPlayed()
		p.recentRes.Reload(func() ([]spotify.Track, error) { return ui.HistoryTracks(h) })
	}
	tab := func(c *ui.Click, label string, on bool) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			c.Label = label
			return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				col := ui.ColText2
				if on || c.Hovered() {
					col = ui.ColText
				}
				return ui.Text(gtx, label, 16, ui.Bold, col)
			})
		}
	}
	return ui.Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return p.header.Layout(gtx, "Close", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(tab(&p.tabQueue, "Queue", !p.recent)),
					layout.Rigid(ui.Gap(24, 0)),
					layout.Rigid(tab(&p.tabRec, "Recently played", p.recent)),
				)
			}, nil)
		},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			if p.recent {
				return p.recentBody(gtx)
			}
			return p.body(gtx)
		},
	)
}

func (p *panel) body(gtx layout.Context) layout.Dimensions {
	t := ui.NowTrack()
	var next []backend.QueueTrack
	if p.res.Ok() {
		next = p.res.V.Next
	} else if p.res.Err != nil && p.res.Err != p.loggedErr {
		p.loggedErr = p.res.Err // once, not every frame
		ui.Log.Warn("queue", "err", p.res.Err)
	}
	if t != nil && t.URI != p.playing {
		p.playing = t.URI
		if i := upcomingIndex(next, t.URI); i >= 0 {
			p.skipPast(i)
			next = p.res.V.Next
		}
	}
	items := layoutItems(t != nil, next)
	queued := 0
	for _, q := range next {
		if q.Provider == "queue" {
			queued++
		}
	}
	if len(items) == 0 {
		if !p.res.Done {
			return ui.Loading(gtx)
		}
		p.dropOn(gtx, next, queued)
		return p.empty(gtx)
	}
	if len(p.rows) < len(items) {
		p.rows = append(p.rows, make([]row, len(items)-len(p.rows))...)
	}
	ctxURI, ctxName := "", ""
	if s := ui.Player.Status; s != nil {
		ctxURI, ctxName = s.ContextURI, p.ctx.Get(s.ContextURI)
	}
	if p.clear.Click.Clicked(gtx) {
		p.set(clearQueued(next))
	}
	if p.ctxLink.Click.Clicked(gtx) && ctxURI != "" {
		ui.OpenURI(ctxURI)
	}
	if len(p.heads) < len(items)+1 {
		p.heads = make([]ui.DropTarget, len(items)+1)
	}
	p.rowPx = ui.Dp(gtx, rowDp)
	// A heading takes a drop at the start of its section (the playing track's
	// at the start of the queue); the room below the list at the end of the
	// last section.
	headDrop := func(gtx layout.Context, n int, d layout.Dimensions, s spot) layout.Dimensions {
		p.heads[n].Group = queueGroup
		if pl, at := p.heads[n].Add(gtx, d.Size, s.code()); pl != nil {
			p.dropAt(pl, decode(at), next, queued)
		}
		return d
	}
	return p.list.Layout(gtx, len(items)+1, func(gtx layout.Context, n int) layout.Dimensions {
		if n == len(items) {
			end := spot{at: len(next), ctx: len(next) > queued}
			return headDrop(gtx, n, layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, ui.Dp(gtx, 80))}, end)
		}
		if k := items[n].kind; k != itemTrack {
			s := spot{}
			if k == itemNextHead {
				s = spot{at: queued, ctx: true}
			}
			d := p.item(gtx, n, items, next, t, queued, ctxName)
			if k != itemNow {
				p.headPx = d.Size.Y
			}
			return headDrop(gtx, n, d, s)
		}
		return p.item(gtx, n, items, next, t, queued, ctxName)
	})
}

// item lays out list item n.
func (p *panel) item(gtx layout.Context, n int, items []item, next []backend.QueueTrack, t *backend.NowTrack, queued int, ctxName string) layout.Dimensions {
	return ui.Pad(gtx, 0, 8, 0, 8, func(gtx layout.Context) layout.Dimensions {
		switch it := items[n]; it.kind {
		case itemNowHead:
			return heading(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, "Now playing", 16, ui.Bold, ui.ColText)
			}, nil)
		case itemNow:
			return p.rows[n].layout(gtx, ui.TrackOf(t), true, nil)
		case itemQueuedHead:
			return heading(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, "Next in queue", 16, ui.Bold, ui.ColText)
			}, func(gtx layout.Context) layout.Dimensions {
				return p.clear.Layout(gtx, "Clear queue", 14, ui.Bold, ui.ColText2, ui.ColText)
			})
		case itemNextHead:
			d := heading(gtx, func(gtx layout.Context) layout.Dimensions {
				if ctxName == "" {
					return ui.Text(gtx, "Next up", 16, ui.Bold, ui.ColText)
				}
				return ui.Inline(gtx, 16, ui.Bold, ui.ColText, ui.ColText,
					ui.Seg{Text: "Next from: "}, ui.Seg{Text: ctxName, Link: &p.ctxLink})
			}, nil)
			// A drop into the empty queue: its line goes above the heading,
			// where the queue would start.
			if code, ok := ui.DropHover(queueGroup); ok && queued == 0 && !decode(code).ctx {
				y := ui.Dp(gtx, 7)
				ui.FillRect(gtx.Ops, image.Rect(0, y, d.Size.X, y+ui.Dp(gtx, 2)), 0, ui.ColAccent)
			}
			return d
		default:
			q := next[it.i]
			inCtx := it.i >= queued
			line := 0
			if code, ok := ui.DropHover(queueGroup); ok {
				if row, below, ok := lineFor(decode(code), len(next), queued); ok && row == it.i {
					line = -1
					if below {
						line = 1
					}
				}
			}
			h := &hooks{
				index: it.i,
				line:  line,
				above: spot{at: it.i, ctx: inCtx},
				below: spot{at: it.i + 1, ctx: inCtx},
				onDrop: func(pl *ui.DragPayload, code int) {
					p.dropAt(pl, decode(code), next, queued)
				},
			}
			h.remove = func() { p.set(removeAt(next, it.i)) }
			var d layout.Dimensions
			ui.Offset(gtx, 0, p.slide(gtx, key(q)), func() { d = p.rows[n].layout(gtx, q.Track, false, h) })
			return d
		}
	})
}

const queueGroup = "queue"

// spot is where a drop lands: before upcoming track at, in the user queue or
// in the rest of the upcoming list (the context). The two meet at the
// queue's end, which belongs to either.
type spot struct {
	at  int
	ctx bool
}

// code packs a spot into a drop target's int.
func (s spot) code() int {
	c := s.at * 2
	if s.ctx {
		c++
	}
	return c
}

func decode(c int) spot { return spot{at: c / 2, ctx: c%2 == 1} }

// lineFor is where the drop line goes for s: over the row it lands before,
// or under the last row of its section; ok is false with no rows at all,
// and for the empty queue, whose line goes above the "Next from" heading.
func lineFor(s spot, n, queued int) (row int, below, ok bool) {
	lo, hi := 0, queued
	if s.ctx {
		lo, hi = queued, n
	}
	at := max(lo, min(s.at, hi))
	switch {
	case at < hi:
		return at, false, true
	case hi > lo:
		return hi - 1, true, true
	case !s.ctx:
		return 0, false, false
	case n > 0:
		return n - 1, true, true
	}
	return 0, false, false
}

// dropOn makes the empty panel a drop target (the queue's start). It goes
// under the content: on top it would take the button's clicks.
func (p *panel) dropOn(gtx layout.Context, next []backend.QueueTrack, queued int) {
	p.drop.Group = queueGroup
	if pl, code := p.drop.Add(gtx, gtx.Constraints.Max, spot{}.code()); pl != nil {
		p.dropAt(pl, decode(code), next, queued)
	}
}

// dropAt applies a drop at s.
func (p *panel) dropAt(pl *ui.DragPayload, s spot, next []backend.QueueTrack, queued int) {
	if p.res.V == nil {
		ui.Go("Couldn't add to queue", func() error { return ui.B.AddToQueue(uris(pl.Tracks)) }, nil)
		return
	}
	out, appended := dropInto(next, queued, pl.Tracks, pl.QueueIndex, s, newUID)
	switch {
	case out == nil:
	case appended:
		// Plain adds at the end go through add_to_queue, which any device
		// takes, remote ones included.
		p.res.V.Next = out
		ui.Go("Couldn't add to queue", func() error { return ui.B.AddToQueue(uris(pl.Tracks)) }, nil)
	default:
		p.set(out)
	}
}

// dropInto is the upcoming list after a drop at s, where the first queued
// entries are the user queue and the rest the context. A track dragged from
// the list (from >= 0) moves, joining the section it lands in; tracks from
// elsewhere are put in. appended reports a plain add at the end of the queue;
// out is nil when nothing changes.
func dropInto(next []backend.QueueTrack, queued int, tracks []spotify.Track, from int, s spot, uid func() string) (out []backend.QueueTrack, appended bool) {
	rest, at := next, s.at
	var add []backend.QueueTrack
	if from >= 0 && from < len(next) {
		rest = removeAt(next, from)
		if from < at {
			at--
		}
		if from < queued {
			queued--
		}
		q := next[from]
		switch {
		case !s.ctx:
			q.Provider = "queue"
		case q.Provider == "queue":
			q.Provider = "context"
		}
		add = []backend.QueueTrack{q}
	} else {
		provider := "queue"
		if s.ctx {
			provider = "context"
		}
		for _, t := range tracks {
			t.UID = uid()
			add = append(add, backend.QueueTrack{Track: t, Provider: provider})
		}
	}
	if s.ctx {
		at = max(queued, min(at, len(rest)))
	} else {
		at = max(0, min(at, queued))
	}
	out = make([]backend.QueueTrack, 0, len(rest)+len(add))
	out = append(out, rest[:at]...)
	out = append(out, add...)
	out = append(out, rest[at:]...)
	if from >= 0 && slices.EqualFunc(out, next, func(a, b backend.QueueTrack) bool {
		return a.UID == b.UID && a.URI == b.URI && a.Provider == b.Provider
	}) {
		return nil, false
	}
	return out, from < 0 && !s.ctx && at == queued
}

func uris(ts []spotify.Track) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.URI
	}
	return out
}

// newUID names a track queued here (the cluster tells queue entries apart
// by uid).
func newUID() string { return "q" + strconv.FormatInt(time.Now().UnixNano(), 36) }

func (p *panel) recentBody(gtx layout.Context) layout.Dimensions {
	switch {
	case !p.recentRes.Done:
		return ui.Loading(gtx)
	case p.recentRes.Err != nil:
		return ui.Unavailable(gtx, "Recently played", p.recentRes.Err)
	}
	ts := p.recentRes.V
	if len(ts) == 0 {
		return historyEmpty(gtx)
	}
	if len(p.recentRows) < len(ts) {
		p.recentRows = make([]row, len(ts))
	}
	return p.recentList.Layout(gtx, len(ts), func(gtx layout.Context, i int) layout.Dimensions {
		return ui.Pad(gtx, 0, 8, 0, 8, func(gtx layout.Context) layout.Dimensions {
			return p.recentRows[i].layout(gtx, ts[i], ui.IsCurrentTrack(ts[i], ""), nil)
		})
	})
}

// historyEmpty is the desktop client's empty "Recently played" tab.
func historyEmpty(gtx layout.Context) layout.Dimensions {
	return ui.Pad(gtx, 64, 24, 0, 24, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.Icon(gtx, "clock", 64, ui.ColText2) }),
			layout.Rigid(ui.Gap(0, 16)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Label{Text: "See what you've listened to", Size: 16, Weight: ui.Bold, Align: text.Middle, MaxLines: 2}.Layout(gtx)
			}),
			layout.Rigid(ui.Gap(0, 4)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Label{Text: "Your listening history will appear here", Size: 14, Color: ui.ColText2, Align: text.Middle, MaxLines: 2}.Layout(gtx)
			}),
		)
	})
}

// layoutItems orders the list: playing, user queue, then the rest.
func layoutItems(playing bool, next []backend.QueueTrack) []item {
	var items []item
	if playing {
		items = append(items, item{kind: itemNowHead}, item{kind: itemNow})
	}
	for _, kind := range []int{itemQueuedHead, itemNextHead} {
		start := len(items)
		items = append(items, item{kind: kind})
		for i, q := range next {
			if (q.Provider == "queue") == (kind == itemQueuedHead) {
				items = append(items, item{kind: itemTrack, i: i})
			}
		}
		if len(items) == start+1 {
			items = items[:start]
		}
	}
	return items
}

// slideTime is how long a moved row takes to reach its new place.
const slideTime = 250 * time.Millisecond

// key names an upcoming entry for the slide.
func key(q backend.QueueTrack) string {
	if q.UID != "" {
		return q.UID
	}
	return q.URI
}

// rowTops is the y (px) of each track row of the list items for next.
func (p *panel) rowTops(next []backend.QueueTrack) map[string]int {
	out := map[string]int{}
	y := 0
	for _, it := range layoutItems(ui.NowTrack() != nil, next) {
		switch it.kind {
		case itemTrack:
			out[key(next[it.i])] = y
			y += p.rowPx
		case itemNow:
			y += p.rowPx
		default:
			y += p.headPx
		}
	}
	return out
}

// startSlide sets the rows that move between old and next sliding.
func (p *panel) startSlide(old, next []backend.QueueTrack) {
	from, to := p.rowTops(old), p.rowTops(next)
	p.slides = map[string]int{}
	for k, y := range to {
		if y0, ok := from[k]; ok && y0 != y {
			p.slides[k] = y0 - y
		}
	}
	p.slideAt = time.Now()
}

// slide is row k's offset now, easing out to 0.
func (p *panel) slide(gtx layout.Context, k string) int {
	d, ok := p.slides[k]
	if !ok {
		return 0
	}
	t := float64(gtx.Now.Sub(p.slideAt)) / float64(slideTime)
	if t >= 1 {
		delete(p.slides, k)
		return 0
	}
	gtx.Execute(op.InvalidateCmd{})
	return int(float64(d) * math.Pow(1-t, 3))
}

// set shows next at once and sends it to the cluster.
func (p *panel) set(next []backend.QueueTrack) {
	p.startSlide(p.res.V.Next, next)
	p.res.V.Next = next
	ui.Go("Couldn't update queue", func() error { return ui.B.SetQueue(queueItems(next)) }, nil)
}

// skipPast drops the upcoming tracks up to and including i, the one starting.
func (p *panel) skipPast(i int) {
	old := p.res.V.Next
	p.startSlide(old, old[i+1:])
	p.res.V.Next = old[i+1:]
}

// upcomingIndex is where track uri first comes up in next, or -1.
func upcomingIndex(next []backend.QueueTrack, uri string) int {
	for i, q := range next {
		if q.URI == uri || q.LinkedURI == uri {
			return i
		}
	}
	return -1
}

func moveTo(next []backend.QueueTrack, from, to int) []backend.QueueTrack {
	out := removeAt(next, from)
	out = append(out, backend.QueueTrack{})
	copy(out[to+1:], out[to:])
	out[to] = next[from]
	return out
}

func removeAt(next []backend.QueueTrack, i int) []backend.QueueTrack {
	out := make([]backend.QueueTrack, 0, len(next))
	out = append(out, next[:i]...)
	return append(out, next[i+1:]...)
}

func clearQueued(next []backend.QueueTrack) []backend.QueueTrack {
	var out []backend.QueueTrack
	for _, q := range next {
		if q.Provider != "queue" {
			out = append(out, q)
		}
	}
	return out
}

func queueItems(next []backend.QueueTrack) []spotify.QueueItem {
	out := make([]spotify.QueueItem, len(next))
	for i, q := range next {
		out[i] = spotify.QueueItem{URI: q.URI, UID: q.UID, Provider: q.Provider}
	}
	return out
}

func heading(gtx layout.Context, left, right layout.Widget) layout.Dimensions {
	return ui.Pad(gtx, 16, 8, 8, 8, func(gtx layout.Context) layout.Dimensions {
		kids := []layout.FlexChild{layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			d := left(gtx)
			d.Size.X = gtx.Constraints.Max.X
			return d
		})}
		if right != nil {
			kids = append(kids, layout.Rigid(right))
		}
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, kids...)
	})
}

// empty is shown when nothing is playing and nothing is queued: centered
// text and a "Find something to play" button, sitting 40dp below the middle.
func (p *panel) empty(gtx layout.Context) layout.Dimensions {
	if p.find.Click.Clicked(gtx) {
		ui.Navigate("/search")
	}
	size := gtx.Constraints.Max
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(size.X-2*ui.Dp(gtx, 16), size.Y)}
	d := ui.Column(g,
		func(gtx layout.Context) layout.Dimensions {
			// Align centers the lines within the text's own width; N centers
			// that block in the panel.
			return layout.N.Layout(gtx, ui.Label{Text: "Add to your queue", Size: 24, Weight: ui.Bold, MaxLines: 2, Align: text.Middle, LineHeight: 29}.Layout)
		},
		ui.Gap(0, 8),
		func(gtx layout.Context) layout.Dimensions {
			return layout.N.Layout(gtx, ui.Label{Text: "Tap \"Add to queue\" from a track's menu to see it here", Size: 16, Color: ui.ColText2, MaxLines: 3, Align: text.Middle, LineHeight: 22}.Layout)
		},
		ui.Gap(0, 40),
		func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return p.find.Layout(gtx, ui.PillStyle{Text: "Find something to play", Size: 16, Weight: ui.Bold, Height: 48, PadX: 32, Bg: ui.ColText, Fg: ui.ColBg})
			})
		},
	)
	call := m.Stop()
	ui.Offset(gtx, ui.Dp(gtx, 16), (size.Y-d.Size.Y)/2+ui.Dp(gtx, 40), func() { call.Add(gtx.Ops) })
	return layout.Dimensions{Size: size}
}

// row is one 56dp track row.
type row struct {
	click, play ui.Click
	title       ui.Link
	artists     []ui.Link
	drag        ui.TrackDrag
	// above and below take drops on the row's halves: before it, after it.
	above, below ui.DropTarget
	remove       ui.IconButton // shown on hover for upcoming rows
}

// hooks make an upcoming row draggable (to reorder the queue) and a drop
// target, and a row of the user queue removable.
type hooks struct {
	index        int // in the upcoming tracks
	remove       func()
	line         int  // draw the drop line above (-1) or below (1) this row
	above, below spot // where drops on the row's halves land
	onDrop       func(pl *ui.DragPayload, code int)
}

func (r *row) layout(gtx layout.Context, t spotify.Track, current bool, h *hooks) layout.Dimensions {
	var remove func()
	if h != nil {
		remove = h.remove
	}
	if r.play.Clicked(gtx) || r.click.DoubleClicked(gtx) {
		if h != nil { // the row itself, not an earlier copy of its track
			p.playing = t.URI
			p.skipPast(h.index)
		}
		playTrack(t, current, h != nil)
	}
	if r.title.Click.Clicked(gtx) && t.Album != nil {
		ui.OpenURI(t.Album.URI)
	}
	if len(r.artists) < len(t.Artists) {
		r.artists = make([]ui.Link, len(t.Artists))
	}
	for i, a := range t.Artists {
		if r.artists[i].Click.Clicked(gtx) {
			ui.OpenURI(a.URI)
		}
	}
	if r.remove.Click.Clicked(gtx) && remove != nil {
		remove()
	}
	if _, ok := r.click.RightClicked(gtx); ok {
		ui.OpenMenu(ui.TrackMenu(t, ui.TrackMenuOpts{RemoveFromQue: remove}))
	}

	w, rh := gtx.Constraints.Max.X, ui.Dp(gtx, rowDp)
	r.click.Label, r.click.Arrow = t.Name, true
	return r.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hov := r.click.Hovered() || r.play.Hovered() || r.remove.Click.Hovered()
		if hov {
			ui.FillRect(gtx.Ops, image.Rect(0, 0, w, rh), ui.Dp(gtx, 4), ui.ColRowHover)
		}
		is, pad := ui.Dp(gtx, 40), ui.Dp(gtx, 8)
		var imgs []spotify.Image
		if t.Album != nil {
			imgs = t.Album.Images
		}
		ui.Offset(gtx, pad, pad, func() {
			r.play.Label = "Play " + t.Name
			r.play.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				ui.Image{URL: ui.Pick(imgs, is), Radius: ui.Dp(gtx, 4), Icon: "note"}.Layout(gtx, image.Pt(is, is))
				if hov {
					ui.FillRect(gtx.Ops, image.Rect(0, 0, is, is), ui.Dp(gtx, 4), ui.Alpha(ui.ColBg, .55))
					icon := "play"
					if current && ui.Playing() {
						icon = "pause"
					}
					ui.Offset(gtx, (is-ui.Dp(gtx, 16))/2, (is-ui.Dp(gtx, 16))/2, func() { ui.IconPx(gtx, icon, ui.Dp(gtx, 16), ui.ColText) })
				}
				return layout.Dimensions{Size: image.Pt(is, is)}
			})
		})
		tx := pad + is + ui.Dp(gtx, 12)
		g := gtx
		bw := 0 // the remove button's room
		if hov && remove != nil {
			bw = ui.Dp(gtx, 32+8)
		}
		g.Constraints = layout.Constraints{Max: image.Pt(max(0, w-tx-pad-bw), rh)}
		m := op.Record(gtx.Ops)
		d := r.text(g, t, current)
		call := m.Stop()
		ui.Offset(gtx, tx, (rh-d.Size.Y)/2, func() { call.Add(gtx.Ops) })
		if bw > 0 {
			b := ui.Dp(gtx, 32)
			ui.Offset(gtx, w-pad-b, (rh-b)/2, func() {
				r.remove.Layout(gtx, ui.IconStyle{Icon: "close", Label: "Remove from queue"})
			})
		}
		if h != nil && h.line != 0 {
			lh := ui.Dp(gtx, 2)
			y := 0
			if h.line > 0 {
				y = rh - lh
			}
			ui.FillRect(gtx.Ops, image.Rect(0, y, w, y+lh), 0, ui.ColAccent)
		}
		pl := ui.TrackPayload(t)
		if h != nil {
			pl.QueueIndex = h.index
		}
		r.drag.Handle(gtx, pl, nil, 0, rh, nil)
		if h != nil {
			r.above.Group, r.below.Group = queueGroup, queueGroup
			half := rh / 2
			if pl, code := r.above.Add(gtx, image.Pt(w, half), h.above.code()); pl != nil {
				h.onDrop(pl, code)
			}
			ui.Offset(gtx, 0, half, func() {
				if pl, code := r.below.Add(gtx, image.Pt(w, rh-half), h.below.code()); pl != nil {
					h.onDrop(pl, code)
				}
			})
		}
		return layout.Dimensions{Size: image.Pt(w, rh)}
	})
}

// text is the title and artists, vertically centered.
func (r *row) text(gtx layout.Context, t spotify.Track, current bool) layout.Dimensions {
	col := ui.ColText
	if current {
		col = ui.ColAccent
	}
	var segs []ui.Seg
	if t.Explicit {
		segs = append(segs, ui.Seg{Widget: ui.ExplicitBadge})
	}
	for i, a := range t.Artists {
		if i > 0 {
			segs = append(segs, ui.Seg{Text: ", "})
		}
		segs = append(segs, ui.Seg{Text: a.Name, Link: &r.artists[i]})
	}
	return ui.Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return r.title.Layout(gtx, t.Name, 16, ui.Regular, col, ui.ColNone)
		},
		func(gtx layout.Context) layout.Dimensions {
			return ui.Inline(gtx, 14, ui.Regular, ui.ColText2, ui.ColText, segs...)
		},
	)
}

// playTrack plays a row's track: an upcoming one is skipped to, keeping the
// context (it may not even be in it: a queued song).
func playTrack(t spotify.Track, current, upcoming bool) {
	switch s := ui.Player.Status; {
	case current:
		ui.PlayPause()
	case upcoming:
		ui.SkipTo(t)
	case s != nil && s.ContextURI != "":
		ui.PlayTrack(s.ContextURI, t)
	default:
		ui.PlayTrack("", t)
	}
}
