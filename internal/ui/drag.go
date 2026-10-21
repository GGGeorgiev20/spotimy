package ui

import (
	"image"
	"io"
	"math"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// Reorder is the shared state of a drag-to-reorder over fixed-height rows:
// the dragged row and where it would land (above or below row To).
type Reorder struct {
	From, To int
	After    bool
	Active   bool
}

// Noop reports a drop that wouldn't move anything.
func (r *Reorder) Noop() bool {
	return r.To == r.From || r.After && r.To == r.From-1 || !r.After && r.To == r.From+1
}

// DropLine is the target row's side the accent line goes on: -1 above, 1 below, 0 none.
func (r *Reorder) DropLine(i int) int {
	switch {
	case !r.Active || r.Noop() || r.To != i || dropHover.t != nil:
		return 0
	case r.After:
		return 1
	}
	return -1
}

// TracksMime is the transfer type of dragged tracks.
const TracksMime = "application/x-personal-spotify-tracks"

// DragPayload is what a dragged row carries.
type DragPayload struct {
	Tracks []spotify.Track
	Label  string // shown next to the pointer while dragging
	// QueueIndex is the row's index in the queue's upcoming tracks when the
	// drag starts there, -1 otherwise.
	QueueIndex int
}

// TrackPayload is what a track row carries when dragged.
func TrackPayload(t spotify.Track) DragPayload {
	label := t.Name
	if len(t.Artists) > 0 {
		label += " • " + t.Artists[0].Name
	}
	return DragPayload{Tracks: []spotify.Track{t}, Label: label, QueueIndex: -1}
}

// dragged is the payload of the last drag that went past the click slop;
// targets act on it when it's dropped on them. nil after a press that
// didn't become a drag, so a click never drops anything.
var dragged *DragPayload

// dropHover is the innermost drop target under a drag, and the spot within
// it the drop would land on.
var dropHover struct {
	t    *DropTarget
	spot int
}

// DropHover reports the spot a drag over one of group's targets would land on.
func DropHover(group string) (int, bool) {
	if dragged == nil || dropHover.t == nil || dropHover.t.Group != group {
		return 0, false
	}
	return dropHover.spot, true
}

// TrackDrag is a track row's drag handle: dragging the row carries its tracks
// to drop targets (the queue), and, given a Reorder, also moves the row
// within its own list. Call Handle after drawing the row, in the row's own
// coordinate space.
type TrackDrag struct {
	pressed, active bool
	start, pos      f32.Point
	payload         DragPayload
	laidOut         time.Time // the frame Handle last ran in
}

// dragSource is the row whose drag Gio is transferring from. It has to stay
// registered until the transfer ends: Gio dereferences the source's handler
// on every frame of it, and crashes if the row left the screen meanwhile (a
// song started playing and the queue changed, say). KeepDragSource keeps it.
var dragSource *TrackDrag

// Handle feeds the drag of row i, rowH px high and as wide as the
// constraints. ro and move, when set, reorder the list on a drop inside it.
func (d *TrackDrag) Handle(gtx layout.Context, p DragPayload, ro *Reorder, i, rowH int, move func(from, to int, after bool)) {
	d.laidOut = gtx.Now
	d.update(gtx, &p, ro, i, rowH, move)
	// The row's own area (the clip in force may be the whole list's), on top
	// of the row's links and passing their events through: a press on a
	// link has to start a drag too.
	pass := pointer.PassOp{}.Push(gtx.Ops)
	st := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, rowH)}.Push(gtx.Ops)
	event.Op(gtx.Ops, d)
	st.Pop()
	pass.Pop()
	if d.pressed {
		d.everywhere(gtx)
	}
	if d.active {
		d.ghost(gtx)
	}
}

// KeepDragSource keeps the drag source registered while Gio transfers from
// it when its row is no longer laid out, and draws the ghost in its place.
// Call it once per frame, after everything else, at the window's origin.
// (While the row is there, the row registers it: event positions are in the
// coordinates of the handler's last registration, and the row's own are
// the ones its drag works in.)
func KeepDragSource(gtx layout.Context) {
	d := dragSource
	if d == nil || d.laidOut.Equal(gtx.Now) {
		return
	}
	d.update(gtx, nil, nil, 0, 0, nil)
	if dragSource == d {
		d.everywhere(gtx)
		if d.active {
			d.ghost(gtx)
		}
	}
}

// everywhere registers the handler over the whole window, on top and passing
// everything through. Gio makes a drag's source the handler the pointer is
// over when it first moves, and a quick flick can be past the row by then.
func (d *TrackDrag) everywhere(gtx layout.Context) {
	m := op.Record(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	big := clip.Rect{Min: image.Pt(-1<<20, -1<<20), Max: image.Pt(1<<20, 1<<20)}.Push(gtx.Ops)
	event.Op(gtx.Ops, d)
	big.Pop()
	pass.Pop()
	op.Defer(gtx.Ops, m.Stop())
}

// update handles the handler's events; p is nil from KeepDragSource.
func (d *TrackDrag) update(gtx layout.Context, p *DragPayload, ro *Reorder, i, rowH int, move func(from, to int, after bool)) {
	for {
		filters := []event.Filter{pointer.Filter{Target: d, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel}}
		// Only a drag past the click slop offers data: Gio starts a
		// transfer on the first movement, and a click with a jitter
		// shouldn't become one.
		if d.active || dragSource == d {
			filters = append(filters, transfer.SourceFilter{Target: d, Type: TracksMime})
		}
		e, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		switch e := e.(type) {
		case pointer.Event:
			switch e.Kind {
			case pointer.Press:
				d.pressed, d.active = e.Buttons.Contain(pointer.ButtonPrimary), false
				d.start, d.pos = e.Position, e.Position
				if d.pressed {
					dragged = nil
					if dragSource != d {
						// A drag that never became Gio's transfer gets no
						// CancelEvent; any real one ended with its release.
						dragSource = nil
					}
					gtx.Execute(op.InvalidateCmd{}) // to take the whole window
				}
			case pointer.Drag:
				if !d.pressed {
					continue
				}
				d.pos = e.Position
				if !d.active && p != nil && math.Hypot(float64(d.pos.X-d.start.X), float64(d.pos.Y-d.start.Y)) >= float64(gtx.Dp(4)) {
					d.active, d.payload = true, *p
					dragged, dragSource = &d.payload, d
				}
				if d.active && ro != nil && move != nil {
					rel := e.Position.Y / float32(rowH)
					row := int(math.Floor(float64(rel)))
					*ro = Reorder{From: i, To: max(0, i+row), After: rel-float32(row) >= .5, Active: true}
				}
				gtx.Execute(op.InvalidateCmd{}) // rows laid out before this one, and targets, need redrawing
			case pointer.Release, pointer.Cancel:
				// A drop on a target (the queue) isn't a move within the list.
				if d.active && ro != nil && ro.Active && ro.From == i {
					if e.Kind == pointer.Release && !ro.Noop() && dropHover.t == nil {
						move(ro.From, ro.To, ro.After)
					}
					*ro = Reorder{}
				}
				if e.Kind == pointer.Cancel {
					dragged = nil
				}
				d.pressed, d.active = false, false
				gtx.Execute(op.InvalidateCmd{})
			}
		case transfer.RequestEvent:
			// The drop target acts on dragged; the data is the URIs for any
			// other taker.
			uris := make([]string, len(d.payload.Tracks))
			for k, t := range d.payload.Tracks {
				uris[k] = t.URI
			}
			gtx.Execute(transfer.OfferCmd{Tag: d, Type: e.Type, Data: io.NopCloser(strings.NewReader(strings.Join(uris, "\n")))})
		case transfer.CancelEvent:
			// The transfer is over (dropped, or released over no target).
			if dragSource == d {
				dragSource = nil
			}
		}
	}
}

// ghost draws the dragged track's name in a small dark box by the pointer,
// above everything else.
func (d *TrackDrag) ghost(gtx layout.Context) {
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(Dp(gtx, 320), Dp(gtx, 40))}
	tm := op.Record(gtx.Ops)
	td := Text(g, d.payload.Label, 14, Bold, ColText)
	text := tm.Stop()
	px, py := Dp(gtx, 12), Dp(gtx, 8)
	box := image.Rect(0, 0, td.Size.X+2*px, td.Size.Y+2*py)
	Shadow(gtx.Ops, box, Dp(gtx, 4), Dp(gtx, 16), Dp(gtx, 4), .5)
	FillRect(gtx.Ops, box, Dp(gtx, 4), RGB(0x282828))
	Offset(gtx, px, py, func() { text.Add(gtx.Ops) })
	call := m.Stop()
	st := op.Offset(image.Pt(int(d.pos.X)+Dp(gtx, 12), int(d.pos.Y)+Dp(gtx, 12))).Push(gtx.Ops)
	op.Defer(gtx.Ops, call)
	st.Pop()
}

// DropTarget is an area that takes dragged tracks.
type DropTarget struct {
	Group string // the owner's name, for DropHover
}

// Add makes the current clip area, size px from the origin, a drop target
// (passing other events through). spot names
// the place in the target the area stands for. On a drop it returns the
// payload and the spot of the innermost target the pointer was over.
func (t *DropTarget) Add(gtx layout.Context, size image.Point, spot int) (*DragPayload, int) {
	var dropped *DragPayload
	at := spot
	for {
		e, ok := gtx.Event(
			pointer.Filter{Target: t, Kinds: pointer.Enter | pointer.Leave | pointer.Cancel},
			transfer.TargetFilter{Target: t, Type: TracksMime},
		)
		if !ok {
			break
		}
		switch e := e.(type) {
		case pointer.Event:
			switch {
			case e.Kind == pointer.Enter && dragged != nil:
				dropHover.t, dropHover.spot = t, spot
			case e.Kind != pointer.Enter && dropHover.t == t:
				dropHover.t = nil
			}
			gtx.Execute(op.InvalidateCmd{})
		case transfer.DataEvent:
			if rc := e.Open(); rc != nil {
				_ = rc.Close()
			}
			if dragged != nil {
				dropped = dragged
				if dropHover.t != nil {
					at = dropHover.spot
				}
			}
			dragged, dropHover.t = nil, nil
		case transfer.CancelEvent:
			if dropHover.t == t {
				dropHover.t = nil
			}
		}
	}
	if dropHover.t == t {
		dropHover.spot = spot // the list may have moved under the pointer
	}
	pass := pointer.PassOp{}.Push(gtx.Ops)
	st := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, t)
	st.Pop()
	pass.Pop()
	return dropped, at
}
