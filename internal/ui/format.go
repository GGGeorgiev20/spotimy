package ui

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// FmtDuration formats a track length: 3:07, 1:02:03.
func FmtDuration(ms int64) string {
	s := ms / 1000
	h, m := s/3600, s%3600/60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s%60)
	}
	return fmt.Sprintf("%d:%02d", m, s%60)
}

// FmtTotal formats a list total: "47 min 12 sec", "1 hr 23 min", "about 12 hr".
func FmtTotal(ms int64) string {
	min := ms / 60000
	if min < 60 {
		return fmt.Sprintf("%d min %d sec", min, ms/1000%60)
	}
	h := min / 60
	if h >= 10 {
		return fmt.Sprintf("about %d hr", h)
	}
	return fmt.Sprintf("%d hr %d min", h, min%60)
}

// FmtListLength is exact once every row is loaded, otherwise an estimate from
// the loaded rows (what the official client's "about" means).
func FmtListLength(loaded []spotify.Track, total int) string {
	if len(loaded) == 0 {
		return ""
	}
	var ms int64
	for _, t := range loaded {
		ms += int64(t.DurationMs)
	}
	if len(loaded) >= total {
		return FmtTotal(ms)
	}
	est := FmtTotal(ms / int64(len(loaded)) * int64(total))
	if strings.HasPrefix(est, "about") {
		return est
	}
	return "about " + est
}

func plural(n int64, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// FmtAdded formats "Date added": relative for the last few weeks, otherwise "Feb 12, 2026".
func FmtAdded(iso string, now time.Time) string {
	if iso == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return ""
	}
	d := now.Sub(t)
	days := int64(d / (24 * time.Hour))
	switch {
	case days < 1:
		if h := int64(d / time.Hour); h >= 1 {
			return plural(h, "hour") + " ago"
		}
		return "just now"
	case days < 7:
		return plural(days, "day") + " ago"
	case days < 28:
		return plural(days/7, "week") + " ago"
	}
	t = t.Local()
	return fmt.Sprintf("%s %d, %d", t.Month().String()[:3], t.Day(), t.Year())
}

// FmtAgo is the short relative time of Friend Activity: "3 min", "5 hr", "2 d", "1 w".
func FmtAgo(ms int64, now time.Time) string {
	d := now.Sub(time.UnixMilli(ms))
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min", max(1, int64(d/time.Minute)))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hr", int64(d/time.Hour))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d d", int64(d/(24*time.Hour)))
	}
	return fmt.Sprintf("%d w", int64(d/(7*24*time.Hour)))
}

// FmtNumber groups thousands: 1234567 -> 1,234,567. Strings are parsed first.
func FmtNumber[T int | int64 | string](n T) string {
	var v int64
	switch x := any(n).(type) {
	case int:
		v = int64(x)
	case int64:
		v = x
	case string:
		v, _ = strconv.ParseInt(x, 10, 64)
	}
	s := strconv.FormatInt(max(v, -v), 10)
	var b strings.Builder
	if v < 0 {
		b.WriteByte('-')
	}
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// HeaderColor darkens a cover color so white text stays readable, like
// Spotify's extracted "dark" variant.
func HeaderColor(c color.NRGBA) color.NRGBA {
	lum := .299*float32(c.R) + .587*float32(c.G) + .114*float32(c.B)
	f := float32(1)
	if lum > 120 {
		f = 120 / lum
	}
	return Scale(c, f*.85)
}

// LyricsColor keeps the cover's hue vivid but brings luminance into a mid band.
func LyricsColor(c color.NRGBA) color.NRGBA {
	lum := .299*float32(c.R) + .587*float32(c.G) + .114*float32(c.B)
	if lum == 0 {
		lum = 1
	}
	return Scale(c, max(90, min(lum, 130))/lum)
}

// ArtistNames joins artist names with ", ".
func ArtistNames(as []spotify.Artist) string {
	names := make([]string, len(as))
	for i, a := range as {
		names[i] = a.Name
	}
	return strings.Join(names, ", ")
}

// Tone is c moved to CIE L* lightness l, keeping its hue and as much of its
// chroma as sRGB holds. Spotify's dynamic colors are tones of a cover's
// color: the Now Playing lyrics card is about L* 22, its upcoming lines 82.
func Tone(c color.NRGBA, l float64) color.NRGBA {
	_, a, b := toLab(c)
	ch, h := math.Hypot(a, b), math.Atan2(b, a)
	lo, hi := 0.0, ch // the largest chroma still inside sRGB
	for range 24 {
		m := (lo + hi) / 2
		if _, ok := fromLab(l, m*math.Cos(h), m*math.Sin(h)); ok {
			lo = m
		} else {
			hi = m
		}
	}
	out, _ := fromLab(l, lo*math.Cos(h), lo*math.Sin(h))
	return out
}

// Lightness is c's CIE L* (0–100).
func Lightness(c color.NRGBA) float64 { l, _, _ := toLab(c); return l }

var labWhite = [3]float64{0.95047, 1, 1.08883} // D65

func toLab(c color.NRGBA) (l, a, b float64) {
	lin := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.04045 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	r, g, bl := lin(c.R), lin(c.G), lin(c.B)
	x := (0.4124*r + 0.3576*g + 0.1805*bl) / labWhite[0]
	y := (0.2126*r + 0.7152*g + 0.0722*bl) / labWhite[1]
	z := (0.0193*r + 0.1192*g + 0.9505*bl) / labWhite[2]
	f := func(t float64) float64 {
		if t > 216.0/24389 {
			return math.Cbrt(t)
		}
		return (24389.0/27*t + 16) / 116
	}
	fx, fy, fz := f(x), f(y), f(z)
	return 116*fy - 16, 500 * (fx - fy), 200 * (fy - fz)
}

// fromLab converts back to sRGB; ok is false outside the gamut.
func fromLab(l, a, b float64) (color.NRGBA, bool) {
	fi := func(t float64) float64 {
		if t*t*t > 216.0/24389 {
			return t * t * t
		}
		return (116*t - 16) / (24389.0 / 27)
	}
	fy := (l + 16) / 116
	x, y, z := labWhite[0]*fi(fy+a/500), labWhite[1]*fi(fy), labWhite[2]*fi(fy-b/200)
	rgb := [3]float64{3.2406*x - 1.5372*y - 0.4986*z, -0.9689*x + 1.8758*y + 0.0415*z, 0.0557*x - 0.2040*y + 1.0570*z}
	ok := true
	var out [3]uint8
	for i, v := range rgb {
		if v < -1e-4 || v > 1+1e-4 {
			ok = false
		}
		v = min(1, max(0, v))
		if v <= 0.0031308 {
			v *= 12.92
		} else {
			v = 1.055*math.Pow(v, 1/2.4) - 0.055
		}
		out[i] = uint8(math.Round(v * 255))
	}
	return color.NRGBA{R: out[0], G: out[1], B: out[2], A: 255}, ok
}
