package ui

import (
	"fmt"
	"image"
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// User actions shared by context menus, buttons and pages.

// PlaylistVersion bumps per playlist URI when its items change (pages reload).
var PlaylistVersion = map[string]int{}

var clipboardPending *string

// CopyText puts text on the clipboard (at the next frame).
func CopyText(s string) {
	clipboardPending = &s
	Invalidate()
}

func flushClipboard(gtx layout.Context) {
	if clipboardPending != nil {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(*clipboardPending))})
		clipboardPending = nil
	}
}

func CopyLink(uri string) {
	CopyText(ShareURL(uri))
	Toast("Link copied to clipboard")
}

func AddToQueue(uris []string) {
	Go("Couldn't add to queue", func() error { return B.AddToQueue(uris) }, func() {
		if len(uris) == 1 {
			Toast("Added to queue")
		} else {
			Toast(fmt.Sprintf("Added %d songs to queue", len(uris)))
		}
		RefreshPlayer()
	})
}

// QueueContext queues a whole album or playlist (a playlist's first 100 tracks).
func QueueContext(uri string) {
	Async(func() ([]string, error) {
		var tracks []spotify.Track
		switch URIType(uri) {
		case "album":
			a, err := B.Album(uri)
			if err != nil {
				return nil, err
			}
			tracks = a.Tracks
		case "playlist":
			p, err := B.Playlist(uri, 0, 100)
			if err != nil {
				return nil, err
			}
			tracks = p.Tracks.Items
		}
		var uris []string
		for _, t := range tracks {
			if t.Playable {
				uris = append(uris, t.URI)
			}
		}
		return uris, nil
	}, func(uris []string, err error) {
		if err != nil {
			Toast("Couldn't add to queue: " + err.Error())
			return
		}
		AddToQueue(uris)
	})
}

func AddToPlaylist(pl spotify.Card, uris []string) {
	Go("Couldn't add to playlist", func() error { return B.AddToPlaylist(pl.URI, uris) }, func() {
		PlaylistVersion[pl.URI]++
		Toast("Added to " + pl.Name)
	})
}

// CreatePlaylist makes "My Playlist #n" (with uris added) and opens it.
func CreatePlaylist(uris []string) {
	Async(func() (string, error) {
		n := 0
		if own, err := B.EditablePlaylists(nil); err == nil {
			n = len(own)
		}
		uri, err := B.CreatePlaylist(fmt.Sprintf("My Playlist #%d", n+1))
		if err != nil {
			return "", err
		}
		if len(uris) > 0 {
			if err := B.AddToPlaylist(uri, uris); err != nil {
				return uri, err
			}
		}
		return uri, nil
	}, func(uri string, err error) {
		if err != nil {
			Toast("Couldn't create playlist: " + err.Error())
		}
		if uri != "" {
			LibraryVersion++
			Navigate("/playlist/" + URIID(uri))
		}
	})
}

// EditPlaylist opens "Edit details" and saves the result.
func EditPlaylist(uri, name, description string, done func()) {
	EditDetails(name, description, func(n, d string) {
		Go("Couldn't save", func() error { return B.RenamePlaylist(uri, n, d) }, func() {
			LibraryVersion++
			PlaylistVersion[uri]++
			if done != nil {
				done()
			}
		})
	})
}

func DeletePlaylist(uri, name string) {
	Confirm("Delete from Your Library?", "This will delete "+name+" from Your Library.", "Delete", func() {
		Go("Couldn't delete", func() error { return B.DeletePlaylist(uri) }, func() {
			LibraryVersion++
			Toast("Removed from Your Library")
			if strings.HasSuffix(Current().Path, URIID(uri)) {
				Back()
			}
		})
	})
}

func SetPinned(uri string, pinned bool) {
	verb := "pin"
	if !pinned {
		verb = "unpin"
	}
	Go("Couldn't "+verb, func() error { return B.SetPinned(uri, pinned) }, func() { LibraryVersion++ })
}

// StartRadio plays the radio playlist for a seed and opens it.
func StartRadio(seed string) {
	Async(func() (string, error) { return B.StartRadio(seed) }, func(pl string, err error) {
		if err != nil {
			Toast("Couldn't start radio: " + err.Error())
			return
		}
		RefreshPlayer()
		if pl != "" {
			Navigate("/playlist/" + URIID(pl))
		}
	})
}

// ---- Context menus ----

// TrackMenuOpts adds context to a track's menu.
type TrackMenuOpts struct {
	PlaylistURI   string // the playlist the row is in
	CanEdit       bool   // ...and it's editable (offers "Remove from this playlist")
	RemoveFromQue func() // set for queue rows
}

// TrackMenu is the right-click menu of a song.
func TrackMenu(t spotify.Track, o TrackMenuOpts) []MenuItem {
	key := LikeKey(t)
	items := []MenuItem{{Label: "Add to playlist", Icon: "plus", Sub: playlistPicker([]string{t.URI})}}
	if o.CanEdit && o.PlaylistURI != "" && t.UID != "" {
		pl, uid := o.PlaylistURI, t.UID
		items = append(items, MenuItem{Label: "Remove from this playlist", Icon: "trash", Action: func() {
			Go("Couldn't remove", func() error { return B.RemoveFromPlaylist(pl, []string{uid}) }, func() {
				PlaylistVersion[pl]++
				Toast("Removed from playlist")
			})
		}})
	}
	if Saved[key] {
		items = append(items, MenuItem{Label: "Remove from your Liked Songs", Icon: "check-circle", Action: func() { ToggleSaved(key) }})
	} else {
		items = append(items, MenuItem{Label: "Save to your Liked Songs", Icon: "plus-circle", Action: func() { ToggleSaved(key) }})
	}
	if o.RemoveFromQue != nil {
		items = append(items, MenuItem{Label: "Remove from queue", Icon: "close", Action: o.RemoveFromQue})
	} else {
		items = append(items, MenuItem{Label: "Add to queue", Icon: "queue", Action: func() { AddToQueue([]string{t.URI}) }})
	}
	items = append(items, MenuItem{Divider: true})
	if !t.Episode {
		items = append(items, MenuItem{Label: "Go to song radio", Icon: "radio", Action: func() { StartRadio(t.URI) }})
	}
	switch len(t.Artists) {
	case 0:
	case 1:
		a := t.Artists[0].URI
		items = append(items, MenuItem{Label: "Go to artist", Icon: "artist", Action: func() { OpenURI(a) }})
	default:
		artists := t.Artists
		items = append(items, MenuItem{Label: "Go to artist", Icon: "artist", Sub: func() []MenuItem {
			var sub []MenuItem
			for _, a := range artists {
				uri := a.URI
				sub = append(sub, MenuItem{Label: a.Name, Action: func() { OpenURI(uri) }})
			}
			return sub
		}})
	}
	if t.Album != nil && t.Album.URI != "" {
		al := t.Album.URI
		items = append(items, MenuItem{Label: "Go to album", Icon: "album", Action: func() { OpenURI(al) }})
	}
	if !t.Episode {
		items = append(items, MenuItem{Label: "View credits", Icon: "info", Action: func() { ShowCredits(t.Name, t.URI) }})
	}
	items = append(items, MenuItem{Divider: true}, MenuItem{Label: "Share", Icon: "share", Sub: func() []MenuItem {
		return []MenuItem{{Label: "Copy Song Link", Icon: "share", Action: func() { CopyLink(t.URI) }}}
	}})
	return items
}

// CardMenuOpts adds context to an album/playlist/artist menu.
type CardMenuOpts struct {
	Library     bool // shown in Your Library (offers Pin/Unpin)
	Pinned      bool
	Own         bool // the user's own playlist (Edit details, Delete)
	Description string
	OnEdit      func()
}

// CardMenu is the right-click menu of an album, playlist, artist, show...
func CardMenu(c spotify.Card, o CardMenuOpts) []MenuItem {
	LoadSaved([]string{c.URI})
	var items []MenuItem
	typ := c.Type
	if typ == "" {
		typ = URIType(c.URI)
	}
	own := typ == "playlist" && o.Own
	if typ == "album" || typ == "playlist" {
		items = append(items, MenuItem{Label: "Add to queue", Icon: "queue", Action: func() { QueueContext(c.URI) }})
	}
	switch {
	case typ == "artist" && Saved[c.URI]:
		items = append(items, MenuItem{Label: "Unfollow", Icon: "close", Action: func() { ToggleSaved(c.URI) }})
	case typ == "artist":
		items = append(items, MenuItem{Label: "Follow", Icon: "user", Action: func() { ToggleSaved(c.URI) }})
	case typ == "collection" || typ == "folder" || typ == "user" || typ == "genre" || own:
	case Saved[c.URI]:
		items = append(items, MenuItem{Label: "Remove from Your Library", Icon: "check-circle", Action: func() { ToggleSaved(c.URI) }})
	default:
		items = append(items, MenuItem{Label: "Add to Your Library", Icon: "plus-circle", Action: func() { ToggleSaved(c.URI) }})
	}
	if typ == "artist" || typ == "album" || typ == "playlist" {
		items = append(items, MenuItem{Label: "Go to " + typ + " radio", Icon: "radio", Action: func() { StartRadio(c.URI) }})
	}
	if o.Library && typ != "folder" {
		label := "Pin " + typeLabel(typ)
		if o.Pinned {
			label = "Unpin " + typeLabel(typ)
		}
		items = append(items, MenuItem{Label: label, Icon: "pin", Action: func() { SetPinned(c.URI, !o.Pinned) }})
	}
	if own {
		items = append(items,
			MenuItem{Label: "Edit details", Icon: "edit", Action: func() { EditPlaylist(c.URI, c.Name, o.Description, o.OnEdit) }},
			MenuItem{Label: "Delete", Icon: "trash", Danger: true, Action: func() { DeletePlaylist(c.URI, c.Name) }},
		)
	}
	if typ == "playlist" {
		LoadTasteExcluded(c.URI)
		label, icon := "Exclude from your taste profile", "x-circle"
		if TasteExcluded[c.URI] {
			label, icon = "Include in your taste profile", "check-circle"
		}
		items = append(items, MenuItem{Label: label, Icon: icon, Action: func() { ToggleTasteExcluded(c.URI) }})
	}
	if typ == "collection" {
		items = append(items, MenuItem{Label: "Create playlist", Icon: "plus", Action: func() { CreatePlaylist(nil) }})
	}
	if typ != "collection" && typ != "folder" {
		label := "Copy link to " + typeLabel(typ)
		items = append(items, MenuItem{Divider: true}, MenuItem{Label: "Share", Icon: "share", Sub: func() []MenuItem {
			return []MenuItem{{Label: label, Icon: "share", Action: func() { CopyLink(c.URI) }}}
		}})
	}
	return items
}

func typeLabel(typ string) string {
	switch typ {
	case "user":
		return "profile"
	case "show":
		return "podcast"
	case "collection":
		return "playlist"
	}
	return typ
}

// playlistPicker is the "Add to playlist" flyout: a filter field, "New
// playlist" and the user's editable playlists (checked if they contain it).
func playlistPicker(uris []string) func() []MenuItem {
	var res Res[[]spotify.EditablePlaylist]
	var find Input
	return func() []MenuItem {
		res.Once(func() ([]spotify.EditablePlaylist, error) { return B.EditablePlaylists(uris) })
		items := []MenuItem{
			{Widget: func(gtx layout.Context) layout.Dimensions {
				return Pad(gtx, 4, 4, 4, 4, func(gtx layout.Context) layout.Dimensions {
					find.Poll(gtx)
					h := Dp(gtx, 36)
					w := gtx.Constraints.Max.X
					FillRect(gtx.Ops, image.Rect(0, 0, w, h), Dp(gtx, 4), RGB(0x3e3e3e))
					Offset(gtx, Dp(gtx, 10), Dp(gtx, 10), func() {
						IconPx(gtx, "search", Dp(gtx, 16), ColText2)
					})
					Offset(gtx, Dp(gtx, 34), Dp(gtx, 9), func() {
						g := gtx
						g.Constraints = layout.Constraints{Min: image.Pt(w-Dp(gtx, 44), 0), Max: image.Pt(w-Dp(gtx, 44), h)}
						find.Layout(g, InputStyle{Hint: "Find a playlist"})
					})
					return layout.Dimensions{Size: image.Pt(w, h)}
				})
			}},
			{Label: "New playlist", Icon: "plus", Action: func() { CreatePlaylist(uris) }},
			{Divider: true},
		}
		switch {
		case !res.Done:
			items = append(items, MenuItem{Label: "Loading…", Muted: true})
		case res.Err != nil:
			items = append(items, MenuItem{Label: "Couldn't load playlists", Muted: true})
		default:
			q := strings.ToLower(find.Text())
			for _, p := range res.V {
				if q != "" && !strings.Contains(strings.ToLower(p.Name), q) {
					continue
				}
				card := p.Card
				items = append(items, MenuItem{Label: p.Name, Check: p.Contains, Action: func() { AddToPlaylist(card, uris) }})
			}
		}
		return items
	}
}

// ---- Dialogs ----

type confirmModal struct {
	title, body, ok string
	onOK            func()
	cancel, okBtn   Pill
}

func (m *confirmModal) Layout(gtx layout.Context) layout.Dimensions {
	if m.cancel.Click.Clicked(gtx) {
		CloseModal()
	}
	if m.okBtn.Click.Clicked(gtx) {
		CloseModal()
		m.onOK()
	}
	return Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return Pad(gtx, 0, 0, 16, 0, func(gtx layout.Context) layout.Dimensions {
				return Label{Text: m.title, Size: 24, Weight: Bold, MaxLines: 3}.Layout(gtx)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return Pad(gtx, 0, 0, 24, 0, func(gtx layout.Context) layout.Dimensions {
				return Label{Text: m.body, Size: 14, Color: ColText2, MaxLines: -1}.Layout(gtx)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Spacing: layout.SpaceStart, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return m.cancel.Layout(gtx, PillStyle{Text: "Cancel", Weight: Bold, Height: 48, PadX: 24, Fg: ColText2})
					}),
					layout.Rigid(Gap(8, 0)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return m.okBtn.Layout(gtx, PillStyle{Text: m.ok, Weight: Bold, Height: 48, PadX: 32, Bg: ColText, Fg: ColBg})
					}),
				)
			})
		},
	)
}

// Confirm asks a yes/no question; onOK runs if confirmed.
func Confirm(title, body, ok string, onOK func()) {
	ShowModal(&confirmModal{title: title, body: body, ok: ok, onOK: onOK}, 0)
}

type editModal struct {
	name, desc Input
	save       Pill
	onSave     func(name, desc string)
	focused    bool
}

func (m *editModal) Layout(gtx layout.Context) layout.Dimensions {
	if !m.focused {
		m.name.Focus(gtx)
		m.focused = true
	}
	_, sub := m.name.Poll(gtx)
	m.desc.Poll(gtx)
	name := strings.TrimSpace(m.name.Text())
	if (m.save.Click.Clicked(gtx) || sub) && name != "" {
		CloseModal()
		m.onSave(name, m.desc.Text())
	}
	return Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return Pad(gtx, 0, 0, 16, 0, func(gtx layout.Context) layout.Dimensions {
				return Text(gtx, "Edit details", 24, Bold, ColText)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return Pad(gtx, 0, 0, 12, 0, func(gtx layout.Context) layout.Dimensions {
				return FieldBox(gtx, &m.name, InputStyle{Hint: "Add a name"}, 40)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return Pad(gtx, 0, 0, 12, 0, func(gtx layout.Context) layout.Dimensions {
				return FieldBox(gtx, &m.desc, InputStyle{Hint: "Add an optional description", MaxLines: 5}, 120)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				s := PillStyle{Text: "Save", Weight: Bold, Height: 48, PadX: 32, Bg: ColText, Fg: ColBg}
				if name == "" {
					s.Bg = Alpha(ColText, .5)
				}
				return m.save.Layout(gtx, s)
			})
		},
	)
}

// EditDetails edits a playlist's name and description.
func EditDetails(name, desc string, onSave func(name, desc string)) {
	m := &editModal{onSave: onSave}
	m.name.SetText(name)
	m.desc.SetText(desc)
	ShowModal(m, 0)
}

type creditsModal struct {
	track string
	res   Res[*spotify.NowPlayingInfo]
	links []Link
	list  List
}

func (m *creditsModal) Layout(gtx layout.Context) layout.Dimensions {
	return Column(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return Pad(gtx, 0, 0, 8, 0, func(gtx layout.Context) layout.Dimensions {
				return Text(gtx, "Credits", 24, Bold, ColText)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return Pad(gtx, 0, 0, 16, 0, func(gtx layout.Context) layout.Dimensions {
				return Text(gtx, m.track, 14, Bold, ColText)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			switch {
			case !m.res.Done:
				return Text(gtx, "Loading…", 14, Regular, ColText2)
			case m.res.Err != nil || len(m.res.V.Credits) == 0:
				return Text(gtx, "No credits available.", 14, Regular, ColText2)
			}
			cs := m.res.V.Credits
			if len(m.links) < len(cs) {
				m.links = make([]Link, len(cs))
			}
			rows := make([]layout.Widget, len(cs))
			for i, c := range cs {
				l := &m.links[i]
				if l.Click.Clicked(gtx) && c.URI != "" {
					CloseModal()
					OpenURI(c.URI)
				}
				rows[i] = func(gtx layout.Context) layout.Dimensions {
					return Pad(gtx, 0, 0, 12, 0, func(gtx layout.Context) layout.Dimensions {
						return Column(gtx,
							func(gtx layout.Context) layout.Dimensions {
								if c.URI == "" {
									return Text(gtx, c.Name, 16, Regular, ColText)
								}
								return l.Layout(gtx, c.Name, 16, Regular, ColText, ColNone)
							},
							func(gtx layout.Context) layout.Dimensions {
								return Text(gtx, c.Roles, 14, Regular, ColText2)
							},
						)
					})
				}
			}
			return Column(gtx, rows...)
		},
	)
}

// ShowCredits opens the credits of a track.
func ShowCredits(track, uri string) {
	m := &creditsModal{track: track}
	m.res.Load(func() (*spotify.NowPlayingInfo, error) { return B.NowPlaying(uri, "") })
	ShowModal(m, 0)
}
