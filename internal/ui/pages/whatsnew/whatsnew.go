// Package whatsnew is the What's New page (/content-feed), opened by the top
// bar's bell: new releases and episodes from the artists and shows you follow.
package whatsnew

import (
	"image"
	"slices"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() {
	ui.Register("content-feed", func([]string) ui.Page {
		p := &page{}
		p.load()
		return p
	})
}

// The filter chips: ids are what the feed query takes.
var chips = []struct{ id, label string }{{"ALBUM", "Music"}, {"EPISODE", "Podcast & Shows"}}

// bannerGone is kept until the app closes, like the web player's local flag.
var bannerGone bool

type page struct {
	res    ui.Res[[]spotify.FeedItem]
	active []string // chips that are on
	list   ui.List
	pills  [2]ui.Pill
	close  ui.IconButton
	links  [2]ui.Link
	retry  ui.Pill
	rows   map[string]*item
}

func (p *page) load() {
	kinds := slices.Clone(p.active)
	p.res.Load(func() ([]spotify.FeedItem, error) { return ui.B.WhatsNew(kinds) })
}

func (p *page) Layout(gtx layout.Context) layout.Dimensions {
	px := func(v float32) int { return ui.Dp(gtx, v) }
	w := gtx.Constraints.Max.X
	// A centered column, at most 800dp wide with 40dp around it, padded like content.
	colW := min(px(800), w-2*px(40))
	innerW := colW - 2*px(24)
	x0 := (w-colW)/2 + px(24)
	inner := func(f func(gtx layout.Context) layout.Dimensions) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			var d layout.Dimensions
			ui.Offset(gtx, x0, 0, func() {
				g := gtx
				g.Constraints = layout.Constraints{Max: image.Pt(innerW, gtx.Constraints.Max.Y)}
				d = f(g)
			})
			return layout.Dimensions{Size: image.Pt(w, d.Size.Y)}
		}
	}
	gap := func(h float32) layout.Widget { return ui.Gap(0, h) }

	for i := range p.pills {
		if p.pills[i].Click.Clicked(gtx) {
			if j := slices.Index(p.active, chips[i].id); j >= 0 {
				p.active = slices.Delete(p.active, j, j+1)
			} else {
				p.active = append(p.active, chips[i].id)
			}
			p.load()
		}
	}
	if p.close.Click.Clicked(gtx) {
		bannerGone = true
	}
	for i := range p.links {
		if p.links[i].Click.Clicked(gtx) {
			ui.Navigate("/")
		}
	}

	items := []layout.Widget{gap(40)}
	if !bannerGone {
		items = append(items, inner(p.banner))
	}
	items = append(items, inner(func(gtx layout.Context) layout.Dimensions {
		return ui.Column(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return ui.Pad(gtx, 48, 0, 4, 0, func(gtx layout.Context) layout.Dimensions {
					return ui.Text(gtx, "What's New", 32, ui.Bold, ui.ColText)
				})
			},
			func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, "The latest releases from artists, podcasts, and shows you follow.", 14, ui.Regular, ui.ColText2)
			},
			func(gtx layout.Context) layout.Dimensions { return p.filters(gtx) },
		)
	}))

	feed := p.res.V
	switch {
	case !p.res.Done:
		items = append(items, gap(48), ui.Loading)
	case p.res.Err != nil:
		items = append(items, inner(p.failed))
	case len(feed) == 0:
		items = append(items, inner(p.empty))
	default:
		items = append(items, gap(48))
		if p.rows == nil {
			p.rows = map[string]*item{}
		}
		earlier := slices.IndexFunc(feed, func(f spotify.FeedItem) bool { return !f.New })
		for i, f := range feed {
			it := p.rows[f.ID]
			if it == nil {
				it = &item{}
				p.rows[f.ID] = it
			}
			f := f
			head := ""
			switch {
			case i == 0 && f.New:
				head = "New"
			case i == earlier:
				head = "Earlier"
			}
			items = append(items, func(gtx layout.Context) layout.Dimensions {
				var d layout.Dimensions
				// The list reaches 8dp beyond the column on both sides.
				ui.Offset(gtx, x0-px(8), 0, func() {
					g := gtx
					g.Constraints = layout.Constraints{Max: image.Pt(innerW+px(16), gtx.Constraints.Max.Y)}
					d = it.layout(g, f, head, i == 0 || head != "")
				})
				return layout.Dimensions{Size: image.Pt(w, d.Size.Y)}
			})
		}
	}
	items = append(items, gap(32))
	return p.list.Layout(gtx, len(items), func(gtx layout.Context, i int) layout.Dimensions { return items[i](gtx) })
}

// banner points at the Home feeds; it can be hidden.
func (p *page) banner(gtx layout.Context) layout.Dimensions {
	px := func(v float32) int { return ui.Dp(gtx, v) }
	w, h := gtx.Constraints.Max.X, px(48)
	ui.FillRect(gtx.Ops, image.Rect(0, 0, w, h), px(6), ui.RGB(0x0d72ea))
	ui.Offset(gtx, px(12), (h-px(24))/2, func() { ui.IconPx(gtx, "info", px(24), ui.ColText) })
	tx := px(12 + 24 + 16)
	ui.Offset(gtx, tx, (h-px(20))/2, func() {
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(w-tx-px(12+24+16), px(20))}
		ui.Inline(g, 14, ui.Regular, ui.ColText, ui.ColNone,
			ui.Seg{Text: "Looking for the latest from creators you follow? Check the "},
			ui.Seg{Text: "Music", Link: &p.links[0]},
			ui.Seg{Text: " or "},
			ui.Seg{Text: "Podcast", Link: &p.links[1]},
			ui.Seg{Text: " feeds on Home"})
	})
	ui.Offset(gtx, w-px(12+24), (h-px(24))/2, func() {
		p.close.Layout(gtx, ui.IconStyle{Icon: "close", Size: 16, Box: 24, Color: ui.ColText, Label: "Hide notification"})
	})
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// filters is the Music / Podcast & Shows chips; each one toggles.
func (p *page) filters(gtx layout.Context) layout.Dimensions {
	return ui.Pad(gtx, 16, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
		x := 0
		for i, c := range chips {
			on := slices.Contains(p.active, c.id)
			ui.Offset(gtx, x, 0, func() {
				d := p.pills[i].Layout(gtx, ui.PillStyle{Text: c.label, PadX: 12, Bg: ui.Mix(ui.ColSurface, ui.ColText, .07), BgHover: ui.Mix(ui.ColSurface, ui.ColText, .1), Selected: on})
				x += d.Size.X + ui.Dp(gtx, 8)
			})
		}
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, ui.Dp(gtx, 32))}
	})
}

// empty is what shows when there's nothing to list.
func (p *page) empty(gtx layout.Context) layout.Dimensions {
	title, msg := "We don't have any updates for you yet", "When there's news, we'll post it here. Follow your favorite artists and podcasts to stay updated on them too."
	switch {
	case slices.Equal(p.active, []string{"EPISODE"}):
		title, msg = "Nothing new in podcasts", "Follow your favorite podcasts and we'll keep you updated on them."
	case slices.Equal(p.active, []string{"ALBUM"}):
		title, msg = "Nothing new in music", "Follow your favorite artists and we'll keep you updated on them."
	}
	return ui.Pad(gtx, 96, 0, 96, 0, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Label{Text: title, Size: 32, Weight: ui.Bold, Align: text.Middle, MaxLines: 2}.Layout(gtx)
			}),
			layout.Rigid(ui.Gap(0, 8)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Label{Text: msg, Size: 16, Align: text.Middle, MaxLines: 3}.Layout(gtx)
			}),
		)
	})
}

func (p *page) failed(gtx layout.Context) layout.Dimensions {
	if p.retry.Click.Clicked(gtx) {
		p.load()
	}
	ui.Log.Warn("what's new unavailable", "err", p.res.Err)
	return ui.Pad(gtx, 96, 0, 96, 0, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Label{Text: "Something went wrong.", Size: 24, Weight: ui.Bold, Align: text.Middle}.Layout(gtx)
			}),
			layout.Rigid(ui.Gap(0, 8)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Label{Text: "Check your internet connection and try again", Size: 16, Align: text.Middle, MaxLines: 2}.Layout(gtx)
			}),
			layout.Rigid(ui.Gap(0, 16)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.retry.Layout(gtx, ui.PillStyle{Text: "Try again", Weight: ui.Bold, Selected: true})
			}),
		)
	})
}

// item is one feed entry: a release or an episode.
type item struct {
	row, play, like, more ui.Click
	artists               []ui.Link
	show                  ui.Link
}

func (it *item) layout(gtx layout.Context, f spotify.FeedItem, head string, first bool) layout.Dimensions {
	px := func(v float32) int { return ui.Dp(gtx, v) }
	w := gtx.Constraints.Max.X
	card := spotify.Card{URI: f.URI, Type: spotify.URIType(f.URI), Name: f.Name, Images: f.Images}
	ui.LoadSaved([]string{f.URI})

	episode := f.Episode != nil
	current := ui.IsContext(f.URI)
	if episode {
		current = ui.IsCurrent(f.URI, "")
	}
	if it.play.Clicked(gtx) {
		switch {
		case current:
			ui.PlayPause()
		case episode:
			ui.Play(f.URI, "")
		default:
			ui.PlayContext(f.URI)
		}
	}
	if it.like.Clicked(gtx) {
		ui.ToggleSaved(f.URI)
	}
	if it.more.Clicked(gtx) {
		ui.OpenMenu(ui.CardMenu(card, ui.CardMenuOpts{}))
	}
	if it.row.Clicked(gtx) {
		ui.OpenURI(f.URI)
	}
	if _, ok := it.row.RightClicked(gtx); ok {
		ui.OpenMenu(ui.CardMenu(card, ui.CardMenuOpts{}))
	}
	if len(it.artists) < len(f.Artists) {
		it.artists = make([]ui.Link, len(f.Artists))
	}
	for k := range f.Artists {
		if it.artists[k].Click.Clicked(gtx) {
			ui.OpenURI(f.Artists[k].URI)
		}
	}
	if it.show.Click.Clicked(gtx) && episode {
		ui.OpenURI(f.Episode.ShowURI)
	}

	y := 0
	if head != "" {
		top := 0
		if !first || head == "Earlier" {
			top = px(24)
		}
		ui.Offset(gtx, px(8), top, func() { ui.Text(gtx, head, 24, ui.Bold, ui.ColText) })
		y = top + px(24+12) + px(5)
	}
	// Divider above the row (hidden on hover, like the web).
	hov := it.row.Hovered()
	if head == "" && !hov {
		ui.Offset(gtx, 0, y, func() { ui.Hline(gtx, w, ui.ColDivider) })
	}
	large := w >= px(600)
	pad := px(16)
	var rowH int
	ui.Offset(gtx, 0, y, func() {
		it.row.Label, it.row.Arrow = f.Name, false
		it.row.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			media := px(64)
			mediaH := media
			if large {
				media, mediaH = px(176), px(99)
			}
			tx := pad + media + px(24)
			if !large {
				tx = pad + media + px(16)
			}
			m := op.Record(gtx.Ops)
			g := gtx
			g.Constraints = layout.Constraints{Max: image.Pt(max(0, w-tx-pad), px(4000))}
			var th int
			ui.Offset(gtx, tx, pad, func() {
				th = ui.Column(g,
					func(gtx layout.Context) layout.Dimensions {
						col := ui.ColText
						if current {
							col = ui.ColAccent
						}
						return ui.Label{Text: f.Name, Size: 16, Weight: ui.Bold, Color: col, MaxLines: 2}.Layout(gtx)
					},
					func(gtx layout.Context) layout.Dimensions { return it.subtitle(gtx, f) },
					func(gtx layout.Context) layout.Dimensions { return it.body(gtx, f) },
					func(gtx layout.Context) layout.Dimensions { return it.footer(gtx, f, card, current, hov) },
				).Size.Y
			})
			call := m.Stop()
			rowH = 2*pad + max(mediaH, th)
			if hov {
				ui.FillRect(gtx.Ops, image.Rect(0, 0, w, rowH), px(6), ui.ColRowHover)
			}
			ui.Offset(gtx, pad, pad, func() { cover(gtx, f, media, mediaH, large) })
			call.Add(gtx.Ops)
			return layout.Dimensions{Size: image.Pt(w, rowH)}
		})
	})
	return layout.Dimensions{Size: image.Pt(w, y+rowH)}
}

// cover draws the release art: in wide rows centered on a 16:9 gradient tile.
func cover(gtx layout.Context, f spotify.FeedItem, w, h int, large bool) {
	r := ui.Dp(gtx, 4)
	if !large {
		ui.Image{URL: ui.Pick(f.Images, w), Radius: r, Icon: "note"}.Layout(gtx, image.Pt(w, w))
		return
	}
	ui.VGradient(gtx.Ops, image.Rect(0, 0, w, h), ui.RGB(0x535353), ui.RGB(0x121212))
	ui.Offset(gtx, (w-h)/2, 0, func() {
		ui.Shadow(gtx.Ops, image.Rect(0, 0, h, h), r, ui.Dp(gtx, 28), ui.Dp(gtx, 4), .8)
		ui.Image{URL: ui.Pick(f.Images, h), Radius: r, Icon: "note"}.Layout(gtx, image.Pt(h, h))
	})
}

func (it *item) subtitle(gtx layout.Context, f spotify.FeedItem) layout.Dimensions {
	if f.Episode != nil {
		return it.show.Layout(gtx, f.Episode.ShowName, 14, ui.Regular, ui.ColText2, ui.ColText)
	}
	var segs []ui.Seg
	for k, a := range f.Artists {
		if k > 0 {
			segs = append(segs, ui.Seg{Text: ", "})
		}
		segs = append(segs, ui.Seg{Text: a.Name, Link: &it.artists[k]})
	}
	return ui.Inline(gtx, 14, ui.Regular, ui.ColText2, ui.ColText, segs...)
}

// body is "Single · 2 days ago" for a release, the description for an episode.
func (it *item) body(gtx layout.Context, f spotify.FeedItem) layout.Dimensions {
	if f.Episode != nil {
		return ui.Pad(gtx, 8, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
			return ui.Label{Text: f.Episode.Description, Size: 14, Color: ui.ColText2, MaxLines: 2}.Layout(gtx)
		})
	}
	s := f.AlbumType
	if d := ui.FmtAdded(f.Date, gtx.Now); d != "" {
		s += " · " + d
	}
	return ui.Pad(gtx, 4, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
		return ui.Text(gtx, s, 14, ui.Regular, ui.ColText2)
	})
}

// footer holds the episode's tag line, the save and more buttons (the heart
// stays, "more" shows on hover) and the play button.
func (it *item) footer(gtx layout.Context, f spotify.FeedItem, card spotify.Card, current, hov bool) layout.Dimensions {
	px := func(v float32) int { return ui.Dp(gtx, v) }
	return ui.Pad(gtx, 8, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
		var kids []layout.Widget
		if f.Episode != nil {
			kids = append(kids, func(gtx layout.Context) layout.Dimensions {
				s := "Episode"
				if d := ui.FmtAdded(f.Date, gtx.Now); d != "" {
					s += " · " + d
				}
				return ui.Pad(gtx, 0, 0, 8, 0, func(gtx layout.Context) layout.Dimensions { return ui.Text(gtx, s, 14, ui.Regular, ui.ColText2) })
			})
		}
		kids = append(kids, func(gtx layout.Context) layout.Dimensions {
			bs := px(32)
			g := gtx
			g.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, bs))
			layout.Flex{Alignment: layout.Middle}.Layout(g,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					saved := ui.Saved[f.URI]
					icon, c, label := "heart", ui.ColText2, "Save to Your Library"
					if saved {
						icon, c, label = "heart-filled", ui.ColAccent, "Remove from Your Library"
					} else if it.like.Hovered() {
						c = ui.ColText
					}
					it.like.Label = label
					return it.like.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						ui.Offset(gtx, px(8), px(8), func() { ui.IconPx(gtx, icon, px(16), c) })
						return layout.Dimensions{Size: image.Pt(bs, bs)}
					})
				}),
				layout.Rigid(ui.Gap(32, 0)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !hov && !it.more.Hovered() {
						return layout.Dimensions{Size: image.Pt(bs, bs)}
					}
					c := ui.ColText2
					if it.more.Hovered() {
						c = ui.ColText
					}
					it.more.Label = "More options for " + f.Name
					return it.more.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						ui.Offset(gtx, px(8), px(8), func() { ui.IconPx(gtx, "more", px(16), c) })
						return layout.Dimensions{Size: image.Pt(bs, bs)}
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, bs)}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					icon, label := "play", "Play "+f.Name
					if current && ui.Playing() {
						icon, label = "pause", "Pause "+f.Name
					}
					it.play.Label = label
					return it.play.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						ui.FillCircle(gtx.Ops, bs, ui.ColText)
						ui.Offset(gtx, bs/4, bs/4, func() { ui.IconPx(gtx, icon, bs/2, ui.ColBg) })
						return layout.Dimensions{Size: image.Pt(bs, bs)}
					})
				}),
			)
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, bs)}
		})
		return ui.Column(gtx, kids...)
	})
}
