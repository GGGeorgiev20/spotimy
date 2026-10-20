package ui

import (
	"image"
	"math"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// Paged is a long track list loaded page by page as rows come into view.
type Paged struct {
	Items []*spotify.Track // nil = not loaded yet
	Total int
	Err   error
	fetch func(offset, limit int) (*spotify.TrackPage, error)
	size  int
	busy  map[int]bool
}

func NewPaged(fetch func(offset, limit int) (*spotify.TrackPage, error), pageSize int) *Paged {
	return &Paged{fetch: fetch, size: pageSize, busy: map[int]bool{}}
}

// Seed fills the first page that came with the page's header.
func (p *Paged) Seed(page spotify.TrackPage) {
	p.Total = page.Total
	p.Items = make([]*spotify.Track, page.Total)
	p.put(page.Offset, page.Items)
}

// SetAll fills a list that is already complete (albums).
func (p *Paged) SetAll(tracks []spotify.Track) {
	p.Seed(spotify.TrackPage{Total: len(tracks), Items: tracks})
}

func (p *Paged) put(off int, items []spotify.Track) {
	keys := make([]string, 0, len(items))
	for i := range items {
		if off+i < len(p.Items) {
			t := items[i]
			p.Items[off+i] = &t
			keys = append(keys, LikeKey(t))
		}
	}
	LoadSaved(keys)
}

// Ensure loads the pages covering rows [start, end).
func (p *Paged) Ensure(start, end int) {
	if p.fetch == nil || p.size == 0 {
		return
	}
	end = min(end, p.Total)
	for pg := max(0, start) / p.size; pg*p.size < end; pg++ {
		off := pg * p.size
		if p.Items[off] != nil || p.busy[pg] {
			continue
		}
		p.busy[pg] = true
		Async(func() (*spotify.TrackPage, error) { return p.fetch(off, p.size) }, func(tp *spotify.TrackPage, err error) {
			delete(p.busy, pg)
			if err != nil {
				p.Err = err
				return
			}
			p.put(off, tp.Items)
		})
	}
}

// Loaded returns the rows loaded so far.
func (p *Paged) Loaded() []spotify.Track {
	var out []spotify.Track
	for _, t := range p.Items {
		if t != nil {
			out = append(out, *t)
		}
	}
	return out
}

// TrackTable draws the official track list rows. Variant: "playlist"
// (# Title Album Date-added Duration), "album" (no art, track numbers),
// "artist" (plays column, no header), "search" (Title Album Duration).
type TrackTable struct {
	Variant    string
	ContextURI string // "" plays tracks on their own
	// PlaylistURI and CanEdit enable "Remove from this playlist".
	PlaylistURI string
	CanEdit     bool
	// OnPlayed runs when a row starts playing (search saves a recent search).
	OnPlayed func(spotify.Track)

	Selected int
	rows     map[int]*trackRow
}

type trackRow struct {
	row, play, save, more Click
	artists               []Link
	album                 Link
}

func (tt *TrackTable) row(i int) *trackRow {
	if tt.rows == nil {
		tt.rows = map[int]*trackRow{}
		tt.Selected = -1
	}
	r := tt.rows[i]
	if r == nil {
		r = &trackRow{}
		tt.rows[i] = r
	}
	return r
}

// Reset drops per-row state (after the list's contents change).
func (tt *TrackTable) Reset() { tt.rows = nil; tt.Selected = -1 }

type cols struct {
	idx, title, album, added, plays, dur int
	gap                                  int
}

func (tt *TrackTable) columns(gtx layout.Context, w int) cols {
	px := func(v float32) int { return Dp(gtx, v) }
	gap := px(16)
	inner := w - 2*px(16)
	mainW := w + 2*px(24) // the table sits inside 24dp padding of the main view
	var fr []float64
	var c cols
	c.gap, c.idx = gap, px(16)
	durMin := px(120)
	switch tt.Variant {
	case "album":
		fr = []float64{4}
	case "search", "artist":
		fr = []float64{4, 2}
	default:
		fr = []float64{6, 4, 3}
		if mainW <= px(900) {
			fr = fr[:2]
		}
		if mainW <= px(640) {
			fr = fr[:1]
		}
	}
	if tt.Variant == "search" && mainW <= px(640) {
		fr = fr[:1]
	}
	gaps := gap * (len(fr) + 1)
	free := inner - c.idx - gaps
	sum := 1.0
	for _, f := range fr {
		sum += f
	}
	unit := float64(free) / sum
	if int(unit) < durMin { // minmax(120px, 1fr)
		unit = float64(free-durMin) / (sum - 1)
		c.dur = durMin
	} else {
		c.dur = int(unit)
	}
	c.title = int(unit * fr[0])
	if len(fr) > 1 {
		if tt.Variant == "artist" {
			c.plays = int(unit * fr[1])
		} else {
			c.album = int(unit * fr[1])
		}
	}
	if len(fr) > 2 {
		c.added = int(unit * fr[2])
	}
	return c
}

// Header draws the column header row ("# Title Album Date added ⏱").
// stuck gives it the darker background used while it's pinned under the top bar.
func (tt *TrackTable) Header(gtx layout.Context, stuck bool) layout.Dimensions {
	w := gtx.Constraints.Max.X
	h := Dp(gtx, 36)
	if stuck {
		FillRect(gtx.Ops, image.Rect(0, 0, w, h), 0, ColSurface2)
	} else {
		Offset(gtx, Dp(gtx, 24), h-1, func() { Hline(gtx, w-Dp(gtx, 48), ColDivider) })
	}
	c := tt.columns(gtx, w-Dp(gtx, 48))
	x := Dp(gtx, 24+16)
	cell := func(cw int, s string, right bool) {
		if cw <= 0 {
			return
		}
		Offset(gtx, x, 0, func() {
			g := gtx
			g.Constraints = layout.Exact(image.Pt(cw, h))
			al := layout.W
			if right {
				al = layout.E
			}
			al.Layout(g, func(gtx layout.Context) layout.Dimensions { return Text(gtx, s, 14, Regular, ColText2) })
		})
		x += cw + c.gap
	}
	cell(c.idx, "#", true)
	cell(c.title, "Title", false)
	cell(c.album, "Album", false)
	cell(c.added, "Date added", false)
	cell(c.plays, "", false)
	Offset(gtx, x+c.dur-Dp(gtx, 16)-Dp(gtx, 32)-Dp(gtx, 16), (h-Dp(gtx, 16))/2, func() {
		IconPx(gtx, "clock", Dp(gtx, 16), ColText2)
	})
	return layout.Dimensions{Size: image.Pt(w, h+Dp(gtx, 16))}
}

func (tt *TrackTable) play(t spotify.Track) {
	if !t.Playable {
		return
	}
	if tt.OnPlayed != nil {
		tt.OnPlayed(t)
	}
	switch {
	case IsCurrent(t.URI, tt.ContextURI):
		PlayPause()
	case tt.ContextURI != "":
		Play(tt.ContextURI, t.URI)
	default:
		Play(t.URI, "")
	}
}

// Row draws row i (t nil = still loading). The row includes the table's
// 24dp side padding.
func (tt *TrackTable) Row(gtx layout.Context, i int, t *spotify.Track) layout.Dimensions {
	r := tt.row(i)
	w := gtx.Constraints.Max.X
	h := Dp(gtx, 56)
	if t == nil {
		Offset(gtx, Dp(gtx, 24+16), 0, func() {
			g := gtx
			g.Constraints = layout.Exact(image.Pt(Dp(gtx, 16), h))
			layout.E.Layout(g, func(gtx layout.Context) layout.Dimensions {
				return Text(gtx, FmtNumber(i+1), 16, Regular, ColText2)
			})
			FillRect(gtx.Ops, image.Rect(Dp(gtx, 32), h/2-Dp(gtx, 7), Dp(gtx, 32)+w*4/10, h/2+Dp(gtx, 7)), Dp(gtx, 7), ColSurface2)
		})
		return layout.Dimensions{Size: image.Pt(w, h)}
	}
	tr := *t
	key := LikeKey(tr)
	current := IsCurrent(tr.URI, tt.ContextURI)
	playingNow := current && Playing()

	if r.play.Clicked(gtx) {
		tt.play(tr)
	}
	if r.save.Clicked(gtx) {
		ToggleSaved(key)
	}
	if r.more.Clicked(gtx) {
		tt.Selected = i
		OpenMenu(TrackMenu(tr, TrackMenuOpts{PlaylistURI: tt.PlaylistURI, CanEdit: tt.CanEdit}))
	}
	if r.row.Clicked(gtx) {
		tt.Selected = i
	}
	if r.row.DoubleClicked(gtx) {
		tt.play(tr)
	}
	if _, ok := r.row.RightClicked(gtx); ok {
		tt.Selected = i
		OpenMenu(TrackMenu(tr, TrackMenuOpts{PlaylistURI: tt.PlaylistURI, CanEdit: tt.CanEdit}))
	}
	if len(r.artists) < len(tr.Artists) {
		r.artists = make([]Link, len(tr.Artists))
	}
	for k := range tr.Artists {
		if r.artists[k].Click.Clicked(gtx) {
			OpenURI(tr.Artists[k].URI)
		}
	}
	if r.album.Click.Clicked(gtx) && tr.Album != nil {
		OpenURI(tr.Album.URI)
	}

	r.row.Label = tr.Name
	r.row.Arrow = true // rows keep the arrow cursor, like the official client
	pad := Dp(gtx, 24)
	return Pad(gtx, 0, 24, 0, 24, func(gtx layout.Context) layout.Dimensions {
		rw := w - 2*pad
		return r.row.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			hov := r.row.Hovered()
			switch {
			case tt.Selected == i:
				FillRect(gtx.Ops, image.Rect(0, 0, rw, h), Dp(gtx, 4), ColRowSelected)
			case hov:
				FillRect(gtx.Ops, image.Rect(0, 0, rw, h), Dp(gtx, 4), ColRowHover)
			}
			alpha := float32(1)
			if !tr.Playable {
				alpha = .4
			}
			c := tt.columns(gtx, rw)
			x := Dp(gtx, 16)
			sub := Alpha(ColText2, alpha)
			if hov || tt.Selected == i {
				sub = Alpha(ColText, alpha)
			}
			// Index / equalizer / play.
			Offset(gtx, x, 0, func() {
				g := gtx
				g.Constraints = layout.Exact(image.Pt(c.idx, h))
				switch {
				case hov:
					icon := "play"
					if playingNow {
						icon = "pause"
					}
					is := Dp(gtx, 16)
					Offset(gtx, c.idx-is+Dp(gtx, 4), (h-is)/2, func() {
						r.play.Label = "Play " + tr.Name
						r.play.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return IconPx(gtx, icon, is, ColText)
						})
					})
				case playingNow:
					Offset(gtx, c.idx-Dp(gtx, 14), (h-Dp(gtx, 14))/2, func() { Equalizer(gtx, Dp(gtx, 14)) })
				default:
					num := FmtNumber(i + 1)
					if tt.Variant == "album" && tr.TrackNumber > 0 {
						num = FmtNumber(tr.TrackNumber)
					}
					col := sub
					if current {
						col = ColAccent
					}
					// Two digits are wider than the column: let them grow leftwards.
					g.Constraints = layout.Exact(image.Pt(2*c.idx, h))
					Offset(gtx, -c.idx, 0, func() {
						layout.E.Layout(g, func(gtx layout.Context) layout.Dimensions {
							return Text(gtx, num, 16, Regular, col)
						})
					})
				}
			})
			x += c.idx + c.gap
			// Title cell.
			Offset(gtx, x, 0, func() {
				tx := 0
				if tt.Variant != "album" {
					is := Dp(gtx, 40)
					var imgs []spotify.Image
					if tr.Album != nil {
						imgs = tr.Album.Images
					}
					Offset(gtx, 0, (h-is)/2, func() {
						Image{URL: Pick(imgs, is), Radius: Dp(gtx, 4), Icon: "note"}.Layout(gtx, image.Pt(is, is))
					})
					tx = is + Dp(gtx, 12)
				}
				tw := max(0, c.title-tx)
				nameCol := Alpha(ColText, alpha)
				if current {
					nameCol = ColAccent
				}
				m := op.Record(gtx.Ops)
				g := gtx
				g.Constraints = layout.Constraints{Max: image.Pt(tw, h)}
				nd := Text(g, tr.Name, 16, Regular, nameCol)
				gap := Dp(gtx, 2)
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
				var ad layout.Dimensions
				Offset(gtx, 0, nd.Size.Y+gap, func() {
					ad = Inline(g, 14, Regular, sub, Alpha(ColText, alpha), segs...)
				})
				call := m.Stop()
				th := nd.Size.Y + gap + ad.Size.Y
				Offset(gtx, tx, (h-th)/2, func() { call.Add(gtx.Ops) })
			})
			x += c.title + c.gap
			cellText := func(cw int, f func(gtx layout.Context) layout.Dimensions) {
				if cw <= 0 {
					return
				}
				Offset(gtx, x, 0, func() {
					g := gtx
					g.Constraints = layout.Exact(image.Pt(cw, h))
					layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min = image.Point{}
						return f(gtx)
					})
				})
				x += cw + c.gap
			}
			cellText(c.album, func(gtx layout.Context) layout.Dimensions {
				if tr.Album == nil {
					return layout.Dimensions{}
				}
				return r.album.Layout(gtx, tr.Album.Name, 14, Regular, sub, Alpha(ColText, alpha))
			})
			cellText(c.added, func(gtx layout.Context) layout.Dimensions {
				return Text(gtx, FmtAdded(tr.AddedAt, gtx.Now), 14, Regular, sub)
			})
			cellText(c.plays, func(gtx layout.Context) layout.Dimensions {
				if tr.Playcount == "" {
					return layout.Dimensions{}
				}
				return Text(gtx, FmtNumber(tr.Playcount), 14, Regular, sub)
			})
			// Duration cell: [save] 3:07 [more], right-aligned.
			Offset(gtx, x, 0, func() {
				bs := Dp(gtx, 16)
				moreX := c.dur - bs
				durX := moreX - Dp(gtx, 16) - Measure(gtx, "0:00", 14, Regular)
				saveX := durX - Dp(gtx, 16) - bs
				saved := Saved[key]
				if hov || saved {
					Offset(gtx, saveX, (h-bs)/2, func() {
						icon, col := "plus-circle", ColText2
						if saved {
							icon, col = "check-circle", ColAccent
						} else if r.save.Hovered() {
							col = ColText
						}
						r.save.Label = "Save to Liked Songs"
						if saved {
							r.save.Label = "Remove from Liked Songs"
						}
						r.save.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return IconPx(gtx, icon, bs, col) })
					})
				}
				dur := FmtDuration(int64(tr.DurationMs))
				dw := Measure(gtx, dur, 14, Regular)
				Offset(gtx, moreX-Dp(gtx, 16)-dw, 0, func() {
					g := gtx
					g.Constraints = layout.Exact(image.Pt(dw, h))
					layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions { return Text(gtx, dur, 14, Regular, sub) })
				})
				if hov {
					Offset(gtx, moreX, (h-bs)/2, func() {
						r.more.Label = "More options for " + tr.Name
						r.more.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return IconPx(gtx, "more", bs, ColText) })
					})
				}
			})
			return layout.Dimensions{Size: image.Pt(rw, h)}
		})
	})
}

// ExplicitBadge is the small "E" tag.
func ExplicitBadge(gtx layout.Context) layout.Dimensions {
	s := Dp(gtx, 16)
	FillRect(gtx.Ops, image.Rect(0, 0, s, s), Dp(gtx, 2), Alpha(ColText, .6))
	g := gtx
	g.Constraints = layout.Exact(image.Pt(s, s))
	layout.Center.Layout(g, func(gtx layout.Context) layout.Dimensions { return Text(gtx, "E", 9, Bold, ColBg) })
	return layout.Dimensions{Size: image.Pt(s+Dp(gtx, 6), s), Baseline: Dp(gtx, 3)}
}

// Equalizer draws the animated "playing" bars (size px square).
func Equalizer(gtx layout.Context, size int) layout.Dimensions {
	bw := max(1, size/7)
	t := float64(gtx.Now.UnixMilli()) / 1000
	for k, ph := range []float64{0, .3, .6, .45} {
		v := .5 + .5*math.Sin(2*math.Pi*(t/0.9+ph))
		bh := int(float64(size)*0.2 + float64(size)*0.8*v)
		x := k * (bw + max(1, size/14)*2)
		FillRect(gtx.Ops, image.Rect(x, size-bh, x+bw, size), 0, ColAccent)
	}
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(50 * time.Millisecond)})
	return layout.Dimensions{Size: image.Pt(size, size)}
}
