package artist

import (
	"image"
	"strings"

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
	return ui.PageInset(gtx, 0, sectionGap(len(a.Merch) == 0), func(gtx layout.Context) layout.Dimensions {
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
					ui.VGradient(gtx.Ops, image.Rect(0, 0, w, h), ui.Alpha(ui.ColBg, 0), veil(.7))
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
	w, h := min(ui.Dp(gtx, 768), win.X*9/10), min(ui.Dp(gtx, 560), win.Y*7/10)
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
		n := 1
		if len(m.a.Gallery) > 0 {
			n = 2
		}
		m.list.Layout(g, n, func(gtx layout.Context, i int) layout.Dimensions {
			if i == 0 && n == 2 {
				s := image.Pt(w, w*9/16)
				ui.Image{URL: ui.Pick(m.a.Gallery, w)}.Layout(gtx, s)
				return layout.Dimensions{Size: s}
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
