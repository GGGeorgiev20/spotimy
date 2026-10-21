package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// SongRowHeight is the height of a "compact" row: 64dp, or 80dp (with a
// larger cover) once the main view is 1024dp wide.
func SongRowHeight(gtx layout.Context) int {
	if MainDp(gtx) >= 1024 {
		return Dp(gtx, 80)
	}
	return Dp(gtx, 64)
}

// compactRow is a Search result song: cover (play on hover), name over
// artists, a "Song" tag, the save button and, on hover, the more button.
func (tt *TrackTable) compactRow(gtx layout.Context, i int, tr spotify.Track, r *trackRow) layout.Dimensions {
	w, h := gtx.Constraints.Max.X, SongRowHeight(gtx)
	art := h - Dp(gtx, 16)
	current := IsCurrentTrack(tr, tt.ContextURI)
	playingNow := current && Playing()
	return r.row.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hov := r.row.Hovered()
		switch {
		case tt.Selected == i:
			FillRect(gtx.Ops, image.Rect(0, 0, w, h), Dp(gtx, 6), ColRowSelected)
		case hov:
			FillRect(gtx.Ops, image.Rect(0, 0, w, h), Dp(gtx, 6), ColRowHover)
		}
		alpha := float32(1)
		if !tr.Playable {
			alpha = .4
		}
		x := Dp(gtx, 12)
		var imgs []spotify.Image
		if tr.Album != nil {
			imgs = tr.Album.Images
		}
		Offset(gtx, x, (h-art)/2, func() {
			Image{URL: Pick(imgs, art), Radius: Dp(gtx, 4), Icon: "note"}.Layout(gtx, image.Pt(art, art))
			if hov || playingNow {
				FillRect(gtx.Ops, image.Rect(0, 0, art, art), Dp(gtx, 4), Alpha(ColBg, .5))
				icon, is := "play", Dp(gtx, 24)
				if playingNow {
					icon = "pause"
				}
				Offset(gtx, (art-is)/2, (art-is)/2, func() {
					r.play.Label = "Play " + tr.Name
					r.play.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return IconPx(gtx, icon, is, ColText) })
				})
			}
		})
		x += art + Dp(gtx, 12)

		// Trailing: tag at the start of a 270dp box, save and more at its end.
		bs, end := Dp(gtx, 16), w-Dp(gtx, 12)
		tagX := end - Dp(gtx, 270)
		Offset(gtx, tagX, 0, func() {
			th := Dp(gtx, 16)
			tw := Measure(gtx, "Song", 12, Bold) + Dp(gtx, 8)
			Offset(gtx, 0, (h-th)/2, func() {
				FillRect(gtx.Ops, image.Rect(0, 0, tw, th), Dp(gtx, 4), Mix(ColSurface, ColText, .1))
				Offset(gtx, Dp(gtx, 4), 0, func() {
					g := gtx
					g.Constraints = layout.Exact(image.Pt(tw, th))
					layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions { return Text(gtx, "Song", 12, Bold, ColText2) })
				})
			})
		})
		saved := Added(LikeKey(tr))
		Offset(gtx, end-bs, (h-bs)/2, func() {
			icon, col := "plus-circle", ColText2
			if saved {
				icon, col = "check-circle", ColAccent
			} else if r.save.Hovered() {
				col = ColText
			}
			r.save.Label = "Save to Liked Songs"
			if saved {
				r.save.Label = "Add to playlist"
			}
			r.save.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return IconPx(gtx, icon, bs, col) })
		})
		if hov {
			Offset(gtx, end-bs-Dp(gtx, 24)-bs, (h-bs)/2, func() {
				r.more.Label = "More options for " + tr.Name
				r.more.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return IconPx(gtx, "more", bs, ColText) })
			})
		}

		// Name over artists.
		tw := max(0, tagX-x-Dp(gtx, 12))
		nameCol := Alpha(ColText, alpha)
		if current {
			nameCol = ColAccent
		}
		sub := Alpha(ColText2, alpha)
		var segs []Seg
		if tr.Explicit {
			segs = append(segs, Seg{Widget: ExplicitBadge})
		}
		for k, a := range tr.Artists {
			if k > 0 {
				segs = append(segs, Seg{Text: ", "})
			}
			segs = append(segs, Seg{Text: a.Name, Link: &r.artists[k]})
		}
		m := op.Record(gtx.Ops)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(tw, h)}
		nd := Text(g, tr.Name, 16, Regular, nameCol)
		var ad layout.Dimensions
		Offset(gtx, 0, nd.Size.Y+Dp(gtx, 2), func() { ad = Inline(g, 14, Regular, sub, Alpha(ColText, alpha), segs...) })
		call := m.Stop()
		th := nd.Size.Y + Dp(gtx, 2) + ad.Size.Y
		Offset(gtx, x, (h-th)/2, func() { call.Add(gtx.Ops) })
		r.drag.Handle(gtx, TrackPayload(tr), nil, i, h, nil)
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}
