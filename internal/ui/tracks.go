package ui

import (
	"image"
	"image/color"
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
	Gen   int // bumped whenever rows arrive
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
	p.Gen++
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
		if p.Items[pg*p.size] == nil {
			p.load(pg)
		}
	}
}

func (p *Paged) load(pg int) {
	if p.busy[pg] {
		return
	}
	p.busy[pg] = true
	off := pg * p.size
	Async(func() (*spotify.TrackPage, error) { return p.fetch(off, p.size) }, func(tp *spotify.TrackPage, err error) {
		delete(p.busy, pg)
		if err != nil {
			p.Err = err
			return
		}
		p.put(off, tp.Items)
	})
}

// Refresh takes the list's reloaded first page. The rows past it stay
// shown, where they were, while the pages they're on reload too.
func (p *Paged) Refresh(page spotify.TrackPage) {
	old := p.Items
	p.Total = page.Total
	p.Items = make([]*spotify.Track, page.Total)
	copy(p.Items, old)
	p.put(page.Offset, page.Items)
	if p.fetch == nil || p.size == 0 {
		return
	}
	for pg := 1; pg*p.size < min(len(old), p.Total); pg++ {
		if old[pg*p.size] != nil {
			p.load(pg)
		}
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
// (# Title Album Date-added Duration), "album" (no art, track numbers, plays),
// "artist" (plays column, no header, no artist names), "search" (Title Album Duration), "compact" (cover, title, duration: search
// top songs).
type TrackTable struct {
	Variant    string
	ContextURI string // "" plays tracks on their own
	// PlaylistURI and CanEdit enable "Remove from this playlist".
	PlaylistURI string
	CanEdit     bool
	// OnPlayed runs when a row starts playing (search saves a recent search).
	OnPlayed func(spotify.Track)
	// OnMove enables drag-reorder: row from was dropped above/below row to.
	OnMove func(from, to int, after bool)
	// NoAdded drops the date added column (generated playlists have no dates).
	NoAdded bool
	// NoCheck hides the "saved" check button: in Liked Songs and playlists
	// you can edit every song is already saved somewhere.
	NoCheck bool
	// Sort makes the Title, Album, Date added and duration headers sort
	// the list, as clicking them does in the desktop client.
	Sort *ListControls

	Selected int
	hdr      [2][4]Click // header buttons, unpinned and pinned
	rows     map[int]*trackRow
	reorder  Reorder
}

type trackRow struct {
	row, play, save, more Click
	drag                  TrackDrag
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
func (tt *TrackTable) Reset() { tt.rows = nil; tt.Selected = -1; tt.reorder = Reorder{} }

type cols struct {
	idx, title, album, added, plays, dur int
	gap                                  int
}

// columns resolves the CSS grid of the web player's rows: a 16dp index column,
// then 4fr (title), 2fr (album or plays), 2fr (date added) and 1fr (duration)
// tracks of at least 120dp each, 16dp apart. Album and date added appear
// from 600dp and 750dp of main view width. w is the table's width inside the
// page padding.
func (tt *TrackTable) columns(gtx layout.Context, w int) cols {
	px := func(v float32) int { return Dp(gtx, v) }
	gap := px(16)
	main := MainDp(gtx)
	vars := 0 // album/plays, date added
	switch tt.Variant {
	case "search", "artist", "album":
		if main >= 600 {
			vars = 1
		}
	case "playlist":
		if main >= 600 {
			vars = 1
		}
		if main >= 750 && !tt.NoAdded {
			vars = 2
		}
	}
	fr := []float64{4, 2, 2}[:1+vars]
	fr = append(fr, 1) // the duration track
	size := make([]float64, len(fr))
	min120 := float64(px(120))
	free := float64(w - 2*px(16) - px(16) - gap*len(fr)) // minus row padding, index, gaps
	done := make([]bool, len(fr))
	for {
		sum, rest := 0.0, free
		for i, f := range fr {
			if done[i] {
				rest -= size[i]
			} else {
				sum += f
			}
		}
		clamped := false
		for i, f := range fr {
			if done[i] {
				continue
			}
			size[i] = max(0, rest) * f / sum
			if size[i] < min120 {
				size[i], done[i], clamped = min120, true, true
			}
		}
		if !clamped {
			break
		}
	}
	c := cols{gap: gap, idx: px(16), title: int(size[0]), dur: int(size[len(fr)-1])}
	if vars > 0 {
		if tt.Variant == "artist" || tt.Variant == "album" {
			c.plays = int(size[1])
		} else {
			c.album = int(size[1])
		}
	}
	if vars > 1 {
		c.added = int(size[2])
	}
	return c
}

// headerKeys are the sort keys of the Title, Album, Date added and duration
// headers.
var headerKeys = [4]string{SortTitle, SortAlbum, SortAdded, SortDuration}

// Header draws the column header row ("# Title Album Date added ⏱").
// stuck gives it the darker background used while it's pinned under the top bar.
func (tt *TrackTable) Header(gtx layout.Context, stuck bool) layout.Dimensions {
	pin := 0
	if stuck {
		pin = 1
	}
	btns := &tt.hdr[pin]
	sortable := func(k int) bool { return tt.Sort != nil && tt.Sort.Has(headerKeys[k]) }
	for k := range btns {
		if btns[k].Clicked(gtx) && sortable(k) {
			tt.Sort.HeaderSort(headerKeys[k])
		}
	}
	w := gtx.Constraints.Max.X
	h, pad := Dp(gtx, 36), ContentPad(gtx)
	if stuck {
		FillRect(gtx.Ops, image.Rect(0, 0, w, h), 0, ColSurface2)
	} else {
		Offset(gtx, pad, h-1, func() { Hline(gtx, w-2*pad, ColDivider) })
	}
	c := tt.columns(gtx, w-2*pad)
	x := pad + Dp(gtx, 16)
	// label draws a header's text (or clock), with the sort caret when the
	// list is sorted by it; k < 0 isn't sortable.
	label := func(gtx layout.Context, k int, s string) layout.Dimensions {
		draw := func(gtx layout.Context, col color.NRGBA) layout.Dimensions {
			var d layout.Dimensions
			if s == "" {
				d = IconPx(gtx, "clock", Dp(gtx, 16), col)
			} else {
				d = Text(gtx, s, 14, Regular, col)
			}
			if k >= 0 && tt.Sort != nil && tt.Sort.SortedBy(headerKeys[k]) {
				is := Dp(gtx, 16)
				Offset(gtx, d.Size.X+Dp(gtx, 4), (d.Size.Y-is)/2, func() { IconPx(gtx, tt.Sort.Caret(), is, ColAccent) })
				d.Size.X += Dp(gtx, 4) + is
			}
			return d
		}
		if k < 0 || !sortable(k) {
			return draw(gtx, ColText2)
		}
		b := &btns[k]
		b.Label = "Sort by " + map[int]string{0: "title", 1: "album", 2: "date added", 3: "duration"}[k]
		return b.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			col := ColText2
			if b.Hovered() || tt.Sort.SortedBy(headerKeys[k]) {
				col = ColText
			}
			return draw(gtx, col)
		})
	}
	cell := func(cw, k int, s string) {
		if cw <= 0 {
			return
		}
		Offset(gtx, x, 0, func() {
			g := gtx
			g.Constraints = layout.Exact(image.Pt(cw, h))
			al := layout.W
			if k == -2 { // "#"
				al = layout.E
			}
			al.Layout(g, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				return label(gtx, k, s)
			})
		})
		x += cw + c.gap
	}
	cell(c.idx, -2, "#")
	cell(c.title, 0, "Title")
	cell(c.album, 1, "Album")
	cell(c.added, 2, "Date added")
	cell(c.plays, -1, "Plays")
	// The clock sits over the duration text; a caret goes after it.
	Offset(gtx, x+c.dur-Dp(gtx, 16)-Dp(gtx, 32)-Dp(gtx, 16), 0, func() {
		g := gtx
		g.Constraints = layout.Exact(image.Pt(Dp(gtx, 64), h))
		layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return label(gtx, 3, "")
		})
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
	case IsCurrentTrack(t, tt.ContextURI):
		PlayPause()
	default:
		PlayTrack(tt.ContextURI, t)
	}
}

// Row draws row i (t nil = still loading). The row includes the table's
// 24dp side padding.
func (tt *TrackTable) Row(gtx layout.Context, i int, t *spotify.Track) layout.Dimensions {
	r := tt.row(i)
	w := gtx.Constraints.Max.X
	h := Dp(gtx, 56)
	if t == nil {
		Offset(gtx, ContentPad(gtx)+Dp(gtx, 16), 0, func() {
			g := gtx
			ex := Dp(gtx, 16)
			g.Constraints = layout.Exact(image.Pt(Dp(gtx, 16)+ex, h))
			Offset(gtx, -ex, 0, func() {
				layout.E.Layout(g, func(gtx layout.Context) layout.Dimensions {
					return Text(gtx, FmtNumber(i+1), 16, Regular, ColText2)
				})
			})
			FillRect(gtx.Ops, image.Rect(Dp(gtx, 32), h/2-Dp(gtx, 7), Dp(gtx, 32)+w*4/10, h/2+Dp(gtx, 7)), Dp(gtx, 7), ColSurface2)
		})
		return layout.Dimensions{Size: image.Pt(w, h)}
	}
	tr := *t
	key := LikeKey(tr)
	current := IsCurrentTrack(tr, tt.ContextURI)
	playingNow := current && Playing()

	if r.play.Clicked(gtx) {
		tt.play(tr)
	}
	if r.save.Clicked(gtx) {
		AddedClicked(key)
	}
	LoadInPlaylists()
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
	if tt.Variant == "compact" {
		return tt.compactRow(gtx, i, tr, r)
	}
	pad := ContentPad(gtx)
	return Pad(gtx, 0, float32(pad)/gtx.Metric.PxPerDp, 0, float32(pad)/gtx.Metric.PxPerDp, func(gtx layout.Context) layout.Dimensions {
		rw := w - 2*pad
		return r.row.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			hov := r.row.Hovered()
			switch {
			case tt.reorder.Active && tt.reorder.From == i:
				FillRect(gtx.Ops, image.Rect(0, 0, rw, h), Dp(gtx, 4), ColRowHover)
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
				num := FmtNumber(i + 1)
				if tt.Variant == "album" && tr.TrackNumber > 0 {
					num = FmtNumber(tr.TrackNumber)
				}
				switch {
				case hov:
					icon := "play"
					if playingNow {
						icon = "pause"
					}
					// Centered where the number is (right-aligned in the column).
					is := Dp(gtx, 16)
					nw := Measure(gtx, num, 16, Regular)
					Offset(gtx, c.idx-(nw+is)/2, (h-is)/2, func() {
						r.play.Label = "Play " + tr.Name
						r.play.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return IconPx(gtx, icon, is, ColText)
						})
					})
				case playingNow:
					Offset(gtx, c.idx-Dp(gtx, 14), (h-Dp(gtx, 14))/2, func() { Equalizer(gtx, Dp(gtx, 14)) })
				default:
					col := sub
					if current {
						col = ColAccent
					}
					// Right-aligned and allowed to overflow left, like the CSS column.
					m := op.Record(gtx.Ops)
					g.Constraints = layout.Constraints{Max: image.Pt(Dp(gtx, 64), h)}
					d := Text(g, num, 16, Regular, col)
					call := m.Stop()
					Offset(gtx, c.idx-d.Size.X, (h-d.Size.Y)/2, func() { call.Add(gtx.Ops) })
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
				if tr.Video {
					segs = append(segs, Seg{Widget: func(gtx layout.Context) layout.Dimensions { return VideoBadge(gtx, sub) }}, Seg{Text: "Music video"})
					if tt.Variant != "artist" && len(tr.Artists) > 0 {
						segs = append(segs, Seg{Text: " • "})
					}
				}
				for k, a := range tr.Artists {
					if tt.Variant == "artist" {
						break // an artist's Popular rows only show the title
					}
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
				saved := Added(key) // Liked Songs or any of your playlists
				if !tt.NoCheck && (hov || saved) {
					Offset(gtx, saveX, (h-bs)/2, func() {
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
			var ro *Reorder
			var move func(from, to int, after bool)
			if tt.OnMove != nil && t.UID != "" {
				ro, move = &tt.reorder, tt.OnMove
			}
			r.drag.Handle(gtx, TrackPayload(tr), ro, i, h, move)
			if d := tt.reorder.DropLine(i); d != 0 {
				ly := 0
				if d > 0 {
					ly = h - Dp(gtx, 2)
				}
				FillRect(gtx.Ops, image.Rect(0, ly, rw, ly+Dp(gtx, 2)), 0, ColAccent)
			}
			return layout.Dimensions{Size: image.Pt(rw, h)}
		})
	})
}

// ExplicitBadge is the small "E" tag.
func ExplicitBadge(gtx layout.Context) layout.Dimensions {
	s := Dp(gtx, 16)
	FillRect(gtx.Ops, image.Rect(0, 0, s, s), Dp(gtx, 4), Alpha(ColText, .1))
	g := gtx
	g.Constraints = layout.Exact(image.Pt(s, s))
	layout.Center.Layout(g, func(gtx layout.Context) layout.Dimensions { return Text(gtx, "E", 12, Bold, ColText) })
	return layout.Dimensions{Size: image.Pt(s+Dp(gtx, 4), s), Baseline: Dp(gtx, 3)}
}

// VideoBadge is the screen icon before "Music video" in a row's subtitle.
func VideoBadge(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	s := Dp(gtx, 16)
	IconPx(gtx, "video", s, c)
	return layout.Dimensions{Size: image.Pt(s+Dp(gtx, 4), s), Baseline: Dp(gtx, 3)}
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
