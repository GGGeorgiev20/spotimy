package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/f32"
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
	// No frames are drawn while nothing moves, so the time since the last
	// call can be seconds: count at most a frame's worth, or the first step
	// of every animation would jump straight to its end.
	dt := float32(min(gtx.Now.Sub(h.last), maxFrameGap)) / float32(dur)
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

// maxFrameGap is the most time one animation step counts.
const maxFrameGap = 20 * time.Millisecond

// Appear is an entrance animation's progress: 0 to 1 over dur from *since,
// eased out like CSS's cubic-bezier(0, 0, .2, 1), asking for frames until it
// ends. A zero *since is stamped with this frame's time, so reset it to
// replay the animation.
func Appear(gtx layout.Context, since *time.Time, dur time.Duration) float32 {
	if since.IsZero() {
		*since = gtx.Now
	}
	return easeOut(gtx, gtx.Now.Sub(*since), dur)
}

// easeOut is the eased progress at elapsed of dur (requesting the next frame
// while it's below 1).
func easeOut(gtx layout.Context, elapsed, dur time.Duration) float32 {
	t := float32(elapsed) / float32(dur)
	if t >= 1 {
		return 1
	}
	gtx.Execute(op.InvalidateCmd{})
	t = max(0, t)
	return 1 - (1-t)*(1-t)
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

// Layout draws a size-dp green circle with a play or pause icon.
func (b *PlayButton) Layout(gtx layout.Context, size float32, playing bool) layout.Dimensions {
	b.Click.Label = "Play"
	name := "play"
	if playing {
		b.Click.Label, name = "Pause", "pause"
	}
	return b.Round(gtx, RoundStyle{Size: size, Icon: name, IconScale: 3. / 7, Bg: ColAccent, BgHover: ColAccentHover, Fg: ColBg})
}

// RoundStyle is a round button's look: a Size dp circle in Bg (BgHover on
// hover) with Icon in Fg, IconScale of the circle wide.
type RoundStyle struct {
	Size        float32
	Icon        string
	IconScale   float32
	Bg, BgHover color.NRGBA
	Fg          color.NRGBA
}

// Round draws a round button that grows 4% on hover, eased like the web
// player's (scale 1.04, 150ms, cubic-bezier(.3, 0, 0, 1)). It's scaled
// rather than resized so the growth is smooth instead of snapping a pixel at
// a time, and drawn outside the click area's clip so the edge isn't cut off.
func (b *PlayButton) Round(gtx layout.Context, s RoundStyle) layout.Dimensions {
	b.Click.update(gtx) // this frame's hover, not the last one's
	v := b.hover.Step(gtx, b.Click.Hovered(), 150*time.Millisecond)
	e := 1 - (1-v)*(1-v)*(1-v)
	d := Dp(gtx, s.Size)
	bg := s.Bg
	if s.BgHover != ColNone {
		bg = Mix(s.Bg, s.BgHover, e)
	}
	if b.Click.Disabled {
		bg = Alpha(s.Bg, .3)
	}
	c := float32(d) / 2
	st := op.Affine(f32.AffineId().Scale(f32.Pt(c, c), f32.Pt(1+.04*e, 1+.04*e))).Push(gtx.Ops)
	FillCircle(gtx.Ops, d, bg)
	is := int(float32(d)*s.IconScale + .5)
	Offset(gtx, (d-is)/2, (d-is)/2, func() { IconPx(gtx, s.Icon, is, s.Fg) })
	st.Pop()
	return b.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
			StrokeRRect(gtx.Ops, image.Rect(0, 0, w, h), h/2, max(1, Dp(gtx, 1)), bc)
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
