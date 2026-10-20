package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// EntityHeader is the big gradient header of album, playlist, artist and
// profile pages: artwork, type, title and a meta line.
type EntityHeader struct {
	Art   Click
	Title Click
}

type HeaderOpts struct {
	Images []spotify.Image
	Type   string // "Playlist", "Album", "Single", "Profile"...
	Title  string
	Color  color.NRGBA
	Round  bool
	// Meta is the line under the title (owner, year, counts).
	Meta layout.Widget
	// Description is shown between the title and Meta (playlists).
	Description string
	// OnEdit makes the title clickable ("Edit details").
	OnEdit func()
	Icon   string // placeholder icon when there's no image
}

// Layout draws the header at full width.
func (h *EntityHeader) Layout(gtx layout.Context, o HeaderOpts) layout.Dimensions {
	if h.Art.Clicked(gtx) && len(o.Images) > 0 {
		ShowArtwork(Pick(o.Images, 640))
	}
	if h.Title.Clicked(gtx) && o.OnEdit != nil {
		o.OnEdit()
	}
	w := gtx.Constraints.Max.X
	px := func(v float32) int { return Dp(gtx, v) }
	art := max(px(128), min(px(232), w*20/100))
	// Title size: smaller for long titles, as in the official client.
	n := len([]rune(o.Title))
	var size float32
	pw := float32(w) / gtx.Metric.PxPerSp
	switch {
	case n > 40:
		size = max(24, min(48, pw*.04))
	case n > 18:
		size = max(32, min(72, pw*.06))
	default:
		size = max(32, min(96, pw*.08))
	}
	infoX := px(24) + art + px(24)
	infoW := max(0, w-infoX-px(24))

	// Info column, measured first so the header grows to fit it.
	m := op.Record(gtx.Ops)
	info := Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = infoW
			return Text(gtx, o.Type, 14, Medium, ColText)
		},
		Gap(0, 8),
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = infoW
			title := func(gtx layout.Context) layout.Dimensions {
				return Label{Text: o.Title, Size: size, Weight: Black, MaxLines: 3, LineHeight: size * 1.08}.Layout(gtx)
			}
			if o.OnEdit != nil {
				h.Title.Label = "Edit details"
				return h.Title.Layout(gtx, title)
			}
			return title(gtx)
		},
		func(gtx layout.Context) layout.Dimensions {
			if o.Description == "" {
				return layout.Dimensions{}
			}
			gtx.Constraints.Max.X = infoW
			return Pad(gtx, 8, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
				return Label{Text: o.Description, Size: 14, Color: ColText2, MaxLines: 2}.Layout(gtx)
			})
		},
		Gap(0, 8),
		func(gtx layout.Context) layout.Dimensions {
			if o.Meta == nil {
				return layout.Dimensions{}
			}
			gtx.Constraints.Max.X = infoW
			return o.Meta(gtx)
		},
	)
	call := m.Stop()

	contentH := max(art, info.Size.Y)
	total := px(32) + contentH + px(24)
	if total < px(232)+px(56) {
		total = max(total, art+px(56))
	}
	c := o.Color
	if c == ColNone {
		c = RGB(0x535353)
	}
	VGradient(gtx.Ops, image.Rect(0, 0, w, total), c, Scale(c, .5))
	ay := total - px(24) - art
	Offset(gtx, px(24), ay, func() {
		radius := px(4)
		if o.Round {
			radius = art / 2
		}
		Shadow(gtx.Ops, image.Rect(0, 0, art, art), radius, px(60), px(4), .5)
		h.Art.Label = "View artwork"
		h.Art.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			icon := o.Icon
			if icon == "" {
				icon = "note"
			}
			return Image{URL: Pick(o.Images, art), Radius: radius, Round: o.Round, Icon: icon}.Layout(gtx, image.Pt(art, art))
		})
	})
	Offset(gtx, infoX, total-px(24)-info.Size.Y, func() { call.Add(gtx.Ops) })
	return layout.Dimensions{Size: image.Pt(w, total)}
}

// HeaderFade paints the 232dp gradient that continues a header's color
// behind the action bar. Draw it before the page list with top = the header's
// bottom edge minus the list's scroll offset.
func HeaderFade(gtx layout.Context, c color.NRGBA, top int) {
	if c == ColNone {
		c = RGB(0x535353)
	}
	h := Dp(gtx, 232)
	if top+h <= 0 {
		return
	}
	VGradient(gtx.Ops, image.Rect(0, top, gtx.Constraints.Max.X, top+h), Scale(c, .4), ColSurface)
}

type artworkView struct {
	url  string
	back Click
}

func (a *artworkView) Layout(gtx layout.Context) layout.Dimensions {
	if a.back.Clicked(gtx) {
		CloseModal()
	}
	win := gtx.Constraints.Max
	s := min(Dp(gtx, 640), win.X*8/10, win.Y*8/10)
	Offset(gtx, (win.X-s)/2, (win.Y-s)/2, func() {
		Shadow(gtx.Ops, image.Rect(0, 0, s, s), Dp(gtx, 8), Dp(gtx, 24), Dp(gtx, 8), .5)
		a.back.Label = "Close"
		a.back.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return Image{URL: a.url, Radius: Dp(gtx, 8)}.Layout(gtx, image.Pt(s, s))
		})
	})
	return layout.Dimensions{Size: win}
}

// ShowArtwork opens a full-size artwork viewer (click or Escape closes).
func ShowArtwork(url string) { ShowOverlay(&artworkView{url: url}) }

// ActionBar is the row under a header: big play, shuffle, save, more.
type ActionBar struct {
	Play          PlayButton
	Shuffle, Save IconButton
	More          IconButton
}

type ActionOpts struct {
	// ContextURI is played by the play button (and shuffled by shuffle).
	ContextURI string
	// OnPlay overrides the play button (default PlayContext(ContextURI)).
	OnPlay func()
	// Playing overrides whether the pause icon shows (default CtxPlaying).
	Playing *bool
	Shuffle bool // show the shuffle toggle
	// Save shows the add/check (or heart) toggle for SaveURI.
	SaveURI string
	Heart   bool
	// Extra widgets after the save button (Follow pill...).
	Extra layout.Widget
	// Menu items for "…" (nil hides it).
	Menu func() []MenuItem
}

func (a *ActionBar) Layout(gtx layout.Context, o ActionOpts) layout.Dimensions {
	if a.Play.Click.Clicked(gtx) {
		if o.OnPlay != nil {
			o.OnPlay()
		} else {
			PlayContext(o.ContextURI)
		}
	}
	shuffleOn := IsContext(o.ContextURI) && Player.Status != nil && Player.Status.Shuffle
	if a.Shuffle.Click.Clicked(gtx) {
		// Like the official button: turn shuffle on for this context, starting it if needed.
		if !IsContext(o.ContextURI) {
			uri := o.ContextURI
			Go("Couldn't shuffle", func() error {
				if err := B.SetShuffle(true); err != nil {
					return err
				}
				return B.Play(uri, "")
			}, RefreshPlayer)
		} else {
			ToggleShuffle()
		}
	}
	if a.Save.Click.Clicked(gtx) && o.SaveURI != "" {
		ToggleSaved(o.SaveURI)
	}
	if a.More.Click.Clicked(gtx) && o.Menu != nil {
		OpenMenu(o.Menu())
	}
	playing := CtxPlaying(o.ContextURI)
	if o.Playing != nil {
		playing = *o.Playing
	}
	var kids []layout.FlexChild
	kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return a.Play.Layout(gtx, 56, playing)
	}))
	gap := layout.Rigid(Gap(24, 0))
	if o.Shuffle {
		kids = append(kids, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d := a.Shuffle.Layout(gtx, IconStyle{Icon: "shuffle", Size: 32, Box: 32, Active: shuffleOn, Dot: true, Label: "Shuffle"})
			tip := "Enable shuffle"
			if shuffleOn {
				tip = "Disable shuffle"
			}
			Tip(gtx, &a.Shuffle.Click, d, tip, false)
			return d
		}))
	}
	if o.SaveURI != "" {
		kids = append(kids, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			saved := Saved[o.SaveURI]
			icon, tip := "plus-circle", "Save to Your Library"
			switch {
			case o.Heart && saved:
				icon, tip = "heart-filled", "Remove from Your Library"
			case o.Heart:
				icon = "heart"
			case saved:
				icon, tip = "check-circle", "Remove from Your Library"
			}
			d := a.Save.Layout(gtx, IconStyle{Icon: icon, Size: 32, Box: 32, Active: saved, Label: tip})
			Tip(gtx, &a.Save.Click, d, tip, false)
			return d
		}))
	}
	if o.Extra != nil {
		kids = append(kids, gap, layout.Rigid(o.Extra))
	}
	if o.Menu != nil {
		kids = append(kids, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d := a.More.Layout(gtx, IconStyle{Icon: "more", Size: 32, Box: 32, Label: "More options"})
			Tip(gtx, &a.More.Click, d, "More options", false)
			return d
		}))
	}
	return Pad(gtx, 24, 24, 24, 24, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
	})
}

// CoverColor loads the header color for a cover (darkened with HeaderColor).
func CoverColor(images []spotify.Image, done func(color.NRGBA)) {
	url := Pick(images, 64)
	if url == "" {
		done(RGB(0x535353))
		return
	}
	Async(func() (string, error) { return B.ImageColor(url) }, func(hex string, err error) {
		if err != nil {
			done(RGB(0x535353))
			return
		}
		done(HeaderColor(Hex(hex)))
	})
}
