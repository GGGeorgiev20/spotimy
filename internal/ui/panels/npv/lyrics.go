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

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

// lyricsCard is the Now Playing view's "Lyrics" card: the song's lines on a
// dark tone of the cover's color, the sung one white and kept in the middle.
type lyricsCard struct {
	res   *ui.Res[*spotify.Lyrics]
	base  color.NRGBA // the cover's color
	uri   string
	lines []ui.Click
	// scroll is the eased scroll offset in px; at is the last frame's time.
	scroll float64
	at     time.Time
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

// layout draws the card (at most 440dp tall, like the official one).
func (c *lyricsCard) layout(gtx layout.Context) layout.Dimensions {
	l := c.res.V
	n := len(l.Lines)
	if len(c.lines) != n {
		c.lines = make([]ui.Click, n)
	}
	dp := func(v float32) int { return ui.Dp(gtx, v) }
	bg := ui.Tone(c.base, min(ui.Lightness(c.base), 22))
	upcoming := ui.Tone(c.base, 82)
	past := ui.Mix(bg, upcoming, .5)

	active := -1
	pos := ui.Position(gtx.Now).Milliseconds()
	if l.Synced {
		active = sort.Search(n, func(i int) bool { return l.Lines[i].StartMs > pos }) - 1
		if ui.Playing() && active+1 < n {
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Duration(l.Lines[active+1].StartMs-pos) * time.Millisecond)})
		}
	}
	for i := range c.lines {
		if c.lines[i].Clicked(gtx) && l.Synced {
			ui.Seek(l.Lines[i].StartMs)
		}
	}

	// The lines, recorded to measure them first.
	w := gtx.Constraints.Max.X
	tw := w - 2*dp(16)
	m := op.Record(gtx.Ops)
	y, activeY, activeH := 0, 0, 0
	for i, ln := range l.Lines {
		words := ln.Words
		if words == "" {
			words = "♪"
		}
		col := upcoming
		switch {
		case !l.Synced || i == active:
			col = ui.ColText
		case i < active:
			col = past
		}
		lc := &c.lines[i]
		lc.Disabled, lc.Arrow, lc.Label = !l.Synced, !l.Synced, words
		var d layout.Dimensions
		ui.Offset(gtx, 0, y, func() {
			g := gtx
			g.Constraints = layout.Constraints{Max: image.Pt(tw, 1<<20)}
			d = lc.Layout(g, func(gtx layout.Context) layout.Dimensions {
				return ui.Label{Text: words, Size: 24, Weight: ui.Bold, Color: col, MaxLines: -1, LineHeight: 24 * 1.2}.Layout(gtx)
			})
		})
		if i == active {
			activeY, activeH = y, d.Size.Y
		}
		y += d.Size.Y + dp(14)
	}
	lines := m.Stop()
	contentH := y - dp(14)

	head := dp(16 + 25)                                           // padding and the "Lyrics" row
	h := min(dp(440), head+dp(16)+contentH+dp(16))                // the area pads 16 above and below
	view := h - head                                              // the scrolling area, edge to edge
	maxScroll := float64(max(0, contentH+2*dp(16)-view))          // its content is padded 16
	target := float64(activeY+activeH/2+dp(16)) - float64(view)/2 // the sung line centered
	if active < 0 {
		target = 0
	}
	target = min(maxScroll, max(0, target))
	dt := gtx.Now.Sub(c.at)
	if c.at.IsZero() || dt > 100*time.Millisecond {
		dt = 16 * time.Millisecond
	}
	c.at = gtx.Now
	if math.Abs(target-c.scroll) > float64(3*view) {
		c.scroll = target
	} else if math.Abs(target-c.scroll) >= .5 {
		c.scroll += (target - c.scroll) * (1 - math.Exp(-float64(dt)/float64(110*time.Millisecond)))
		gtx.Execute(op.InvalidateCmd{})
	}

	r := dp(8)
	defer clip.UniformRRect(image.Rect(0, 0, w, h), r).Push(gtx.Ops).Pop()
	ui.FillRect(gtx.Ops, image.Rect(0, 0, w, h), 0, bg)
	ui.Offset(gtx, dp(16), dp(16), func() { ui.Text(gtx, "Lyrics", 16, ui.Bold, ui.ColText) })
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
	return layout.Dimensions{Size: image.Pt(w, h)}
}
