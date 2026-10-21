package ui

import (
	"container/list"
	"context"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// Images are decoded off the UI goroutine, scaled to a size bucket just above
// what's drawn (so the GPU never minifies much) and kept in an LRU of
// ImageOps; reusing an ImageOp keeps its texture cached on the GPU.

var sizeBuckets = []int{48, 64, 96, 128, 192, 256, 384, 512, 768, 1024}

type imgKey struct {
	url  string
	size int
}

type imgEntry struct {
	key   imgKey
	op    paint.ImageOp
	bytes int
	ready bool
	err   bool
	w, h  int
}

const imgBudget = 160 << 20

var (
	imgLRU   = list.New() // front = most recent
	imgIndex = map[imgKey]*list.Element{}
	imgBytes int
	imgSem   = make(chan struct{}, 4)
)

func bucket(px int) int {
	for _, b := range sizeBuckets {
		if b >= px {
			return b
		}
	}
	return sizeBuckets[len(sizeBuckets)-1]
}

// lookupImage returns the cached image for url at about px, starting a load
// on a miss. ok is false while loading or on error.
func lookupImage(url string, px int) (*imgEntry, bool) {
	k := imgKey{url, bucket(px)}
	if el, ok := imgIndex[k]; ok {
		imgLRU.MoveToFront(el)
		e := el.Value.(*imgEntry)
		return e, e.ready
	}
	e := &imgEntry{key: k}
	imgIndex[k] = imgLRU.PushFront(e)
	Async(func() (*image.RGBA, error) { return decodeImage(url, k.size) }, func(m *image.RGBA, err error) {
		if err != nil || m == nil {
			e.err = true
			return
		}
		e.op, e.ready = paint.NewImageOp(m), true
		e.w, e.h = m.Bounds().Dx(), m.Bounds().Dy()
		e.bytes = len(m.Pix)
		imgBytes += e.bytes
		for imgBytes > imgBudget && imgLRU.Len() > 1 {
			old := imgLRU.Back()
			oe := old.Value.(*imgEntry)
			imgLRU.Remove(old)
			delete(imgIndex, oe.key)
			imgBytes -= oe.bytes
		}
	})
	return e, false
}

func decodeImage(url string, size int) (*image.RGBA, error) {
	imgSem <- struct{}{}
	defer func() { <-imgSem }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path, err := B.ImageFile(ctx, url)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	// Scale so the shorter side is `size` (cover), never upscale.
	if s := min(w, h); s > size {
		w, h = w*size/s, h*size/s
	}
	dst := image.NewRGBA(image.Rect(0, 0, max(1, w), max(1, h)))
	if w == b.Dx() && h == b.Dy() {
		draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	} else {
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	}
	return dst, nil
}

// Image draws a picture cropped to fill size (px), with rounded corners
// (radius px; Round makes it a circle). While loading it shows Placeholder.
type Image struct {
	URL         string
	Radius      int
	Round       bool
	Placeholder color.NRGBA // zero = ColSurface3
	// Icon is drawn centered on the placeholder (e.g. "note", "user").
	Icon string
	// FocusY is where a vertically cropped image is anchored, 0..1 from the
	// top (0 = centered, like CSS object-position 50% 50%).
	FocusY float32
	// Contain fits the whole image inside the box (CSS object-fit: contain)
	// instead of covering it.
	Contain bool
}

func (im Image) Layout(gtx layout.Context, size image.Point) layout.Dimensions {
	r := image.Rectangle{Max: size}
	var cl clip.Stack
	if im.Round {
		cl = clip.Ellipse(r).Push(gtx.Ops)
	} else {
		cl = clip.UniformRRect(r, im.Radius).Push(gtx.Ops)
	}
	defer cl.Pop()
	var e *imgEntry
	ok := false
	if im.URL != "" {
		e, ok = lookupImage(im.URL, max(size.X, size.Y))
	}
	if !ok {
		ph := im.Placeholder
		if ph == ColNone {
			ph = ColSurface3
		}
		paint.ColorOp{Color: ph}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		if im.Icon != "" && (im.URL == "" || e != nil && e.err) {
			is := min(size.X, size.Y) * 2 / 5
			Offset(gtx, (size.X-is)/2, (size.Y-is)/2, func() {
				g := gtx
				g.Constraints = layout.Exact(image.Pt(is, is))
				IconPx(g, im.Icon, is, ColText2)
			})
		}
		return layout.Dimensions{Size: size}
	}
	sc := max(float32(size.X)/float32(e.w), float32(size.Y)/float32(e.h))
	if im.Contain {
		sc = min(float32(size.X)/float32(e.w), float32(size.Y)/float32(e.h))
	}
	fy := im.FocusY
	if fy == 0 {
		fy = .5
	}
	off := f32.Pt((float32(size.X)-float32(e.w)*sc)/2, (float32(size.Y)-float32(e.h)*sc)*fy)
	defer op.Affine(f32.AffineId().Scale(f32.Pt(0, 0), f32.Pt(sc, sc)).Offset(off)).Push(gtx.Ops).Pop()
	e.op.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

// Pick returns the URL of the smallest image at least min px wide, else the
// largest (images without a width count as 640).
func Pick(images []spotify.Image, minPx int) string {
	best, bestW := "", 0
	largest, largestW := "", -1
	for _, im := range images {
		w := im.Width
		if w == 0 {
			w = 640
		}
		if w >= minPx && (best == "" || w < bestW) {
			best, bestW = im.URL, w
		}
		if w > largestW {
			largest, largestW = im.URL, w
		}
	}
	if best != "" {
		return best
	}
	return largest
}

// Cover draws the best image from images for a size×size square.
func Cover(gtx layout.Context, images []spotify.Image, size, radius int, round bool, icon string) layout.Dimensions {
	return Image{URL: Pick(images, size), Radius: radius, Round: round, Icon: icon}.Layout(gtx, image.Pt(size, size))
}

// imagesPending reports images still loading (for the test harness).
func imagesPending() int {
	n := 0
	for el := imgLRU.Front(); el != nil; el = el.Next() {
		if e := el.Value.(*imgEntry); !e.ready && !e.err {
			n++
		}
	}
	return n
}
