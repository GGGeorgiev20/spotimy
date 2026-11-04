package npv

import (
	"context"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
	"github.com/GGGeorgiev20/personal-spotify/internal/video"
)

// canvasVideo is a track's looping Canvas video. It decodes only while the
// panel keeps drawing it: the decoder stops itself a moment after the last
// draw (panel closed or another panel opened).
type canvasVideo struct {
	url     string
	cancel  context.CancelFunc
	seen    atomic.Int64 // unix nanos of the last draw
	pl      *video.Player
	stopped bool

	op paint.ImageOp
	n  uint64
}

// The Canvas follows the official view: the video spans the panel's width at
// 9:16 from its very top (behind the header), and scrolls with the content.
// The list holds a frame over it, a square while collapsed and 9:14.5 once
// clicked, so the title and the cards below sit over the video's lower part,
// which a scrim fades into the panel.
const (
	canvasHeaderDp = 64                      // the header row the video starts behind
	expandTime     = 1000 * time.Millisecond // the official layout transition
	dimTime        = 600 * time.Millisecond
)

// canvasURL is the playing track's Canvas, or "" for its cover.
func (p *panel) canvasURL() string {
	if p.canvas != nil && p.canvas.Ok() && ui.Settings.Canvas {
		return p.canvas.V
	}
	return ""
}

func (p *panel) media(gtx layout.Context, t *backend.NowTrack) layout.Dimensions {
	w := gtx.Constraints.Max.X
	if p.canvasURL() != "" {
		return p.canvasFrame(gtx, w)
	}
	if p.vid != nil { // Canvas was turned off
		p.stopVideo()
	}
	size := image.Pt(w, w)
	rad := ui.Dp(gtx, 8)
	ui.Shadow(gtx.Ops, image.Rectangle{Max: size}, rad, ui.Dp(gtx, 12), ui.Dp(gtx, 8), .3)
	ui.Image{URL: ui.Pick(t.Images, w), Radius: rad, Icon: "note"}.Layout(gtx, size)
	return layout.Dimensions{Size: size}
}

// canvasFrame is the list's room for the Canvas: clicking it expands or
// collapses it, hovering it lifts the dimming.
func (p *panel) canvasFrame(gtx layout.Context, w int) layout.Dimensions {
	if p.frame.Clicked(gtx) {
		p.grow.toggle(gtx.Now)
	}
	h := w + int(float32(w*145/90-w)*p.grow.at(gtx))
	p.frame.Label = "Expand Canvas"
	if p.grow.open {
		p.frame.Label = "Collapse Canvas"
	}
	return p.frame.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}

// backdrop draws the Canvas behind the panel, which is w px wide, with the
// list scrolled by scroll px; the header gains its background as the content
// moves under it. It draws nothing for a track without one.
func (p *panel) backdrop(gtx layout.Context, t *backend.NowTrack, w, scroll int) {
	url := p.canvasURL()
	if url == "" {
		return
	}
	vh := w * 16 / 9
	if p.vid == nil {
		p.vid = startVideo(url, w, vh)
	}
	ui.Offset(gtx, 0, -scroll, func() {
		if !p.vid.draw(gtx, image.Pt(w, vh), 0) {
			// Until the first frame: the cover, where the frame is.
			pad, s := ui.Dp(gtx, 16), w-2*ui.Dp(gtx, 16)
			ui.Offset(gtx, pad, ui.Dp(gtx, canvasHeaderDp), func() {
				ui.Image{URL: ui.Pick(t.Images, s), Radius: ui.Dp(gtx, 8), Icon: "note"}.Layout(gtx, image.Pt(s, s))
			})
		}
		canvasScrims(gtx, image.Rect(0, 0, w, vh), p.grow.at(gtx), 1-p.dim.Step(gtx, p.frame.Hovered(), dimTime))
	})
	hh := ui.Dp(gtx, canvasHeaderDp)
	if a := min(float32(scroll)/float32(ui.Dp(gtx, 32)), 1); a > 0 {
		ui.FillRect(gtx.Ops, image.Rect(0, 0, w, hh), 0, ui.Alpha(ui.ColSurface, a))
	}
}

// canvasScrims are the official overlays on the video r: a shade under the
// header, the scrim fading the bottom into the panel (shorter once expanded,
// e = 1), and the dimming that hovering lifts (at opacity dim).
func canvasScrims(gtx layout.Context, r image.Rectangle, e, dim float32) {
	band := func(from, to float32, a0, a1 float32) {
		y0 := r.Min.Y + int(from*float32(r.Dy()))
		y1 := r.Min.Y + int(to*float32(r.Dy()))
		ui.VGradient(gtx.Ops, image.Rect(r.Min.X, y0, r.Max.X, y1), shade(a0), shade(a1))
	}
	band(0, .3, .85, 0)
	lerp := func(a, b float32) float32 { return a + (b-a)*e }
	clr, mid, solid := lerp(.55, .80), lerp(.72, .9325), lerp(.90, .99)
	band(clr, mid, 0, .8)
	band(mid, solid, .8, 1)
	band(solid, 1, 1, 1)
	if dim > 0 {
		band(0, .3, .63*dim, .53*dim)
		band(.3, .7, .53*dim, .53*dim)
		band(.7, .95, .53*dim, dim)
		band(.95, 1, dim, dim)
	}
}

// shade is the panel's color at CSS opacity a (see ui.Veil: Gio blends in
// linear light).
func shade(a float32) color.NRGBA {
	return ui.Alpha(ui.ColSurface, float32(1-math.Pow(float64(1-a), 2.2)))
}

// expansion animates the Canvas between collapsed (0) and expanded (1) with
// the official ease-out, from wherever it was when toggled.
type expansion struct {
	open     bool
	from     float32
	start    time.Time
	finished bool
}

func (x *expansion) toggle(now time.Time) {
	x.from, x.open, x.start, x.finished = x.at0(now), !x.open, now, false
}

// at is the expansion now, asking for frames until it settles.
func (x *expansion) at(gtx layout.Context) float32 {
	v := x.at0(gtx.Now)
	if !x.finished {
		gtx.Execute(op.InvalidateCmd{})
	}
	return v
}

func (x *expansion) at0(now time.Time) float32 {
	to := float32(0)
	if x.open {
		to = 1
	}
	if x.start.IsZero() {
		x.finished = true
		return to
	}
	t := float32(now.Sub(x.start)) / float32(expandTime)
	if t >= 1 {
		x.finished = true
		return to
	}
	ease := 1 - float32(math.Pow(float64(1-t), 4))
	return x.from + (to-x.from)*ease
}

func (p *panel) stopVideo() {
	if p.vid != nil {
		p.vid.release()
		p.vid = nil
	}
}

// startVideo downloads and decodes url in the background, scaled to w×h px.
func startVideo(url string, w, h int) *canvasVideo {
	ctx, cancel := context.WithCancel(context.Background())
	v := &canvasVideo{url: url, cancel: cancel}
	v.seen.Store(time.Now().UnixNano())
	dir, _ := os.UserCacheDir()
	dir = filepath.Join(dir, "spotimy", "canvas")
	go func() {
		pl, err := video.New(ctx, url, dir, w, h, func() {
			if time.Since(time.Unix(0, v.seen.Load())) > time.Second {
				ui.Post(func() {
					v.release()
					if p.vid == v {
						p.vid = nil
					}
				})
				return
			}
			ui.Invalidate()
		})
		ui.Post(func() {
			switch {
			case err != nil:
				if ctx.Err() == nil {
					ui.Log.Warn("canvas video", "err", err)
				}
			case v.stopped:
				go pl.Close()
			default:
				v.pl = pl
			}
		})
	}()
	return v
}

func (v *canvasVideo) release() {
	if v.stopped {
		return
	}
	v.stopped = true
	v.cancel()
	if v.pl != nil {
		go v.pl.Close()
	}
}

// draw paints the latest frame cropped to fill size; false if there is none yet.
func (v *canvasVideo) draw(gtx layout.Context, size image.Point, rad int) bool {
	v.seen.Store(time.Now().UnixNano())
	if v.pl == nil {
		return false
	}
	fr, n := v.pl.Frame()
	if fr == nil {
		return false
	}
	if n != v.n {
		v.n, v.op = n, paint.NewImageOp(fr)
	}
	fw, fh := float32(fr.Bounds().Dx()), float32(fr.Bounds().Dy())
	sc := max(float32(size.X)/fw, float32(size.Y)/fh)
	off := f32.Pt((float32(size.X)-fw*sc)/2, (float32(size.Y)-fh*sc)/2)
	defer clip.UniformRRect(image.Rectangle{Max: size}, rad).Push(gtx.Ops).Pop()
	defer op.Affine(f32.AffineId().Scale(f32.Pt(0, 0), f32.Pt(sc, sc)).Offset(off)).Push(gtx.Ops).Pop()
	v.op.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return true
}
