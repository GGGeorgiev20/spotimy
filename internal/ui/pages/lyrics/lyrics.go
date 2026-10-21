// Package lyrics is the lyrics page (/lyrics): the playing song's lyrics on
// the cover's color, the current line highlighted and kept in view.
package lyrics

import (
	"image"
	"image/color"
	"math"
	"sort"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() {
	ui.Register("lyrics", func([]string) ui.Page { return &page{} })
}

// Line colors are the official rgba(0,0,0,.55) / rgba(255,255,255,.7) / .9
// overlays, pre-blended onto the background in sRGB (Gio blends in linear
// light, which would wash them out).
func upcoming(bg color.NRGBA) color.NRGBA { return ui.Mix(bg, ui.ColBg, .55) }
func past(bg color.NRGBA) color.NRGBA     { return ui.Mix(bg, ui.ColText, .7) }
func hover(bg color.NRGBA) color.NRGBA    { return ui.Mix(bg, ui.ColText, .9) }

// followIdle is how long a manual scroll keeps the view from jumping back.
const followIdle = 4 * time.Second

type page struct {
	uri   string
	res   *ui.Res[*spotify.Lyrics]
	color color.NRGBA
	list  ui.List
	lines []ui.Click

	heights   []int // height of each list item seen so far (0 = not laid out yet)
	scrollTag bool
	userAt    time.Time
	lastFrame time.Time
}

// Background is the page's color, for views that frame it.
func (p *page) Background() color.NRGBA { return p.color }

// activeLine is the index of the line being sung at ms, -1 before the first.
func activeLine(lines []spotify.LyricLine, ms int64) int {
	return sort.Search(len(lines), func(i int) bool { return lines[i].StartMs > ms }) - 1
}

// load switches to the playing track's lyrics and cover color.
func (p *page) load(t string, imgs []spotify.Image) {
	p.uri, p.color = t, ui.RGB(0x535353)
	p.res = &ui.Res[*spotify.Lyrics]{}
	p.lines, p.heights = nil, nil
	p.list.ScrollToTop()
	if t == "" {
		return
	}
	p.res.Load(func() (*spotify.Lyrics, error) { return ui.B.Lyrics(t) })
	if u := ui.Pick(imgs, 64); u != "" {
		ui.Async(func() (string, error) { return ui.B.ImageColor(u) }, func(hex string, err error) {
			if err == nil && p.uri == t {
				p.color = ui.LyricsColor(ui.Hex(hex))
			}
		})
	}
}

// Item indexes: top padding, lines, provider footer, bottom padding.
const itemTop = 0

func (p *page) Layout(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	t := ui.NowTrack()
	uri := ""
	if t != nil {
		uri = t.URI
	}
	if p.res == nil || (uri != p.uri && ui.Settled(gtx)) {
		var imgs []spotify.Image
		if t != nil {
			imgs = t.Images
		}
		p.load(uri, imgs)
	}
	ui.FillRect(gtx.Ops, image.Rectangle{Max: size}, 0, p.color)

	dpW := float32(size.X) / gtx.Metric.PxPerDp
	fs := min(32, max(24, dpW*.032))
	side := max(32, dpW*.08)
	msg := func(s string) layout.Dimensions {
		return ui.Pad(gtx, 96, side, 0, side, func(gtx layout.Context) layout.Dimensions {
			return ui.Label{Text: s, Size: 24, Weight: ui.Bold, Color: color.NRGBA{R: 255, G: 255, B: 255, A: 204}, MaxLines: 3}.Layout(gtx)
		})
	}
	switch {
	case t == nil:
		return msg("Play something to see its lyrics.")
	case !p.res.Done:
		return msg("Loading lyrics…")
	case p.res.V == nil || len(p.res.V.Lines) == 0:
		return msg("Lyrics aren't available for this song.")
	}
	l := p.res.V
	n := len(l.Lines)
	if len(p.lines) != n {
		p.lines = make([]ui.Click, n)
	}

	pos := ui.Position(gtx.Now).Milliseconds()
	active := -1
	if l.Synced {
		active = activeLine(l.Lines, pos)
		if ui.Playing() && active+1 < n {
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Duration(l.Lines[active+1].StartMs-pos) * time.Millisecond)})
		}
	}
	for i := range p.lines {
		if p.lines[i].Clicked(gtx) && l.Synced {
			p.userAt = time.Time{}
			ui.Seek(l.Lines[i].StartMs)
		}
	}
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &p.scrollTag, Kinds: pointer.Scroll}); !ok {
			break
		}
		p.userAt = gtx.Now
	}
	if l.Synced && active >= 0 && gtx.Now.Sub(p.userAt) > followIdle {
		p.follow(gtx, active+1, size.Y)
	}
	p.lastFrame = gtx.Now

	total := n + 3
	if len(p.heights) != total {
		p.heights = make([]int, total)
	}
	d := p.list.Layout(gtx, total, func(gtx layout.Context, i int) layout.Dimensions {
		d := p.item(gtx, i, l, active, fs, side, size.Y)
		p.heights[i] = d.Size.Y
		return d
	})
	// A pass-through layer over the list that only watches the wheel.
	pass := pointer.PassOp{}.Push(gtx.Ops)
	st := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, &p.scrollTag)
	st.Pop()
	pass.Pop()
	return d
}

// follow eases the list so item (the active line) sits a third of the way
// down the view.
func (p *page) follow(gtx layout.Context, item, viewH int) {
	known, sum := 0, 0
	for i := 1; i < len(p.heights)-2; i++ {
		if p.heights[i] > 0 {
			known++
			sum += p.heights[i]
		}
	}
	if known == 0 {
		return
	}
	avg := sum / known
	top := p.heights[itemTop]
	if top == 0 {
		top = ui.Dp(gtx, 96)
	}
	for i := 1; i < item; i++ {
		if p.heights[i] > 0 {
			top += p.heights[i]
		} else {
			top += avg
		}
	}
	delta := float64(top - viewH/3 - p.list.ScrollTop())
	if math.Abs(delta) < 1 {
		return
	}
	dt := gtx.Now.Sub(p.lastFrame)
	if p.lastFrame.IsZero() || dt > 100*time.Millisecond {
		dt = 16 * time.Millisecond
	}
	step := delta * (1 - math.Exp(-float64(dt)/float64(110*time.Millisecond)))
	if math.Abs(delta) > float64(3*viewH) {
		step = delta
	}
	p.list.Position.Offset += int(math.Round(step))
	p.list.Position.BeforeEnd = true
	gtx.Execute(op.InvalidateCmd{})
}

func (p *page) item(gtx layout.Context, i int, l *spotify.Lyrics, active int, fs, side float32, viewH int) layout.Dimensions {
	n := len(l.Lines)
	switch {
	case i == itemTop:
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, ui.Dp(gtx, 96))}
	case i == n+2:
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, viewH/2)}
	case i == n+1:
		if l.Provider == "" {
			return layout.Dimensions{}
		}
		return ui.Pad(gtx, 20, side, 0, side, func(gtx layout.Context) layout.Dimensions {
			return ui.Label{Text: "Lyrics provided by " + l.Provider, Size: 12, Color: past(p.color), MaxLines: 2}.Layout(gtx)
		})
	}
	k := i - 1
	line := l.Lines[k]
	words := line.Words
	if words == "" {
		words = "♪"
	}
	c := &p.lines[k]
	c.Disabled = !l.Synced
	c.Arrow = !l.Synced
	c.Label = words
	return ui.Pad(gtx, 0, side, 12, side, func(gtx layout.Context) layout.Dimensions {
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			col := upcoming(p.color)
			switch {
			case !l.Synced || k == active:
				col = ui.ColText
			case k < active:
				col = past(p.color)
			case c.Hovered():
				col = hover(p.color)
			}
			return ui.Label{Text: words, Size: fs, Weight: ui.Bold, Color: col, MaxLines: -1, LineHeight: fs * 1.3}.Layout(gtx)
		})
	})
}
