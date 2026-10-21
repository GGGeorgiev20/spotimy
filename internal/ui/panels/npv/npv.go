// Package npv is the Now Playing view (Prefs.RightPanel "npv"): cover or
// Canvas video, title, the lyrics card, the artist card, credits and the next
// queued track.
package npv

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui/pages/artist"
)

func init() { ui.RightPanels["npv"] = p.layout }

var p panel

type panel struct {
	hide, expand ui.IconButton
	more         ui.IconButton
	list         ui.List
	ctx          ui.ContextName

	// Per-track state; the Res values are replaced on a track change so late
	// results of the previous track can't land in them.
	uri    string
	info   *ui.Res[*spotify.NowPlayingInfo]
	canvas *ui.Res[string]
	vid    *canvasVideo
	lyr    lyricsCard

	queue    ui.Res[*backend.Queue]
	qVersion int

	ctxLink, title, showAll, openQueue, artistName ui.Link
	artists, credits                               []ui.Link
	creditFollow                                   []ui.Pill
	save                                           ui.IconButton
	about                                          ui.Click
	follow                                         ui.Pill
}

// track switches to the playing track's data when it changed.
func (p *panel) track(gtx layout.Context) *backend.NowTrack {
	t := ui.NowTrack()
	uri := ""
	if t != nil {
		uri = t.URI
	}
	if p.info == nil || (uri != p.uri && ui.Settled(gtx)) {
		p.uri = uri
		p.stopVideo()
		p.list.ScrollToTop()
		p.info, p.canvas = &ui.Res[*spotify.NowPlayingInfo]{}, &ui.Res[string]{}
		if t != nil && ui.URIType(uri) == "track" {
			artist := ""
			if len(t.Artists) > 0 {
				artist = t.Artists[0].URI
			}
			ui.LoadSaved([]string{uri, artist})
			ui.LoadInPlaylists()
			p.lyr.load(uri, t.Images)
			p.canvas.Load(func() (string, error) {
				u, err := ui.B.Canvas(uri)
				if err != nil {
					ui.Log.Warn("canvas", "err", err)
				}
				return u, err
			})
			p.info.Load(func() (*spotify.NowPlayingInfo, error) { return ui.B.NowPlaying(uri, artist) })
		}
	}
	if p.qVersion != ui.Player.Version && (p.qVersion == 0 || ui.Settled(gtx)) {
		p.qVersion = ui.Player.Version
		p.queue.Load(ui.B.Queue)
	}
	return t
}

func (p *panel) layout(gtx layout.Context) layout.Dimensions {
	t := p.track(gtx)
	var ctxURI, ctxName string
	if s := ui.Player.Status; s != nil && t != nil {
		ctxURI, ctxName = s.ContextURI, p.ctx.Get(s.ContextURI)
		if ctxName == "" { // no context (or not resolved yet): the album
			ctxURI, ctxName = t.AlbumURI, t.AlbumName
		}
	}
	if p.ctxLink.Click.Clicked(gtx) && ctxURI != "" {
		ui.OpenURI(ctxURI)
	}
	if p.more.Click.Clicked(gtx) && t != nil {
		ui.OpenMenu(ui.TrackMenu(ui.TrackOf(t), ui.TrackMenuOpts{}))
	}
	return ui.Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return p.headerRow(gtx, t, ctxName)
		},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			if t == nil {
				return ui.Pad(gtx, 0, 16, 0, 16, func(gtx layout.Context) layout.Dimensions {
					return ui.Text(gtx, "Nothing is playing.", 14, ui.Regular, ui.ColText2)
				})
			}
			return p.body(gtx, t)
		},
	)
}

// headerRow is the official header: the button that collapses the view,
// the context's name, then More options and Expand (full screen).
func (p *panel) headerRow(gtx layout.Context, t *backend.NowTrack, ctxName string) layout.Dimensions {
	if p.hide.Click.Clicked(gtx) {
		ui.TogglePanel("npv")
	}
	if p.expand.Click.Clicked(gtx) {
		ui.SetFullscreen(true)
	}
	w, h, b := gtx.Constraints.Max.X, ui.Dp(gtx, 64), ui.Dp(gtx, 32)
	btn := func(x int, i *ui.IconButton, icon, tip string) {
		ui.Offset(gtx, x, (h-b)/2, func() {
			d := i.Layout(gtx, ui.IconStyle{Icon: icon, Label: tip})
			ui.Tip(gtx, &i.Click, d, tip, false)
		})
	}
	btn(ui.Dp(gtx, 8), &p.hide, "npv-hide", "Hide Now Playing view")
	right := w - ui.Dp(gtx, 8) - b
	btn(right, &p.expand, "maximize", "Expand Now Playing view")
	if t != nil {
		right -= b
		btn(right, &p.more, "more", "More options for "+t.Name)
	}
	x := ui.Dp(gtx, 8) + b + ui.Dp(gtx, 4)
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(max(0, right-x-ui.Dp(gtx, 8)), h)}
	var d layout.Dimensions
	if ctxName == "" {
		d = ui.Text(g, "Now playing", 16, ui.Bold, ui.ColText)
	} else {
		p.ctxLink.Click.Label = ctxName
		d = p.ctxLink.Layout(g, ctxName, 16, ui.Bold, ui.ColText, ui.ColNone)
	}
	call := m.Stop()
	ui.Offset(gtx, x, (h-d.Size.Y)/2, func() { call.Add(gtx.Ops) })
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func (p *panel) body(gtx layout.Context, t *backend.NowTrack) layout.Dimensions {
	items := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return p.media(gtx, t) },
		func(gtx layout.Context) layout.Dimensions { return p.titleRow(gtx, t) },
	}
	// The official order: lyrics, the artist, credits.
	if p.lyr.uri == t.URI && p.lyr.has() {
		items = append(items, p.lyr.layout)
	}
	if p.info.Ok() {
		if a := p.info.V.Artist; a != nil {
			items = append(items, func(gtx layout.Context) layout.Dimensions { return p.artistCard(gtx, a) })
		}
		if cs := p.info.V.Credits; len(cs) > 0 {
			items = append(items, func(gtx layout.Context) layout.Dimensions { return p.creditsCard(gtx, t, cs) })
		}
	}
	if p.queue.Ok() && len(p.queue.V.Next) > 0 {
		items = append(items, func(gtx layout.Context) layout.Dimensions { return p.nextCard(gtx, &p.queue.V.Next[0]) })
	}
	return p.list.Layout(gtx, len(items), func(gtx layout.Context, i int) layout.Dimensions {
		return ui.Pad(gtx, 0, 16, 16, 16, items[i])
	})
}

// card draws content on the raised #1f1f1f rounded box.
func card(gtx layout.Context, content layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := content(gtx)
	call := m.Stop()
	ui.FillRect(gtx.Ops, image.Rectangle{Max: d.Size}, ui.Dp(gtx, 8), ui.ColSurface2)
	call.Add(gtx.Ops)
	return d
}

func (p *panel) titleRow(gtx layout.Context, t *backend.NowTrack) layout.Dimensions {
	if p.title.Click.Clicked(gtx) && t.AlbumURI != "" {
		ui.OpenURI(t.AlbumURI)
	}
	if len(p.artists) < len(t.Artists) {
		p.artists = make([]ui.Link, len(t.Artists))
	}
	for i := range t.Artists {
		if p.artists[i].Click.Clicked(gtx) {
			ui.OpenURI(t.Artists[i].URI)
		}
	}
	if p.save.Click.Clicked(gtx) {
		ui.AddedClicked(t.URI)
	}
	saved := ui.Added(t.URI)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.Column(gtx,
				func(gtx layout.Context) layout.Dimensions {
					p.title.Click.Label = t.Name
					return p.title.Layout(gtx, t.Name, 24, ui.Bold, ui.ColText, ui.ColNone)
				},
				func(gtx layout.Context) layout.Dimensions {
					var segs []ui.Seg
					for i, a := range t.Artists {
						if i > 0 {
							segs = append(segs, ui.Seg{Text: ", "})
						}
						segs = append(segs, ui.Seg{Text: a.Name, Link: &p.artists[i]})
					}
					return ui.Pad(gtx, 3, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
						return ui.Inline(gtx, 14, ui.Regular, ui.ColText2, ui.ColText, segs...)
					})
				},
			)
		}),
		layout.Rigid(ui.Gap(8, 0)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			icon, label := "plus-circle", "Save to Liked Songs"
			if saved {
				icon, label = "check-circle", "Add to playlist"
			}
			d := p.save.Layout(gtx, ui.IconStyle{Icon: icon, Size: 24, Box: 32, Active: saved, Label: label})
			ui.Tip(gtx, &p.save.Click, d, label, false)
			return d
		}),
	)
}

func (p *panel) artistCard(gtx layout.Context, a *spotify.ArtistCard) layout.Dimensions {
	if p.about.Clicked(gtx) {
		artist.ShowAbout(a.URI)
	}
	if p.artistName.Click.Clicked(gtx) {
		ui.OpenURI(a.URI)
	}
	if p.follow.Click.Clicked(gtx) {
		ui.ToggleSaved(a.URI)
	}
	p.about.Label = "About the artist " + a.Name
	return card(gtx, func(gtx layout.Context) layout.Dimensions {
		return p.about.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// The artist page's banner, 3:2, like the official card.
			w := gtx.Constraints.Max.X
			ih := w * 2 / 3
			imgs := a.Header
			if len(imgs) == 0 {
				imgs = a.Gallery
			}
			if len(imgs) == 0 {
				imgs = a.Images
			}
			r := ui.Dp(gtx, 8)
			st := clip.RRect{Rect: image.Rect(0, 0, w, ih), NW: r, NE: r}.Push(gtx.Ops)
			ui.Image{URL: ui.Pick(imgs, w), Icon: "artist"}.Layout(gtx, image.Pt(w, ih))
			ui.VGradient(gtx.Ops, image.Rect(0, 0, w, ui.Dp(gtx, 72)), ui.Alpha(ui.ColBg, .6), ui.ColNone)
			st.Pop()
			ui.Offset(gtx, ui.Dp(gtx, 16), ui.Dp(gtx, 16), func() {
				ui.Text(gtx, "About the artist", 16, ui.Bold, ui.ColText)
			})
			var d layout.Dimensions
			ui.Offset(gtx, 0, ih, func() {
				d = ui.Pad(gtx, 16, 16, 16, 16, func(gtx layout.Context) layout.Dimensions {
					return p.artistInfo(gtx, a)
				})
			})
			return layout.Dimensions{Size: image.Pt(w, ih+d.Size.Y)}
		})
	})
}

// artistInfo is the name (and verified badge), then the monthly listeners
// with Follow beside them, then the start of the bio.
func (p *panel) artistInfo(gtx layout.Context, a *spotify.ArtistCard) layout.Dimensions {
	following := ui.Saved[a.URI]
	return ui.Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			p.artistName.Click.Label = a.Name
			segs := []ui.Seg{{Widget: func(gtx layout.Context) layout.Dimensions {
				return p.artistName.Layout(gtx, a.Name, 16, ui.Bold, ui.ColText, ui.ColNone)
			}}}
			if a.Verified {
				segs = append(segs, ui.Seg{Widget: func(gtx layout.Context) layout.Dimensions {
					s, gap := ui.Dp(gtx, 16), ui.Dp(gtx, 6)
					ui.Offset(gtx, gap, 0, func() { ui.IconPx(gtx, "verified", s, ui.RGB(0xc5f0c9)) })
					return layout.Dimensions{Size: image.Pt(gap+s, s), Baseline: ui.Dp(gtx, 2)}
				}})
			}
			return ui.Inline(gtx, 16, ui.Bold, ui.ColText, ui.ColText, segs...)
		},
		func(gtx layout.Context) layout.Dimensions {
			return ui.Pad(gtx, 8, 0, 10, 0, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						d := ui.Label{Text: ui.FmtNumber(a.MonthlyListeners) + " monthly listeners", Size: 16, Color: ui.ColText2, MaxLines: 2}.Layout(gtx)
						d.Size.X = gtx.Constraints.Max.X // Follow goes to the right edge
						return d
					}),
					layout.Rigid(ui.Gap(12, 0)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						txt := "Follow"
						if following {
							txt = "Following"
						}
						return p.follow.Layout(gtx, ui.PillStyle{Text: txt, Weight: ui.Bold, Height: 32, PadX: 16, Outline: true})
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			if a.Bio == "" {
				return layout.Dimensions{}
			}
			return ui.Label{Text: a.Bio, Size: 14, Color: ui.ColText2, MaxLines: 3}.Layout(gtx)
		},
	)
}

func (p *panel) creditsCard(gtx layout.Context, t *backend.NowTrack, cs []spotify.Credit) layout.Dimensions {
	if p.showAll.Click.Clicked(gtx) {
		ui.ShowCredits(t.Name, t.URI)
	}
	n := min(3, len(cs))
	if len(p.credits) < n {
		p.credits, p.creditFollow = make([]ui.Link, n), make([]ui.Pill, n)
	}
	// The song's own artists get a Follow button, as in the official view.
	main := map[string]bool{}
	for _, a := range t.Artists {
		main[a.URI] = true
	}
	for i := 0; i < n; i++ {
		if p.credits[i].Click.Clicked(gtx) && cs[i].URI != "" {
			ui.OpenURI(cs[i].URI)
		}
		if p.creditFollow[i].Click.Clicked(gtx) {
			ui.ToggleSaved(cs[i].URI)
		}
	}
	return card(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.Pad(gtx, 16, 16, 8, 16, func(gtx layout.Context) layout.Dimensions {
			rows := []layout.Widget{func(gtx layout.Context) layout.Dimensions {
				return ui.Pad(gtx, 0, 0, 8, 0, func(gtx layout.Context) layout.Dimensions {
					return header(gtx, "Credits", "Show all", &p.showAll, len(cs) > n)
				})
			}}
			for i := 0; i < n; i++ {
				c := cs[i]
				if main[c.URI] {
					ui.LoadSaved([]string{c.URI})
				}
				rows = append(rows, func(gtx layout.Context) layout.Dimensions {
					return ui.Pad(gtx, 8, 0, 8, 0, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return ui.Column(gtx,
									func(gtx layout.Context) layout.Dimensions {
										if c.URI == "" {
											return ui.Text(gtx, c.Name, 16, ui.Regular, ui.ColText)
										}
										return p.credits[i].Layout(gtx, c.Name, 16, ui.Regular, ui.ColText, ui.ColNone)
									},
									func(gtx layout.Context) layout.Dimensions {
										return ui.Text(gtx, c.Roles, 14, ui.Regular, ui.ColText2)
									},
								)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if !main[c.URI] {
									return layout.Dimensions{}
								}
								txt := "Follow"
								if ui.Saved[c.URI] {
									txt = "Following"
								}
								return ui.Pad(gtx, 0, 0, 0, 12, func(gtx layout.Context) layout.Dimensions {
									return p.creditFollow[i].Layout(gtx, ui.PillStyle{Text: txt, Weight: ui.Bold, Height: 32, PadX: 16, Outline: true})
								})
							}),
						)
					})
				})
			}
			return ui.Column(gtx, rows...)
		})
	})
}

// header is a card title with a link on the right.
func header(gtx layout.Context, title, link string, l *ui.Link, showLink bool) layout.Dimensions {
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			d := ui.Text(gtx, title, 16, ui.Bold, ui.ColText)
			d.Size.X = gtx.Constraints.Max.X
			return d
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !showLink {
				return layout.Dimensions{}
			}
			return l.Layout(gtx, link, 14, ui.Bold, ui.ColText2, ui.ColText)
		}),
	)
}

func (p *panel) nextCard(gtx layout.Context, t *backend.QueueTrack) layout.Dimensions {
	if p.openQueue.Click.Clicked(gtx) {
		ui.Prefs.RightPanel = "queue"
	}
	var imgs []spotify.Image
	if t.Album != nil {
		imgs = t.Album.Images
	}
	return card(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.Pad(gtx, 16, 16, 16, 16, func(gtx layout.Context) layout.Dimensions {
			return ui.Column(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return ui.Pad(gtx, 0, 0, 12, 0, func(gtx layout.Context) layout.Dimensions {
						return header(gtx, "Next in queue", "Open queue", &p.openQueue, true)
					})
				},
				func(gtx layout.Context) layout.Dimensions {
					is := ui.Dp(gtx, 48)
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.Image{URL: ui.Pick(imgs, is), Radius: ui.Dp(gtx, 4), Icon: "note"}.Layout(gtx, image.Pt(is, is))
						}),
						layout.Rigid(ui.Gap(12, 0)),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return ui.Column(gtx,
								func(gtx layout.Context) layout.Dimensions { return ui.Text(gtx, t.Name, 16, ui.Regular, ui.ColText) },
								func(gtx layout.Context) layout.Dimensions {
									return ui.Text(gtx, ui.ArtistNames(t.Artists), 14, ui.Regular, ui.ColText2)
								},
							)
						}),
					)
				},
			)
		})
	})
}
