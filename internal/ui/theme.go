package ui

import (
	"embed"
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
)

// Colors of the official dark theme.
var (
	ColBg          = RGB(0x000000)
	ColSurface     = RGB(0x121212) // main panels
	ColSurface2    = RGB(0x1f1f1f) // hover / inputs
	ColSurface3    = RGB(0x2a2a2a) // menus, placeholders
	ColHighlight   = RGB(0x333333)
	ColText        = RGB(0xffffff)
	ColText2       = RGB(0xb3b3b3) // subdued
	ColText3       = RGB(0x7c7c7c)
	ColAccent      = RGB(0x1ed760)
	ColAccentHover = RGB(0x3be477)
	ColToast       = RGB(0x3d91f4)
	ColRowHover    = ARGB(0x1affffff) // hsla(0,0%,100%,.1)
	ColRowSelected = ARGB(0x4dffffff) // .3
	ColDivider     = ARGB(0x1affffff)
	ColNone        = color.NRGBA{}
)

func RGB(c uint32) color.NRGBA {
	return color.NRGBA{R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c), A: 255}
}

func ARGB(c uint32) color.NRGBA {
	return color.NRGBA{A: uint8(c >> 24), R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c)}
}

// Alpha returns c with its alpha multiplied by a (0..1).
func Alpha(c color.NRGBA, a float32) color.NRGBA {
	c.A = uint8(float32(c.A) * clamp01(a))
	return c
}

// Mix blends a toward b by t (0..1).
func Mix(a, b color.NRGBA, t float32) color.NRGBA {
	t = clamp01(t)
	l := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*t + .5) }
	return color.NRGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), l(a.A, b.A)}
}

// Scale multiplies the RGB channels by f (darkens for f < 1).
func Scale(c color.NRGBA, f float32) color.NRGBA {
	s := func(x uint8) uint8 { return uint8(min(255, float32(x)*f+.5)) }
	return color.NRGBA{s(c.R), s(c.G), s(c.B), c.A}
}

// Hex parses "#rrggbb" (or "rrggbb"); bad input gives the fallback gray.
func Hex(s string) color.NRGBA {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 8 { // rrggbbaa
		s = s[:6]
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil || len(s) != 6 {
		return RGB(0x535353)
	}
	return RGB(uint32(n))
}

func HexString(c color.NRGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

func clamp01(v float32) float32 { return max(0, min(1, v)) }

//go:embed fonts/*.ttf
var fontFS embed.FS

// Font weights used by the official client.
const (
	Regular   = font.Normal
	Medium    = font.Medium
	SemiBold  = font.SemiBold
	Bold      = font.Bold
	ExtraBold = font.ExtraBold
	Black     = font.Black
)

var Shaper = newShaper()

func newShaper() *text.Shaper {
	var faces []font.FontFace
	for name, w := range map[string]font.Weight{
		"Regular": Regular, "Medium": Medium, "SemiBold": SemiBold, "Bold": Bold, "ExtraBold": ExtraBold, "Black": Black,
	} {
		data, err := fontFS.ReadFile("fonts/Figtree-" + name + ".ttf")
		if err != nil {
			panic(err)
		}
		face, err := opentype.Parse(data)
		if err != nil {
			panic(err)
		}
		faces = append(faces, font.FontFace{Font: font.Font{Typeface: "Figtree", Weight: w}, Face: face})
	}
	// System fonts stay loaded as fallbacks for other scripts and emoji.
	return text.NewShaper(text.WithCollection(faces))
}

// Dp converts dp to pixels.
func Dp(gtx layout.Context, v float32) int { return gtx.Dp(unit.Dp(v)) }

// Fill paints a w×h rectangle with rounded corners (radius in px) at the origin.
func Fill(gtx layout.Context, c color.NRGBA, w, h, radius int) {
	FillRect(gtx.Ops, image.Rect(0, 0, w, h), radius, c)
}

// FillRect paints r with rounded corners (radius in px).
func FillRect(ops *op.Ops, r image.Rectangle, radius int, c color.NRGBA) {
	if c.A == 0 || r.Empty() {
		return
	}
	defer clip.UniformRRect(r, radius).Push(ops).Pop()
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
}

// FillCircle paints a circle of diameter d at the origin.
func FillCircle(ops *op.Ops, d int, c color.NRGBA) {
	defer clip.Ellipse{Max: image.Pt(d, d)}.Push(ops).Pop()
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
}

// VGradient paints r with a vertical gradient from top to bottom.
func VGradient(ops *op.Ops, r image.Rectangle, top, bottom color.NRGBA) {
	if r.Empty() {
		return
	}
	defer clip.Rect(r).Push(ops).Pop()
	paint.LinearGradientOp{
		Stop1: f32.Pt(0, float32(r.Min.Y)), Color1: top,
		Stop2: f32.Pt(0, float32(r.Max.Y)), Color2: bottom,
	}.Add(ops)
	paint.PaintOp{}.Add(ops)
}

// Shadow approximates a CSS box-shadow (no blur in Gio) with stacked
// translucent rounded rects around r, offset down by dy.
func Shadow(ops *op.Ops, r image.Rectangle, radius, spread, dy int, alpha float32) {
	const steps = 4
	for i := steps; i >= 1; i-- {
		g := spread * i / steps
		rr := r.Inset(-g).Add(image.Pt(0, dy))
		FillRect(ops, rr, radius+g, color.NRGBA{A: uint8(255 * alpha / steps)})
	}
}

// Hline draws a 1dp horizontal divider across w.
func Hline(gtx layout.Context, w int, c color.NRGBA) layout.Dimensions {
	h := max(1, Dp(gtx, 1))
	FillRect(gtx.Ops, image.Rect(0, 0, w, h), 0, c)
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// Gap is a fixed-size empty widget (dp).
func Gap(w, h float32) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(Dp(gtx, w), Dp(gtx, h))}
	}
}

// Pad lays out w with CSS-style padding in dp (top, right, bottom, left).
func Pad(gtx layout.Context, t, r, b, l float32, w layout.Widget) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(t), Right: unit.Dp(r), Bottom: unit.Dp(b), Left: unit.Dp(l)}.Layout(gtx, w)
}

// Offset runs f translated by (x, y) px.
func Offset(gtx layout.Context, x, y int, f func()) {
	defer op.Offset(image.Pt(x, y)).Push(gtx.Ops).Pop()
	f()
}

// strokeRRect outlines r (rounded by radius px) with a line width px wide.
func strokeRRect(ops *op.Ops, r image.Rectangle, radius, width int, c color.NRGBA) {
	in := r.Inset(width / 2)
	rr := clip.UniformRRect(in, max(0, radius-width/2))
	defer clip.Stroke{Path: rr.Path(ops), Width: float32(width)}.Op().Push(ops).Pop()
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
}

// Unavailable is shown when one section's API call fails: the rest of the app
// keeps working, and the details are in the log.
func Unavailable(gtx layout.Context, what string, err error) layout.Dimensions {
	if err != nil {
		Log.Warn("section unavailable", "what", what, "err", err)
	}
	return Pad(gtx, 96, 24, 96, 24, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Label{Text: what + " isn't available right now", Size: 24, Weight: Bold, Align: text.Middle, MaxLines: 2}.Layout(gtx)
			}),
			layout.Rigid(Gap(0, 8)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Label{Text: "Spotify's API returned something we couldn't read. Check the log for details.", Color: ColText2, Align: text.Middle, MaxLines: 3}.Layout(gtx)
			}),
		)
	})
}

// Loading fills the area with a centered spinner.
func Loading(gtx layout.Context) layout.Dimensions {
	return Pad(gtx, 96, 0, 96, 0, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return Spinner(gtx, 32) })
	})
}
