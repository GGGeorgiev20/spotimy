package ui

import (
	"image/color"
	"net/url"
	"strings"

	"gioui.org/layout"
)

// Page is one view of the main panel. A page instance lives as long as its
// history entry, so going back restores its data and scroll position.
type Page interface {
	Layout(gtx layout.Context) layout.Dimensions
}

// Chrome is what a page tells the main panel's sticky top bar: once the
// list has scrolled past Threshold px, the bar fades in with Color behind a
// play button (OnPlay, for ContextURI) and Title.
type Chrome struct {
	Title      string
	Color      color.NRGBA
	ContextURI string
	OnPlay     func()
	Threshold  int
	// List is the page's scroll list; the bar reads its offset.
	List *List
}

// ChromePage is implemented by pages with a sticky top bar.
type ChromePage interface {
	Chrome() Chrome
}

// Factory creates the page for a path's segments (after the first).
type Factory func(args []string) Page

var routes = map[string]Factory{}

// Register adds a page for paths starting with /name ("" for home).
func Register(name string, f Factory) { routes[name] = f }

type Entry struct {
	Path string
	page Page
}

// Page returns the entry's page, creating it on first use.
func (e *Entry) Page() Page {
	if e.page == nil {
		parts := strings.Split(strings.Trim(e.Path, "/"), "/")
		if f, ok := routes[parts[0]]; ok {
			e.page = f(parts[1:])
		} else {
			e.page = notFound{e.Path}
		}
	}
	return e.page
}

var (
	history = []*Entry{{Path: "/"}}
	histIdx = 0
)

const keepPages = 24 // older entries drop their page and rebuild on revisit

func Current() *Entry { return history[histIdx] }

// Navigate opens path as a new history entry.
func Navigate(path string) {
	if Current().Path == path {
		return
	}
	history = append(history[:histIdx+1], &Entry{Path: path})
	histIdx = len(history) - 1
	for i := 0; i < len(history)-keepPages; i++ {
		history[i].page = nil
	}
	Invalidate()
}

// Replace swaps the current entry (search-as-you-type shouldn't flood history).
func Replace(path string) {
	if Current().Path == path {
		return
	}
	history[histIdx] = &Entry{Path: path}
	Invalidate()
}

// ReplaceKeep changes the current path but keeps the page instance.
func ReplaceKeep(path string) {
	history[histIdx].Path = path
}

func CanBack() bool    { return histIdx > 0 }
func CanForward() bool { return histIdx < len(history)-1 }

func Back() {
	if CanBack() {
		histIdx--
		Invalidate()
	}
}

func Forward() {
	if CanForward() {
		histIdx++
		Invalidate()
	}
}

// URIToPath maps spotify URIs to routes: spotify:playlist:abc -> /playlist/abc,
// spotify:user:x:collection -> /collection/tracks, spotify:page:x -> /genre/x.
func URIToPath(uri string) string {
	p := strings.Split(uri, ":")
	if len(p) < 3 {
		return "/"
	}
	if p[1] == "collection" || p[len(p)-1] == "collection" {
		if p[2] == "your-episodes" {
			return "/collection/episodes"
		}
		return "/collection/tracks"
	}
	switch {
	case p[1] == "user" && len(p) == 3:
		return "/user/" + p[2]
	case p[1] == "user" && len(p) >= 5 && p[3] == "playlist":
		return "/playlist/" + p[4]
	case p[1] == "page":
		return "/genre/" + p[2]
	case p[1] == "section":
		return "/section/" + url.PathEscape(uri)
	}
	return "/" + p[1] + "/" + p[2]
}

// OpenURI navigates to the page for a spotify URI.
func OpenURI(uri string) { Navigate(URIToPath(uri)) }

// ShareURL is the open.spotify.com link for "Copy link".
func ShareURL(uri string) string {
	p := strings.Split(uri, ":")
	if len(p) < 3 {
		return "https://open.spotify.com/"
	}
	if p[1] == "user" && len(p) == 3 {
		return "https://open.spotify.com/user/" + p[2]
	}
	return "https://open.spotify.com/" + p[1] + "/" + p[2]
}

// URIID returns the last segment of a URI (the base62 id).
func URIID(uri string) string { return uri[strings.LastIndex(uri, ":")+1:] }

type notFound struct{ path string }

func (n notFound) Layout(gtx layout.Context) layout.Dimensions {
	return Pad(gtx, 64, 24, 24, 24, func(gtx layout.Context) layout.Dimensions {
		return Label{Text: "This page isn't available yet: " + n.path, Size: 16, Color: ColText2}.Layout(gtx)
	})
}
