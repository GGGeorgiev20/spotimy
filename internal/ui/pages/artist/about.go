package artist

import (
	"image"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

var rankBlue = ui.RGB(0x0d72ea)

// aboutSection is the "About" heading and its picture card (opens the modal).
func (p *page) aboutSection(gtx layout.Context) layout.Dimensions {
	a := p.res.V
	// Merch below gets the grid's 24dp gap (the web stacks them with only
	// 5dp, which looks cramped).
	gap := float32(5)
	if len(a.Merch) > 0 {
		gap = 24
	}
	return ui.PageInset(gtx, 0, gap+sectionGap(len(a.Merch) == 0), func(gtx layout.Context) layout.Dimensions {
		return ui.Column(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return ui.Pad(gtx, 0, 0, 16, 0, func(gtx layout.Context) layout.Dimensions {
					return ui.Text(gtx, "About", 24, ui.Bold, ui.ColText)
				})
			},
			func(gtx layout.Context) layout.Dimensions {
				w, h, r := aboutWidth(gtx.Constraints.Max.X, gtx.Metric.PxPerDp), ui.Dp(gtx, 516), ui.Dp(gtx, 8)
				p.aboutBtn.Label = "About " + a.Name
				return p.aboutBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					ui.FillRect(gtx.Ops, image.Rect(0, 0, w, h), r, ui.ColSurface3)
					if len(a.Gallery) > 0 {
						ui.Image{URL: ui.Pick(a.Gallery, w), Radius: r}.Layout(gtx, image.Pt(w, h))
					}
					ui.VGradient(gtx.Ops, image.Rect(0, 0, w, h), ui.Alpha(ui.ColBg, 0), ui.Veil(.7))
					pad := ui.Dp(gtx, 40)
					if a.WorldRank > 0 {
						ui.Offset(gtx, w-pad-ui.Dp(gtx, 80), pad, func() { rank(gtx, a.WorldRank) })
					}
					g := gtx
					g.Constraints.Max.X = w - 2*pad
					m := op.Record(gtx.Ops)
					d := ui.Column(g,
						func(gtx layout.Context) layout.Dimensions {
							return ui.Pad(gtx, 0, 0, 8, 0, func(gtx layout.Context) layout.Dimensions {
								return ui.Text(gtx, ui.FmtNumber(a.MonthlyListeners)+" monthly listeners", 16, ui.Bold, ui.ColText)
							})
						},
						func(gtx layout.Context) layout.Dimensions {
							return ui.Label{Text: strings.Join(strings.Fields(a.Bio), " "), Size: 16, MaxLines: 3}.Layout(gtx)
						},
					)
					call := m.Stop()
					ui.Offset(gtx, pad, h-pad-d.Size.Y, func() { call.Add(gtx.Ops) })
					return layout.Dimensions{Size: image.Pt(w, h)}
				})
			},
		)
	})
}

// rank is the blue "#84 in the world" badge.
func rank(gtx layout.Context, n int) {
	s := ui.Dp(gtx, 80)
	ui.FillCircle(gtx.Ops, s, rankBlue)
	line := func(txt string, size float32, w font.Weight, y int) {
		ui.Offset(gtx, (s-ui.Measure(gtx, txt, size, w))/2, y, func() { ui.Text(gtx, txt, size, w, ui.ColText) })
	}
	line("#"+ui.FmtNumber(n), 24, ui.Bold, s*22/100)
	line("in the world", 12, ui.Regular, s*55/100)
}

// about is the modal with the full bio, listener stats and top cities.
type about struct {
	a     *spotify.ArtistPage
	list  ui.List
	close ui.Click
	links []ui.Click
	box   bool
	// The gallery carousel: the picture shown, and the slide toward it
	// (from the position it started at, at slideAt).
	prev, next ui.Click
	cur        int
	from       float32
	slideAt    time.Time
}

func (m *about) Layout(gtx layout.Context) layout.Dimensions {
	win := gtx.Constraints.Max
	if m.close.Clicked(gtx) {
		ui.CloseModal()
		return layout.Dimensions{Size: win}
	}
	for i := range m.links {
		if m.links[i].Clicked(gtx) {
			backend.OpenURL(m.a.Links[i].URL)
		}
	}
	// The web player's dialog: 768px wide, at most 70vh tall.
	w, h := min(ui.Dp(gtx, 768), win.X*9/10), win.Y*7/10
	ui.Offset(gtx, (win.X-w)/2, (win.Y-h)/2, func() {
		r := image.Rect(0, 0, w, h)
		ui.Shadow(gtx.Ops, r, ui.Dp(gtx, 8), ui.Dp(gtx, 24), ui.Dp(gtx, 4), .4)
		ui.FillRect(gtx.Ops, r, ui.Dp(gtx, 8), ui.ColSurface)
		st := clip.UniformRRect(r, ui.Dp(gtx, 8)).Push(gtx.Ops)
		event.Op(gtx.Ops, &m.box) // swallow clicks inside the box
		for {
			if _, ok := gtx.Event(pointer.Filter{Target: &m.box, Kinds: pointer.Press}); !ok {
				break
			}
		}
		g := gtx
		g.Constraints = layout.Exact(image.Pt(w, h))
		photos := m.a.Photos
		if len(photos) == 0 && len(m.a.Gallery) > 0 {
			photos = [][]spotify.Image{m.a.Gallery}
		}
		n := 1
		if len(photos) > 0 {
			n = 2
		}
		m.list.Layout(g, n, func(gtx layout.Context, i int) layout.Dimensions {
			if i == 0 && n == 2 {
				return m.carousel(gtx, photos, w)
			}
			return m.body(gtx)
		})
		st.Pop()
		bs := ui.Dp(gtx, 32)
		ui.Offset(gtx, w-ui.Dp(gtx, 16)-bs, ui.Dp(gtx, 16), func() {
			m.close.Label = "Close"
			m.close.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				c := ui.Alpha(ui.ColBg, .7)
				if m.close.Hovered() {
					c = ui.Alpha(ui.ColBg, .9)
				}
				ui.FillCircle(gtx.Ops, bs, c)
				ui.Offset(gtx, bs/4, bs/4, func() { ui.IconPx(gtx, "close", bs/2, ui.ColText) })
				return layout.Dimensions{Size: image.Pt(bs, bs)}
			})
		})
	})
	return layout.Dimensions{Size: win}
}

// carousel is the gallery: one picture at a time, fitted on black in a 16:9
// box, with arrows that slide to the previous and next ones.
func (m *about) carousel(gtx layout.Context, photos [][]spotify.Image, w int) layout.Dimensions {
	if m.prev.Clicked(gtx) && m.cur > 0 {
		m.slideTo(gtx, m.cur-1)
	}
	if m.next.Clicked(gtx) && m.cur < len(photos)-1 {
		m.slideTo(gtx, m.cur+1)
	}
	size := image.Pt(w, w*9/16)
	black := ui.RGB(0x000000)
	pos := m.pos(gtx)
	st := clip.Rect{Max: size}.Push(gtx.Ops)
	ui.FillRect(gtx.Ops, image.Rectangle{Max: size}, 0, black)
	for i, p := range photos {
		if x := int((float32(i) - pos) * float32(w)); x > -w && x < w {
			ui.Offset(gtx, x, 0, func() { ui.Image{URL: ui.Pick(p, w), Contain: true, Placeholder: black}.Layout(gtx, size) })
		}
	}
	st.Pop()
	bs := ui.Dp(gtx, 32)
	arrow := func(c *ui.Click, x int, icon, label string) {
		c.Label = label
		ui.Offset(gtx, x, (size.Y-bs)/2, func() {
			c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				bg, fg := ui.Alpha(black, .7), ui.Alpha(ui.ColText, .7)
				if c.Hovered() {
					bg, fg = ui.ColSurface, ui.ColText
				}
				ui.FillCircle(gtx.Ops, bs, bg)
				ui.Offset(gtx, bs/4, bs/4, func() { ui.IconPx(gtx, icon, bs/2, fg) })
				return layout.Dimensions{Size: image.Pt(bs, bs)}
			})
		})
	}
	if m.cur > 0 {
		arrow(&m.prev, ui.Dp(gtx, 48), "chevron-left", "Previous image")
	}
	if m.cur < len(photos)-1 {
		arrow(&m.next, size.X-ui.Dp(gtx, 48)-bs, "chevron-right", "Next image")
	}
	return layout.Dimensions{Size: size}
}

// slideDur is the carousel's slide (the web player's transform 0.5s).
const slideDur = 500 * time.Millisecond

func (m *about) slideTo(gtx layout.Context, i int) {
	m.from, m.cur, m.slideAt = m.pos(gtx), i, gtx.Now
}

// pos is the carousel's position in pictures, easing toward cur.
func (m *about) pos(gtx layout.Context) float32 {
	t := float32(gtx.Now.Sub(m.slideAt)) / float32(slideDur)
	if m.slideAt.IsZero() || t >= 1 {
		return float32(m.cur)
	}
	gtx.Execute(op.InvalidateCmd{})
	t = 1 - (1-t)*(1-t)*(1-t) // ease out
	return m.from + (float32(m.cur)-m.from)*t
}

// body is the stats column and the bio.
func (m *about) body(gtx layout.Context) layout.Dimensions {
	a := m.a
	pad, w := ui.Dp(gtx, 40), gtx.Constraints.Max.X
	leftW := ui.Dp(gtx, 211)
	stat := func(n int, label string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return ui.Pad(gtx, 0, 0, 40, 0, func(gtx layout.Context) layout.Dimensions {
				return ui.Column(gtx,
					func(gtx layout.Context) layout.Dimensions {
						return ui.Text(gtx, ui.FmtNumber(n), 32, ui.Bold, ui.ColText)
					},
					func(gtx layout.Context) layout.Dimensions { return ui.Text(gtx, label, 14, ui.Regular, ui.ColText2) },
				)
			})
		}
	}
	left := []layout.Widget{}
	if a.WorldRank > 0 {
		left = append(left, func(gtx layout.Context) layout.Dimensions {
			rank(gtx, a.WorldRank)
			return ui.Gap(80, 120)(gtx)
		})
	}
	left = append(left, stat(a.Followers, "Followers"), stat(a.MonthlyListeners, "Monthly Listeners"))
	for _, c := range a.Cities[:min(5, len(a.Cities))] {
		left = append(left, func(gtx layout.Context) layout.Dimensions {
			return ui.Pad(gtx, 0, 0, 16, 0, func(gtx layout.Context) layout.Dimensions {
				return ui.Column(gtx,
					func(gtx layout.Context) layout.Dimensions { return ui.Text(gtx, c.Name, 14, ui.Bold, ui.ColText) },
					func(gtx layout.Context) layout.Dimensions {
						return ui.Text(gtx, ui.FmtNumber(c.Listeners)+" listeners", 14, ui.Regular, ui.ColText2)
					},
				)
			})
		})
	}
	if len(a.Links) > 0 {
		left = append(left, ui.Gap(0, 8))
	}
	for i, l := range a.Links {
		left = append(left, m.linkRow(&m.links[i], l))
	}
	var paras []layout.Widget
	for _, s := range strings.Split(a.Bio, "\n") {
		if s = strings.TrimSpace(s); s != "" {
			paras = append(paras, func(gtx layout.Context) layout.Dimensions {
				return ui.Pad(gtx, 0, 0, 16, 0, func(gtx layout.Context) layout.Dimensions {
					return ui.Label{Text: s, Size: 16, Color: ui.ColText2, MaxLines: -1}.Layout(gtx)
				})
			})
		}
	}
	rightW := max(0, w-2*pad-leftW)
	place := func(x, cw int, ws []layout.Widget) int {
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(cw, 1<<20)}
		var d layout.Dimensions
		ui.Offset(gtx, x, pad, func() { d = ui.Column(g, ws...) })
		return d.Size.Y
	}
	hl := place(pad, leftW, left)
	hr := place(pad+leftW, rightW, paras)
	return layout.Dimensions{Size: image.Pt(w, max(hl, hr)+2*pad)}
}

// linkLabels are the services' display names; other names are shown as sent.
var linkLabels = map[string]string{
	"FACEBOOK": "Facebook", "INSTAGRAM": "Instagram", "TWITTER": "X", "WIKIPEDIA": "Wikipedia", "TIKTOK": "TikTok", "YOUTUBE": "YouTube",
}

// linkIcon is a link's glyph: the service's logo, or a link-out arrow.
func linkIcon(name string) string {
	switch name {
	case "FACEBOOK", "INSTAGRAM", "TIKTOK":
		return strings.ToLower(name)
	case "TWITTER":
		return "x"
	}
	return "link-out"
}

// linkRow is a social link: logo and name, opening in the browser.
func (m *about) linkRow(click *ui.Click, l spotify.Link) layout.Widget {
	label, ok := linkLabels[l.Name]
	if !ok {
		label = strings.ToUpper(l.Name[:1]) + strings.ToLower(l.Name[1:])
	}
	return func(gtx layout.Context) layout.Dimensions {
		click.Label = label
		return ui.Pad(gtx, 0, 0, 16, 0, func(gtx layout.Context) layout.Dimensions {
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				s := ui.Dp(gtx, 24)
				ui.IconPx(gtx, linkIcon(l.Name), s, ui.ColText)
				ui.Offset(gtx, s+ui.Dp(gtx, 4), ui.Dp(gtx, 2), func() {
					g := gtx
					g.Constraints.Max.X = max(0, gtx.Constraints.Max.X-s-ui.Dp(gtx, 4))
					ui.Text(g, label, 16, ui.Regular, ui.ColText)
				})
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, s)}
			})
		})
	}
}

// aboutWidth is the About card's width in a w px wide section: the web player
// puts it in a grid of minmax(180px, 1fr) columns 24px apart where it spans
// four columns (at least 372px).
func aboutWidth(w int, pxPerDp float32) int {
	gap, minCol := 24*pxPerDp, 180*pxPerDp
	n := max(1, int((float32(w)+gap)/(minCol+gap)))
	col := (float32(w)+gap)/float32(n) - gap
	span := min(4, n)
	cw := int(float32(span)*col + float32(span-1)*gap + .5)
	return min(w, max(cw, int(372*pxPerDp)))
}
