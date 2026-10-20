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
	Type   string // "Playlist", "Album", "Single", "Profile"... ("" = none)
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

// HeaderTitleSize is the font size (sp) of a header title that has avail px
// of width: the largest of 96, 72, 48 and 32 that fits on one line (longer
// titles wrap at 32), as the official client fits it.
func HeaderTitleSize(gtx layout.Context, avail int, title string) float32 {
	for _, s := range []float32{96, 72, 48} {
		if Measure(gtx, title, s, ExtraBold) <= avail {
			return s
		}
	}
	return 32
}

// Layout draws the header at full width. Padding and artwork scale with the
// main view's width (ContentT): 16dp and 128dp up to 24dp and 232dp.
func (h *EntityHeader) Layout(gtx layout.Context, o HeaderOpts) layout.Dimensions {
	if h.Art.Clicked(gtx) && len(o.Images) > 0 {
		ShowArtwork(Pick(o.Images, 640))
	}
	if h.Title.Clicked(gtx) && o.OnEdit != nil {
		o.OnEdit()
	}
	w := gtx.Constraints.Max.X
	px := func(v float32) int { return Dp(gtx, v) }
	t := ContentT(gtx)
	pad := ContentPad(gtx)
	art := px(128 + 104*t)
	infoX := pad + art + pad
	infoW := max(0, w-infoX-pad)
	size := HeaderTitleSize(gtx, infoW, o.Title)

	// Info column, measured first so the header grows to fit it.
	m := op.Record(gtx.Ops)
	info := Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			if o.Type == "" {
				return layout.Dimensions{}
			}
			gtx.Constraints.Max.X = infoW
			return Text(gtx, o.Type, 14, Regular, ColText)
		},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = infoW
			title := func(gtx layout.Context) layout.Dimensions {
				return Label{Text: o.Title, Size: size, Weight: ExtraBold, MaxLines: 3, LineHeight: size}.Layout(gtx)
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
			return Label{Text: o.Description, Size: 14, Color: Alpha(ColText, .7), MaxLines: 2}.Layout(gtx)
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

	// The gap above the artwork shrinks from 43.5dp to 20dp as the view widens.
	total := max(pad+px(43.5-23.5*t)+art, pad+info.Size.Y+px(8))
	c := o.Color
	if c == ColNone {
		c = RGB(0x535353)
	}
	FillRect(gtx.Ops, image.Rect(0, 0, w, total), 0, c)
	Offset(gtx, pad, total-pad-art, func() {
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
	Offset(gtx, infoX, total-pad-info.Size.Y, func() { call.Add(gtx.Ops) })
	return layout.Dimensions{Size: image.Pt(w, total)}
}

// HeaderFade paints the 173dp gradient that continues a header's color
// (at 70%) behind the action bar. Draw it before the page list with top = the
// header's bottom edge minus the list's scroll offset.
func HeaderFade(gtx layout.Context, c color.NRGBA, top int) {
	if c == ColNone {
		c = RGB(0x535353)
	}
	h := Dp(gtx, 173)
	if top+h <= 0 {
		return
	}
	VGradient(gtx.Ops, image.Rect(0, top, gtx.Constraints.Max.X, top+h), Mix(ColSurface, c, .7), ColSurface)
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
	// Above is a line drawn over the buttons (an episode's date and length).
	Above layout.Widget
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
	pad := ContentPadDp(gtx)
	gap := layout.Rigid(Gap(pad, 0))
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
	return Pad(gtx, pad, pad, pad, pad, func(gtx layout.Context) layout.Dimensions {
		row := func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
		}
		if o.Above == nil {
			return row(gtx)
		}
		return Column(gtx, o.Above, Gap(0, 16), row)
	})
}

// ExtractedColor is the header color Spotify extracted from the cover (hex,
// when the API gave one); otherwise it falls back to CoverColor.
func ExtractedColor(hex string, images []spotify.Image, done func(color.NRGBA)) {
	if hex != "" {
		done(Hex(hex))
		return
	}
	CoverColor(images, done)
}

// CoverColor loads the header color for a cover (darkened with HeaderColor).
func CoverColor(images []spotify.Image, done func(color.NRGBA)) {
	url := Pick(images, 64)
	if url == "" {
		done(RGB(0x535353))
		return
	}
	type result struct {
		hex   string
		exact bool // Spotify's own dark color, used as is
	}
	Async(func() (result, error) {
		if hex, err := B.CoverColor(url); err == nil {
			return result{hex, true}, nil
		}
		hex, err := B.ImageColor(url)
		return result{hex, false}, err
	}, func(r result, err error) {
		switch {
		case err != nil:
			done(RGB(0x535353))
		case r.exact:
			done(Hex(r.hex))
		default:
			done(HeaderColor(Hex(r.hex)))
		}
	})
}
