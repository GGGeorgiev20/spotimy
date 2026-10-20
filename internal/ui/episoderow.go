package ui

import (
	"fmt"
	"image"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// EpisodeRow is a podcast episode in a list: a 16:9 thumbnail, title, show,
// description, date and length, and a play button.
type EpisodeRow struct {
	Row, Play, Thumb, More Click
}

// Layout draws the row at the full constraint width. ctx is the show to play
// the episode in ("" plays it alone).
func (r *EpisodeRow) Layout(gtx layout.Context, ep spotify.Episode, ctx string) layout.Dimensions {
	current := IsCurrent(ep.URI, "")
	if r.Play.Clicked(gtx) || r.Thumb.Clicked(gtx) {
		switch {
		case current:
			PlayPause()
		case ctx != "":
			Play(ctx, ep.URI)
		default:
			Play(ep.URI, "")
		}
	}
	card := spotify.Card{URI: ep.URI, Type: "episode", Name: ep.Name, Images: ep.Images}
	if r.More.Clicked(gtx) {
		OpenMenu(CardMenu(card, CardMenuOpts{}))
	}
	if r.Row.Clicked(gtx) {
		Navigate("/episode/" + URIID(ep.URI))
	}
	if _, ok := r.Row.RightClicked(gtx); ok {
		OpenMenu(CardMenu(card, CardMenuOpts{}))
	}
	r.Row.Label = ep.Name
	return r.Row.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		px := func(v float32) int { return Dp(gtx, v) }
		padX, padY, top := px(12), px(16), px(2) // the divider takes the first 2dp
		tw, th := px(176), px(99)
		tx := padX + tw + px(24)
		textW := max(0, w-tx-padX)

		// Text column: title and show, description and date, actions.
		m := op.Record(gtx.Ops)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(textW, px(4000))}
		name := ColText
		if current {
			name = ColAccent
		}
		var y int
		put := func(f layout.Widget) { // lays f out at y and moves y below it
			var d layout.Dimensions
			Offset(gtx, 0, y, func() { d = f(g) })
			y += d.Size.Y
		}
		put(func(gtx layout.Context) layout.Dimensions {
			return Label{Text: ep.Name, Size: 16, Color: name, MaxLines: 2, LineHeight: 22 / 1.2}.Layout(gtx)
		})
		y += px(4)
		put(func(gtx layout.Context) layout.Dimensions {
			if ep.ShowName == "" {
				return layout.Dimensions{}
			}
			return Text(gtx, ep.ShowName, 14, Regular, ColText2)
		})
		y += px(8 + 8)
		put(func(gtx layout.Context) layout.Dimensions {
			return Label{Text: oneLine(ep.Description), Size: 14, Color: ColText2, MaxLines: 2, LineHeight: 20 / 1.2}.Layout(gtx)
		})
		y += px(4 + 8)
		put(func(gtx layout.Context) layout.Dimensions { return r.foot(gtx, ep, current) })
		y += px(8)
		// Actions row: the more button, and the play button at the far end.
		ah := px(48)
		Offset(gtx, 0, y+(ah-px(24))/2, func() {
			r.More.Label = "More options for " + ep.Name
			r.More.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return IconPx(gtx, "more", px(24), ColText2) })
		})
		Offset(gtx, textW-px(32), y+(ah-px(32))/2, func() {
			s := px(32)
			icon, label := "play", "Play "+ep.Name
			if current && Playing() {
				icon, label = "pause", "Pause "+ep.Name
			}
			r.Play.Label = label
			r.Play.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				FillCircle(gtx.Ops, s, ColText)
				Offset(gtx, s/4, s/4, func() { IconPx(gtx, icon, s/2, ColBg) })
				return layout.Dimensions{Size: image.Pt(s, s)}
			})
		})
		y += ah
		call := m.Stop()

		h := top + 2*padY + max(th, y)
		if r.Row.Hovered() {
			FillRect(gtx.Ops, image.Rect(0, top, w, h), px(6), ColRowHover)
		}
		Hline(gtx, w, ColDivider)
		// Thumbnail: video episodes fill the 16:9 slot, audio ones center a
		// square on black.
		Offset(gtx, padX, top+padY, func() {
			slot := image.Rect(0, 0, tw, th)
			defer clip.UniformRRect(slot, px(4)).Push(gtx.Ops).Pop()
			FillRect(gtx.Ops, slot, 0, ColBg)
			url := Pick(ep.Images, tw)
			if wide := len(ep.Images) > 0 && ep.Images[0].Width > ep.Images[0].Height*5/4; wide {
				Image{URL: url, Icon: "note"}.Layout(gtx, image.Pt(tw, th))
			} else {
				Offset(gtx, (tw-th)/2, 0, func() { Image{URL: url, Icon: "note"}.Layout(gtx, image.Pt(th, th)) })
			}
			hov := r.Thumb.Hovered()
			if hov || r.Row.Hovered() {
				FillRect(gtx.Ops, slot, 0, Alpha(ColBg, .5))
			}
			r.Thumb.Label = "Play " + ep.Name
			r.Thumb.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if r.Row.Hovered() {
					is := px(24)
					icon := "play"
					if current && Playing() {
						icon = "pause"
					}
					Offset(gtx, (tw-is)/2, (th-is)/2, func() { IconPx(gtx, icon, is, ColText) })
				}
				return layout.Dimensions{Size: slot.Max}
			})
		})
		Offset(gtx, tx, top+padY, func() { call.Add(gtx.Ops) })
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}

// oneLine collapses a description's whitespace and cuts it to what two lines can show.
func oneLine(s string) string {
	if len(s) > 400 {
		s = strings.ToValidUTF8(s[:400], "")
	}
	return strings.Join(strings.Fields(s), " ")
}

func (r *EpisodeRow) foot(gtx layout.Context, ep spotify.Episode, current bool) layout.Dimensions {
	gap := layout.Rigid(Gap(8, 0))
	sub := func(s string) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions { return Text(gtx, s, 14, Regular, ColText) })
	}
	var kids []layout.FlexChild
	if ep.Explicit {
		kids = append(kids, layout.Rigid(ExplicitBadge))
	}
	if d := FmtEpisodeDate(ep.ReleaseDate, gtx.Now); d != "" {
		kids = append(kids, sub(d), gap, sub("•"), gap)
	}
	left := ep.DurationMs - ep.PlayedMs
	switch {
	case ep.State == "COMPLETED":
		kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return Icon(gtx, "check-circle", 14, ColAccent) }),
				layout.Rigid(Gap(4, 0)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return Text(gtx, "Played", 14, Regular, ColAccent) }),
			)
		}))
	case ep.State == "IN_PROGRESS" && left > 0:
		kids = append(kids, sub(FmtEpisodeLen(left)+" left"), gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			bw, bh := Dp(gtx, 100), Dp(gtx, 4)
			FillRect(gtx.Ops, image.Rect(0, 0, bw, bh), bh/2, Alpha(ColText, .3))
			FillRect(gtx.Ops, image.Rect(0, 0, bw*ep.PlayedMs/max(1, ep.DurationMs), bh), bh/2, ColText)
			return layout.Dimensions{Size: image.Pt(bw, bh)}
		}))
	default:
		kids = append(kids, sub(FmtEpisodeLen(ep.DurationMs)))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
}

// FmtEpisodeLen is a length the way the episode list shows it: "35 min 42 sec"
// under an hour, "3 hr 43 min" above.
func FmtEpisodeLen(ms int) string {
	sec := (ms + 500) / 1000
	h, m, s := sec/3600, sec/60%60, sec%60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%d hr %d min", h, m)
	case h > 0:
		return fmt.Sprintf("%d hr", h)
	case s > 0 && m > 0:
		return fmt.Sprintf("%d min %d sec", m, s)
	case m > 0:
		return fmt.Sprintf("%d min", m)
	}
	return fmt.Sprintf("%d sec", s)
}

// FmtEpisodeDate is "Today", "Sep 17" or, for another year, "Sep 17, 2025".
func FmtEpisodeDate(iso string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return ""
	}
	t = t.Local()
	switch {
	case t.Year() == now.Year() && t.YearDay() == now.YearDay():
		return "Today"
	case t.Year() == now.Year():
		return fmt.Sprintf("%s %d", t.Month().String()[:3], t.Day())
	}
	return fmt.Sprintf("%s %d, %d", t.Month().String()[:3], t.Day(), t.Year())
}
