// Package queue is the Queue panel (Prefs.RightPanel "queue"): the playing
// track, what the user queued and what's next from the context.
package queue

import (
	"image"
	"image/color"
	"math"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"

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

	// Reordering the user queue: queue index `from` is dragged dy px.
	dragging bool
	from, dy int
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
	if p.version != ui.Player.Version+1 {
		p.version = ui.Player.Version + 1
		p.res.Load(ui.B.Queue)
	}
	return ui.Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return p.header.Layout(gtx, "Close", func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, "Queue", 16, ui.Bold, ui.ColText)
			}, nil)
		},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
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
	return p.list.Layout(gtx, len(items), func(gtx layout.Context, n int) layout.Dimensions {
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
				if q.Provider != "queue" {
					return p.rows[n].layout(gtx, q.Track, false, nil)
				}
				rowPx := ui.Dp(gtx, rowDp)
				to := dropIndex(p.from, p.dy, rowPx, queued)
				h := &hooks{
					remove: func() { p.set(removeAt(next, it.i)) },
					move:   func(dy int) { p.dragging, p.from, p.dy = true, it.i, dy },
					drop: func() {
						if to := dropIndex(p.from, p.dy, rowPx, queued); p.dragging && to != p.from {
							p.set(moveTo(next, p.from, to))
						}
						p.dragging = false
					},
					dim: p.dragging && p.from == it.i,
				}
				if p.dragging && to != p.from && to == it.i {
					h.marker = 1
					if to < p.from {
						h.marker = -1
					}
				}
				return p.rows[n].layout(gtx, q.Track, false, h)
			}
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

// dropIndex is where row `from` of the user queue lands when dragged dy px.
func dropIndex(from, dy, rowPx, queued int) int {
	return max(0, min(queued-1, from+int(math.Round(float64(dy)/float64(rowPx)))))
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

// empty is shown when nothing is playing and nothing is queued.
func (p *panel) empty(gtx layout.Context) layout.Dimensions {
	if p.find.Click.Clicked(gtx) {
		ui.Navigate("/search")
	}
	return ui.Pad(gtx, 24, 24, 0, 24, func(gtx layout.Context) layout.Dimensions {
		return ui.Column(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return ui.Pad(gtx, 0, 0, 8, 0, func(gtx layout.Context) layout.Dimensions {
					return ui.Label{Text: "Add to your queue", Size: 16, Weight: ui.Bold, MaxLines: 2}.Layout(gtx)
				})
			},
			func(gtx layout.Context) layout.Dimensions {
				return ui.Pad(gtx, 0, 0, 24, 0, func(gtx layout.Context) layout.Dimensions {
					return ui.Label{Text: "Tap \"Add to queue\" from a track's menu to see it here", Size: 14, Color: ui.ColText2, MaxLines: 3}.Layout(gtx)
				})
			},
			func(gtx layout.Context) layout.Dimensions {
				return p.find.Layout(gtx, ui.PillStyle{Text: "Find something to play", Weight: ui.Bold, Height: 48, PadX: 32, Bg: ui.ColText, Fg: ui.ColBg})
			},
		)
	})
}

// row is one 56dp track row.
type row struct {
	click, play ui.Click
	title       ui.Link
	artists     []ui.Link
	drag        gesture.Drag
	y0          float32
}

// hooks make a row of the user queue removable and draggable.
type hooks struct {
	remove func()
	move   func(dy int) // while dragging, dy px from where it was grabbed
	drop   func()
	dim    bool // this row is the one being dragged
	marker int  // draw the drop line above (-1) or below (1) this row
}

func (r *row) layout(gtx layout.Context, t spotify.Track, current bool, h *hooks) layout.Dimensions {
	var remove func()
	if h != nil {
		remove = h.remove
		for {
			e, ok := r.drag.Update(gtx.Metric, gtx.Source, gesture.Vertical)
			if !ok {
				break
			}
			switch e.Kind {
			case pointer.Press:
				r.y0 = e.Position.Y
			case pointer.Drag:
				h.move(int(e.Position.Y - r.y0))
			case pointer.Release, pointer.Cancel:
				h.drop()
			}
		}
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
		if h != nil {
			r.drag.Add(gtx.Ops)
		}
		hov := r.click.Hovered() || r.play.Hovered()
		if hov || h != nil && h.dim {
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
					ui.FillRect(gtx.Ops, image.Rect(0, 0, is, is), ui.Dp(gtx, 4), color.NRGBA{A: 140})
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
		if h != nil && h.marker != 0 {
			lh := ui.Dp(gtx, 2)
			y := 0
			if h.marker > 0 {
				y = rh - lh
			}
			ui.FillRect(gtx.Ops, image.Rect(0, y, w, y+lh), 0, ui.ColAccent)
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
		ui.Play(s.ContextURI, t.URI)
	default:
		ui.Play(t.URI, "")
	}
}
