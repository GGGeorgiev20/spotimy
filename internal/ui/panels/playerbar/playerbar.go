// Package playerbar is the bottom player bar (and full screen mode).
package playerbar

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() {
	ui.PlayerBar = Layout
	ui.FullScreen = layoutFullScreen
}

var bar struct {
	cover   ui.Click
	title   ui.Link
	artists []ui.Link
	save    ui.IconButton
	tr      transport
	vol     volume
	npv     ui.IconButton
	lyrics  ui.IconButton
	queue   ui.IconButton
	devices ui.IconButton
	full    ui.IconButton
	liked   string // track whose saved state was requested
}

// dpf converts dp to px for gtx.
func dpf(gtx layout.Context) func(float32) int {
	return func(v float32) int { return ui.Dp(gtx, v) }
}

// Layout draws the player bar into the whole constraint: an 8dp gap, the
// 72dp bar, then the "Listening on" strip when another device plays.
func Layout(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	dp := dpf(gtx)
	top, h := dp(8), dp(72)
	pad := dp(8)
	total := size.X - 2*pad
	side := max(dp(180), total*30/100)
	col := max(0, total-2*side)
	centerW := min(dp(722), col)
	ui.FillRect(gtx.Ops, image.Rectangle{Max: size}, 0, ui.ColBg)

	at := func(x, w int, f func(gtx layout.Context)) {
		ui.Offset(gtx, x, top, func() {
			g := gtx
			g.Constraints = layout.Exact(image.Pt(w, h))
			defer clip.Rect{Max: g.Constraints.Max}.Push(gtx.Ops).Pop()
			f(g)
		})
	}
	at(pad, side, layoutLeft)
	at(pad+side+(col-centerW)/2, centerW, func(gtx layout.Context) {
		ui.Offset(gtx, 0, dp(8), func() { bar.tr.layout(gtx, 32, 12) })
	})
	at(size.X-pad-side, side, layoutRight)

	if s := ui.Player.Status; s != nil && s.Remote {
		ui.Offset(gtx, 0, top+h, func() { layoutRemote(gtx, s.DeviceName) })
	}
	return layout.Dimensions{Size: size}
}

func layoutLeft(gtx layout.Context) {
	t := ui.NowTrack()
	if t == nil {
		return
	}
	dp := dpf(gtx)
	w, h := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
	if bar.liked != t.URI {
		bar.liked = t.URI
		ui.LoadSaved([]string{t.URI})
	}
	for len(bar.artists) < len(t.Artists) {
		bar.artists = append(bar.artists, ui.Link{})
	}
	if bar.cover.Clicked(gtx) {
		ui.TogglePanel("npv")
	}
	if bar.title.Click.Clicked(gtx) && t.AlbumURI != "" {
		ui.OpenURI(t.AlbumURI)
	}
	for i, a := range t.Artists {
		if bar.artists[i].Click.Clicked(gtx) && a.URI != "" {
			ui.OpenURI(a.URI)
		}
	}
	if bar.save.Click.Clicked(gtx) {
		ui.ToggleSaved(t.URI)
	}

	// Cover: opens the Now Playing view; a corner arrow shows on hover.
	cs := dp(56)
	ui.Offset(gtx, dp(8), (h-cs)/2, func() {
		bar.cover.Label = "Now playing view"
		d := bar.cover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			ui.Cover(gtx, t.Images, cs, dp(4), false, "note")
			if bar.cover.Hovered() {
				b := dp(24)
				ui.Offset(gtx, cs-b-dp(4), dp(4), func() {
					ui.FillCircle(gtx.Ops, b, ui.Alpha(ui.ColBg, .7))
					icon := "chevron-up"
					if ui.Prefs.RightPanel == "npv" {
						icon = "chevron-down"
					}
					ui.Offset(gtx, dp(4), dp(4), func() { ui.Icon(gtx, icon, 16, ui.ColText) })
				})
			}
			return layout.Dimensions{Size: image.Pt(cs, cs)}
		})
		// Left-aligned: a centered tooltip would run off the window edge.
		d.Size.X = ui.Measure(gtx, "Now playing view", 14, ui.Regular) + dp(16)
		ui.Tip(gtx, &bar.cover, d, "Now playing view", false)
	})

	// Title and artists, with the save button right after the longer line.
	x := dp(8 + 56 + 12)
	saveW := dp(32)
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(max(0, w-x-saveW-dp(16)), h)}
	md := layout.Flex{Axis: layout.Vertical}.Layout(g,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return bar.title.Layout(gtx, t.Name, 14, ui.Regular, ui.ColText, ui.ColNone)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			segs := make([]ui.Seg, 0, 2*len(t.Artists))
			for i, a := range t.Artists {
				if i > 0 {
					segs = append(segs, ui.Seg{Text: ", "})
				}
				segs = append(segs, ui.Seg{Text: a.Name, Link: &bar.artists[i]})
			}
			return ui.Inline(gtx, 11, ui.Regular, ui.ColText2, ui.ColText, segs...)
		}),
	)
	call := m.Stop()
	ui.Offset(gtx, x, (h-md.Size.Y)/2, func() { call.Add(gtx.Ops) })

	saved := ui.Saved[t.URI]
	icon, tip := "plus-circle", "Add to Liked Songs"
	if saved {
		icon, tip = "check-circle", "Remove from Liked Songs"
	}
	ui.Offset(gtx, x+md.Size.X+dp(16), (h-saveW)/2, func() {
		d := bar.save.Layout(gtx, ui.IconStyle{Icon: icon, Active: saved, Label: tip})
		ui.Tip(gtx, &bar.save.Click, d, tip, false)
	})
}

func layoutRight(gtx layout.Context) {
	dp := dpf(gtx)
	w, h := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
	s := ui.Player.Status
	t := ui.NowTrack()
	b := dp(32)
	onLyrics := ui.Current().Path == "/lyrics"

	if bar.npv.Click.Clicked(gtx) {
		ui.TogglePanel("npv")
	}
	if bar.lyrics.Click.Clicked(gtx) {
		if onLyrics {
			ui.Back()
		} else {
			ui.Navigate("/lyrics")
		}
	}
	if bar.queue.Click.Clicked(gtx) {
		ui.TogglePanel("queue")
	}
	if bar.devices.Click.Clicked(gtx) {
		ui.TogglePanel("devices")
	}
	if bar.full.Click.Clicked(gtx) {
		ui.SetFullscreen(true)
	}

	// Right-aligned row; the volume slider gives way on narrow windows.
	sw := min(dp(93), w-6*b-dp(8))
	if sw < dp(24) {
		sw = 0
	}
	rowW := 6*b + sw
	if sw > 0 {
		rowW += dp(8)
	}
	x := w - rowW
	ui.Offset(gtx, 0, (h-b)/2, func() {
		btn := func(i *ui.IconButton, icon, tip string, active, disabled bool) {
			ui.Offset(gtx, x, 0, func() {
				d := i.Layout(gtx, ui.IconStyle{Icon: icon, Active: active, Dot: true, Disabled: disabled, Label: tip})
				ui.Tip(gtx, &i.Click, d, tip, false)
			})
			x += b
		}
		btn(&bar.npv, "now-playing", "Now playing view", ui.Prefs.RightPanel == "npv", false)
		btn(&bar.lyrics, "lyrics", "Lyrics", onLyrics, t == nil)
		btn(&bar.queue, "queue", "Queue", ui.Prefs.RightPanel == "queue", false)
		btn(&bar.devices, "devices", "Connect to a device", ui.Prefs.RightPanel == "devices" || s != nil && s.Remote, false)
		ui.Offset(gtx, x, 0, func() { bar.vol.layout(gtx, sw) })
		x += b + sw
		if sw > 0 {
			x += dp(8)
		}
		btn(&bar.full, "fullscreen", "Full screen", false, false)
	})
}

// layoutRemote is the green strip shown while another device plays.
func layoutRemote(gtx layout.Context, device string) {
	dp := dpf(gtx)
	w, h := gtx.Constraints.Max.X-2*dp(8), dp(24)
	ui.Offset(gtx, dp(8), 0, func() {
		r := dp(4)
		defer clip.RRect{Rect: image.Rect(0, 0, w, h), SE: r, SW: r}.Push(gtx.Ops).Pop()
		ui.FillRect(gtx.Ops, image.Rect(0, 0, w, h), 0, ui.ColAccent)
		label := "Listening on " + device
		tw := ui.Measure(gtx, label, 14, ui.Bold)
		x := max(0, w-dp(32)-tw)
		g := gtx
		g.Constraints = layout.Exact(image.Pt(w-dp(32)-dp(22), h))
		ui.Offset(gtx, max(0, x-dp(22)), (h-dp(14))/2, func() { ui.Icon(gtx, "volume", 14, ui.ColBg) })
		ui.Offset(gtx, x, 0, func() {
			layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, label, 14, ui.Bold, ui.ColBg)
			})
		})
	})
}

// ---- Shared pieces (the bar and full screen mode each have their own) ----

// transport is the shuffle/previous/play/next/repeat row and the progress bar.
type transport struct {
	shuffle, prev, next, repeat ui.IconButton
	play                        ui.Click
	prog                        ui.Slider
}

// layout draws the buttons row (play button playSize dp) centered, then the
// progress row below it, filling the constraint width. It's 17dp+8dp+ the
// larger of 32 and playSize tall.
func (t *transport) layout(gtx layout.Context, playSize, timeSize float32) layout.Dimensions {
	d := t.controls(gtx, playSize)
	dp := dpf(gtx)
	ui.Offset(gtx, 0, d.Size.Y+dp(8), func() { t.progress(gtx, timeSize) })
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, d.Size.Y+dp(8+17))}
}

func (t *transport) controls(gtx layout.Context, playSize float32) layout.Dimensions {
	dp := dpf(gtx)
	s := ui.Player.Status
	has := ui.NowTrack() != nil
	b, ps := dp(32), dp(playSize)
	h := max(b, ps)
	repeat := ui.RepeatMode()

	if t.shuffle.Click.Clicked(gtx) {
		ui.ToggleShuffle()
	}
	if t.prev.Click.Clicked(gtx) {
		ui.Prev()
	}
	if t.play.Clicked(gtx) {
		ui.PlayPause()
	}
	if t.next.Click.Clicked(gtx) {
		ui.Next()
	}
	if t.repeat.Click.Clicked(gtx) {
		ui.CycleRepeat()
	}

	rowW := 4*b + ps + dp(8+16+16+8)
	x := (gtx.Constraints.Max.X - rowW) / 2
	place := func(w int, f func()) {
		ui.Offset(gtx, x, 0, f)
		x += w
	}
	btn := func(i *ui.IconButton, icon, tip string, active bool) {
		place(b, func() {
			ui.Offset(gtx, 0, (h-b)/2, func() {
				d := i.Layout(gtx, ui.IconStyle{Icon: icon, Size: playSize / 2, Active: active, Dot: true, Disabled: !has, Label: tip})
				ui.Tip(gtx, &i.Click, d, tip, false)
			})
		})
	}
	shuffleTip := "Enable shuffle"
	if s != nil && s.Shuffle {
		shuffleTip = "Disable shuffle"
	}
	repeatIcon, repeatTip := "repeat", "Enable repeat"
	switch repeat {
	case "context":
		repeatTip = "Enable repeat one"
	case "track":
		repeatIcon, repeatTip = "repeat-one", "Disable repeat"
	}
	btn(&t.shuffle, "shuffle", shuffleTip, s != nil && s.Shuffle)
	x += dp(8)
	btn(&t.prev, "prev", "Previous", false)
	x += dp(16)
	place(ps, func() {
		playing := ui.Playing()
		tip, icon := "Play", "play"
		if playing {
			tip, icon = "Pause", "pause"
		}
		t.play.Label, t.play.Disabled = tip, !has
		ui.Offset(gtx, 0, (h-ps)/2, func() {
			d := t.play.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				bg, draw := ui.ColText, ps
				switch {
				case !has:
					bg = ui.Alpha(bg, .3)
				case t.play.Hovered():
					draw = ps * 104 / 100
				}
				o := (ps - draw) / 2
				ui.Offset(gtx, o, o, func() { ui.FillCircle(gtx.Ops, draw, bg) })
				is := ps / 2
				ui.Offset(gtx, (ps-is)/2, (ps-is)/2, func() { ui.IconPx(gtx, icon, is, ui.ColBg) })
				return layout.Dimensions{Size: image.Pt(ps, ps)}
			})
			ui.Tip(gtx, &t.play, d, tip, false)
		})
	})
	x += dp(16)
	btn(&t.next, "next", "Next", false)
	x += dp(8)
	btn(&t.repeat, repeatIcon, repeatTip, repeat != "off")
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
}

// progress is "elapsed [slider] duration"; it redraws once per second while
// playing, at the second boundary.
func (t *transport) progress(gtx layout.Context, timeSize float32) layout.Dimensions {
	dp := dpf(gtx)
	w, h := gtx.Constraints.Max.X, dp(17)
	var dur int64
	if tr := ui.NowTrack(); tr != nil {
		dur = tr.DurationMs
	}
	pos := ui.Position(gtx.Now)
	ms := pos.Milliseconds()
	frac := float32(0)
	if dur > 0 {
		frac = float32(ms) / float32(dur)
	}
	if t.prog.Dragging() {
		ms = int64(t.prog.Value() * float32(dur))
	} else if ui.Playing() && !ui.Player.Status.Buffering && ms < dur {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second - pos%time.Second + 5*time.Millisecond)})
	}

	tw, gap := dp(50), dp(8)
	label := func(x int, a layout.Direction, s string) {
		g := gtx
		g.Constraints = layout.Exact(image.Pt(tw, h))
		ui.Offset(gtx, x, 0, func() {
			a.Layout(g, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, s, timeSize, ui.Regular, ui.ColText2)
			})
		})
	}
	label(0, layout.E, ui.FmtDuration(ms))
	label(w-tw, layout.W, ui.FmtDuration(dur))
	sh := dp(ui.SliderHeight)
	ui.Offset(gtx, tw+gap, (h-sh)/2, func() {
		g := gtx
		g.Constraints = layout.Exact(image.Pt(max(0, w-2*(tw+gap)), sh))
		t.prog.Label = "Change progress"
		t.prog.Layout(g, frac, dur > 0)
		if t.prog.Released() {
			ui.Seek(int64(t.prog.Value() * float32(dur)))
			gtx.Execute(op.InvalidateCmd{})
		}
	})
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// volume is the mute button and the volume slider.
type volume struct {
	mute ui.IconButton
	sl   ui.Slider
}

// layout draws the 32dp mute button and, after it, a sliderW px slider.
func (v *volume) layout(gtx layout.Context, sliderW int) layout.Dimensions {
	dp := dpf(gtx)
	s := ui.Player.Status
	vol, top := 0, 1
	if s != nil {
		vol, top = s.Volume, max(1, s.VolumeMax)
	}
	if v.mute.Click.Clicked(gtx) {
		ui.ToggleMute()
	}
	icon, tip := "volume", "Mute"
	switch {
	case vol == 0:
		icon, tip = "volume-off", "Unmute"
	case vol*2 < top:
		icon = "volume-low"
	}
	d := v.mute.Layout(gtx, ui.IconStyle{Icon: icon, Label: tip})
	ui.Tip(gtx, &v.mute.Click, d, tip, false)
	if sliderW > 0 {
		sh := dp(ui.SliderHeight)
		ui.Offset(gtx, d.Size.X, (d.Size.Y-sh)/2, func() {
			g := gtx
			g.Constraints = layout.Exact(image.Pt(sliderW, sh))
			v.sl.Label = "Volume"
			v.sl.Layout(g, float32(vol)/float32(top), s != nil)
			moved, released := v.sl.Moved(), v.sl.Released()
			if moved || released {
				ui.SetVolume(int(v.sl.Value()*float32(top) + .5))
			}
		})
	}
	return layout.Dimensions{Size: image.Pt(d.Size.X+sliderW, d.Size.Y)}
}
