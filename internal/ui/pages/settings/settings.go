// Package settings is the Settings page (/preferences), opened from the
// profile menu: the desktop client's sections, with the settings this app
// can honor.
package settings

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

func init() {
	ui.Register("preferences", func([]string) ui.Page {
		p := &page{}
		p.cache.Load(cacheSize)
		return p
	})
}

func cacheSize() (int64, error) { return ui.B.CacheSize(), nil }

const loginMethodsURL = "https://www.spotify.com/account/login-methods/"

type page struct {
	list   ui.List
	search ui.IconButton
	query  ui.Input
	open   bool // the search field is shown

	edit, clear ui.Click
	cache       ui.Res[int64]
	sw          map[string]*ui.Switch
	sel         map[string]*ui.Select
	fade        ui.Slider
}

// row is one setting: its label (and description) on the left, its control
// in the right column.
type row struct {
	label, sub string
	control    func(gtx layout.Context, w int) layout.Dimensions
}

type section struct {
	title string
	rows  []row
}

func (p *page) toggle(key, label string, on bool, set func(bool)) row {
	if p.sw == nil {
		p.sw = map[string]*ui.Switch{}
	}
	s := p.sw[key]
	if s == nil {
		s = &ui.Switch{}
		p.sw[key] = s
	}
	return row{label: label, control: func(gtx layout.Context, w int) layout.Dimensions {
		m := recordRight(gtx, w, func(gtx layout.Context) layout.Dimensions {
			d, clicked := s.Layout(gtx, on, label)
			if clicked {
				set(!on)
			}
			return d
		})
		return m
	}}
}

func (p *page) choice(key, label string, opts []ui.Option, cur string, set func(string)) row {
	if p.sel == nil {
		p.sel = map[string]*ui.Select{}
	}
	s := p.sel[key]
	if s == nil {
		s = &ui.Select{}
		p.sel[key] = s
	}
	return row{label: label, control: func(gtx layout.Context, w int) layout.Dimensions {
		d, v, picked := s.Layout(gtx, w, opts, cur)
		if picked && v != cur {
			set(v)
		}
		return d
	}}
}

// recordRight lays w out right-aligned in a column w px wide.
func recordRight(gtx layout.Context, colW int, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := w(gtx)
	call := m.Stop()
	ui.Offset(gtx, colW-d.Size.X, 0, func() { call.Add(gtx.Ops) })
	return layout.Dimensions{Size: image.Pt(colW, d.Size.Y)}
}

// save stores the settings after a change.
func save() { ui.SaveSettings() }

func (p *page) sections() []section {
	s := &ui.Settings
	var out []section
	out = append(out, section{"Account", []row{{label: "Edit login methods", control: func(gtx layout.Context, w int) layout.Dimensions {
		if p.edit.Clicked(gtx) {
			backend.OpenURL(loginMethodsURL)
		}
		return recordRight(gtx, w, func(gtx layout.Context) layout.Dimensions { return outlineButton(gtx, &p.edit, "Edit", true) })
	}}}})
	out = append(out, section{"Language", []row{p.choice("lang", "Choose language - Changes will be applied after restarting the app", languages, langValue(s.Language), func(v string) {
		s.Language = v
		save()
		ui.Toast("The language changes when you restart the app")
	})}})
	out = append(out, section{"Audio quality", []row{p.choice("quality", "Streaming quality", qualities, s.Quality, func(v string) {
		s.Quality = v
		save()
	})}})
	out = append(out, section{"Your Library", []row{p.toggle("compact", "Use compact library layout", ui.Prefs.LibView == "compact", func(on bool) {
		ui.Prefs.LibView = "list"
		if on {
			ui.Prefs.LibView = "compact"
		}
	})}})
	out = append(out, section{"Display", []row{
		p.toggle("npv", "Show the now-playing panel on click of play", s.NPVOnPlay, func(on bool) { s.NPVOnPlay = on; save() }),
		p.toggle("friends", "See what your friends are playing", s.FriendFeed, func(on bool) {
			s.FriendFeed = on
			if !on && ui.Prefs.RightPanel == "friends" {
				ui.Prefs.RightPanel = ""
			}
			save()
		}),
	}})
	canvas := p.toggle("canvas", "Canvas", s.Canvas, func(on bool) { s.Canvas = on; save() })
	canvas.sub = "Short, looping visuals when a song is playing."
	out = append(out, section{"Videos and Canvas", []row{canvas}})
	zoomRow := p.choice("zoom", "Adjusting the zoom level can help you make the most of Spotify and adapt it to your preferences or needs.", zooms, s.Zoom, func(v string) {
		s.Zoom = v
		save()
	})
	out = append(out, section{"Zoom level", []row{zoomRow}})

	play := []row{p.toggle("crossfade", "Crossfade songs", s.Crossfade, func(on bool) { s.Crossfade = on; save() })}
	if s.Crossfade {
		play = append(play, row{label: "", control: p.crossfadeSlider})
	}
	play = append(play,
		p.toggle("mono", "Mono audio - Makes the left and right speakers play the same audio", s.Mono, func(on bool) { s.Mono = on; save() }),
		p.toggle("normalize", "Normalize volume - Set the same volume level for all songs and podcasts", s.Normalize, func(on bool) { s.Normalize = on; save() }),
	)
	if s.Normalize {
		play = append(play, p.choice("level", "Volume level - Adjust the volume for your environment. Loud may diminish audio quality. No effect on audio quality in Normal or Quiet.", levels, s.VolumeLevel, func(v string) {
			s.VolumeLevel = v
			save()
		}))
	}
	autoplay := p.toggle("autoplay", "Autoplay", s.Autoplay, func(on bool) { s.Autoplay = on; save() })
	autoplay.sub = "Enjoy nonstop listening. When your audio ends, we’ll play you something similar"
	play = append(play, autoplay)
	out = append(out, section{"Playback", play})

	out = append(out, section{"Startup and window behaviour", []row{p.choice("autostart", "Open Spotify automatically after you log into the computer", autostarts, s.Autostart, func(v string) {
		s.Autostart = v
		save()
	})}})

	size := ""
	if p.cache.Ok() {
		size = " " + fmtBytes(p.cache.V)
	}
	out = append(out, section{"Storage", []row{{label: "Cache:" + size, sub: "Temporary files that Spotify stores for a faster experience on slow networks", control: func(gtx layout.Context, w int) layout.Dimensions {
		if p.clear.Clicked(gtx) {
			ui.Confirm("Clear cache?", "Any temporary files that Spotify has saved to this device will be removed.", "Clear cache", func() {
				ui.Go("Sorry, we were unable to clear your cache", ui.B.ClearCache, func() {
					ui.Toast("Your cache has been cleared")
					p.cache.Load(cacheSize)
				})
			})
		}
		return recordRight(gtx, w, func(gtx layout.Context) layout.Dimensions { return outlineButton(gtx, &p.clear, "Clear cache", false) })
	}}}})
	return out
}

// crossfadeSlider is the crossfade length, 1 to 12 seconds.
func (p *page) crossfadeSlider(gtx layout.Context, w int) layout.Dimensions {
	s := &ui.Settings
	sec := max(1, min(12, s.CrossfadeS))
	if p.fade.Dragging() || p.fade.Released() {
		sec = 1 + int(p.fade.Value()*11+.5)
	}
	if p.fade.Released() && sec != s.CrossfadeS {
		s.CrossfadeS = sec
		save()
	}
	p.fade.Label = "Crossfade"
	h := ui.Dp(gtx, 24)
	lw := ui.Dp(gtx, 36)
	ui.Offset(gtx, 0, (h-ui.Dp(gtx, 20))/2, func() { ui.Text(gtx, fmt.Sprintf("%d s", sec), 14, ui.Regular, ui.ColText2) })
	ui.Offset(gtx, lw, (h-ui.Dp(gtx, ui.SliderHeight))/2, func() {
		g := gtx
		g.Constraints.Max.X = max(0, w-2*lw)
		p.fade.Layout(g, float32(sec-1)/11, true)
	})
	ui.Offset(gtx, w-lw+ui.Dp(gtx, 8), (h-ui.Dp(gtx, 20))/2, func() { ui.Text(gtx, "12 s", 14, ui.Regular, ui.ColText2) })
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func (p *page) Layout(gtx layout.Context) layout.Dimensions {
	if p.search.Click.Clicked(gtx) {
		p.open = !p.open
		if p.open {
			p.query.Focus(gtx)
		} else {
			p.query.SetText("")
		}
	}
	p.query.Poll(gtx)
	q := strings.ToLower(strings.TrimSpace(p.query.Text()))

	var secs []section
	for _, s := range p.sections() {
		if q != "" && !strings.Contains(strings.ToLower(s.title), q) {
			kept := s.rows[:0:0]
			for _, r := range s.rows {
				if strings.Contains(strings.ToLower(r.label+" "+r.sub), q) {
					kept = append(kept, r)
				}
			}
			s.rows = kept
		}
		if len(s.rows) > 0 {
			secs = append(secs, s)
		}
	}

	w := gtx.Constraints.Max.X
	pad := ui.Dp(gtx, 32)
	colW := min(ui.Dp(gtx, 900), w) - 2*pad
	x0 := (min(ui.Dp(gtx, 900), w)-colW)/2 + max(0, (w-ui.Dp(gtx, 900))/2)
	items := []layout.Widget{func(gtx layout.Context) layout.Dimensions { return p.header(gtx, colW) }}
	for _, s := range secs {
		items = append(items, func(gtx layout.Context) layout.Dimensions { return layoutSection(gtx, s, colW) })
	}
	if len(secs) == 0 {
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return ui.Label{Text: "No results found for “" + strings.TrimSpace(p.query.Text()) + "”", Size: 16, Weight: ui.Bold, Align: text.Start}.Layout(gtx)
		})
	}
	return p.list.Layout(gtx, len(items), func(gtx layout.Context, i int) layout.Dimensions {
		top, bottom := ui.Dp(gtx, 24), 0
		if i == 0 {
			top = pad
		}
		if i == len(items)-1 {
			bottom = pad
		}
		var d layout.Dimensions
		ui.Offset(gtx, x0, top, func() {
			g := gtx
			g.Constraints = layout.Constraints{Max: image.Pt(colW, 1<<20)}
			d = items[i](g)
		})
		return layout.Dimensions{Size: image.Pt(w, top+d.Size.Y+bottom)}
	})
}

// header is the "Settings" title and the search button (and field).
func (p *page) header(gtx layout.Context, w int) layout.Dimensions {
	d := ui.Text(gtx, "Settings", 24, ui.Bold, ui.ColText)
	b := ui.Dp(gtx, 32)
	if p.open {
		fw := min(ui.Dp(gtx, 220), w/2)
		ui.Offset(gtx, w-b-ui.Dp(gtx, 8)-fw, (d.Size.Y-b)/2, func() {
			ui.FillRect(gtx.Ops, image.Rect(0, 0, fw, b), ui.Dp(gtx, 4), ui.Alpha(ui.ColText, .1))
			ui.Offset(gtx, ui.Dp(gtx, 12), (b-ui.Dp(gtx, 20))/2, func() {
				g := gtx
				g.Constraints = layout.Constraints{Min: image.Pt(fw-ui.Dp(gtx, 24), 0), Max: image.Pt(fw-ui.Dp(gtx, 24), ui.Dp(gtx, 20))}
				p.query.Layout(g, ui.InputStyle{Hint: "Search in Settings"})
			})
		})
	}
	ui.Offset(gtx, w-b, (d.Size.Y-b)/2, func() {
		if p.search.Click.Hovered() || p.open {
			ui.FillCircle(gtx.Ops, b, ui.Alpha(ui.ColText, .1))
		}
		p.search.Layout(gtx, ui.IconStyle{Icon: "search", Label: "Search in Settings"})
	})
	return layout.Dimensions{Size: image.Pt(w, d.Size.Y+ui.Dp(gtx, 16))}
}

// layoutSection is the bold title over the rows, 8dp apart. Rows put the
// label in the first two thirds and the control in the last, 24dp between.
func layoutSection(gtx layout.Context, s section, w int) layout.Dimensions {
	gap := ui.Dp(gtx, 24)
	labelW := (w - gap) * 2 / 3
	ctlW := w - gap - labelW
	y := ui.Text(gtx, s.title, 16, ui.Bold, ui.ColText).Size.Y
	for _, r := range s.rows {
		y += ui.Dp(gtx, 8)
		// The control first, to center the label on it.
		m := op.Record(gtx.Ops)
		var cd layout.Dimensions
		ui.Offset(gtx, labelW+gap, 0, func() {
			g := gtx
			g.Constraints = layout.Constraints{Max: image.Pt(ctlW, 1<<20)}
			cd = r.control(g, ctlW)
		})
		ctl := m.Stop()
		m = op.Record(gtx.Ops)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(labelW, 1<<20)}
		ld := labelBlock(g, r)
		lbl := m.Stop()
		h := max(cd.Size.Y, ld.Size.Y) + ui.Dp(gtx, 4)
		ui.Offset(gtx, 0, y+(h-ld.Size.Y)/2, func() { lbl.Add(gtx.Ops) })
		ui.Offset(gtx, 0, y+(h-cd.Size.Y)/2, func() { ctl.Add(gtx.Ops) })
		y += h
	}
	return layout.Dimensions{Size: image.Pt(w, y)}
}

// labelBlock is a row's label, with its description under it when it has one.
func labelBlock(gtx layout.Context, r row) layout.Dimensions {
	if r.sub == "" {
		return ui.Label{Text: r.label, Size: 14, Color: ui.ColText2, MaxLines: -1}.Layout(gtx)
	}
	return ui.Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return ui.Label{Text: r.label, Size: 14, Color: ui.ColText, MaxLines: -1}.Layout(gtx)
		},
		ui.Gap(0, 2),
		func(gtx layout.Context) layout.Dimensions {
			return ui.Label{Text: r.sub, Size: 13, Color: ui.ColText2, MaxLines: -1}.Layout(gtx)
		},
	)
}

// outlineButton is the small outlined pill ("Edit" with a link-out arrow).
func outlineButton(gtx layout.Context, c *ui.Click, label string, external bool) layout.Dimensions {
	c.Label = label
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		h, pad, is := ui.Dp(gtx, 32), ui.Dp(gtx, 16), ui.Dp(gtx, 16)
		tw := ui.Measure(gtx, label, 14, ui.Bold)
		w := pad + tw + pad
		if external {
			w += ui.Dp(gtx, 4) + is
		}
		border := ui.RGB(0x7c7c7c)
		if c.Hovered() {
			border = ui.ColText
		}
		ui.StrokeRRect(gtx.Ops, image.Rect(0, 0, w, h), h/2, max(1, ui.Dp(gtx, 1)), border)
		ui.Offset(gtx, pad, (h-ui.Dp(gtx, 20))/2, func() { ui.Text(gtx, label, 14, ui.Bold, ui.ColText) })
		if external {
			ui.Offset(gtx, pad+tw+ui.Dp(gtx, 4), (h-is)/2, func() { ui.IconPx(gtx, "link-out", is, ui.ColText) })
		}
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}

// fmtBytes is a size like the desktop client's ("245.3 MB").
func fmtBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

var qualities = []ui.Option{{Value: "automatic", Label: "Automatic"}, {Value: "normal", Label: "Normal"}, {Value: "high", Label: "High"}, {Value: "very-high", Label: "Very high"}}

var levels = []ui.Option{{Value: "loud", Label: "Loud"}, {Value: "normal", Label: "Normal"}, {Value: "quiet", Label: "Quiet"}}

var zooms = []ui.Option{{Value: "dense", Label: "Dense"}, {Value: "default", Label: "Default"}, {Value: "spacious", Label: "Spacious"}}

var autostarts = []ui.Option{{Value: "off", Label: "No"}, {Value: "on", Label: "Yes"}, {Value: "minimized", Label: "Minimized"}}

// langValue is the option shown for a language setting ("" is English, the
// API's default).
func langValue(v string) string {
	if v == "" {
		return "en"
	}
	return v
}
