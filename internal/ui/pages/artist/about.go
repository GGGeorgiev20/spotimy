package artist

import (
	"image"
	"image/color"
	"net/url"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"

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

// ShowAbout opens the artist's About dialog (from the artist page and the
// Now Playing panel's "About the artist" card).
func ShowAbout(uri string) {
	m := &about{uri: uri}
	m.res.Load(func() (*spotify.ArtistAbout, error) { return ui.B.ArtistAbout(uri) })
	ui.ShowOverlay(m)
}

// about is the web player's About dialog: the gallery, then the stats and
// links beside the bio and the artist's verification.
type about struct {
	uri    string
	res    ui.Res[*spotify.ArtistAbout]
	list   ui.List
	close  ui.Click
	links  []ui.Click
	learn  ui.Click // "Learn more" about verification
	report ui.Click
	box    bool
	// contentH is the content's height last frame: the dialog shrinks to it
	// (up to 70% of the window).
	contentH int
	// The gallery carousel: the picture shown, and the slide toward it
	// (from the position it started at, at slideAt).
	prev, next ui.Click
	cur        int
	from       float32
	slideAt    time.Time
}

const verifiedURL = "https://support.spotify.com/article/verified-by-spotify/"

// reportURL is Spotify's form for reporting uri (the web player's
// getReportURL, for the desktop app).
func reportURL(uri string) string {
	return "https://support.spotify.com/abuse?uri=" + url.QueryEscape(uri) + "&platform=desktop"
}

func (m *about) Layout(gtx layout.Context) layout.Dimensions {
	win := gtx.Constraints.Max
	if m.close.Clicked(gtx) {
		ui.CloseModal()
		return layout.Dimensions{Size: win}
	}
	a := m.res.V
	if a != nil {
		if len(m.links) < len(a.Links) {
			m.links = make([]ui.Click, len(a.Links))
		}
		for i := range a.Links {
			if m.links[i].Clicked(gtx) {
				backend.OpenURL(a.Links[i].URL)
			}
		}
		if m.learn.Clicked(gtx) {
			backend.OpenURL(verifiedURL)
		}
		if m.report.Clicked(gtx) {
			backend.OpenURL(reportURL(a.URI))
		}
	}
	// The web player's dialog: 768px wide, at most 70vh tall.
	w, h := min(ui.Dp(gtx, 768), win.X*9/10), win.Y*7/10
	if a != nil && m.contentH > 0 {
		h = min(h, m.contentH)
	} else if a == nil {
		h = min(h, ui.Dp(gtx, 240))
	}
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
		switch {
		case a == nil && m.res.Err != nil:
			layout.Center.Layout(g, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, "Couldn't load this artist. Try again later.", 16, ui.Regular, ui.ColText2)
			})
		case a == nil:
			layout.Center.Layout(g, func(gtx layout.Context) layout.Dimensions { return ui.Spinner(gtx, 32) })
		default:
			n, total := 1, 0
			if len(a.Photos) > 0 {
				n = 2
			}
			m.list.Layout(g, n, func(gtx layout.Context, i int) layout.Dimensions {
				var d layout.Dimensions
				if i == 0 && n == 2 {
					d = m.carousel(gtx, a.Photos, w)
				} else {
					d = m.body(gtx, a)
				}
				total += d.Size.Y
				return d
			})
			if m.list.Position.First == 0 && total > 0 && total != m.contentH {
				m.contentH = total
				gtx.Execute(op.InvalidateCmd{})
			}
		}
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

// body is the stats column beside the bio, who posted it and the artist's
// verification (the web player's 223px and remaining columns, 40px padding).
func (m *about) body(gtx layout.Context, a *spotify.ArtistAbout) layout.Dimensions {
	pad, w := ui.Dp(gtx, 40), gtx.Constraints.Max.X
	leftW := ui.Dp(gtx, 223)
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
		left = append(left, ui.Gap(0, 24))
	}
	for i, l := range a.Links {
		left = append(left, m.linkRow(&m.links[i], l))
	}
	left = append(left, ui.Gap(0, 16), m.reportButton)

	var right []layout.Widget
	block := func(ws ...layout.Widget) {
		if len(right) > 0 {
			right = append(right, ui.Gap(0, 32))
		}
		right = append(right, ws...)
	}
	if a.Bio != "" {
		var paras []layout.Widget
		for _, s := range strings.Split(a.Bio, "\n") {
			if s = strings.TrimSpace(s); s != "" {
				if len(paras) > 0 {
					paras = append(paras, ui.Gap(0, 16))
				}
				paras = append(paras, func(gtx layout.Context) layout.Dimensions {
					return ui.Label{Text: s, Size: 16, Color: ui.ColText2, MaxLines: -1, LineHeight: 22}.Layout(gtx)
				})
			}
		}
		block(paras...)
		if a.Autobiography {
			block(func(gtx layout.Context) layout.Dimensions {
				s := ui.Dp(gtx, 32)
				ui.Cover(gtx, a.Avatar, s, 0, true, "artist")
				ui.Offset(gtx, s+ui.Dp(gtx, 8), (s-ui.Dp(gtx, 20))/2, func() {
					g := gtx
					g.Constraints.Max.X = max(0, gtx.Constraints.Max.X-s-ui.Dp(gtx, 8))
					ui.Text(g, "Posted By "+a.Name, 14, ui.Regular, ui.ColText2)
				})
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, s)}
			})
		}
	}
	var rep []layout.Widget
	if a.Verified {
		rep = append(rep, func(gtx layout.Context) layout.Dimensions { return m.verifiedCard(gtx) })
	}
	if a.Registered {
		if len(rep) > 0 {
			rep = append(rep, ui.Gap(0, 16))
		}
		rep = append(rep, func(gtx layout.Context) layout.Dimensions {
			return card(gtx, ui.RGB(0x1f1f1f), ui.Dp(gtx, 8), func(gtx layout.Context) layout.Dimensions {
				return ui.Column(gtx,
					func(gtx layout.Context) layout.Dimensions {
						return ui.Label{Text: "Registered artist", Size: 16, Weight: ui.Bold, LineHeight: 22}.Layout(gtx)
					},
					ui.Gap(0, 8),
					func(gtx layout.Context) layout.Dimensions {
						return ui.Label{Text: "This artist has a claimed profile in Spotify for Artists.", Size: 14, Color: ui.ColText2, MaxLines: -1, LineHeight: 20}.Layout(gtx)
					},
				)
			})
		})
	}
	if len(rep) > 0 {
		block(rep...)
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
	hr := place(pad+leftW, rightW, right)
	return layout.Dimensions{Size: image.Pt(w, max(hl, hr)+2*pad)}
}

// verifiedCard is "Verified by Spotify" with its badge and explanation,
// ending in a "Learn more" link.
func (m *about) verifiedCard(gtx layout.Context) layout.Dimensions {
	return card(gtx, ui.RGB(0x282828), ui.Dp(gtx, 6), func(gtx layout.Context) layout.Dimensions {
		return ui.Column(gtx,
			func(gtx layout.Context) layout.Dimensions {
				s := ui.Dp(gtx, 24)
				ui.IconPx(gtx, "verified", s, ui.RGB(0xc5f0c9))
				ui.Offset(gtx, s+ui.Dp(gtx, 8), (s-ui.Dp(gtx, 20))/2, func() {
					ui.Text(gtx, "Verified by Spotify", 14, ui.Bold, ui.ColText)
				})
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, s)}
			},
			ui.Gap(0, 4),
			func(gtx layout.Context) layout.Dimensions {
				return tailLink(gtx, ui.Label{
					Text: "This artist has grown an active fanbase to be eligible for review and has met Spotify's criteria for profile authenticity.",
					Size: 14, Color: ui.ColText2, MaxLines: -1, LineHeight: 20,
				}, "Learn more", &m.learn)
			},
		)
	})
}

// card is a rounded panel with 16dp padding, filling the width.
func card(gtx layout.Context, bg color.NRGBA, r int, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := ui.Pad(gtx, 16, 16, 16, 16, w)
	call := m.Stop()
	d.Size.X = gtx.Constraints.Max.X
	ui.FillRect(gtx.Ops, image.Rectangle{Max: d.Size}, r, bg)
	call.Add(gtx.Ops)
	return d
}

// tailLink lays out l with link appended to its last line, like an inline
// text link: link (kept on one line) is underlined and clickable.
func tailLink(gtx layout.Context, l ui.Label, link string, c *ui.Click) layout.Dimensions {
	nbsp := strings.ReplaceAll(link, " ", " ")
	l.Text += " " + nbsp
	d := l.Layout(gtx)
	// Find where the last line ends: the link is its tail.
	size := l.Size
	ui.Shaper.LayoutString(text.Parameters{
		Font: font.Font{Typeface: "Figtree", Weight: l.Weight}, PxPerEm: fixed.I(gtx.Sp(unit.Sp(size))),
		WrapPolicy: text.WrapWords, MaxWidth: gtx.Constraints.Max.X, Locale: gtx.Locale, LineHeight: fixed.I(gtx.Sp(unit.Sp(l.LineHeight))),
	}, l.Text)
	var x1 fixed.Int26_6
	var y, asc, desc int
	for g, ok := ui.Shaper.NextGlyph(); ok; g, ok = ui.Shaper.NextGlyph() {
		if int(g.Y) != y {
			x1 = 0
		}
		y, asc, desc = int(g.Y), g.Ascent.Ceil(), g.Descent.Ceil()
		x1 = max(x1, g.X+g.Advance)
	}
	lw := ui.Measure(gtx, nbsp, size, l.Weight)
	r := image.Rect(x1.Ceil()-lw, y-asc, x1.Ceil(), y+desc)
	c.Label = link
	ui.Offset(gtx, r.Min.X, r.Min.Y, func() {
		c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			th := max(1, gtx.Sp(unit.Sp(size/12)))
			ui.FillRect(gtx.Ops, image.Rect(0, asc+th, r.Dx(), asc+2*th), 0, l.Color)
			return layout.Dimensions{Size: r.Size()}
		})
	})
	return d
}

// reportButton is the small "Report" text button with a link-out arrow: it
// opens Spotify's report form in the browser.
func (m *about) reportButton(gtx layout.Context) layout.Dimensions {
	m.report.Label = "Report"
	return m.report.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		c := ui.ColText2
		if m.report.Hovered() {
			c = ui.ColText
		}
		h, is, gap := ui.Dp(gtx, 32), ui.Dp(gtx, 16), ui.Dp(gtx, 8)
		var d layout.Dimensions
		ui.Offset(gtx, 0, (h-ui.Dp(gtx, 20))/2, func() { d = ui.Text(gtx, "Report", 14, ui.Bold, c) })
		ui.Offset(gtx, d.Size.X+gap, (h-is)/2, func() { ui.IconPx(gtx, "link-out", is, c) })
		return layout.Dimensions{Size: image.Pt(d.Size.X+gap+is, h)}
	})
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
					ui.Text(g, label, 14, ui.Regular, ui.ColText)
				})
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, s)}
			})
		})
	}
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
