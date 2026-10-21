package ui

import (
	"image/color"
	"testing"
	"time"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

func TestFormat(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	for got, want := range map[string]string{
		FmtDuration(187000):                               "3:07",
		FmtDuration(3723000):                              "1:02:03",
		FmtTotal(2832000):                                 "47 min 12 sec",
		FmtTotal(4980000):                                 "1 hr 23 min",
		FmtTotal(43200000):                                "about 12 hr",
		FmtNumber(1234567):                                "1,234,567",
		FmtNumber("999"):                                  "999",
		FmtNumber(-1000):                                  "-1,000",
		FmtAdded("2026-10-20T09:00:00Z", now):             "3 hours ago",
		FmtAdded("2026-10-17T12:00:00Z", now):             "3 days ago",
		FmtAdded("2026-10-06T12:00:00Z", now):             "2 weeks ago",
		FmtAgo(now.Add(-90*time.Minute).UnixMilli(), now): "1 hr",
		FmtListLength([]spotify.Track{{DurationMs: 60000}, {DurationMs: 120000}}, 4): "about 6 min 0 sec",
		URIToPath("spotify:user:me:collection"):                                      "/collection/tracks",
		URIToPath("spotify:playlist:abc"):                                            "/playlist/abc",
		URIToPath("spotify:user:bob"):                                                "/user/bob",
		URIToPath("spotify:page:0JQ5DAqbMKFQ00XGBls6ym"):                             "/genre/0JQ5DAqbMKFQ00XGBls6ym",
	} {
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}

func TestAlphaMatchesBrowser(t *testing.T) {
	// 10% white over #121212 must look like the browser's #2a2a2a after
	// Gio's linear blend: lin(out) = lin(bg) + a*(lin(fg)-lin(bg)).
	a := float64(Alpha(ColText, .1).A) / 255
	out := toLinear(0.07) + a*(1-toLinear(0.07))
	if want := toLinear(42.0 / 255); out < want*0.9 || out > want*1.1 {
		t.Errorf("alpha %.3f gives linear %.4f, want %.4f", a, out, want)
	}
}

// Spotify's lyrics card colors for real covers (its higherContrast set):
// the background is the cover color at L* 22 (darker ones stay), the
// upcoming lines at 82.
func TestTone(t *testing.T) {
	near := func(a, b color.NRGBA) bool {
		d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
		return d(a.R, b.R) <= 12 && d(a.G, b.G) <= 12 && d(a.B, b.B) <= 12
	}
	for _, c := range []struct{ raw, bg, sub string }{
		{"#904870", "#5c1841", "#ffb9e0"},
		{"#777777", "#353535", "#cdcdcd"},
		{"#500810", "#500810", "#ffbdb9"},
		{"#7a7a29", "#393600", "#d1d284"},
	} {
		raw := Hex(c.raw)
		if bg := Tone(raw, min(Lightness(raw), 22)); !near(bg, Hex(c.bg)) {
			t.Errorf("%s: background %v, want ~%s", c.raw, bg, c.bg)
		}
		if sub := Tone(raw, 82); !near(sub, Hex(c.sub)) {
			t.Errorf("%s: upcoming lines %v, want ~%s", c.raw, sub, c.sub)
		}
	}
}
