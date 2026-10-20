package ui

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// Card is the state of one tile in a shelf or grid.
type Card struct {
	Click Click
	Play  PlayButton
	hover Hover
}

type CardOpts struct {
	// Bare shows only the image (Recents on Home).
	Bare bool
	// OnOpen runs before navigating (search uses it to save a recent search).
	OnOpen func(spotify.Card)
	// Menu overrides the right-click menu (nil = CardMenu).
	Menu func() []MenuItem
	// NoNavigate skips opening the card's page on click.
	NoNavigate bool
}

func cardRound(c spotify.Card) bool { return c.Type == "artist" || c.Type == "user" }

func cardPlayable(c spotify.Card) bool {
	switch c.Type {
	case "user", "genre", "folder":
		return false
	}
	return true
}

func cardIcon(typ string) string {
	switch typ {
	case "artist", "user":
		return "user"
	case "album":
		return "album"
	case "show", "episode":
		return "note"
	}
	return "note"
}

// Layout draws the card at the constraint's max width.
func (k *Card) Layout(gtx layout.Context, c spotify.Card, o CardOpts) layout.Dimensions {
	if k.Play.Click.Clicked(gtx) {
		if o.OnOpen != nil {
			o.OnOpen(c)
		}
		PlayContext(c.URI)
	}
	if k.Click.Clicked(gtx) {
		if o.OnOpen != nil {
			o.OnOpen(c)
		}
		if !o.NoNavigate {
			OpenURI(c.URI)
		}
	}
	if _, ok := k.Click.RightClicked(gtx); ok {
		if o.Menu != nil {
			OpenMenu(o.Menu())
		} else {
			OpenMenu(CardMenu(c, CardMenuOpts{}))
		}
	}
	k.Click.Label = c.Name
	w := gtx.Constraints.Max.X
	pad := Dp(gtx, 12)
	if o.Bare {
		pad = Dp(gtx, 8)
	}
	art := max(0, w-2*pad)
	return k.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hov := k.Click.Hovered()
		m := op.Record(gtx.Ops)
		y := pad
		// Artwork with shadow.
		r := image.Rect(pad, y, pad+art, y+art)
		radius := Dp(gtx, 6)
		if cardRound(c) {
			radius = art / 2
		}
		Shadow(gtx.Ops, r, radius, Dp(gtx, 12), Dp(gtx, 8), .3)
		Offset(gtx, pad, y, func() {
			Image{URL: Pick(c.Images, art), Radius: radius, Round: cardRound(c), Icon: cardIcon(c.Type)}.Layout(gtx, image.Pt(art, art))
		})
		// Play button: slides up and fades in on hover; stays while playing.
		if cardPlayable(c) {
			playing := CtxPlaying(c.URI)
			v := k.hover.Step(gtx, hov || playing, 300*time.Millisecond)
			if v > 0 {
				bs := Dp(gtx, 48)
				bx := pad + art - Dp(gtx, 8) - bs
				by := y + art - Dp(gtx, 8) - bs + int((1-v)*float32(Dp(gtx, 8)))
				Offset(gtx, bx, by, func() {
					st := paint.PushOpacity(gtx.Ops, v)
					Shadow(gtx.Ops, image.Rect(0, 0, bs, bs), bs/2, Dp(gtx, 4), Dp(gtx, 6), .3)
					k.Play.Layout(gtx, 48, playing)
					st.Pop()
				})
			}
		}
		y += art
		if !o.Bare {
			y += Dp(gtx, 8)
			Offset(gtx, pad, y, func() {
				g := gtx
				g.Constraints = layout.Constraints{Max: image.Pt(art, Dp(gtx, 30))}
				d := Text(g, c.Name, 16, Regular, ColText)
				y += d.Size.Y
			})
			if c.Subtitle != "" {
				y += Dp(gtx, 4)
				Offset(gtx, pad, y, func() {
					g := gtx
					g.Constraints = layout.Constraints{Max: image.Pt(art, Dp(gtx, 60))}
					d := Label{Text: c.Subtitle, Size: 14, Color: ColText2, MaxLines: 2}.Layout(g)
					y += d.Size.Y
				})
			}
		}
		y += pad
		call := m.Stop()
		if hov {
			FillRect(gtx.Ops, image.Rect(0, 0, w, y), Dp(gtx, 6), ColSurface2)
		}
		call.Add(gtx.Ops)
		return layout.Dimensions{Size: image.Pt(w, y)}
	})
}

// CardGrid lays out cards in rows of equal columns at least minW dp wide
// (CSS grid auto-fill, minmax(minW, 1fr)). rows <= 0 means all rows.
func CardGrid(gtx layout.Context, n int, minW float32, rows int, cell func(gtx layout.Context, i int) layout.Dimensions) layout.Dimensions {
	w := gtx.Constraints.Max.X
	cols := max(1, w/Dp(gtx, minW))
	cw := w / cols
	if rows > 0 {
		n = min(n, cols*rows)
	}
	y := 0
	for r := 0; r*cols < n; r++ {
		rh := 0
		for c := 0; c < cols && r*cols+c < n; c++ {
			Offset(gtx, c*cw, y, func() {
				g := gtx
				g.Constraints = layout.Constraints{Min: image.Pt(cw, 0), Max: image.Pt(cw, 1<<20)}
				d := cell(g, r*cols+c)
				rh = max(rh, d.Size.Y)
			})
		}
		y += rh
	}
	return layout.Dimensions{Size: image.Pt(w, y)}
}

// GridColumns is how many minW-dp columns fit in the constraint's width.
func GridColumns(gtx layout.Context, minW float32) int {
	return max(1, gtx.Constraints.Max.X/Dp(gtx, minW))
}

// Shelf is the state of a titled row of cards.
type Shelf struct {
	Title, ShowAll Link
	Header         Click
	Cards          []Card
}

type ShelfOpts struct {
	Title string
	// Label is small text above the title ("Made For"); Note is below it.
	Label, Note string
	// Header is an entity shown as a 48dp image left of the titles.
	Header *spotify.Card
	// OnShowAll adds "Show all" (and makes the title a link).
	OnShowAll func()
	// ShowAllText replaces "Show all".
	ShowAllText string
	Bare        bool
	Card        CardOpts
	// Rows of cards to show (default 1).
	Rows int
	// Below is drawn between the header and the cards (the discography chips).
	Below layout.Widget
}

// Layout draws the shelf: header row, then one row of cards that fit.
func (s *Shelf) Layout(gtx layout.Context, items []spotify.Card, o ShelfOpts) layout.Dimensions {
	if len(s.Cards) < len(items) {
		s.Cards = append(s.Cards, make([]Card, len(items)-len(s.Cards))...)
	}
	o.Card.Bare = o.Card.Bare || o.Bare
	if s.ShowAll.Click.Clicked(gtx) && o.OnShowAll != nil {
		o.OnShowAll()
	}
	if s.Title.Click.Clicked(gtx) {
		switch {
		case o.Header != nil:
			OpenURI(o.Header.URI)
		case o.OnShowAll != nil:
			o.OnShowAll()
		}
	}
	if s.Header.Clicked(gtx) && o.Header != nil {
		OpenURI(o.Header.URI)
	}
	minW := float32(176)
	if o.Bare {
		minW = 150
	}
	rows := o.Rows
	if rows == 0 {
		rows = 1
	}
	return Pad(gtx, 0, 12, 24, 12, func(gtx layout.Context) layout.Dimensions {
		return Column(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return Pad(gtx, 0, 12, 4, 12, func(gtx layout.Context) layout.Dimensions {
					return s.header(gtx, o)
				})
			},
			o.Below,
			func(gtx layout.Context) layout.Dimensions {
				return CardGrid(gtx, len(items), minW, rows, func(gtx layout.Context, i int) layout.Dimensions {
					return s.Cards[i].Layout(gtx, items[i], o.Card)
				})
			},
		)
	})
}

func (s *Shelf) header(gtx layout.Context, o ShelfOpts) layout.Dimensions {
	w := gtx.Constraints.Max.X
	x := 0
	h := 0
	if o.Header != nil && len(o.Header.Images) > 0 {
		hs := Dp(gtx, 48)
		s.Header.Label = o.Header.Name
		s.Header.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			round := o.Header.Type == "artist"
			return Image{URL: Pick(o.Header.Images, hs), Radius: Dp(gtx, 4), Round: round}.Layout(gtx, image.Pt(hs, hs))
		})
		x, h = hs+Dp(gtx, 12), hs
	}
	allW := 0
	all := o.ShowAllText
	if all == "" {
		all = "Show all"
	}
	if o.OnShowAll != nil {
		allW = Measure(gtx, all, 14, Bold) + Dp(gtx, 12)
	}
	// Titles, bottom-aligned with the header image.
	m := op.Record(gtx.Ops)
	d := Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			if o.Label == "" {
				return layout.Dimensions{}
			}
			return Pad(gtx, 0, 0, 2, 0, func(gtx layout.Context) layout.Dimensions {
				return Text(gtx, o.Label, 12, Regular, ColText2)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = w - x - allW
			if o.Header != nil || o.OnShowAll != nil {
				return s.Title.Layout(gtx, o.Title, 24, Bold, ColText, ColNone)
			}
			return Text(gtx, o.Title, 24, Bold, ColText)
		},
		func(gtx layout.Context) layout.Dimensions {
			if o.Note == "" {
				return layout.Dimensions{}
			}
			return Pad(gtx, 2, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
				return Text(gtx, o.Note, 14, Regular, ColText2)
			})
		},
	)
	call := m.Stop()
	h = max(h, d.Size.Y)
	Offset(gtx, x, h-d.Size.Y, func() { call.Add(gtx.Ops) })
	if o.OnShowAll != nil {
		Offset(gtx, w-allW+Dp(gtx, 12), h-Dp(gtx, 6)-Dp(gtx, 18), func() {
			s.ShowAll.Layout(gtx, all, 14, Bold, ColText2, ColText)
		})
	}
	return layout.Dimensions{Size: image.Pt(w, h)}
}
