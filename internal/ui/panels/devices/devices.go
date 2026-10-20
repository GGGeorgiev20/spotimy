// Package devices is the "Connect to a device" right panel.
package devices

import (
	"image"

	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() { ui.RightPanels["devices"] = layoutPanel }

var p struct {
	list    ui.Res[[]spotify.Device]
	version int
	close   ui.IconButton
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
	if p.close.Click.Clicked(gtx) {
		ui.TogglePanel("devices")
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
	var rows []layout.Widget
	rows = append(rows, func(gtx layout.Context) layout.Dimensions {
		return ui.Pad(gtx, 8, 8, 8, 16, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.Text(gtx, "Connect to a device", 16, ui.Bold, ui.ColText)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					d := p.close.Layout(gtx, ui.IconStyle{Icon: "close", Label: "Close"})
					ui.Tip(gtx, &p.close.Click, d, "Close", true)
					return d
				}),
			)
		})
	})
	if cur != nil {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return ui.Pad(gtx, 8, 16, 16, 16, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if ui.Playing() {
							return ui.Equalizer(gtx, dp(24))
						}
						return ui.Icon(gtx, icon(cur.Type), 24, ui.ColAccent)
					}),
					layout.Rigid(ui.Gap(16, 0)),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.Text(gtx, "Current device", 16, ui.Bold, ui.ColText)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.Text(gtx, name(*cur), 14, ui.Regular, ui.ColAccent)
							}),
						)
					}),
				)
			})
		})
	}
	rows = append(rows, func(gtx layout.Context) layout.Dimensions {
		return ui.Pad(gtx, 8, 16, 8, 16, func(gtx layout.Context) layout.Dimensions {
			return ui.Text(gtx, "Select another device", 14, ui.Bold, ui.ColText)
		})
	})
	if len(others) == 0 && (p.list.Done || p.list.Err != nil) {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return ui.Pad(gtx, 0, 16, 16, 16, func(gtx layout.Context) layout.Dimensions {
				return ui.Label{Text: "No other devices found. Open Spotify on your phone or speaker on the same account.", Size: 14, Color: ui.ColText2, MaxLines: -1}.Layout(gtx)
			})
		})
	}
	for i, d := range others {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			k := &p.rows[i]
			if k.Clicked(gtx) {
				ui.Go("Couldn't switch to "+name(d), func() error { return ui.B.TransferPlayback(d.ID) }, ui.RefreshPlayer)
			}
			k.Label = name(d)
			return ui.Pad(gtx, 0, 8, 0, 8, func(gtx layout.Context) layout.Dimensions {
				return k.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					w, h := gtx.Constraints.Max.X, dp(48)
					if k.Hovered() {
						ui.FillRect(gtx.Ops, image.Rect(0, 0, w, h), dp(4), ui.ColRowHover)
					}
					ui.Offset(gtx, dp(8), (h-dp(24))/2, func() { ui.Icon(gtx, icon(d.Type), 24, ui.ColText2) })
					ui.Offset(gtx, dp(48), 0, func() {
						g := gtx
						g.Constraints = layout.Exact(image.Pt(w-dp(48+8), h))
						layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions {
							return ui.Text(gtx, name(d), 14, ui.Regular, ui.ColText)
						})
					})
					return layout.Dimensions{Size: image.Pt(w, h)}
				})
			})
		})
	}
	return ui.Column(gtx, rows...)
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
