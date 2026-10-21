// Package queue is the Queue panel (Prefs.RightPanel "queue"): the playing
// track, what the user queued and what's next from the context.
package queue

import (
	"image"
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

	// The "Recently played" tab.
	recent           bool
	tabQueue, tabRec ui.Click
	recentRes        ui.Res[[]spotify.Track]
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
		p.recentRes.Load(ui.B.RecentlyPlayed)
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
	} else if p.res.Err != nil {
		ui.Log.Warn("queue", "err", p.res.Err)
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
	// A heading or the space below the list takes a drop at the start of the
	// user queue (the headings above it) or at its end.
	headDrop := func(gtx layout.Context, n int, d layout.Dimensions, spot int) layout.Dimensions {
		p.heads[n].Group = queueGroup
		if pl, at := p.heads[n].Add(gtx, d.Size, spot); pl != nil {
			p.dropAt(pl, at, next, queued)
		}
		return d
	}
	return p.list.Layout(gtx, len(items)+1, func(gtx layout.Context, n int) layout.Dimensions {
		if n == len(items) {
			// Some room under the last row to drop onto.
			return headDrop(gtx, n, layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, ui.Dp(gtx, 80))}, queued)
		}
		if k := items[n].kind; k != itemTrack {
			spot := 0
			if k == itemNextHead {
				spot = queued
			}
			return headDrop(gtx, n, p.item(gtx, n, items, next, t, queued, ctxName), spot)
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
			return heading(gtx, func(gtx layout.Context) layout.Dimensions {
				if ctxName == "" {
					return ui.Text(gtx, "Next up", 16, ui.Bold, ui.ColText)
				}
				return ui.Inline(gtx, 16, ui.Bold, ui.ColText, ui.ColText,
					ui.Seg{Text: "Next from: "}, ui.Seg{Text: ctxName, Link: &p.ctxLink})
			}, nil)
		default:
			q := next[it.i]
			// The drop line goes where a drop would land: over a row of
			// the queue, or at its end for anything below it.
			line := 0
			if spot, ok := ui.DropHover(queueGroup); ok {
				switch at := min(spot, queued); {
				case at == it.i:
					line = -1
				case at == it.i+1 && it.i+1 == len(next):
					line = 1
				}
			}
			h := &hooks{
				index: it.i,
				line:  line,
				onDrop: func(pl *ui.DragPayload, spot int) {
					p.dropAt(pl, spot, next, queued)
				},
			}
			if q.Provider == "queue" {
				h.remove = func() { p.set(removeAt(next, it.i)) }
			}
			return p.rows[n].layout(gtx, q.Track, false, h)
		}
	})
}

const queueGroup = "queue"

// dropOn makes the empty panel a drop target (the queue's start). It goes
// under the content: on top it would take the button's clicks.
func (p *panel) dropOn(gtx layout.Context, next []backend.QueueTrack, queued int) {
	p.drop.Group = queueGroup
	if pl, spot := p.drop.Add(gtx, gtx.Constraints.Max, queued); pl != nil {
		p.dropAt(pl, spot, next, queued)
	}
}

// dropAt applies a drop before upcoming track spot.
func (p *panel) dropAt(pl *ui.DragPayload, spot int, next []backend.QueueTrack, queued int) {
	if p.res.V == nil {
		ui.Go("Couldn't add to queue", func() error { return ui.B.AddToQueue(uris(pl.Tracks)) }, nil)
		return
	}
	out, appended := dropInto(next, queued, pl.Tracks, pl.QueueIndex, spot, newUID)
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

// dropInto is the upcoming list after dropping tracks before spot, where the
// first queued entries are the user queue. Drops land within the user queue
// (anywhere past it means its end). A track moved from the user queue moves;
// one from the rest of the upcoming list, or from elsewhere, is queued there.
// appended reports a plain add at the end; out is nil when nothing changes.
func dropInto(next []backend.QueueTrack, queued int, tracks []spotify.Track, from, spot int, uid func() string) (out []backend.QueueTrack, appended bool) {
	at := max(0, min(spot, queued))
	if from >= 0 && from < queued {
		if at > from {
			at--
		}
		if at == from {
			return nil, false
		}
		return moveTo(next, from, at), false
	}
	add := make([]backend.QueueTrack, len(tracks))
	for i, t := range tracks {
		t.UID = uid()
		add[i] = backend.QueueTrack{Track: t, Provider: "queue"}
	}
	out = make([]backend.QueueTrack, 0, len(next)+len(add))
	out = append(out, next[:at]...)
	out = append(out, add...)
	out = append(out, next[at:]...)
	return out, at == queued && from < 0
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
	if len(p.recentRows) < len(ts) {
		p.recentRows = make([]row, len(ts))
	}
	return p.recentList.Layout(gtx, len(ts), func(gtx layout.Context, i int) layout.Dimensions {
		return ui.Pad(gtx, 0, 8, 0, 8, func(gtx layout.Context) layout.Dimensions {
			return p.recentRows[i].layout(gtx, ts[i], ui.IsCurrentTrack(ts[i], ""), nil)
		})
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

// set shows next at once and sends it to the cluster.
func (p *panel) set(next []backend.QueueTrack) {
	p.res.V.Next = next
	ui.Go("Couldn't update queue", func() error { return ui.B.SetQueue(queueItems(next)) }, nil)
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
}

// hooks make an upcoming row draggable (to reorder the queue) and a drop
// target, and a row of the user queue removable.
type hooks struct {
	index  int // in the upcoming tracks
	remove func()
	line   int // draw the drop line above (-1) or below (1) this row
	onDrop func(pl *ui.DragPayload, spot int)
}

func (r *row) layout(gtx layout.Context, t spotify.Track, current bool, h *hooks) layout.Dimensions {
	var remove func()
	if h != nil {
		remove = h.remove
	}
	if r.play.Clicked(gtx) || r.click.DoubleClicked(gtx) {
		playTrack(t, current)
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
	if _, ok := r.click.RightClicked(gtx); ok {
		ui.OpenMenu(ui.TrackMenu(t, ui.TrackMenuOpts{RemoveFromQue: remove}))
	}

	w, rh := gtx.Constraints.Max.X, ui.Dp(gtx, rowDp)
	r.click.Label, r.click.Arrow = t.Name, true
	return r.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hov := r.click.Hovered() || r.play.Hovered()
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
		g.Constraints = layout.Constraints{Max: image.Pt(max(0, w-tx-pad), rh)}
		m := op.Record(gtx.Ops)
		d := r.text(g, t, current)
		call := m.Stop()
		ui.Offset(gtx, tx, (rh-d.Size.Y)/2, func() { call.Add(gtx.Ops) })
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
			if pl, spot := r.above.Add(gtx, image.Pt(w, half), h.index); pl != nil {
				h.onDrop(pl, spot)
			}
			ui.Offset(gtx, 0, half, func() {
				if pl, spot := r.below.Add(gtx, image.Pt(w, rh-half), h.index+1); pl != nil {
					h.onDrop(pl, spot)
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

func playTrack(t spotify.Track, current bool) {
	switch s := ui.Player.Status; {
	case current:
		ui.PlayPause()
	case s != nil && s.ContextURI != "":
		ui.PlayTrack(s.ContextURI, t)
	default:
		ui.PlayTrack("", t)
	}
}
