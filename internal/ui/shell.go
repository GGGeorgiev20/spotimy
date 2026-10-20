package ui

import (
	"image"
	"math"
	"net/url"
	"os"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
	"github.com/GGGeorgiev20/personal-spotify/internal/player"
)

// Slots filled by the panel packages in init.
var (
	// LibraryPanel draws Your Library; collapsed is the 72dp icon rail.
	LibraryPanel func(gtx layout.Context, collapsed bool) layout.Dimensions
	// PlayerBar draws the bottom bar (PlayerBarHeight dp tall).
	PlayerBar func(gtx layout.Context) layout.Dimensions
	// RightPanels by Prefs.RightPanel name: queue, npv, friends, devices.
	RightPanels = map[string]func(gtx layout.Context) layout.Dimensions{}
	// FullScreen, when it returns true, has drawn the whole window itself.
	FullScreen func(gtx layout.Context) bool
	// SearchQuery is the top bar's search text (the search page reads it).
	SearchQuery Input
)

const (
	PlayerBarHeight = 72
	// RemoteBarHeight is the green "Listening on <device>" strip under the
	// player bar while another device plays; the player bar draws it itself.
	RemoteBarHeight = 28
)

// window is the native window (nil in the harness).
var (
	window     *app.Window
	fullscreen bool
)

// IsFullscreen reports full screen mode (the FullScreen slot draws the window).
func IsFullscreen() bool { return fullscreen }

// SetFullscreen enters or leaves full screen mode, also making the native
// window full screen (a no-op in the harness).
func SetFullscreen(on bool) {
	fullscreen = on
	if w := window; w != nil {
		opt := app.Windowed.Option()
		if on {
			opt = app.Fullscreen.Option()
		}
		go w.Option(opt)
	}
}

// playerBarHeight is the player bar's height in px, with the remote strip.
func playerBarHeight(gtx layout.Context) int {
	h := Dp(gtx, PlayerBarHeight)
	if s := Player.Status; s != nil && s.Remote {
		h += Dp(gtx, RemoteBarHeight)
	}
	return h
}

var appState = backend.AppState{State: player.StateConnecting}

// Init wires the backend's callbacks; call before Run or RunHarness.
func Init(b *backend.App, configDir string) {
	B = b
	loadPrefs(configDir)
}

// OnState and OnPlayer are the backend callbacks (any goroutine).
func OnState(s backend.AppState) {
	Post(func() {
		was := appState.State
		appState = s
		if s.State == player.StateReady && was != player.StateReady {
			loadMe()
			RefreshPlayer()
		}
	})
}

func OnPlayer() { RefreshPlayer() }

// Run opens the window and runs the UI until it's closed. It must run on a
// goroutine other than main (app.Main owns the main thread).
func Run() {
	w := new(app.Window)
	window = w
	w.Option(
		app.Title("Spotify"),
		app.Size(unit.Dp(1280), unit.Dp(800)),
		app.MinSize(unit.Dp(800), unit.Dp(600)),
	)
	inv := w.Invalidate
	invalidate.Store(&inv)
	// Re-sync playback occasionally so drift and missed events can't pile up.
	go func() {
		for range time.Tick(15 * time.Second) {
			RefreshPlayer()
		}
	}()
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			if e.Err != nil {
				Log.Error("window", "err", e.Err)
			}
			return
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			Frame(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

var rootTag = new(int)

// Frame lays out the whole window.
func Frame(gtx layout.Context) {
	runPosted()
	clickDepth = 0
	win := gtx.Constraints.Max
	paint.Fill(gtx.Ops, ColBg)
	// Everything is laid out inside the root area so its handler sees every
	// pointer event (ancestors receive events too) and can track the position.
	area := clip.Rect{Max: win}.Push(gtx.Ops)
	defer area.Pop()
	trackPointer(gtx, rootTag)
	shortcuts(gtx)
	inputFocused = false
	defer func() { inputFocusedLast = inputFocused }()

	if appState.State != player.StateReady {
		layoutLogin(gtx)
	} else if FullScreen == nil || !FullScreen(gtx) {
		layoutApp(gtx)
	}
	layoutToasts(gtx, Dp(gtx, 108))
	layoutModal(gtx)
	layoutMenus(gtx)
	layoutHoverTip(gtx)
	flushClipboard(gtx)
	savePrefs()
}

var (
	resizer  gesture.Drag
	resizeX  float32
	resizeW  int
	resizeOn bool
)

func layoutApp(gtx layout.Context) {
	win := gtx.Constraints.Max
	top, bar := Dp(gtx, 64), playerBarHeight(gtx)
	midH := win.Y - top - bar

	// Top bar.
	g := gtx
	g.Constraints = layout.Exact(image.Pt(win.X, top))
	layoutTopBar(g)

	// Middle row: library | resizer | main | right panel.
	pad, gap := Dp(gtx, 8), Dp(gtx, 8)
	libW := Dp(gtx, float32(Prefs.LibWidth))
	if Prefs.LibCollapsed {
		libW = Dp(gtx, 72)
	}
	rightW := 0
	panel := RightPanels[Prefs.RightPanel]
	if panel != nil {
		rightW = max(Dp(gtx, 280), min(Dp(gtx, 420), win.X/4))
	}
	mainW := win.X - 2*pad - libW - gap
	if panel != nil {
		mainW -= gap + rightW
	}
	if mainW < Dp(gtx, 300) && !Prefs.LibCollapsed { // too narrow: squeeze the library first
		libW = max(Dp(gtx, 72), libW-(Dp(gtx, 300)-mainW))
		mainW = win.X - 2*pad - libW - gap
		if panel != nil {
			mainW -= gap + rightW
		}
	}
	panelBox := func(x, w int, f func(gtx layout.Context) layout.Dimensions) {
		Offset(gtx, x, top, func() {
			g := gtx
			g.Constraints = layout.Exact(image.Pt(w, midH))
			defer clip.UniformRRect(image.Rect(0, 0, w, midH), Dp(gtx, 8)).Push(gtx.Ops).Pop()
			paint.Fill(gtx.Ops, ColSurface)
			f(g)
		})
	}
	x := pad
	if LibraryPanel != nil {
		panelBox(x, libW, func(gtx layout.Context) layout.Dimensions { return LibraryPanel(gtx, Prefs.LibCollapsed) })
	}
	x += libW
	layoutResizer(gtx, x, top, gap, midH)
	x += gap
	panelBox(x, mainW, layoutMain)
	x += mainW
	if panel != nil {
		x += gap
		panelBox(x, rightW, panel)
	}

	// Player bar.
	if PlayerBar != nil {
		Offset(gtx, 0, win.Y-bar, func() {
			g := gtx
			g.Constraints = layout.Exact(image.Pt(win.X, bar))
			PlayerBar(g)
		})
	}
}

// layoutResizer is the gap between the library and the main view: drag it to
// resize the library; dragging far left collapses it.
func layoutResizer(gtx layout.Context, x, y, w, h int) {
	const minW, maxW, collapsed = 280, 600, 72
	for {
		e, ok := resizer.Update(gtx.Metric, gtx.Source, gesture.Horizontal)
		if !ok {
			break
		}
		switch e.Kind {
		case pointer.Press:
			resizeOn, resizeX = true, e.Position.X
			resizeW = Prefs.LibWidth
			if Prefs.LibCollapsed {
				resizeW = collapsed
			}
		case pointer.Drag:
			nw := resizeW + int((e.Position.X-resizeX)/gtx.Metric.PxPerDp)
			if nw < minW-80 {
				Prefs.LibCollapsed = true
			} else {
				Prefs.LibCollapsed = false
				Prefs.LibWidth = max(minW, min(maxW, nw))
			}
		case pointer.Release, pointer.Cancel:
			resizeOn = false
		}
	}
	Offset(gtx, x, y, func() {
		r := image.Rect(0, 0, w, h)
		st := clip.Rect(r).Push(gtx.Ops)
		resizer.Add(gtx.Ops)
		pointer.CursorColResize.Add(gtx.Ops)
		hover := resizeOn || r.Add(image.Pt(x, y)).Overlaps(image.Rectangle{Min: PointerPos(), Max: PointerPos().Add(image.Pt(1, 1))})
		st.Pop()
		if hover { // a 1dp line, 8dp short of both ends; white while dragging
			lw, c := max(1, Dp(gtx, 1)), ColText3
			if resizeOn {
				c = ColText
			}
			FillRect(gtx.Ops, image.Rect((w-lw)/2, Dp(gtx, 8), (w+lw)/2, h-Dp(gtx, 8)), lw/2, c)
		}
	})
}

// ---- Main view ----

var stickyPlay PlayButton

func layoutMain(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	mainWidth = size.X
	page := Current().Page()
	d := page.Layout(gtx)

	cp, ok := page.(ChromePage)
	if !ok {
		return d
	}
	ch := cp.Chrome()
	if ch.List == nil {
		return d
	}
	h := Dp(gtx, 64)
	o := float32(ch.List.ScrollTop()-ch.Threshold+h) / float32(h)
	if !ch.List.Scrolled(0) {
		o = 0
	}
	o = clamp01(o)
	if o <= 0 {
		return d
	}
	barW := size.X - Dp(gtx, 12)
	bg := ch.Color
	FillRect(gtx.Ops, image.Rect(0, 0, barW, h), 0, Alpha(bg, o))
	// Block clicks to the list under the bar.
	st := clip.Rect{Max: image.Pt(barW, h)}.Push(gtx.Ops)
	event.Op(gtx.Ops, &stickyPlay.Click.Label)
	st.Pop()
	if stickyPlay.Click.Clicked(gtx) && o >= 1 && ch.OnPlay != nil {
		ch.OnPlay()
	}
	m := op.Record(gtx.Ops)
	x := Dp(gtx, 16)
	if ch.OnPlay != nil {
		Offset(gtx, x, (h-Dp(gtx, 48))/2, func() {
			stickyPlay.Layout(gtx, 48, CtxPlaying(ch.ContextURI))
		})
		x += Dp(gtx, 48+8)
	}
	Offset(gtx, x, 0, func() {
		g := gtx
		g.Constraints = layout.Exact(image.Pt(max(0, barW-x-Dp(gtx, 16)), h))
		layout.W.Layout(g, func(gtx layout.Context) layout.Dimensions {
			return Text(gtx, ch.Title, 24, Bold, ColText)
		})
	})
	call := m.Stop()
	fade := paint.PushOpacity(gtx.Ops, o)
	call.Add(gtx.Ops)
	fade.Pop()
	return d
}

// ---- Top bar ----

var (
	backBtn, fwdBtn, homeBtn, friendsBtn IconButton
	avatarBtn, clearSearch, browseBtn    Click
	searchBox                            Click
	searchTimer                          *time.Timer
	searchFocused                        bool
)

var bellBtn IconButton

// centerBlock places a w-wide block centered in a win-wide bar, or, when that
// would overlap what is left of left or right of right, against left with
// whatever width remains.
func centerBlock(win, w, left, right int) (x, bw int) {
	if x = (win - w) / 2; x < left || x+w > right {
		return left, min(w, right-left)
	}
	return x, w
}

func layoutTopBar(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	h := size.Y

	if backBtn.Click.Clicked(gtx) {
		Back()
	}
	if fwdBtn.Click.Clicked(gtx) {
		Forward()
	}
	if homeBtn.Click.Clicked(gtx) {
		Navigate("/")
	}
	if friendsBtn.Click.Clicked(gtx) {
		TogglePanel("friends")
	}
	if bellBtn.Click.Clicked(gtx) {
		Navigate("/content-feed")
	}
	if avatarBtn.Clicked(gtx) {
		x := size.X - Dp(gtx, 8) - Dp(gtx, 196)
		OpenMenuAt(image.Pt(x, Dp(gtx, 56)), []MenuItem{
			{Label: "Account", External: true, Action: B.OpenAccountPage},
			{Label: "Profile", Action: func() {
				if Me != nil {
					Navigate("/user/" + Me.Username)
				}
			}},
			{Divider: true},
			{Label: "Log out", Action: func() {
				Go("Couldn't log out", B.Logout, func() { Me = nil })
			}},
		})
	}
	if clearSearch.Clicked(gtx) {
		SearchQuery.SetText("")
		searchChanged()
	}
	if browseBtn.Clicked(gtx) {
		Navigate("/search")
	}
	if searchBox.Clicked(gtx) {
		SearchQuery.Focus(gtx)
	}

	// Left: back / forward.
	Offset(gtx, Dp(gtx, 16), (h-Dp(gtx, 32))/2, func() {
		navCircle(gtx, &backBtn, "chevron-left", "Go back", !CanBack())
		Offset(gtx, Dp(gtx, 40), 0, func() { navCircle(gtx, &fwdBtn, "chevron-right", "Go forward", !CanForward()) })
	})

	// Center: home + search, a 546dp block (8 + 48 + 8 + 474 + 8) that stays
	// centered while it fits between the nav buttons and the right group, and
	// otherwise sits next to the nav buttons and shrinks.
	bx, blockW := centerBlock(size.X, Dp(gtx, 546), Dp(gtx, 96), size.X-Dp(gtx, 104))
	searchW := blockW - Dp(gtx, 72)
	cx := bx + Dp(gtx, 8)
	home := Current().Path == "/"
	Offset(gtx, cx, (h-Dp(gtx, 48))/2, func() {
		homeBtn.Click.Label = "Home"
		d := homeBtn.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			s := Dp(gtx, 48)
			if homeBtn.Click.Hovered() {
				s = s * 104 / 100
			}
			o := (Dp(gtx, 48) - s) / 2
			bg := ColSurface2
			if homeBtn.Click.Hovered() {
				bg = ColSurface3
			}
			Offset(gtx, o, o, func() { FillCircle(gtx.Ops, s, bg) })
			name, c := "home", ColText2
			if home {
				name, c = "home-filled", ColText
			}
			if homeBtn.Click.Hovered() {
				c = ColText
			}
			is := Dp(gtx, 24)
			Offset(gtx, (Dp(gtx, 48)-is)/2, (Dp(gtx, 48)-is)/2, func() { IconPx(gtx, name, is, c) })
			return layout.Dimensions{Size: image.Pt(Dp(gtx, 48), Dp(gtx, 48))}
		})
		Tip(gtx, &homeBtn.Click, d, "Home", true)
		Offset(gtx, Dp(gtx, 56), 0, func() { layoutSearchBox(gtx, searchW) })
	})

	// Right: friends + avatar.
	ax := size.X - Dp(gtx, 8) - Dp(gtx, 48)
	Offset(gtx, ax, (h-Dp(gtx, 48))/2, func() {
		avatarBtn.Label = "Profile"
		d := avatarBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			s := Dp(gtx, 48)
			if avatarBtn.Hovered() {
				s = s * 104 / 100
			}
			o := (Dp(gtx, 48) - s) / 2
			bg := ColSurface2
			if avatarBtn.Hovered() {
				bg = ColSurface3
			}
			Offset(gtx, o, o, func() { FillCircle(gtx.Ops, s, bg) })
			is := Dp(gtx, 32)
			Offset(gtx, (Dp(gtx, 48)-is)/2, (Dp(gtx, 48)-is)/2, func() { Avatar(gtx, is) })
			return layout.Dimensions{Size: image.Pt(Dp(gtx, 48), Dp(gtx, 48))}
		})
		name := ""
		if Me != nil {
			name = Me.Name
		}
		Tip(gtx, &avatarBtn, d, name, true)
	})
	Offset(gtx, ax-Dp(gtx, 8)-Dp(gtx, 32), (h-Dp(gtx, 32))/2, func() {
		d := friendsBtn.Layout(gtx, IconStyle{Icon: "friends", Label: "Friend Activity", Active: Prefs.RightPanel == "friends"})
		Tip(gtx, &friendsBtn.Click, d, "Friend Activity", true)
	})
	Offset(gtx, ax-2*Dp(gtx, 8)-2*Dp(gtx, 32), (h-Dp(gtx, 32))/2, func() {
		d := bellBtn.Layout(gtx, IconStyle{Icon: "bell", Label: "What's New", Active: Current().Path == "/content-feed"})
		Tip(gtx, &bellBtn.Click, d, "What's New", true)
	})
	return layout.Dimensions{Size: size}
}

// Avatar draws the logged-in user's picture (or initial on their color).
func Avatar(gtx layout.Context, px int) layout.Dimensions {
	if Me != nil && len(Me.Images) > 0 {
		return Image{URL: Pick(Me.Images, px), Round: true}.Layout(gtx, image.Pt(px, px))
	}
	bg := RGB(0xff6437)
	initial := ""
	if Me != nil {
		bg = Hex(Me.Color)
		if Me.Name != "" {
			initial = strings.ToUpper(string([]rune(Me.Name)[:1]))
		}
	}
	FillCircle(gtx.Ops, px, bg)
	g := gtx
	g.Constraints = layout.Exact(image.Pt(px, px))
	layout.Center.Layout(g, func(gtx layout.Context) layout.Dimensions {
		return Text(gtx, initial, float32(px)*14/32/gtx.Metric.PxPerSp, Bold, ColBg)
	})
	return layout.Dimensions{Size: image.Pt(px, px)}
}

func navCircle(gtx layout.Context, b *IconButton, icon, label string, disabled bool) {
	b.Click.Label, b.Click.Disabled = label, disabled
	d := b.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		s := Dp(gtx, 32)
		FillCircle(gtx.Ops, s, Alpha(ColBg, .7))
		c := ColText
		if disabled {
			c = Alpha(ColText2, .6)
		}
		is := Dp(gtx, 16)
		Offset(gtx, (s-is)/2, (s-is)/2, func() { IconPx(gtx, icon, is, c) })
		return layout.Dimensions{Size: image.Pt(s, s)}
	})
	Tip(gtx, &b.Click, d, label, true)
}

func layoutSearchBox(gtx layout.Context, w int) {
	h := Dp(gtx, 48)
	changed, _ := SearchQuery.Poll(gtx)
	focused := gtx.Focused(&SearchQuery.Editor)
	if focused && !searchFocused && !strings.HasPrefix(Current().Path, "/search") {
		q := strings.TrimSpace(SearchQuery.Text())
		if q != "" {
			Navigate("/search/" + url.PathEscape(q))
		} else {
			Navigate("/search")
		}
	}
	searchFocused = focused
	// Like the web player, the box empties when you leave search (going back
	// to a search page puts its query back).
	if !focused && SearchQuery.Len() > 0 && !strings.HasPrefix(Current().Path, "/search") {
		SearchQuery.SetText("")
	}
	if changed && focused { // programmatic SetText must not navigate
		searchChanged()
	}
	for {
		e, ok := gtx.Event(key.Filter{Focus: &SearchQuery.Editor, Name: key.NameEscape})
		if !ok {
			break
		}
		if ke, ok := e.(key.Event); ok && ke.State == key.Press {
			SearchQuery.Blur(gtx)
		}
	}
	searchBox.Cursor = pointer.CursorText
	searchBox.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := ColSurface2
		if searchBox.Hovered() || focused {
			bg = ColSurface3
		}
		r := image.Rect(0, 0, w, h)
		FillRect(gtx.Ops, r, h/2, bg)
		switch {
		case focused:
			strokeRRect(gtx.Ops, r, h/2, Dp(gtx, 2), ColText)
		case searchBox.Hovered():
			strokeRRect(gtx.Ops, r, h/2, max(1, Dp(gtx, 1)), Alpha(ColText, .1))
		}
		ic := ColText2
		if focused {
			ic = ColText
		}
		is := Dp(gtx, 24)
		Offset(gtx, Dp(gtx, 12), (h-is)/2, func() { IconPx(gtx, "search", is, ic) })
		return layout.Dimensions{Size: r.Max}
	})
	// Trailing: [clear] | [browse], a 1dp divider and the Browse button.
	icon := func(c *Click, label, name string, x, size int) {
		Offset(gtx, x, (h-Dp(gtx, 24))/2, func() {
			c.Label = label
			d := c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				col, s := ColText2, size
				if c.Hovered() {
					col, s = ColText, s*104/100
				}
				Offset(gtx, (Dp(gtx, 24)-s)/2, (Dp(gtx, 24)-s)/2, func() { IconPx(gtx, name, s, col) })
				return layout.Dimensions{Size: image.Pt(Dp(gtx, 24), Dp(gtx, 24))}
			})
			Tip(gtx, c, d, label, true)
		})
	}
	if SearchQuery.Len() > 0 {
		icon(&clearSearch, "Clear search field", "close", w-Dp(gtx, 89), Dp(gtx, 26))
	}
	dh := Dp(gtx, 25)
	FillRect(gtx.Ops, image.Rect(w-Dp(gtx, 53), (h-dh)/2, w-Dp(gtx, 53)+max(1, Dp(gtx, 1)), (h+dh)/2), 0, ColText3)
	icon(&browseBtn, "Browse", "browse", w-Dp(gtx, 40), Dp(gtx, 24))
	right := Dp(gtx, 96)
	x := Dp(gtx, 12+24+12)
	Offset(gtx, x, (h-Dp(gtx, 22))/2, func() {
		g := gtx
		g.Constraints = layout.Constraints{Min: image.Pt(w-x-right, 0), Max: image.Pt(w-x-right, Dp(gtx, 24))}
		SearchQuery.Layout(g, InputStyle{Hint: "What do you want to play?", Size: 16})
	})
}

// routeQuery is the query of a /search/<query>[/<tab>] path ("" for others).
func routeQuery(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[0] != "search" {
		return ""
	}
	q, _ := url.PathUnescape(parts[1])
	return q
}

// searchChanged updates the /search route 250ms after typing stops; it
// replaces the history entry so each keystroke isn't a "back" step.
func searchChanged() {
	if searchTimer != nil {
		searchTimer.Stop()
	}
	searchTimer = time.AfterFunc(250*time.Millisecond, func() {
		Post(func() {
			q := strings.TrimSpace(SearchQuery.Text())
			if cur := Current().Path; strings.HasPrefix(cur, "/search") && routeQuery(cur) == q {
				return // set programmatically from the route (or typing came back to it)
			}
			path := "/search"
			if q != "" {
				path += "/" + url.PathEscape(q)
			}
			if strings.HasPrefix(Current().Path, "/search") {
				Replace(path)
			} else {
				Navigate(path)
			}
		})
	})
}

// ---- Login ----

var loginBtn Pill

func layoutLogin(gtx layout.Context) {
	win := gtx.Constraints.Max
	VGradient(gtx.Ops, image.Rectangle{Max: win}, RGB(0x2a2a2a), ColBg)
	if loginBtn.Click.Clicked(gtx) {
		B.OpenLoginURL()
	}
	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, Dp(gtx, 420))
		switch appState.State {
		case player.StateLogin:
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Text(gtx, "Log in to Spotify", 32, Bold, ColText)
				}),
				layout.Rigid(Gap(0, 16)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Label{Text: "Finish logging in on the Spotify page that opened in your browser.", Color: ColText2, MaxLines: 3, Align: 1}.Layout(gtx)
				}),
				layout.Rigid(Gap(0, 24)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return loginBtn.Layout(gtx, PillStyle{Text: "Open login page again", Weight: Bold, Height: 48, PadX: 32, Bg: ColAccent, BgHover: ColAccentHover, Fg: ColBg})
				}),
			)
		case player.StateError:
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Text(gtx, "Can't connect", 32, Bold, ColText)
				}),
				layout.Rigid(Gap(0, 16)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Label{Text: appState.Error, Size: 13, Color: RGB(0xf3727f), MaxLines: 6, Align: 1}.Layout(gtx)
				}),
				layout.Rigid(Gap(0, 8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Text(gtx, "Retrying…", 14, Regular, ColText2)
				}),
			)
		}
		return Spinner(gtx, 40)
	})
}

// Spinner draws the loading ring (dp).
func Spinner(gtx layout.Context, size float32) layout.Dimensions {
	s := Dp(gtx, size)
	w := float32(Dp(gtx, 4))
	r := float32(s)/2 - w/2
	c := f32.Pt(float32(s)/2, float32(s)/2)
	ring := func(from, sweep float64, col [4]uint8) {
		var p clip.Path
		p.Begin(gtx.Ops)
		start := f32.Pt(c.X+r*float32(math.Cos(from)), c.Y+r*float32(math.Sin(from)))
		p.MoveTo(start)
		p.ArcTo(c, c, float32(sweep))
		st := clip.Stroke{Path: p.End(), Width: w}.Op().Push(gtx.Ops)
		paintAll(gtx.Ops, RGB(uint32(col[0])<<16|uint32(col[1])<<8|uint32(col[2])))
		st.Pop()
	}
	ring(0, 2*math.Pi-0.001, [4]uint8{0x2a, 0x2a, 0x2a})
	a := float64(gtx.Now.UnixMilli()%1000) / 1000 * 2 * math.Pi
	ring(a, math.Pi/2, [4]uint8{0x1e, 0xd7, 0x60})
	gtx.Execute(op.InvalidateCmd{})
	return layout.Dimensions{Size: image.Pt(s, s)}
}

// ---- Shortcuts ----

var (
	inputFocused bool
)

func shortcuts(gtx layout.Context) {
	ctrl := key.ModShortcut
	f := []event.Filter{
		key.Filter{Name: key.NameSpace},
		key.Filter{Name: key.NameEscape},
		key.Filter{Name: key.NameRightArrow, Required: ctrl},
		key.Filter{Name: key.NameLeftArrow, Required: ctrl},
		key.Filter{Name: key.NameUpArrow, Required: ctrl},
		key.Filter{Name: key.NameDownArrow, Required: ctrl},
		key.Filter{Name: key.NameRightArrow, Required: key.ModShift},
		key.Filter{Name: key.NameLeftArrow, Required: key.ModShift},
		key.Filter{Name: key.NameRightArrow, Required: key.ModAlt},
		key.Filter{Name: key.NameLeftArrow, Required: key.ModAlt},
		key.Filter{Name: "M"},
		key.Filter{Name: "S", Required: ctrl},
		key.Filter{Name: "R", Required: ctrl},
		key.Filter{Name: "N", Required: ctrl},
		key.Filter{Name: "K", Required: ctrl},
		key.Filter{Name: "L", Required: ctrl},
		key.Filter{Name: "/", Required: ctrl, Optional: key.ModShift},
	}
	for _, n := range []key.Name{"B", "H", "S", "Q", "J", "R", "L"} {
		f = append(f, key.Filter{Name: n, Required: key.ModAlt | key.ModShift})
	}
	for {
		e, ok := gtx.Event(f...)
		if !ok {
			break
		}
		ke, ok := e.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		handleShortcut(gtx, ke)
	}
}

func handleShortcut(gtx layout.Context, e key.Event) {
	m := e.Modifiers
	ctrl := m.Contain(key.ModShortcut)
	alt, shift := m.Contain(key.ModAlt), m.Contain(key.ModShift)
	if e.Name == key.NameEscape {
		switch {
		case MenuIsOpen():
			CloseMenu()
		case ModalIsOpen():
			CloseModal()
		case fullscreen:
			SetFullscreen(false)
		}
		return
	}
	if MenuIsOpen() || ModalIsOpen() {
		return
	}
	if ctrl && e.Name == "/" {
		ShowShortcuts()
		return
	}
	if ctrl && (e.Name == "K" || e.Name == "L") {
		SearchQuery.Focus(gtx)
		return
	}
	if inputFocusedLast {
		return
	}
	s := Player.Status
	switch {
	case e.Name == key.NameSpace && m == 0:
		PlayPause()
	case ctrl && e.Name == key.NameRightArrow:
		Next()
	case ctrl && e.Name == key.NameLeftArrow:
		Prev()
	case ctrl && (e.Name == key.NameUpArrow || e.Name == key.NameDownArrow) && s != nil:
		step := s.VolumeMax / 10
		if e.Name == key.NameDownArrow {
			step = -step
		}
		SetVolume(max(0, min(s.VolumeMax, s.Volume+step)))
	case shift && !ctrl && !alt && (e.Name == key.NameRightArrow || e.Name == key.NameLeftArrow) && s != nil && s.Track != nil:
		d := int64(5000)
		if e.Name == key.NameLeftArrow {
			d = -d
		}
		Seek(max(0, min(s.Track.DurationMs, Position(time.Now()).Milliseconds()+d)))
	case e.Name == "M" && m == 0:
		ToggleMute()
	case ctrl && e.Name == "S":
		ToggleShuffle()
	case ctrl && e.Name == "R":
		CycleRepeat()
	case ctrl && e.Name == "N":
		CreatePlaylist(nil)
	case alt && !shift && e.Name == key.NameLeftArrow:
		Back()
	case alt && !shift && e.Name == key.NameRightArrow:
		Forward()
	case alt && shift:
		switch e.Name {
		case "B":
			if t := NowTrack(); t != nil {
				ToggleSaved(t.URI)
			}
		case "H":
			Navigate("/")
		case "S":
			Navigate("/collection/tracks")
		case "Q":
			Prefs.RightPanel = "queue"
		case "J":
			Prefs.RightPanel = "npv"
		case "R":
			TogglePanel("npv")
		case "L":
			Prefs.LibCollapsed = !Prefs.LibCollapsed
		}
	}
}

var inputFocusedLast bool

type shortcutsModal struct{}

var shortcutList = [][2]string{
	{"Play / pause", "Space"},
	{"Next track", "Ctrl + →"},
	{"Previous track", "Ctrl + ←"},
	{"Seek forward 5 s", "Shift + →"},
	{"Seek backward 5 s", "Shift + ←"},
	{"Volume up / down", "Ctrl + ↑ / ↓"},
	{"Mute / unmute", "M"},
	{"Shuffle", "Ctrl + S"},
	{"Repeat", "Ctrl + R"},
	{"Like / unlike song", "Alt + Shift + B"},
	{"Search", "Ctrl + K, Ctrl + L"},
	{"Go back / forward", "Alt + ← / →"},
	{"Create playlist", "Ctrl + N"},
	{"Home", "Alt + Shift + H"},
	{"Liked Songs", "Alt + Shift + S"},
	{"Queue", "Alt + Shift + Q"},
	{"Now Playing view", "Alt + Shift + J"},
	{"Toggle Now Playing sidebar", "Alt + Shift + R"},
	{"Toggle Your Library sidebar", "Alt + Shift + L"},
	{"Keyboard shortcuts", "Ctrl + /"},
}

func (shortcutsModal) Layout(gtx layout.Context) layout.Dimensions {
	rows := []layout.Widget{func(gtx layout.Context) layout.Dimensions {
		return Pad(gtx, 0, 0, 16, 0, func(gtx layout.Context) layout.Dimensions {
			return Text(gtx, "Keyboard shortcuts", 24, Bold, ColText)
		})
	}}
	for _, r := range shortcutList {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return Pad(gtx, 5, 0, 5, 0, func(gtx layout.Context) layout.Dimensions {
				w := gtx.Constraints.Max.X
				d := Text(gtx, r[0], 14, Regular, ColText)
				kw := Measure(gtx, r[1], 14, Regular)
				Offset(gtx, w-kw, 0, func() { Text(gtx, r[1], 14, Regular, ColText2) })
				return layout.Dimensions{Size: image.Pt(w, d.Size.Y)}
			})
		})
	}
	return Column(gtx, rows...)
}

// ShowShortcuts opens the keyboard shortcuts list.
func ShowShortcuts() { ShowModal(shortcutsModal{}, 560) }

// Exit quits the process (window closed).
func Exit() { os.Exit(0) }
