package ui

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// FmtMins formats an episode length: "45 min", "2 hr 24 min".
func FmtMins(ms int) string {
	m := max(1, (ms+30000)/60000)
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	if m%60 == 0 {
		return fmt.Sprintf("%d hr", m/60)
	}
	return fmt.Sprintf("%d hr %d min", m/60, m%60)
}

// EpisodeRow is a podcast episode in a list: cover, title, description and
// a play button with date and progress.
type EpisodeRow struct {
	Row, Play Click
}

// Layout draws the row at the full constraint width. ctx is the show to play
// the episode in ("" plays it alone).
func (r *EpisodeRow) Layout(gtx layout.Context, ep spotify.Episode, ctx string) layout.Dimensions {
	current := IsCurrent(ep.URI, "")
	if r.Play.Clicked(gtx) {
		switch {
		case current:
			PlayPause()
		case ctx != "":
			Play(ctx, ep.URI)
		default:
			Play(ep.URI, "")
		}
	}
	if r.Row.Clicked(gtx) {
		Navigate("/episode/" + URIID(ep.URI))
	}
	r.Row.Label = ep.Name
	return r.Row.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		pad, img := Dp(gtx, 16), Dp(gtx, 112)
		tx := pad + img + Dp(gtx, 20)

		m := op.Record(gtx.Ops)
		var th int
		Offset(gtx, tx, pad, func() {
			g := gtx
			g.Constraints.Max = image.Pt(max(0, w-tx-pad), Dp(gtx, 4000))
			name := ColText
			if current {
				name = ColAccent
			}
			th = Column(g,
				func(gtx layout.Context) layout.Dimensions {
					return Label{Text: ep.Name, Size: 16, Weight: Bold, Color: name, MaxLines: 2}.Layout(gtx)
				},
				func(gtx layout.Context) layout.Dimensions {
					if ep.ShowName == "" || ctx != "" { // the show page lists its own episodes
						return layout.Dimensions{}
					}
					return Pad(gtx, 6, 0, 0, 0, func(gtx layout.Context) layout.Dimensions { return Text(gtx, ep.ShowName, 14, Regular, ColText) })
				},
				func(gtx layout.Context) layout.Dimensions {
					return Pad(gtx, 6, 0, 0, 0, func(gtx layout.Context) layout.Dimensions {
						return Label{Text: oneLine(ep.Description), Size: 14, Color: ColText2, MaxLines: 2}.Layout(gtx)
					})
				},
				func(gtx layout.Context) layout.Dimensions {
					return Pad(gtx, 10, 0, 0, 0, func(gtx layout.Context) layout.Dimensions { return r.foot(gtx, ep, current) })
				},
			).Size.Y
		})
		call := m.Stop()

		h := 2*pad + max(img, th)
		if r.Row.Hovered() {
			FillRect(gtx.Ops, image.Rect(0, 0, w, h), Dp(gtx, 8), ColRowHover)
		}
		Hline(gtx, w, ColDivider)
		Offset(gtx, pad, pad, func() {
			Image{URL: Pick(ep.Images, img), Radius: Dp(gtx, 8), Icon: "note"}.Layout(gtx, image.Pt(img, img))
		})
		call.Add(gtx.Ops)
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
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions { return Text(gtx, s, 14, Regular, ColText2) })
	}
	kids := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		s := Dp(gtx, 32)
		icon, label := "play", "Play "+ep.Name
		if current && Playing() {
			icon, label = "pause", "Pause "+ep.Name
		}
		r.Play.Label = label
		return r.Play.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			FillCircle(gtx.Ops, s, ColText)
			Offset(gtx, s/4, s/4, func() { IconPx(gtx, icon, s/2, ColBg) })
			return layout.Dimensions{Size: image.Pt(s, s)}
		})
	}), gap}
	if ep.Explicit {
		kids = append(kids, layout.Rigid(ExplicitBadge))
	}
	if d := FmtAdded(ep.ReleaseDate, gtx.Now); d != "" {
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
		kids = append(kids, sub(FmtMins(left)+" left"), gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			bw, bh := Dp(gtx, 100), Dp(gtx, 4)
			FillRect(gtx.Ops, image.Rect(0, 0, bw, bh), bh/2, Alpha(ColText, .3))
			FillRect(gtx.Ops, image.Rect(0, 0, bw*ep.PlayedMs/max(1, ep.DurationMs), bh), bh/2, ColText)
			return layout.Dimensions{Size: image.Pt(bw, bh)}
		}))
	default:
		kids = append(kids, sub(FmtMins(ep.DurationMs)))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
}
