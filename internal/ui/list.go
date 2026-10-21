package ui

import (
	"image"
	"math"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"
)

// List is a virtualized vertical scroll view with the official overlay
// scrollbar (shown while hovered). It remembers item heights so the scroll
// offset, scrollbar and sticky headers are exact for items already seen.
type List struct {
	layout.List
	bar     widget.Scrollbar
	hover   gesture.Hover
	heights []int
	n       int
	viewH   int

	// Sticky, if set, is drawn at StickyTop px from the top once item
	// StickyIndex has scrolled past (the track table's column header).
	Sticky      layout.Widget
	StickyIndex int
	StickyTop   int
	// NoBar hides the overlay scrollbar (the library's icon rail).
	NoBar bool

	// Mouse-wheel notches glide instead of jumping.
	wheel glide
}

// wheelNotch is the smallest wheel delta seen, taken as one notch: Gio
// reports ten times the compositor's value (100px on GNOME, 150px on KDE).
var wheelNotch float32

// scrollWheel applies wheel and touchpad scrolling. Touchpads already send
// small, continuous deltas, so only wheel notches are animated, each one 53dp
// like Chromium on Linux.
func (l *List) scrollWheel(gtx layout.Context) {
	for {
		e, ok := gtx.Event(pointer.Filter{Target: &l.wheel, Kinds: pointer.Scroll, ScrollY: pointer.ScrollRange{Min: -1e6, Max: 1e6}})
		if !ok {
			break
		}
		dy := e.(pointer.Event).Scroll.Y
		ady := max(dy, -dy)
		if ady < float32(Dp(gtx, 40)) {
			l.Position.Offset += int(dy)
			l.Position.BeforeEnd = true
			continue
		}
		if wheelNotch == 0 || ady < wheelNotch {
			wheelNotch = ady
		}
		l.wheel.add(gtx.Now, dy/wheelNotch*53*gtx.Metric.PxPerDp, gtx.Metric.PxPerDp)
	}
	if !l.wheel.active() {
		return
	}
	atTop := l.Position.First == 0 && l.Position.Offset <= 0
	atEnd := !l.Position.BeforeEnd && l.Position.First+l.Position.Count >= l.n
	if l.wheel.dist < 0 && atTop || l.wheel.dist > 0 && atEnd {
		l.wheel = glide{}
		return
	}
	l.Position.Offset += l.wheel.step(gtx.Now)
	l.Position.BeforeEnd = true
	if l.wheel.active() {
		gtx.Execute(op.InvalidateCmd{})
	}
}

// glide is one wheel scroll animation, after Chromium's smooth scrolling
// (cc/animation/scroll_offset_animation_curve.cc): an ease-in-out curve over
// 100–200ms, shorter for longer distances, which keeps its speed when another
// notch moves the target.
type glide struct {
	dist, done float32 // px to scroll and px scrolled so far
	start      time.Time
	dur        time.Duration
	slope      float64 // the curve's initial slope (0: plain ease-in-out)
}

func (g *glide) active() bool { return g.dist != 0 }

// glideDuration is Chromium's inverse-delta duration for a distance in dp.
func glideDuration(dp float64) time.Duration {
	frames := min(12, max(6, 12+(dp-120)*(6-12)/(480-120))) // at 60 fps
	return time.Duration(frames / 60 * float64(time.Second))
}

// add moves the target by d px, keeping the current speed.
func (g *glide) add(now time.Time, d, pxPerDp float32) {
	v := 0.0 // px/s
	if g.active() {
		_, s := g.curve(g.frac(now))
		v = float64(g.dist) * s / g.dur.Seconds()
	}
	rem := g.dist - g.done + d
	dur := glideDuration(math.Abs(float64(rem / pxPerDp)))
	slope := 0.0
	if rem != 0 {
		slope = min(1000, max(-1000, v*dur.Seconds()/float64(rem)))
	}
	*g = glide{dist: rem, start: now, dur: dur, slope: slope}
}

// step returns the whole px to scroll this frame.
func (g *glide) step(now time.Time) int {
	x := g.frac(now)
	y, _ := g.curve(x)
	px := int(g.dist*float32(y) - g.done)
	g.done += float32(px)
	if x >= 1 {
		px += int(math.Round(float64(g.dist - g.done)))
		*g = glide{}
	}
	return px
}

func (g *glide) frac(now time.Time) float64 {
	return min(1, max(0, now.Sub(g.start).Seconds()/g.dur.Seconds()))
}

// curve evaluates cubic-bezier(.42, .42*slope, .58, 1) at x: the progress and
// its slope.
func (g *glide) curve(x float64) (y, dydx float64) {
	x1, y1, x2, y2 := .42, .42*g.slope, .58, 1.0
	bez := func(u, a, b float64) float64 { return 3*(1-u)*(1-u)*u*a + 3*(1-u)*u*u*b + u*u*u }
	der := func(u, a, b float64) float64 { return 3*(1-u)*(1-u)*a + 6*(1-u)*u*(b-a) + 3*u*u*(1-b) }
	lo, hi := 0.0, 1.0 // x(u) is monotonic: bisect for u
	for range 30 {
		if bez((lo+hi)/2, x1, x2) < x {
			lo = (lo + hi) / 2
		} else {
			hi = (lo + hi) / 2
		}
	}
	u := (lo + hi) / 2
	if dx := der(u, x1, x2); dx > 1e-9 {
		dydx = der(u, y1, y2) / dx
	}
	return bez(u, y1, y2), dydx
}

func (l *List) height(i, avg int) int {
	if i < len(l.heights) && l.heights[i] > 0 {
		return l.heights[i]
	}
	return avg
}

func (l *List) avg() int {
	sum, n := 0, 0
	for _, h := range l.heights {
		if h > 0 {
			sum += h
			n++
		}
	}
	if n == 0 {
		return 56
	}
	return sum / n
}

// ScrollTop is the distance in px from the top of the content to the top of the view.
func (l *List) ScrollTop() int {
	avg := l.avg()
	top := l.Position.Offset
	for i := 0; i < l.Position.First; i++ {
		top += l.height(i, avg)
	}
	return top
}

// itemTop is the content y of item i's top edge.
func (l *List) itemTop(i int) int {
	avg, top := l.avg(), 0
	for k := 0; k < i; k++ {
		top += l.height(k, avg)
	}
	return top
}

// Scrolled reports whether item i's top edge is above the view.
func (l *List) Scrolled(i int) bool {
	return l.Position.First > i || l.Position.First == i && l.Position.Offset > 0
}

func (l *List) ScrollToTop() {
	l.Position = layout.Position{}
}

// Range is the index range of items currently visible.
func (l *List) Range() (first, last int) {
	return l.Position.First, l.Position.First + l.Position.Count
}

func (l *List) Layout(gtx layout.Context, n int, item layout.ListElement) layout.Dimensions {
	l.Axis = layout.Vertical
	l.n = n
	if len(l.heights) < n {
		l.heights = append(l.heights, make([]int, n-len(l.heights))...)
	} else if len(l.heights) > n {
		l.heights = l.heights[:n]
	}
	size := gtx.Constraints.Max
	l.viewH = size.Y
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	l.hover.Add(gtx.Ops)
	hovered := l.hover.Update(gtx.Source)

	// Scrollbar drag/click from the last frame.
	avg := l.avg()
	total := 0
	for i := 0; i < n; i++ {
		total += l.height(i, avg)
	}
	if d := l.bar.ScrollDistance(); d != 0 && total > 0 {
		l.Position.Offset += int(d * float32(total))
		l.Position.BeforeEnd = true
	}
	l.scrollWheel(gtx)

	gtx.Constraints.Min = image.Point{}
	dims := l.List.Layout(gtx, n, func(gtx layout.Context, i int) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		d := item(gtx, i)
		if i < len(l.heights) {
			l.heights[i] = d.Size.Y
		}
		return d
	})

	if l.Sticky != nil && l.ScrollTop()+l.StickyTop > l.itemTop(l.StickyIndex) {
		Offset(gtx, 0, l.StickyTop, func() {
			g := gtx
			g.Constraints = layout.Constraints{Min: image.Pt(size.X, 0), Max: size}
			l.Sticky(g)
		})
	}

	// Overlay scrollbar.
	avg = l.avg()
	total = 0
	for i := 0; i < n; i++ {
		total += l.height(i, avg)
	}
	if total > size.Y && size.Y > 0 && !l.NoBar {
		top := float32(l.ScrollTop())
		start, end := top/float32(total), (top+float32(size.Y))/float32(total)
		start, end = clamp01(start), clamp01(end)
		w := Dp(gtx, 12)
		Offset(gtx, size.X-w, 0, func() {
			g := gtx
			g.Constraints = layout.Exact(image.Pt(w, size.Y))
			l.bar.Update(g, layout.Vertical, start, end)
			track := clip.Rect{Max: image.Pt(w, size.Y)}.Push(gtx.Ops)
			l.bar.AddTrack(gtx.Ops)
			track.Pop()
			if hovered || l.bar.Dragging() {
				y0, y1 := int(start*float32(size.Y)), int(end*float32(size.Y))
				if mh := Dp(gtx, 24); y1-y0 < mh {
					y1 = y0 + mh
				}
				thumb := image.Rect(0, y0, w, y1)
				c := Alpha(ColText, .3)
				if l.bar.IndicatorHovered() || l.bar.Dragging() {
					c = Alpha(ColText, .5)
				}
				FillRect(gtx.Ops, thumb, 0, c)
				st := clip.Rect(thumb).Push(gtx.Ops)
				l.bar.AddIndicator(gtx.Ops)
				l.bar.AddDrag(gtx.Ops)
				pointer.CursorDefault.Add(gtx.Ops)
				st.Pop()
			}
		})
	}
	// Take the wheel before the list's own scroll gesture does (this area is
	// on top), passing every other event through to the items.
	pass := pointer.PassOp{}.Push(gtx.Ops)
	wa := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, &l.wheel)
	wa.Pop()
	pass.Pop()
	return layout.Dimensions{Size: size, Baseline: dims.Baseline}
}

// Column lays out widgets top to bottom at full width (a non-scrolling stack).
func Column(gtx layout.Context, ws ...layout.Widget) layout.Dimensions {
	y, w := 0, gtx.Constraints.Max.X
	for _, wd := range ws {
		if wd == nil {
			continue
		}
		g := gtx
		g.Constraints.Min = image.Pt(w, 0)
		g.Constraints.Max.Y = max(0, gtx.Constraints.Max.Y-y)
		st := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		d := wd(g)
		st.Pop()
		y += d.Size.Y
	}
	return layout.Dimensions{Size: image.Pt(w, y)}
}
