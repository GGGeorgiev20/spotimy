package ui

import (
	"embed"
	"math"
	"fmt"
	"image"
	"image/color"
	"os"
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
	ColRowHover    = Alpha(ColText, .1) // hsla(0,0%,100%,.1)
	ColRowSelected = Alpha(ColText, .3)
	ColDivider     = Alpha(ColText, .1)
	ColNone        = color.NRGBA{}
)

func RGB(c uint32) color.NRGBA {
	return color.NRGBA{R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c), A: 255}
}

func ARGB(c uint32) color.NRGBA {
	return color.NRGBA{A: uint8(c >> 24), R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c)}
}

// Alpha returns c with its alpha multiplied by a (0..1), as the browser would
// show it. Gio blends in linear light, where a translucent white over a dark
// panel comes out much brighter than in the browser's sRGB blending (10%
// white over #121212 gives #5c5c5c instead of #2a2a2a), so the alpha is
// corrected for the backdrop these colors are usually drawn on: the dark
// panels for light colors, mid tones (artwork, gradients) for dark ones.
func Alpha(c color.NRGBA, a float32) color.NRGBA {
	a = clamp01(a) * float32(c.A) / 255
	fg := (float64(c.R) + float64(c.G) + float64(c.B)) / 3 / 255
	bg := 0.07 // #121212
	if fg < 0.5 {
		bg = 0.35
	}
	want := toLinear(bg + float64(a)*(fg-bg)) // the browser's sRGB blend, in linear light
	lb, lf := toLinear(bg), toLinear(fg)
	if lf != lb {
		a = float32(max(0, min(1, (want-lb)/(lf-lb))))
	}
	c.A = uint8(a*255 + .5)
	return c
}

func toLinear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
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
	// System fonts stay loaded as fallbacks for other scripts. Go-text's
	// system scan skips color bitmap fonts, so emoji need their font added.
	for _, p := range []string{
		"/usr/share/fonts/truetype/NotoColorEmoji.ttf",
		"/usr/share/fonts/noto/NotoColorEmoji.ttf",
		"/usr/share/fonts/google-noto-emoji/NotoColorEmoji.ttf",
	} {
		if data, err := os.ReadFile(p); err == nil {
			if face, err := opentype.Parse(data); err == nil {
				faces = append(faces, font.FontFace{Font: font.Font{Typeface: "Noto Color Emoji"}, Face: face})
				break
			}
		}
	}
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

// Shadow draws a CSS-like box-shadow under r: a black rounded rect blurred
// by blur px, moved down by dy, at the given opacity. Masks are rendered once
// per size and cached.
func Shadow(ops *op.Ops, r image.Rectangle, radius, blur, dy int, alpha float32) {
	if r.Empty() || alpha <= 0 {
		return
	}
	blur = max(1, blur)
	k := shadowKey{r.Dx(), r.Dy(), radius, blur}
	img, ok := shadows[k]
	if !ok {
		if len(shadows) > 128 {
			clear(shadows)
		}
		img = paint.NewImageOp(shadowMask(k))
		shadows[k] = img
	}
	defer op.Offset(r.Min.Add(image.Pt(-blur, dy-blur))).Push(ops).Pop()
	defer paint.PushOpacity(ops, clamp01(alpha)).Pop()
	img.Add(ops)
	paint.PaintOp{}.Add(ops)
}

type shadowKey struct{ w, h, r, blur int }

var shadows = map[shadowKey]paint.ImageOp{}

// shadowMask renders a rounded rect inset by blur into an alpha mask and
// blurs it with three box blurs (close to a Gaussian). The alpha is shaped
// for Gio's linear blending so the falloff looks like the browser's.
func shadowMask(k shadowKey) *image.NRGBA {
	w, h := k.w+2*k.blur, k.h+2*k.blur
	a := make([]float32, w*h)
	rr := float64(min(k.r, k.w/2, k.h/2))
	for y := 0; y < k.h; y++ {
		for x := 0; x < k.w; x++ {
			// Distance outside the rounded corner (0 inside).
			cx, cy := float64(x)+.5, float64(y)+.5
			dx := max(rr-cx, cx-(float64(k.w)-rr), 0)
			dy := max(rr-cy, cy-(float64(k.h)-rr), 0)
			if dx*dx+dy*dy <= rr*rr {
				a[(y+k.blur)*w+x+k.blur] = 1
			}
		}
	}
	box := max(1, k.blur/3)
	tmp := make([]float32, len(a))
	for range 3 {
		boxBlur(a, tmp, w, h, box, 1, w)
		boxBlur(tmp, a, h, w, box, w, 1)
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i, v := range a {
		// Browser-like falloff under linear blending over mid tones.
		img.Pix[i*4+3] = uint8(255*(1-math.Pow(1-float64(v), 2.2)) + .5)
	}
	return img
}

// boxBlur averages src along rows of length n (stride step within a row,
// rows rowStride apart) into dst, with a window of 2r+1.
func boxBlur(src, dst []float32, n, rows, r, step, rowStride int) {
	inv := 1 / float32(2*r+1)
	for row := 0; row < rows; row++ {
		base := row * rowStride
		var sum float32
		at := func(i int) float32 {
			if i < 0 || i >= n {
				return 0
			}
			return src[base+i*step]
		}
		for i := -r; i <= r; i++ {
			sum += at(i)
		}
		for i := 0; i < n; i++ {
			dst[base+i*step] = sum * inv
			sum += at(i+r+1) - at(i-r)
		}
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
