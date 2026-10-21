package npv

import (
	"context"
	"image"
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

func (p *panel) media(gtx layout.Context, t *backend.NowTrack) layout.Dimensions {
	w := gtx.Constraints.Max.X
	h, url := w, ""
	if p.canvas.Ok() && ui.Settings.Canvas {
		url = p.canvas.V
	}
	if url == "" && p.vid != nil { // Canvas was turned off
		p.stopVideo()
	}
	if url != "" {
		h = w * 16 / 9
		if p.vid == nil {
			p.vid = startVideo(url, w, h)
		}
	}
	size := image.Pt(w, h)
	rad := ui.Dp(gtx, 8)
	ui.Shadow(gtx.Ops, image.Rectangle{Max: size}, rad, ui.Dp(gtx, 12), ui.Dp(gtx, 8), .3)
	if p.vid == nil || !p.vid.draw(gtx, size, rad) {
		ui.Image{URL: ui.Pick(t.Images, w), Radius: rad, Icon: "note"}.Layout(gtx, size)
	}
	return layout.Dimensions{Size: size}
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
	dir = filepath.Join(dir, "personal-spotify", "canvas")
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
