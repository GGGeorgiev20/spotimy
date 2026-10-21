// Package search is the search page (/search[/<query>[/<tab>]]): "Browse
// all" while the query is empty, results otherwise. Recent searches are in
// the search box's dropdown (internal/ui/searchdrop.go).
package search

import (
	"net/url"
	"strings"

	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

// tabs in the official order; the id is the SearchType kind ("all" = overview).
var tabs = []struct{ id, label string }{
	{"all", "All"}, {"tracks", "Songs"}, {"playlists", "Playlists"}, {"artists", "Artists"}, {"genres", "Genres & Moods"},
	{"albums", "Albums"}, {"users", "Profiles"}, {"podcasts", "Podcasts & Shows"}, {"episodes", "Episodes"},
}

var tabLabels = func() []string {
	l := make([]string, len(tabs))
	for i, t := range tabs {
		l[i] = t.label
	}
	return l
}()

// groupTab maps the overview shelves to the tab their "Show all" opens.
var groupTab = map[string]string{
	"Artists": "artists", "Albums": "albums", "Playlists": "playlists",
	"Podcasts": "podcasts", "Episodes": "episodes", "Profiles": "users",
}

func init() {
	ui.Register("search", func(args []string) ui.Page {
		p := &page{tab: "all"}
		p.chips.Scroll = true
		if len(args) > 0 {
			p.query, _ = url.PathUnescape(args[0])
			p.query = strings.TrimSpace(p.query)
		}
		if len(args) > 1 {
			for _, t := range tabs {
				if t.id == args[1] {
					p.tab = t.id
				}
			}
		}
		p.start()
		return p
	})
}

type page struct {
	query, tab string
	list       ui.List
	chips      ui.Chips

	// Empty query.
	browse  ui.Res[[]spotify.Category]
	catGrid ui.CategoryGrid

	// "All": top result, the first songs, one shelf per kind.
	all     ui.Res[*spotify.SearchResults]
	top     topCard
	topRows *ui.Paged
	topTab  ui.TrackTable
	shelves []ui.Shelf
	built   bool

	// One kind: songs are paged, everything else is a card grid.
	res      ui.Res[*spotify.SearchPage]
	paged    *ui.Paged
	table    ui.TrackTable
	cards    []spotify.Card
	total    int
	loading  bool
	cardGrid ui.CardRows
}

func (p *page) start() {
	q := p.query
	switch {
	case q == "":
		p.browse.Load(func() ([]spotify.Category, error) {
			r, _ := ui.B.BrowseAll()
			return r, nil
		})
	case p.tab == "all":
		p.all.Load(func() (*spotify.SearchResults, error) { return ui.B.Search(q) })
	default:
		kind := p.tab
		p.res.Load(func() (*spotify.SearchPage, error) { return ui.B.SearchType(q, kind, 0, 50) })
	}
}

// remember saves an opened or played result as a recent search.
func remember(uri string) { ui.RememberSearch(uri) }

func (p *page) tabPath(id string) string {
	path := "/search/" + url.PathEscape(p.query)
	if id != "all" {
		path += "/" + id
	}
	return path
}

// shown is the page whose query the top bar's search box was last synced to.
var shown *page

// syncBox puts the route's query in the top bar (back/forward, direct
// links); after Enter it's already the typed text.
func (p *page) syncBox() {
	if shown == p {
		return
	}
	shown = p
	if strings.TrimSpace(ui.SearchQuery.Text()) != p.query {
		ui.SearchQuery.SetText(p.query)
	}
}

func (p *page) Layout(gtx layout.Context) layout.Dimensions {
	p.syncBox()
	p.list.Sticky = nil
	if p.query == "" {
		return p.layoutEmpty(gtx)
	}
	return p.layoutResults(gtx)
}

// ---- Empty query ----

func (p *page) layoutEmpty(gtx layout.Context) layout.Dimensions {
	cats := p.browse.V
	catRows := p.catGrid.Count(gtx, len(cats))
	if catRows == 0 && !p.browse.Done {
		return ui.Loading(gtx)
	}
	return p.list.Layout(gtx, catRows+2, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == 0:
			return ui.PageInset(gtx, 67, 20, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, "Browse all", 24, ui.Bold, ui.ColText)
			})
		case i <= catRows:
			return p.catGrid.Row(gtx, i-1, cats)
		}
		return ui.Gap(0, 24)(gtx)
	})
}
