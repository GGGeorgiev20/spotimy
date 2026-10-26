package ui

import (
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// Click makes an area clickable: hover, left click, double click and right
// click. Gio delivers a press to every clickable under the pointer (the
// innermost and all its ancestors); like the DOM's stopPropagation, only the
// innermost one that cares about that button reports it.
type Click struct {
	gesture gesture.Click
	depth   int

	clicks     int
	double     bool
	right      bool
	rightAt    image.Point
	wantsRight bool
	pressAt    time.Duration // Time of the last primary press seen
	hoverSince time.Time
	// at is the pointer's last position over it, in its own coordinates:
	// with PointerPos it places the clickable in the window.
	at f32.Point

	// Label is the accessible name (also how the test harness finds it).
	Label string
	// Cursor defaults to the pointer hand; Arrow keeps the default arrow.
	Cursor pointer.Cursor
	Arrow  bool
	// Disabled ignores input and shows the default cursor.
	Disabled bool
}

var (
	clickDepth int
	// deepest clickable that saw a press, by event time and button.
	pressOwners = map[pressKey]int{}
	// Pointer position in window coordinates, tracked at the root.
	pointerPos f32.Point
)

type pressKey struct {
	t     time.Duration
	right bool
}

func claimPress(k pressKey, depth int) {
	if d, ok := pressOwners[k]; !ok || depth > d {
		pressOwners[k] = depth
	}
	if len(pressOwners) > 64 {
		for key := range pressOwners {
			if key.t < k.t-10*time.Second {
				delete(pressOwners, key)
			}
		}
	}
}

// PointerPos is the last pointer position in window coordinates.
func PointerPos() image.Point { return pointerPos.Round() }

func (c *Click) update(gtx layout.Context) {
	for {
		e, ok := gtx.Event(pointer.Filter{Target: c, Kinds: pointer.Press | pointer.Move | pointer.Enter})
		if !ok {
			break
		}
		pe := e.(pointer.Event)
		c.at = pe.Position
		if pe.Kind != pointer.Press || c.Disabled {
			continue
		}
		switch {
		case pe.Buttons.Contain(pointer.ButtonPrimary) || pe.Source != pointer.Mouse:
			c.pressAt = pe.Time
			claimPress(pressKey{pe.Time, false}, c.depth)
		case pe.Buttons.Contain(pointer.ButtonSecondary) && c.wantsRight:
			// Right clicks fire on press, like the browser's contextmenu event.
			// The innermost interested clickable has the highest depth; deeper
			// ones were laid out (and handled this event) before us only if
			// they're siblings, so defer the decision to the next frame.
			claimPress(pressKey{pe.Time, true}, c.depth)
			k := pressKey{pe.Time, true}
			at := PointerPos()
			Post(func() {
				if pressOwners[k] == c.depth {
					c.right, c.rightAt = true, at
				}
			})
		}
	}
	for {
		e, ok := c.gesture.Update(gtx.Source)
		if !ok {
			break
		}
		if e.Kind != gesture.KindClick || c.Disabled {
			continue
		}
		if d, ok := pressOwners[pressKey{c.pressAt, false}]; ok && d > c.depth {
			continue // an inner clickable took it
		}
		if e.NumClicks >= 2 {
			c.double = true
		} else {
			c.clicks++
		}
	}
}

// Clicked reports (once) a left click since the last call.
func (c *Click) Clicked(gtx layout.Context) bool {
	c.update(gtx)
	if c.clicks > 0 {
		c.clicks--
		return true
	}
	return false
}

// DoubleClicked reports (once) a double click. The first click of the pair
// was also reported by Clicked.
func (c *Click) DoubleClicked(gtx layout.Context) bool {
	c.update(gtx)
	d := c.double
	c.double = false
	return d
}

// RightClicked reports (once) a right click and where it happened in window
// coordinates. Only clickables that call it take part in right clicks.
func (c *Click) RightClicked(gtx layout.Context) (image.Point, bool) {
	c.wantsRight = true
	c.update(gtx)
	r := c.right
	c.right = false
	return c.rightAt, r
}

// origin is where the clickable's top-left corner is in the window, from
// where the pointer is over it.
func (c *Click) origin() image.Point { return pointerPos.Sub(c.at).Round() }

func (c *Click) Hovered() bool { return !c.Disabled && c.gesture.Hovered() }
func (c *Click) Pressed() bool { return !c.Disabled && c.gesture.Pressed() }

// Layout lays out w and makes its area clickable.
func (c *Click) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	c.update(gtx)
	clickDepth++
	c.depth = clickDepth
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	clickDepth--
	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
	if c.Label != "" {
		semantic.DescriptionOp(c.Label).Add(gtx.Ops)
	}
	semantic.ClassOp(semantic.Button).Add(gtx.Ops)
	semantic.EnabledOp(!c.Disabled).Add(gtx.Ops)
	c.gesture.Add(gtx.Ops)
	event.Op(gtx.Ops, c)
	switch {
	case c.Disabled || c.Arrow:
		pointer.CursorDefault.Add(gtx.Ops)
	case c.Cursor != pointer.CursorDefault:
		c.Cursor.Add(gtx.Ops)
	default:
		pointer.CursorPointer.Add(gtx.Ops)
	}
	call.Add(gtx.Ops)
	return dims
}

// trackPointer registers the root handler that follows the pointer in window
// coordinates. Call it at the root, before laying out anything else.
func trackPointer(gtx layout.Context, tag event.Tag) {
	for {
		e, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Move | pointer.Press | pointer.Release | pointer.Drag | pointer.Enter})
		if !ok {
			break
		}
		if pe, ok := e.(pointer.Event); ok {
			pointerPos = pe.Position
			// A press anywhere drops keyboard focus, like the browser; a text
			// field that was hit takes focus again later in this frame.
			if pe.Kind == pointer.Press && !inSearchDropdown(pe.Position.Round()) {
				gtx.Execute(key.FocusCmd{})
			}
		}
	}
	event.Op(gtx.Ops, tag)
}
