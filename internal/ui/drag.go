package ui

import (
	"math"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
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
	case !r.Active || r.Noop() || r.To != i:
		return 0
	case r.After:
		return 1
	}
	return -1
}

// RowDrag is a row's drag handle. Call Handle while laying out the row, in
// the row's own coordinate space.
type RowDrag struct{ g gesture.Drag }

// Handle feeds ro while row i (rowH px high) is dragged; move runs on drop.
func (d *RowDrag) Handle(gtx layout.Context, ro *Reorder, i, rowH int, move func(from, to int, after bool)) {
	for {
		e, ok := d.g.Update(gtx.Metric, gtx.Source, gesture.Vertical)
		if !ok {
			break
		}
		switch e.Kind {
		case pointer.Drag:
			if e.Priority < pointer.Grabbed { // still inside the click slop
				continue
			}
			rel := e.Position.Y / float32(rowH)
			row := int(math.Floor(float64(rel)))
			*ro = Reorder{From: i, To: max(0, i+row), After: rel-float32(row) >= .5, Active: true}
			gtx.Execute(op.InvalidateCmd{}) // rows laid out before this one need the new line
		case pointer.Release, pointer.Cancel:
			if ro.Active && ro.From == i {
				if e.Kind == pointer.Release && !ro.Noop() {
					move(ro.From, ro.To, ro.After)
				}
				*ro = Reorder{}
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
	d.g.Add(gtx.Ops)
}
