package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
)

func gtxFor(main int) layout.Context {
	mainWidth = main
	return layout.Context{
		Constraints: layout.Exact(image.Pt(main, 800)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
}

func TestContentPad(t *testing.T) {
	for main, want := range map[int]int{476: 16, 600: 16, 812: 20, 1025: 24, 1564: 24} {
		if got := ContentPad(gtxFor(main)); got != want {
			t.Errorf("ContentPad(%d) = %d, want %d", main, got, want)
		}
	}
}

// The web player's track grid: 4fr 2fr 2fr and a duration track of at least
// 120dp, with album and date added appearing from 600dp and 750dp.
func TestTrackColumns(t *testing.T) {
	tt := &TrackTable{Variant: "playlist"}
	for _, c := range []struct {
		main                     int
		album, added             bool
		title, album2, added2, d int // expected widths (±5: the web rows have a 1dp border)
	}{
		{516, false, false, 280, 0, 0, 120},
		{616, true, false, 242, 121, 0, 120},
		{916, true, true, 318, 159, 159, 120},
	} {
		g := gtxFor(c.main)
		pad := ContentPad(g)
		cs := tt.columns(g, c.main-2*pad)
		near := func(got, want int) bool { return got >= want-5 && got <= want+5 }
		if !near(cs.title, c.title) || !near(cs.album, c.album2) || !near(cs.added, c.added2) || !near(cs.dur, c.d) {
			t.Errorf("columns at %d = title %d album %d added %d dur %d, want %d %d %d %d",
				c.main, cs.title, cs.album, cs.added, cs.dur, c.title, c.album2, c.added2, c.d)
		}
	}
}

func TestEpisodeDate(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.Local)
	for iso, want := range map[string]string{
		now.Format(time.RFC3339): "Today",
		"2026-09-17T08:00:00Z":   "Sep 17",
		"2025-12-31T12:00:00Z":   "Dec 31, 2025",
		"not a date":             "",
	} {
		if got := FmtEpisodeDate(iso, now); got != want {
			t.Errorf("FmtEpisodeDate(%q) = %q, want %q", iso, got, want)
		}
	}
}

func TestCarouselSnap(t *testing.T) {
	for _, c := range []struct{ pos, pitch, max, want float32 }{
		{0, 200, 50, 0},   // a row that nearly fits stays at the start
		{50, 200, 50, 50}, // ...until scrolled to its end
		{290, 200, 1000, 200},
		{310, 200, 1000, 400},
		{990, 200, 1000, 1000},
	} {
		if got := carouselSnap(c.pos, c.pitch, c.max); got != c.want {
			t.Errorf("snap(%v, %v, %v) = %v, want %v", c.pos, c.pitch, c.max, got, c.want)
		}
	}
}

func TestHeaderHeight(t *testing.T) {
	for _, c := range []struct{ w, vh, want float32 }{
		{1113, 939, 281.7}, // 30vh
		{1113, 1080, 324},  // 30vh in a full-HD window
		{1500, 1400, 336},  // capped
		{600, 600, 186},    // narrow: the minimum
	} {
		if got := headerHeightDp(c.w, c.vh); got < c.want-.5 || got > c.want+.5 {
			t.Errorf("headerHeightDp(%v, %v) = %v, want %v", c.w, c.vh, got, c.want)
		}
	}
}
