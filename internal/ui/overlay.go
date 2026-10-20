package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// ---- Context menus ----

// MenuItem is one row of a context menu.
type MenuItem struct {
	Label  string
	Icon   string
	Action func()
	// Sub opens a flyout submenu on hover. It's called every frame while the
	// flyout is open, so it can show async-loaded rows.
	Sub      func() []MenuItem
	Danger   bool
	Divider  bool // a separator line instead of a row
	Check    bool // check mark on the right (selected sort, playlist has it)
	External bool // external-link icon on the right
	Accent   bool // green label (the selected sort)
	Muted    bool // greyed, not clickable (Loading…)
	// Widget replaces the row (e.g. the "Find a playlist" field).
	Widget layout.Widget
}

type menuLevel struct {
	items  []MenuItem
	sub    func() []MenuItem
	at     image.Point
	clicks []Click
	open   int // index of the item whose flyout is open, -1 none
	rect   image.Rectangle
	rows   []image.Rectangle
	ptr    bool    // opened at the pointer: 8dp off it, flipping to its other side
	minW   float32 // dp
}

var (
	menus   []*menuLevel
	menuTag = new(int)
)

// OpenMenu shows a context menu at the pointer, like a right click: 8dp below
// it, or above / to the left when there is no room.
func OpenMenu(items []MenuItem) {
	OpenMenuAt(PointerPos(), items)
	menus[0].ptr = true
}

// OpenMenuAt shows a context menu with its top-left corner at pos (window px),
// moved as needed to stay inside the window.
func OpenMenuAt(pos image.Point, items []MenuItem) {
	menus = []*menuLevel{{items: items, at: pos, open: -1}}
	Invalidate()
}

// OpenMenuWide is OpenMenuAt for a menu at least minW dp wide.
func OpenMenuWide(pos image.Point, minW float32, items []MenuItem) {
	OpenMenuAt(pos, items)
	menus[0].minW = minW
}

func CloseMenu() {
	menus = nil
	Invalidate()
}

func MenuIsOpen() bool { return len(menus) > 0 }

func layoutMenus(gtx layout.Context) {
	if len(menus) == 0 {
		return
	}
	win := gtx.Constraints.Max
	// Backdrop: a click outside closes the menu.
	for {
		e, ok := gtx.Event(pointer.Filter{Target: menuTag, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pe := e.(pointer.Event); pe.Kind == pointer.Press {
			CloseMenu()
			return
		}
	}
	st := clip.Rect{Max: win}.Push(gtx.Ops)
	event.Op(gtx.Ops, menuTag)
	st.Pop()

	for li := 0; li < len(menus); li++ {
		lv := menus[li]
		items := lv.items
		if lv.sub != nil {
			items = lv.sub()
		}
		if len(lv.clicks) < len(items) {
			lv.clicks = append(lv.clicks, make([]Click, len(items)-len(lv.clicks))...)
		}
		// Measure.
		m := op.Record(gtx.Ops)
		d := layoutMenuBox(gtx, lv, items, li)
		call := m.Stop()
		at := lv.at
		switch {
		case lv.ptr:
			at.Y += Dp(gtx, 8)
			if at.X+d.Size.X > win.X {
				at.X -= d.Size.X
			}
			if at.Y+d.Size.Y > win.Y {
				at.Y = max(0, lv.at.Y-Dp(gtx, 8)-d.Size.Y)
			}
		case at.X+d.Size.X > win.X-8:
			if li > 0 {
				at.X = menus[li-1].rect.Min.X - d.Size.X // flip to the left of the parent
			} else {
				at.X = win.X - 8 - d.Size.X
			}
			fallthrough
		default:
			if at.Y+d.Size.Y > win.Y-8 {
				at.Y = max(8, win.Y-8-d.Size.Y)
			}
		}
		at.X = max(Dp(gtx, 8), at.X)
		if lv.ptr {
			at.X = max(0, at.X)
		}
		lv.rect = image.Rectangle{Min: at, Max: at.Add(d.Size)}
		Offset(gtx, at.X, at.Y, func() { call.Add(gtx.Ops) })
		if li == len(menus)-1 {
			break
		}
	}
}

func layoutMenuBox(gtx layout.Context, lv *menuLevel, items []MenuItem, li int) layout.Dimensions {
	pad := Dp(gtx, 4)
	minW, maxW := Dp(gtx, max(160, lv.minW)), Dp(gtx, 350)
	rowH := Dp(gtx, 40)
	// Width: widest label plus the 4dp frame, 12dp before and 8dp after the
	// row's content, its 16dp icon and 12dp gap, and the 16dp right icon.
	w := minW
	for _, it := range items {
		if it.Divider || it.Widget != nil {
			continue
		}
		lw := Measure(gtx, it.Label, 14, Regular) + Dp(gtx, 2*4+12+8)
		if it.Icon != "" {
			lw += Dp(gtx, 16+12)
		}
		if it.Sub != nil || it.Check || it.External {
			lw += Dp(gtx, 16+12)
		}
		w = max(w, min(maxW, lw))
	}
	lv.rows = lv.rows[:0]
	m := op.Record(gtx.Ops)
	y := pad
	for i, it := range items {
		switch {
		case it.Divider:
			Offset(gtx, pad, y, func() { Hline(gtx, w-2*pad, ColDivider) })
			lv.rows = append(lv.rows, image.Rectangle{})
			y += max(1, Dp(gtx, 1))
			continue
		case it.Widget != nil:
			var d layout.Dimensions
			Offset(gtx, pad, y, func() {
				g := gtx
				g.Constraints = layout.Constraints{Min: image.Pt(w-2*pad, 0), Max: image.Pt(w-2*pad, Dp(gtx, 400))}
				d = it.Widget(g)
			})
			lv.rows = append(lv.rows, image.Rect(pad, y, w-pad, y+d.Size.Y))
			y += d.Size.Y
			continue
		}
		c := &lv.clicks[i]
		c.Disabled = it.Muted
		c.Label = it.Label
		row := image.Rect(pad, y, w-pad, y+rowH)
		lv.rows = append(lv.rows, row)
		if c.Clicked(gtx) {
			switch {
			case it.Sub != nil:
				openFlyout(lv, li, i, it.Sub)
			case it.Action != nil:
				act := it.Action
				CloseMenu()
				act()
				m.Stop()
				return layout.Dimensions{}
			}
		}
		if c.Hovered() && it.Sub != nil && lv.open != i {
			openFlyout(lv, li, i, it.Sub)
		} else if c.Hovered() && it.Sub == nil && lv.open >= 0 && li == len(menus)-2 {
			menus = menus[:li+1]
			lv.open = -1
		}
		Offset(gtx, row.Min.X, row.Min.Y, func() {
			c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				rw := row.Dx()
				if (c.Hovered() || lv.open == i) && !it.Muted {
					FillRect(gtx.Ops, image.Rect(0, 0, rw, rowH), Dp(gtx, 2), Alpha(ColText, .1))
				}
				fg := Alpha(ColText, .9)
				if c.Hovered() || lv.open == i {
					fg = ColText
				}
				switch {
				case it.Danger:
					fg = RGB(0xf3727f)
				case it.Accent:
					fg = ColAccent
				case it.Muted:
					fg = ColText2
				}
				x := Dp(gtx, 12)
				if it.Icon != "" {
					is := Dp(gtx, 16)
					Offset(gtx, x, (rowH-is)/2, func() { IconPx(gtx, it.Icon, is, fg) })
					x += is + Dp(gtx, 12)
				}
				right := Dp(gtx, 8)
				if it.Sub != nil || it.Check || it.External {
					right += Dp(gtx, 16) + Dp(gtx, 12)
				}
				Offset(gtx, x, 0, func() {
					g := gtx
					g.Constraints = layout.Exact(image.Pt(max(0, rw-x-right), rowH))
					layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions {
						return Text(gtx, it.Label, 14, Regular, fg)
					})
				})
				ri := ""
				rc := fg
				switch {
				case it.Sub != nil:
					ri = "chevron-right"
				case it.Check:
					ri, rc = "check", ColAccent
				case it.External:
					ri = "external"
				}
				if ri != "" {
					is := Dp(gtx, 16)
					Offset(gtx, rw-Dp(gtx, 8)-is, (rowH-is)/2, func() { IconPx(gtx, ri, is, rc) })
				}
				return layout.Dimensions{Size: image.Pt(rw, rowH)}
			})
		})
		y += rowH
	}
	y += pad
	call := m.Stop()
	r := image.Rect(0, 0, w, y)
	popupShadow(gtx, r)
	FillRect(gtx.Ops, r, Dp(gtx, 4), RGB(0x282828))
	// Swallow clicks on the menu's own background.
	st := clip.Rect(r).Push(gtx.Ops)
	event.Op(gtx.Ops, lv)
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: lv, Kinds: pointer.Press}); !ok {
			break
		}
	}
	st.Pop()
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: r.Max}
}

func openFlyout(lv *menuLevel, li, i int, sub func() []MenuItem) {
	menus = menus[:li+1]
	lv.open = i
	row := lv.rows[i]
	at := lv.rect.Min.Add(image.Pt(lv.rect.Dx()-4, row.Min.Y))
	menus = append(menus, &menuLevel{sub: sub, at: at, open: -1})
}

// tipLine is one 14sp tooltip line with the web player's 20dp line height.
func tipLine(gtx layout.Context, s string, w font.Weight, c color.NRGBA) layout.Dimensions {
	return Label{Text: s, Size: 14, Weight: w, Color: c, LineHeight: 20}.Layout(gtx)
}

// popupShadow is the elevated shadow of menus and tooltips.
func popupShadow(gtx layout.Context, r image.Rectangle) {
	Shadow(gtx.Ops, r, Dp(gtx, 4), Dp(gtx, 24), Dp(gtx, 16), .3)
	Shadow(gtx.Ops, r, Dp(gtx, 4), Dp(gtx, 8), Dp(gtx, 6), .2)
}

// ---- Modals ----

// Modal is a dialog drawn over a dimmed window. Layout gets the box's inner
// width as its max constraint.
type Modal interface {
	Layout(gtx layout.Context) layout.Dimensions
}

// ModalCloser is implemented by modals that react to Escape/backdrop close.
type ModalCloser interface {
	Closed()
}

var (
	modal    Modal
	modalTag = new(int)
	// ModalWidth is the box width in dp (0 = 524).
	modalWidth  float32
	modalBare   bool
	modalBg     color.NRGBA
	modalPad    float32 // dp
	modalShadow bool
)

// ShowModal opens m in the standard dialog box (width dp, 0 = 524).
func ShowModal(m Modal, width float32) { showBox(m, width, RGB(0x282828), 24, true) }

// ShowLightModal opens m in the white confirmation dialog (black text).
func ShowLightModal(m Modal, width float32) { showBox(m, width, ColText, 32, false) }

func showBox(m Modal, width float32, bg color.NRGBA, pad float32, shadow bool) {
	modal, modalWidth, modalBare, modalBg, modalPad, modalShadow = m, width, false, bg, pad, shadow
	Invalidate()
}

// ShowOverlay opens m full-window over the dim backdrop, without the box
// (e.g. the full-size artwork viewer).
func ShowOverlay(m Modal) {
	modal, modalBare = m, true
	Invalidate()
}

func CloseModal() {
	if c, ok := modal.(ModalCloser); ok {
		c.Closed()
	}
	modal = nil
	Invalidate()
}

func ModalIsOpen() bool { return modal != nil }

func layoutModal(gtx layout.Context) {
	if modal == nil {
		return
	}
	win := gtx.Constraints.Max
	for {
		e, ok := gtx.Event(pointer.Filter{Target: modalTag, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pe := e.(pointer.Event); pe.Buttons.Contain(pointer.ButtonPrimary) {
			CloseModal()
			return
		}
	}
	FillRect(gtx.Ops, image.Rectangle{Max: win}, 0, Alpha(ColBg, .7))
	st := clip.Rect{Max: win}.Push(gtx.Ops)
	event.Op(gtx.Ops, modalTag)
	st.Pop()
	if modalBare {
		g := gtx
		g.Constraints = layout.Exact(win)
		modal.Layout(g)
		return
	}
	w := modalWidth
	if w == 0 {
		w = 524
	}
	bw := min(Dp(gtx, w), win.X*9/10)
	pad := Dp(gtx, modalPad)
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Min: image.Pt(bw-2*pad, 0), Max: image.Pt(bw-2*pad, win.Y*8/10)}
	d := modal.Layout(g)
	call := m.Stop()
	bh := d.Size.Y + 2*pad
	x, y := (win.X-bw)/2, (win.Y-bh)/2
	Offset(gtx, x, y, func() {
		r := image.Rect(0, 0, bw, bh)
		if modalShadow {
			Shadow(gtx.Ops, r, Dp(gtx, 8), Dp(gtx, 4), Dp(gtx, 4), .3)
		}
		FillRect(gtx.Ops, r, Dp(gtx, 8), modalBg)
		st := clip.Rect(r).Push(gtx.Ops)
		event.Op(gtx.Ops, &modal) // swallow clicks inside the box
		for {
			if _, ok := gtx.Event(pointer.Filter{Target: &modal, Kinds: pointer.Press}); !ok {
				break
			}
		}
		st.Pop()
		Offset(gtx, pad, pad, func() { call.Add(gtx.Ops) })
	})
}

// ModalTitle is a dialog's 24sp title (and an optional 16sp subtitle) with
// the close button at the right. Hang puts the button 8dp into the box's
// padding, like the Edit details dialog.
type ModalTitle struct {
	Close IconButton
	Hang  bool
}

func (t *ModalTitle) Layout(gtx layout.Context, title, sub string) layout.Dimensions {
	if t.Close.Click.Clicked(gtx) {
		CloseModal()
	}
	w, b := gtx.Constraints.Max.X, Dp(gtx, 32)
	g := gtx
	g.Constraints.Max.X = w - Dp(gtx, 40)
	rows := []layout.Widget{func(gtx layout.Context) layout.Dimensions {
		return Label{Text: title, Size: 24, Weight: Bold, Color: ColText, LineHeight: 29}.Layout(gtx)
	}}
	if sub != "" {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return Pad(gtx, 4, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
				return Label{Text: sub, Size: 16, Weight: Bold, Color: ColText, MaxLines: 2, LineHeight: 22}.Layout(gtx)
			})
		})
	}
	d := Column(g, rows...)
	x, y := w-b, 0
	if t.Hang {
		x, y = x+Dp(gtx, 8), (Dp(gtx, 29)-b)/2
	}
	Offset(gtx, x, y, func() {
		if t.Close.Click.Hovered() {
			FillCircle(gtx.Ops, b, Alpha(ColText, .1))
		}
		t.Close.Layout(gtx, IconStyle{Icon: "close", Label: "Close", Color: Alpha(ColText, .7)})
	})
	return layout.Dimensions{Size: image.Pt(w, d.Size.Y)}
}

// ---- Toasts ----

func layoutToasts(gtx layout.Context, bottom int) {
	const life = 5 * time.Second
	now := gtx.Now
	kept := toasts[:0]
	for _, t := range toasts {
		if now.Sub(t.at) < life {
			kept = append(kept, t)
			gtx.Execute(op.InvalidateCmd{At: t.at.Add(life)})
		}
	}
	toasts = kept
	win := gtx.Constraints.Max
	y := win.Y - bottom
	for i := len(toasts) - 1; i >= 0; i-- {
		t := toasts[i]
		m := op.Record(gtx.Ops)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(min(Dp(gtx, 450), win.X*2/3), Dp(gtx, 200))}
		d := Pad(g, 12, 12, 14, 12, func(gtx layout.Context) layout.Dimensions {
			return Label{Text: t.text, Size: 16, Color: ColBg, MaxLines: 3, LineHeight: 22}.Layout(gtx)
		})
		call := m.Stop()
		y -= d.Size.Y
		x := (win.X - d.Size.X) / 2
		Offset(gtx, x, y, func() {
			r := image.Rectangle{Max: d.Size}
			Shadow(gtx.Ops, r, Dp(gtx, 6), Dp(gtx, 12), Dp(gtx, 4), .3)
			FillRect(gtx.Ops, r, Dp(gtx, 6), ColText)
			call.Add(gtx.Ops)
		})
		y -= Dp(gtx, 8)
	}
}

// ---- Tooltips ----

// Tip shows text in a tooltip after the pointer rests on c for a moment.
// Call it right after laying out the clickable, with its dimensions.
// below puts it under the element (top bar) instead of above.
func Tip(gtx layout.Context, c *Click, d layout.Dimensions, text string, below bool) {
	if !c.Hovered() || text == "" {
		c.hoverSince = time.Time{}
		return
	}
	if c.hoverSince.IsZero() {
		c.hoverSince = gtx.Now
	}
	const delay = 375 * time.Millisecond
	if gtx.Now.Sub(c.hoverSince) < delay {
		gtx.Execute(op.InvalidateCmd{At: c.hoverSince.Add(delay)})
		return
	}
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(Dp(gtx, 300), Dp(gtx, 100))}
	td := Pad(g, 4, 8, 4, 8, func(gtx layout.Context) layout.Dimensions {
		return tipLine(gtx, text, Regular, ColText)
	})
	call := m.Stop()
	x := (d.Size.X - td.Size.X) / 2
	y := -td.Size.Y - Dp(gtx, 8)
	if below {
		y = d.Size.Y + Dp(gtx, 8)
	}
	m = op.Record(gtx.Ops)
	Offset(gtx, x, y, func() {
		r := image.Rectangle{Max: td.Size}
		popupShadow(gtx, r)
		FillRect(gtx.Ops, r, Dp(gtx, 4), RGB(0x282828))
		call.Add(gtx.Ops)
	})
	op.Defer(gtx.Ops, m.Stop())
}

// paintAll fills the current clip with c.
func paintAll(ops *op.Ops, c color.NRGBA) {
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
}
