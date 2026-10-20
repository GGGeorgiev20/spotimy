// Package npv is the Now Playing view (Prefs.RightPanel "npv"): cover or
// Canvas video, title, the artist card, credits and the next queued track.
package npv

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() { ui.RightPanels["npv"] = p.layout }

var p panel

type panel struct {
	header ui.PanelHeader
	more   ui.IconButton
	list   ui.List
	ctx    ui.ContextName

	// Per-track state; the Res values are replaced on a track change so late
	// results of the previous track can't land in them.
	uri    string
	info   *ui.Res[*spotify.NowPlayingInfo]
	canvas *ui.Res[string]
	vid    *canvasVideo

	queue    ui.Res[*backend.Queue]
	qVersion int

	ctxLink, title, showAll, openQueue, artistName ui.Link
	artists, credits                               []ui.Link
	save                                           ui.IconButton
	about                                          ui.Click
	follow                                         ui.Pill
}

// track switches to the playing track's data when it changed.
func (p *panel) track() *backend.NowTrack {
	t := ui.NowTrack()
	uri := ""
	if t != nil {
		uri = t.URI
	}
	if uri != p.uri || p.info == nil {
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
	if p.qVersion != ui.Player.Version {
		p.qVersion = ui.Player.Version
		p.queue.Load(ui.B.Queue)
	}
	return t
}

func (p *panel) layout(gtx layout.Context) layout.Dimensions {
	t := p.track()
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
	var extra layout.Widget
	if t != nil {
		extra = func(gtx layout.Context) layout.Dimensions {
			d := p.more.Layout(gtx, ui.IconStyle{Icon: "more", Label: "More options for " + t.Name})
			ui.Tip(gtx, &p.more.Click, d, "More options for "+t.Name, false)
			return d
		}
	}
	return ui.Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return p.header.Layout(gtx, "Hide Now Playing view", func(gtx layout.Context) layout.Dimensions {
				if ctxName == "" {
					return ui.Text(gtx, "Now playing", 16, ui.Bold, ui.ColText)
				}
				p.ctxLink.Click.Label = ctxName
				return p.ctxLink.Layout(gtx, ctxName, 16, ui.Bold, ui.ColText, ui.ColNone)
			}, extra)
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

func (p *panel) body(gtx layout.Context, t *backend.NowTrack) layout.Dimensions {
	items := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return p.media(gtx, t) },
		func(gtx layout.Context) layout.Dimensions { return p.titleRow(gtx, t) },
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
		ui.ToggleSaved(t.URI)
	}
	saved := ui.Saved[t.URI]
	return ui.Pad(gtx, 16, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
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
						return ui.Pad(gtx, 4, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
							return ui.Inline(gtx, 16, ui.Regular, ui.ColText2, ui.ColText, segs...)
						})
					},
				)
			}),
			layout.Rigid(ui.Gap(8, 0)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				icon, label := "plus-circle", "Save to Liked Songs"
				if saved {
					icon, label = "check-circle", "Remove from Liked Songs"
				}
				d := p.save.Layout(gtx, ui.IconStyle{Icon: icon, Size: 24, Box: 32, Active: saved, Label: label})
				ui.Tip(gtx, &p.save.Click, d, label, false)
				return d
			}),
		)
	})
}

func (p *panel) artistCard(gtx layout.Context, a *spotify.ArtistCard) layout.Dimensions {
	if p.about.Clicked(gtx) {
		ui.ShowModal(&aboutModal{a: a}, 524)
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
			w := gtx.Constraints.Max.X
			ih := min(w*9/10, ui.Dp(gtx, 330))
			imgs := a.Gallery
			if len(imgs) == 0 {
				imgs = a.Images
			}
			r := ui.Dp(gtx, 8)
			st := clip.RRect{Rect: image.Rect(0, 0, w, ih), NW: r, NE: r}.Push(gtx.Ops)
			ui.Image{URL: ui.Pick(imgs, w), Icon: "artist"}.Layout(gtx, image.Pt(w, ih))
			ui.VGradient(gtx.Ops, image.Rect(0, 0, w, ui.Dp(gtx, 72)), ui.ARGB(0x99000000), ui.ARGB(0))
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

func (p *panel) artistInfo(gtx layout.Context, a *spotify.ArtistCard) layout.Dimensions {
	following := ui.Saved[a.URI]
	return ui.Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return ui.Pad(gtx, 0, 0, 12, 0, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return ui.Column(gtx,
							func(gtx layout.Context) layout.Dimensions {
								p.artistName.Click.Label = a.Name
								return p.artistName.Layout(gtx, a.Name, 16, ui.Bold, ui.ColText, ui.ColNone)
							},
							func(gtx layout.Context) layout.Dimensions {
								return ui.Pad(gtx, 4, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
									return ui.Label{Text: ui.FmtNumber(a.MonthlyListeners) + " monthly listeners", Size: 14, Color: ui.ColText2, MaxLines: 2}.Layout(gtx)
								})
							},
						)
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
		p.credits = make([]ui.Link, n)
	}
	for i := 0; i < n; i++ {
		if p.credits[i].Click.Clicked(gtx) && cs[i].URI != "" {
			ui.OpenURI(cs[i].URI)
		}
	}
	return card(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.Pad(gtx, 16, 16, 4, 16, func(gtx layout.Context) layout.Dimensions {
			rows := []layout.Widget{func(gtx layout.Context) layout.Dimensions {
				return ui.Pad(gtx, 0, 0, 12, 0, func(gtx layout.Context) layout.Dimensions {
					return header(gtx, "Credits", "Show all", &p.showAll, len(cs) > n)
				})
			}}
			for i := 0; i < n; i++ {
				c := cs[i]
				rows = append(rows, func(gtx layout.Context) layout.Dimensions {
					return ui.Pad(gtx, 0, 0, 12, 0, func(gtx layout.Context) layout.Dimensions {
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

// aboutModal is the full artist bio dialog.
type aboutModal struct{ a *spotify.ArtistCard }

func (m *aboutModal) Layout(gtx layout.Context) layout.Dimensions {
	a := m.a
	return ui.Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return ui.Pad(gtx, 0, 0, 8, 0, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, a.Name, 24, ui.Bold, ui.ColText)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return ui.Pad(gtx, 0, 0, 16, 0, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, ui.FmtNumber(a.MonthlyListeners)+" monthly listeners", 14, ui.Bold, ui.ColText2)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return ui.Label{Text: a.Bio, Size: 14, Color: ui.ColText2, MaxLines: -1}.Layout(gtx)
		},
	)
}
