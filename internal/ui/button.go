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
	"gioui.org/unit"

	"github.com/GGGeorgiev20/personal-spotify/internal/ui/icon"
)

// Icon draws a named icon size dp square.
func Icon(gtx layout.Context, name string, size float32, c color.NRGBA) layout.Dimensions {
	return icon.Layout(gtx, name, unit.Dp(size), c)
}

// IconPx draws a named icon px pixels square.
func IconPx(gtx layout.Context, name string, px int, c color.NRGBA) layout.Dimensions {
	return icon.Layout(gtx, name, unit.Dp(float32(px)/gtx.Metric.PxPerDp), c)
}

// Hover animates a 0..1 value toward on over dur, requesting frames while it moves.
type Hover struct {
	v    float32
	last time.Time
}

func (h *Hover) Step(gtx layout.Context, on bool, dur time.Duration) float32 {
	target := float32(0)
	if on {
		target = 1
	}
	if h.last.IsZero() {
		h.last = gtx.Now
	}
	dt := float32(gtx.Now.Sub(h.last)) / float32(dur)
	h.last = gtx.Now
	if h.v < target {
		h.v = min(target, h.v+dt)
	} else if h.v > target {
		h.v = max(target, h.v-dt)
	}
	if h.v != target {
		gtx.Execute(op.InvalidateCmd{})
	}
	return h.v
}

// IconButton is a bare icon that brightens on hover, like the player controls.
type IconButton struct {
	Click Click
}

type IconStyle struct {
	Icon  string
	Size  float32 // icon size, dp (default 16)
	Box   float32 // clickable box, dp (default 32)
	Color color.NRGBA
	Hover color.NRGBA
	// Active shows the accent color with a dot underneath (shuffle, repeat, panels).
	Active bool
	// Dot draws the small dot under an active icon.
	Dot      bool
	Disabled bool
	Label    string
}

func (b *IconButton) Layout(gtx layout.Context, s IconStyle) layout.Dimensions {
	if s.Size == 0 {
		s.Size = 16
	}
	if s.Box == 0 {
		s.Box = 32
	}
	if s.Color == ColNone {
		s.Color = ColText2
	}
	if s.Hover == ColNone {
		s.Hover = ColText
	}
	b.Click.Label, b.Click.Disabled = s.Label, s.Disabled
	box := Dp(gtx, s.Box)
	return b.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		c := s.Color
		switch {
		case s.Disabled:
			c = Alpha(s.Color, .3)
		case s.Active:
			c = ColAccent
			if b.Click.Hovered() {
				c = ColAccentHover
			}
		case b.Click.Hovered():
			c = s.Hover
		}
		px := Dp(gtx, s.Size)
		if b.Click.Hovered() && !s.Disabled {
			px = px * 104 / 100
		}
		Offset(gtx, (box-px)/2, (box-px)/2, func() { IconPx(gtx, s.Icon, px, c) })
		if s.Active && s.Dot {
			d := Dp(gtx, 4)
			Offset(gtx, (box-d)/2, box-d, func() { FillCircle(gtx.Ops, d, c) })
		}
		return layout.Dimensions{Size: image.Pt(box, box)}
	})
}

// PlayButton is the round green play/pause button.
type PlayButton struct {
	Click Click
	hover Hover
}

// Layout draws a size-dp green circle with a play or pause icon. On hover it
// grows 4% and brightens, eased like the web player's transition; the circle
// is drawn outside the click area's clip so the grown edge isn't cut off.
func (b *PlayButton) Layout(gtx layout.Context, size float32, playing bool) layout.Dimensions {
	b.Click.Label = "Play"
	if playing {
		b.Click.Label = "Pause"
	}
	b.Click.update(gtx) // this frame's hover, not the last one's
	v := b.hover.Step(gtx, b.Click.Hovered(), 100*time.Millisecond)
	d := Dp(gtx, size)
	draw := d + int(float32(d)*.04*v+.5)
	o := (d - draw) / 2
	Offset(gtx, o, o, func() { FillCircle(gtx.Ops, draw, Mix(ColAccent, ColAccentHover, v)) })
	return b.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		name := "play"
		if playing {
			name = "pause"
		}
		is := draw * 3 / 7
		Offset(gtx, (d-is)/2, (d-is)/2, func() { IconPx(gtx, name, is, ColBg) })
		return layout.Dimensions{Size: image.Pt(d, d)}
	})
}

// Pill is a rounded text button. Outline draws the "Follow" style border
// button; otherwise it's a filled chip (filled with Bg).
type Pill struct {
	Click Click
}

type PillStyle struct {
	Text     string
	Size     float32 // font size (default 14)
	Weight   font.Weight
	Height   float32 // dp (default 32)
	PadX     float32 // dp (default 16)
	Bg       color.NRGBA
	BgHover  color.NRGBA
	Fg       color.NRGBA
	Outline  bool
	Selected bool // chips: white with black text
	Icon     string
}

func (p *Pill) Layout(gtx layout.Context, s PillStyle) layout.Dimensions {
	if s.Size == 0 {
		s.Size = 14
	}
	if s.Height == 0 {
		s.Height = 32
	}
	if s.PadX == 0 {
		s.PadX = 16
	}
	if s.Fg == ColNone {
		s.Fg = ColText
	}
	p.Click.Label = s.Text
	return p.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		h := Dp(gtx, s.Height)
		pad := Dp(gtx, s.PadX)
		iconW := 0
		if s.Icon != "" {
			iconW = Dp(gtx, 16) + Dp(gtx, 8)
		}
		m := op.Record(gtx.Ops)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(max(0, gtx.Constraints.Max.X-2*pad-iconW), h)}
		fg := s.Fg
		if s.Selected {
			fg = ColBg
		}
		td := Text(g, s.Text, s.Size, s.Weight, fg)
		call := m.Stop()
		w := td.Size.X + 2*pad + iconW
		bg := s.Bg
		switch {
		case s.Selected:
			bg = ColText
			if p.Click.Hovered() {
				bg = RGB(0xf0f0f0)
			}
		case p.Click.Hovered() && s.BgHover != ColNone:
			bg = s.BgHover
		}
		FillRect(gtx.Ops, image.Rect(0, 0, w, h), h/2, bg)
		if s.Outline {
			bc := Alpha(ColText, .5)
			if p.Click.Hovered() {
				bc = ColText
			}
			strokeRRect(gtx.Ops, image.Rect(0, 0, w, h), h/2, max(1, Dp(gtx, 1)), bc)
		}
		if s.Icon != "" {
			is := Dp(gtx, 16)
			Offset(gtx, pad, (h-is)/2, func() { IconPx(gtx, s.Icon, is, fg) })
		}
		Offset(gtx, pad+iconW, (h-td.Size.Y)/2, func() { call.Add(gtx.Ops) })
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}

// HoverArea tracks whether the pointer is over an area without taking any
// events from the widgets in it: add it after them, on top, and it passes
// everything through.
type HoverArea struct{ in bool }

// Add registers the area (size from the origin) and reports whether the
// pointer is over it.
func (h *HoverArea) Add(gtx layout.Context, size image.Point) bool {
	for {
		e, ok := gtx.Event(pointer.Filter{Target: h, Kinds: pointer.Enter | pointer.Leave | pointer.Cancel})
		if !ok {
			break
		}
		if pe, ok := e.(pointer.Event); ok {
			h.in = pe.Kind == pointer.Enter
		}
	}
	pass := pointer.PassOp{}.Push(gtx.Ops)
	st := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, h)
	st.Pop()
	pass.Pop()
	return h.in
}

// Hovered reports what the last Add saw.
func (h *HoverArea) Hovered() bool { return h.in }
