package npv

import (
	"image"
	"image/color"
	"math"
	"sort"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

// lyricsExpanded is whether "Show more" opened the card. Like the official
// client it stays open for the tracks that follow until "Show less".
var lyricsExpanded bool

// lyricsCard is the Now Playing view's lyrics card on a dark tone of the
// cover's color. Closed it is a still "Lyrics preview" of the first lines;
// "Show more" opens it into the synced lyrics, the sung line white and kept in
// the middle, each line seeking there when clicked.
type lyricsCard struct {
	res   *ui.Res[*spotify.Lyrics]
	base  color.NRGBA // the cover's color
	uri   string
	lines []ui.Click
	// scroll is the eased scroll offset in px; at is the last frame's time.
	scroll float64
	at     time.Time
	// h is the card's height in px, eased toward its target when it opens
	// or closes.
	h float64

	toggle ui.Pill
	// full and cinema open the full screen lyrics, full screen or in the
	// window; the open card shows them in its title row while hovered.
	full, cinema ui.IconButton
	hover        ui.HoverArea
	btn          ui.Hover // the toggle's fade-in while the card is hovered
}

// load switches to the track's lyrics and cover color.
func (c *lyricsCard) load(uri string, imgs []spotify.Image) {
	*c = lyricsCard{res: &ui.Res[*spotify.Lyrics]{}, base: ui.RGB(0x535353), uri: uri}
	c.res.Load(func() (*spotify.Lyrics, error) { return ui.B.Lyrics(uri) })
	if u := ui.Pick(imgs, 64); u != "" {
		ui.Async(func() (string, error) { return ui.B.ImageColor(u) }, func(hex string, err error) {
			if err == nil && c.uri == uri {
				c.base = ui.Hex(hex)
			}
		})
	}
}

func (c *lyricsCard) has() bool {
	return c.res != nil && c.res.Ok() && c.res.V != nil && len(c.res.V.Lines) > 0
}

// Card heights (the official max-heights): closed and open.
const (
	lyricsPreviewH = 177
	lyricsOpenH    = 440
)

// layout draws the card.
func (c *lyricsCard) layout(gtx layout.Context) layout.Dimensions {
	if c.toggle.Click.Clicked(gtx) {
		lyricsExpanded = !lyricsExpanded
		c.scroll = 0
	}
	if c.full.Click.Clicked(gtx) {
		ui.OpenLyricsView(true)
	}
	if c.cinema.Click.Clicked(gtx) {
		ui.OpenLyricsView(false)
	}
	open := lyricsExpanded
	l := c.res.V
	n := len(l.Lines)
	if len(c.lines) != n {
		c.lines = make([]ui.Click, n)
	}
	dp := func(v float32) int { return ui.Dp(gtx, v) }
	bg := ui.Tone(c.base, min(ui.Lightness(c.base), 22))
	upcoming := ui.Tone(c.base, 82)
	past := ui.Mix(bg, upcoming, .5)
	synced := open && l.Synced // the preview is a still excerpt

	active := -1
	if synced {
		pos := ui.Position(gtx.Now).Milliseconds()
		active = sort.Search(n, func(i int) bool { return l.Lines[i].StartMs > pos }) - 1
		if ui.Playing() && active+1 < n {
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Duration(l.Lines[active+1].StartMs-pos) * time.Millisecond)})
		}
	}
	for i := range c.lines {
		if c.lines[i].Clicked(gtx) && synced {
			ui.Seek(l.Lines[i].StartMs)
		}
	}

	// The lines, recorded to measure them first.
	w := gtx.Constraints.Max.X
	tw := w - 2*dp(16)
	m := op.Record(gtx.Ops)
	y, activeY, activeH := 0, 0, 0
	if open && !l.Synced {
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(tw, 1<<20)}
		d := ui.Pad(g, 8, 0, 8, 0, func(gtx layout.Context) layout.Dimensions {
			return ui.Label{Text: "These lyrics aren't synced to the song yet.", Size: 14, Color: upcoming, MaxLines: 2}.Layout(gtx)
		})
		y += d.Size.Y + dp(14)
	}
	for i, ln := range l.Lines {
		words := ln.Words
		if words == "" {
			words = "♪"
		}
		col := upcoming
		switch {
		case !synced || i == active:
			col = ui.ColText
		case i < active:
			col = past
		}
		lc := &c.lines[i]
		lc.Disabled, lc.Arrow, lc.Label = !synced, !synced, words
		var d layout.Dimensions
		ui.Offset(gtx, 0, y, func() {
			g := gtx
			g.Constraints = layout.Constraints{Max: image.Pt(tw, 1<<20)}
			d = lc.Layout(g, func(gtx layout.Context) layout.Dimensions {
				return ui.Label{Text: words, Size: 24, Weight: ui.Bold, Color: col, MaxLines: -1, LineHeight: 24 * 1.2, Underline: synced && lc.Hovered()}.Layout(gtx)
			})
		})
		if i == active {
			activeY, activeH = y, d.Size.Y
		}
		y += d.Size.Y + dp(14)
	}
	lines := m.Stop()
	contentH := y - dp(14)

	head := dp(16 + 25) // padding and the title row
	maxH := lyricsPreviewH
	if open {
		maxH = lyricsOpenH
	}
	target := float64(min(dp(float32(maxH)), head+dp(16)+contentH+dp(16))) // the area pads 16 above and below
	dt := gtx.Now.Sub(c.at)
	if c.at.IsZero() || dt > 100*time.Millisecond {
		dt = 16 * time.Millisecond
	}
	c.at = gtx.Now
	// The height moves at the official transition's pace: the whole change
	// between the two heights in 100ms, linearly.
	if step := float64(dp(lyricsOpenH-lyricsPreviewH)) * float64(dt) / float64(100*time.Millisecond); c.h == 0 || math.Abs(target-c.h) <= step {
		c.h = target
	} else {
		c.h += math.Copysign(step, target-c.h)
		gtx.Execute(op.InvalidateCmd{})
	}
	h := int(math.Round(c.h))

	view := h - head                                                // the scrolling area, edge to edge
	maxScroll := float64(max(0, contentH+2*dp(16)-view))            // its content is padded 16
	scrollTo := float64(activeY+activeH/2+dp(16)) - float64(view)/2 // the sung line centered
	if active < 0 {
		scrollTo = 0
	}
	scrollTo = min(maxScroll, max(0, scrollTo))
	if math.Abs(scrollTo-c.scroll) > float64(3*view) {
		c.scroll = scrollTo
	} else if math.Abs(scrollTo-c.scroll) >= .5 {
		c.scroll += (scrollTo - c.scroll) * (1 - math.Exp(-float64(dt)/float64(110*time.Millisecond)))
		gtx.Execute(op.InvalidateCmd{})
	}

	r := dp(8)
	defer clip.UniformRRect(image.Rect(0, 0, w, h), r).Push(gtx.Ops).Pop()
	ui.FillRect(gtx.Ops, image.Rect(0, 0, w, h), 0, bg)
	title := "Lyrics preview"
	if open {
		title = "Lyrics"
	}
	ui.Offset(gtx, dp(16), dp(16), func() { ui.Text(gtx, title, 16, ui.Bold, ui.ColText) })
	if open && c.hover.Hovered() {
		// 32dp round buttons centered on the 25dp title row, 8dp apart, the
		// last 4dp past the padding.
		b := dp(32)
		x, y := w-dp(16)+dp(4)-b, dp(16)+(dp(25)-b)/2
		for _, ib := range []struct {
			btn       *ui.IconButton
			icon, tip string
		}{{&c.cinema, "maximize", "Open in cinema mode"}, {&c.full, "fullscreen", "Enter full screen"}} {
			ui.Offset(gtx, x, y, func() {
				if ib.btn.Click.Hovered() {
					ui.FillCircle(gtx.Ops, b, ui.Alpha(ui.ColText, .1))
				}
				d := ib.btn.Layout(gtx, ui.IconStyle{Icon: ib.icon, Label: ib.tip})
				ui.Tip(gtx, &ib.btn.Click, d, ib.tip, false)
			})
			x -= b + dp(8)
		}
	}
	st := clip.Rect(image.Rect(0, head, w, h)).Push(gtx.Ops)
	ui.Offset(gtx, dp(16), head+dp(16)-int(math.Round(c.scroll)), func() { lines.Add(gtx.Ops) })
	st.Pop()
	// The official mask: hidden in the outer 16dp, fading in over 8dp at the
	// top and 48dp at the bottom.
	clear := ui.Alpha(bg, 0)
	ui.FillRect(gtx.Ops, image.Rect(0, head, w, head+dp(16)), 0, bg)
	ui.VGradient(gtx.Ops, image.Rect(0, head+dp(16), w, head+dp(24)), bg, clear)
	ui.VGradient(gtx.Ops, image.Rect(0, h-dp(64), w, h-dp(16)), clear, bg)
	ui.FillRect(gtx.Ops, image.Rect(0, h-dp(16), w, h), 0, bg)

	// "Show more" / "Show less": a white pill at the bottom right that rises
	// and fades in while the card is hovered.
	if v := c.btn.Step(gtx, c.hover.Hovered(), 200*time.Millisecond); v > 0 {
		label := "Show more"
		if open {
			label = "Show less"
		}
		bm := op.Record(gtx.Ops)
		bd := c.toggle.Layout(gtx, ui.PillStyle{Text: label, Weight: ui.Bold, Selected: true})
		btn := bm.Stop()
		lift := int(float32(bd.Size.Y) * .8 * (1 - v))
		ui.Offset(gtx, w-dp(16)-bd.Size.X, h-dp(16)-bd.Size.Y+lift, func() {
			defer paint.PushOpacity(gtx.Ops, v).Pop()
			btn.Add(gtx.Ops)
		})
	}
	c.hover.Add(gtx, image.Pt(w, h))
	return layout.Dimensions{Size: image.Pt(w, h)}
}
