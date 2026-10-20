package ui

import (
	"image"
	"strings"

	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// TrackOf is the playing track as a list-row track (for menus).
func TrackOf(t *backend.NowTrack) spotify.Track {
	return spotify.Track{URI: t.URI, Name: t.Name, Artists: t.Artists, DurationMs: int(t.DurationMs), Playable: true,
		Album: &spotify.AlbumRef{URI: t.AlbumURI, Name: t.AlbumName, Images: t.Images}}
}

// PanelHeader is the 64dp title row of a right panel: title on the left, an
// optional extra widget (the "…" button) and the close button on the right.
type PanelHeader struct {
	Close IconButton
}

// Layout draws the header; closeLabel is the close button's tooltip.
func (h *PanelHeader) Layout(gtx layout.Context, closeLabel string, title, extra layout.Widget) layout.Dimensions {
	if h.Close.Click.Clicked(gtx) {
		Prefs.RightPanel = ""
	}
	w, hh := gtx.Constraints.Max.X, Dp(gtx, 64)
	g := gtx
	g.Constraints = layout.Exact(image.Pt(w-2*Dp(gtx, 16), hh))
	kids := []layout.FlexChild{layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		d := title(gtx)
		d.Size.X = gtx.Constraints.Max.X
		return d
	})}
	if extra != nil {
		kids = append(kids, layout.Rigid(extra))
	}
	kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		d := h.Close.Layout(gtx, IconStyle{Icon: "close", Label: closeLabel})
		Tip(gtx, &h.Close.Click, d, closeLabel, false)
		return d
	}))
	Offset(gtx, Dp(gtx, 16), 0, func() {
		layout.Flex{Alignment: layout.Middle}.Layout(g, kids...)
	})
	return layout.Dimensions{Size: image.Pt(w, hh)}
}

// ContextName resolves the title of a playback context ("Liked Songs", the
// playlist, album or artist name) in the background.
type ContextName struct {
	uri, name string
}

// Get returns the name of context uri, or "" while it loads or if unknown.
func (c *ContextName) Get(uri string) string {
	if uri == c.uri {
		return c.name
	}
	c.uri, c.name = uri, ""
	if strings.HasSuffix(uri, ":collection") {
		c.name = "Liked Songs"
		return c.name
	}
	switch URIType(uri) {
	case "playlist", "album", "artist":
	default:
		return ""
	}
	Async(func() (string, error) {
		switch URIType(uri) {
		case "playlist":
			p, err := B.Playlist(uri, 0, 1)
			if err != nil {
				return "", err
			}
			return p.Name, nil
		case "album":
			a, err := B.Album(uri)
			if err != nil {
				return "", err
			}
			return a.Name, nil
		}
		a, err := B.Artist(uri)
		if err != nil {
			return "", err
		}
		return a.Name, nil
	}, func(n string, err error) {
		if c.uri == uri && err == nil {
			c.name = n
		}
	})
	return ""
}
