package artist

import (
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
)

func TestCompact(t *testing.T) {
	for n, want := range map[int]string{
		0: "0", 999: "999", 5432: "5,432", 12345: "12.3K", 123456: "123K",
		1_000_000: "1M", 46_442_786: "46.4M", 105_983_595: "106M", 2_500_000_000: "2.5B",
	} {
		if got := compact(n); got != want {
			t.Errorf("compact(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestConcertCols(t *testing.T) {
	for w, want := range map[int]int{300: 1, 359: 1, 360: 2, 539: 2, 540: 3, 1096: 3} {
		if got := concertCols(w); got != want {
			t.Errorf("concertCols(%d) = %d, want %d", w, got, want)
		}
	}
}

func TestAboutWidth(t *testing.T) {
	// 1065px: five 193.8px columns, the card spans four (847px, as on the web).
	// 630px: three columns, so it takes the whole row.
	for w, want := range map[int]int{1065: 847, 630: 630, 380: 380} {
		if got := aboutWidth(w, 1); got != want {
			t.Errorf("aboutWidth(%d) = %d, want %d", w, got, want)
		}
	}
}

func TestCarouselSlide(t *testing.T) {
	var m about
	gtx := layout.Context{Now: time.Unix(100, 0), Ops: new(op.Ops)}
	m.slideTo(gtx, 1)
	if p := m.pos(gtx); p != 0 {
		t.Errorf("slide start at %v, want 0", p)
	}
	gtx.Now = gtx.Now.Add(slideDur / 2)
	if p := m.pos(gtx); p <= .5 || p >= 1 {
		t.Errorf("halfway (eased out) at %v", p)
	}
	// Going back mid-slide starts from where the picture is.
	m.slideTo(gtx, 0)
	gtx.Now = gtx.Now.Add(slideDur)
	if p := m.pos(gtx); p != 0 {
		t.Errorf("slide end at %v, want 0", p)
	}
}
