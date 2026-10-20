// Package devices is the "Connect to a device" right panel.
package devices

import (
	"image"
	"image/color"

	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() { ui.RightPanels["devices"] = layoutPanel }

var p struct {
	list    ui.Res[[]spotify.Device]
	version int
	header  ui.PanelHeader
	cur     ui.Click
	rows    []ui.Click
}

func layoutPanel(gtx layout.Context) layout.Dimensions {
	// The device list lives in the Connect state; re-read it whenever playback changes.
	if p.version != ui.Player.Version {
		p.version = ui.Player.Version
		p.list.Load(ui.B.Devices)
	} else {
		p.list.Once(ui.B.Devices)
	}
	var cur *spotify.Device
	var others []spotify.Device
	for _, d := range p.list.V {
		if cur == nil && d.Active {
			cur = &d
		}
	}
	for _, d := range p.list.V {
		if cur == nil && d.IsPlayer {
			cur = &d
		}
		if cur == nil || d.ID != cur.ID {
			others = append(others, d)
		}
	}
	for len(p.rows) < len(others) {
		p.rows = append(p.rows, ui.Click{})
	}

	dp := func(v float32) int { return ui.Dp(gtx, v) }
	// row is a 48dp device row: icon, 12dp gap, name; padded 20dp, with a
	// hover highlight (the current device sits on a card instead).
	row := func(gtx layout.Context, k *ui.Click, icon layout.Widget, label string, col color.NRGBA) layout.Dimensions {
		w, h := gtx.Constraints.Max.X, dp(48)
		return k.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if k.Hovered() {
				ui.FillRect(gtx.Ops, image.Rect(0, 0, w, h), dp(6), ui.ColSurface3)
			}
			ui.Offset(gtx, dp(20), (h-dp(24))/2, func() { icon(gtx) })
			ui.Offset(gtx, dp(20+24+12), 0, func() {
				g := gtx
				g.Constraints = layout.Exact(image.Pt(w-dp(20+24+12+20), h))
				layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions {
					return ui.Text(gtx, label, 16, ui.Regular, col)
				})
			})
			return layout.Dimensions{Size: image.Pt(w, h)}
		})
	}
	var rows []layout.Widget
	rows = append(rows, func(gtx layout.Context) layout.Dimensions {
		return p.header.Layout(gtx, "Close", func(gtx layout.Context) layout.Dimensions {
			return ui.Text(gtx, "Connect", 16, ui.Bold, ui.ColText)
		}, nil)
	})
	if cur != nil {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return ui.Pad(gtx, 0, 8, 24, 8, func(gtx layout.Context) layout.Dimensions {
				w := gtx.Constraints.Max.X
				ui.FillRect(gtx.Ops, image.Rect(0, 0, w, dp(64)), dp(8), ui.ColSurface2)
				ui.Offset(gtx, 0, dp(8), func() {
					row(gtx, &p.cur, func(gtx layout.Context) layout.Dimensions {
						if ui.Playing() {
							return ui.Equalizer(gtx, dp(24))
						}
						return ui.Icon(gtx, icon(cur.Type), 24, ui.ColAccent)
					}, name(*cur), ui.ColAccent)
				})
				return layout.Dimensions{Size: image.Pt(w, dp(64))}
			})
		})
	}
	if len(others) > 0 || p.list.Done || p.list.Err != nil {
		heading := "Select another device"
		if len(others) == 0 {
			heading = "No other devices found"
		}
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return ui.Pad(gtx, 0, 20, 0, 20, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, heading, 16, ui.Bold, ui.ColText)
			})
		})
	}
	if len(others) == 0 && (p.list.Done || p.list.Err != nil) {
		for _, t := range tips {
			rows = append(rows, func(gtx layout.Context) layout.Dimensions {
				return ui.Pad(gtx, 24, 20, 0, 20, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.Icon(gtx, t.icon, 24, ui.ColText) }),
						layout.Rigid(ui.Gap(12, 0)),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return ui.Column(gtx,
								func(gtx layout.Context) layout.Dimensions {
									return ui.Label{Text: t.title, Size: 16, LineHeight: 22}.Layout(gtx)
								},
								ui.Gap(0, 2),
								func(gtx layout.Context) layout.Dimensions {
									return ui.Label{Text: t.text, Size: 14, Color: ui.ColText2, MaxLines: 3, LineHeight: 20}.Layout(gtx)
								},
							)
						}),
					)
				})
			})
		}
	}
	for i, d := range others {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			k := &p.rows[i]
			if k.Clicked(gtx) {
				ui.Go("Couldn't switch to "+name(d), func() error { return ui.B.TransferPlayback(d.ID) }, ui.RefreshPlayer)
			}
			k.Label = name(d)
			return ui.Pad(gtx, 0, 8, 0, 8, func(gtx layout.Context) layout.Dimensions {
				return row(gtx, k, func(gtx layout.Context) layout.Dimensions { return ui.Icon(gtx, icon(d.Type), 24, ui.ColText) }, name(d), ui.ColText)
			})
		})
	}
	return ui.Column(gtx, rows...)
}

// tips are shown while no other device is found.
var tips = []struct{ icon, title, text string }{
	{"devices", "Check your WiFi", "Connect the devices you’re using to the same WiFi."},
	{"volume", "Play from another device", "It will automatically appear here."},
}

func name(d spotify.Device) string {
	if d.IsPlayer {
		return "This computer"
	}
	return d.Name
}

// icon picks a glyph for the device type.
func icon(t string) string {
	if t == "SPEAKER" {
		return "volume"
	}
	return "devices"
}
