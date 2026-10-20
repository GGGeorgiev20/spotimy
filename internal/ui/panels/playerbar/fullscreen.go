package playerbar

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

var fs struct {
	tr   transport
	vol  volume
	exit ui.IconButton
	// Background color of the cover shown (dominant color; Gio has no blur).
	coverURL string
	color    color.NRGBA
	ctxURI   string
	ctxName  ui.Res[string]
	title    ui.Link
	artists  []ui.Link
}

// layoutFullScreen draws full screen mode and reports whether it's active.
func layoutFullScreen(gtx layout.Context) bool {
	if !ui.IsFullscreen() {
		return false
	}
	dp := dpf(gtx)
	size := gtx.Constraints.Max
	t := ui.NowTrack()
	if fs.exit.Click.Clicked(gtx) {
		ui.SetFullscreen(false)
	}

	col := ui.RGB(0x535353)
	if t != nil {
		if url := ui.Pick(t.Images, 64); url != fs.coverURL {
			fs.coverURL, fs.color = url, col
			ui.CoverColor(t.Images, func(c color.NRGBA) { fs.color = c })
		}
		col = fs.color
	}
	ui.VGradient(gtx.Ops, image.Rectangle{Max: size}, col, ui.Scale(col, .3))

	pad := dp(48)
	// Playing from <context>.
	if name := contextName(); name != "" {
		ui.Offset(gtx, pad, dp(40), func() {
			g := gtx
			g.Constraints.Max.X = size.X - 2*pad
			ui.Inline(g, 14, ui.Regular, ui.ColText2, ui.ColNone,
				ui.Seg{Text: "Playing from "}, ui.Seg{Text: name, Weight: ui.Bold, Color: ui.ColText})
		})
	}

	// Bottom up: buttons, progress bar, cover with title and artists.
	ctlH := dp(48)
	ctlY := size.Y - dp(40) - ctlH
	progY := ctlY - dp(16) - dp(15)
	g := gtx
	g.Constraints = layout.Exact(image.Pt(size.X-2*pad, ctlH))
	ui.Offset(gtx, pad, ctlY, func() {
		fs.tr.controls(g, 48)
		right := dp(32) + dp(120) + dp(8) + dp(32)
		ui.Offset(g, g.Constraints.Max.X-right, (ctlH-dp(32))/2, func() {
			fs.vol.layout(g, dp(120))
			ui.Offset(g, dp(32+120+8), 0, func() {
				d := fs.exit.Layout(g, ui.IconStyle{Icon: "fullscreen-exit", Label: "Exit full screen"})
				ui.Tip(g, &fs.exit.Click, d, "Exit full screen", false)
			})
		})
	})
	ui.Offset(gtx, pad, progY, func() { fs.tr.progress(g, 12) })

	if t == nil {
		return true
	}
	cs := max(dp(120), min(dp(300), size.Y*28/100))
	bottom := progY - dp(32)
	ui.Offset(gtx, pad, bottom-cs, func() { ui.Cover(gtx, t.Images, cs, dp(8), false, "note") })
	for len(fs.artists) < len(t.Artists) {
		fs.artists = append(fs.artists, ui.Link{})
	}
	if fs.title.Click.Clicked(gtx) && t.AlbumURI != "" {
		ui.SetFullscreen(false)
		ui.OpenURI(t.AlbumURI)
	}
	for i, a := range t.Artists {
		if fs.artists[i].Click.Clicked(gtx) && a.URI != "" {
			ui.SetFullscreen(false)
			ui.OpenURI(a.URI)
		}
	}
	x := pad + cs + dp(24)
	var md layout.Dimensions
	m := op.Record(gtx.Ops)
	{
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(size.X-x-pad, size.Y)}
		md = layout.Flex{Axis: layout.Vertical}.Layout(g,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fs.title.Layout(gtx, t.Name, 32, ui.Bold, ui.ColText, ui.ColNone)
			}),
			layout.Rigid(ui.Gap(0, 4)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				segs := make([]ui.Seg, 0, 2*len(t.Artists))
				for i, a := range t.Artists {
					if i > 0 {
						segs = append(segs, ui.Seg{Text: ", "})
					}
					segs = append(segs, ui.Seg{Text: a.Name, Link: &fs.artists[i]})
				}
				return ui.Inline(gtx, 16, ui.Regular, ui.ColText2, ui.ColText, segs...)
			}),
		)
	}
	call := m.Stop()
	ui.Offset(gtx, x, bottom-md.Size.Y, func() { call.Add(gtx.Ops) })
	return true
}

// contextName is the title of the playing context (album, playlist...),
// fetched when the context changes.
func contextName() string {
	s := ui.Player.Status
	if s == nil || s.ContextURI == "" {
		return ""
	}
	if ui.IsContext("spotify:collection:tracks") {
		return "Liked Songs"
	}
	if s.ContextURI != fs.ctxURI {
		uri := s.ContextURI
		fs.ctxURI = uri
		fs.ctxName.Load(func() (string, error) {
			switch ui.URIType(uri) {
			case "album":
				a, err := ui.B.Album(uri)
				if err != nil {
					return "", err
				}
				return a.Name, nil
			case "playlist":
				p, err := ui.B.Playlist(uri, 0, 1)
				if err != nil {
					return "", err
				}
				return p.Name, nil
			case "artist":
				a, err := ui.B.Artist(uri)
				if err != nil {
					return "", err
				}
				return a.Name, nil
			}
			return "", nil
		})
	}
	if !fs.ctxName.Ok() {
		return ""
	}
	return fs.ctxName.V
}
