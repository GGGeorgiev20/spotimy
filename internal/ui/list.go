package ui

import (
	"image"

	"gioui.org/gesture"
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

	gtx.Constraints.Min = image.Point{}
	dims := l.List.Layout(gtx, n, func(gtx layout.Context, i int) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		d := item(gtx, i)
		if i < len(l.heights) {
			l.heights[i] = d.Size.Y
		}
		return d
	})

	if l.Sticky != nil && l.Scrolled(l.StickyIndex) {
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
	if total > size.Y && size.Y > 0 {
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
