// Package search is the search page (/search[/<query>[/<tab>]]): recent
// searches and "Browse all" while the query is empty, results otherwise.
package search

import (
	"image"
	"net/url"
	"strings"

	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

// tabs in the official order; the id is the SearchType kind ("all" = overview).
var tabs = []struct{ id, label string }{
	{"all", "All"}, {"tracks", "Songs"}, {"playlists", "Playlists"}, {"albums", "Albums"},
	{"artists", "Artists"}, {"users", "Profiles"}, {"podcasts", "Podcasts & Shows"}, {"episodes", "Episodes"},
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
	recents ui.Res[[]spotify.Card]
	browse  ui.Res[[]spotify.Category]
	recGrid ui.CardRows
	catGrid ui.CategoryGrid
	forget  map[string]*ui.IconButton

	// "All": top result, the first songs, one shelf per kind.
	all     ui.Res[*spotify.SearchResults]
	top     topCard
	topRows *ui.Paged
	topTab  ui.TrackTable
	songs   ui.Link
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
		p.recents.Load(func() ([]spotify.Card, error) {
			r, _ := ui.B.RecentSearches() // failures just show no recents
			return r, nil
		})
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
func remember(uri string) {
	ui.Async(func() (struct{}, error) { return struct{}{}, ui.B.SaveRecentSearch(uri) }, func(struct{}, error) {})
}

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
// links); while typing, the route is already the typed text.
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
	rec, cats := p.recents.V, p.browse.V
	recRows := p.recGrid.Count(gtx, len(rec), 176)
	catRows := p.catGrid.Count(gtx, len(cats))
	n, recHead, catHead := 0, -1, -1
	if recRows > 0 {
		recHead, n = n, n+1+recRows
	}
	if catRows > 0 {
		catHead, n = n, n+1+catRows
	}
	heading := func(gtx layout.Context, text string, first bool, bottom float32) layout.Dimensions {
		top := float32(24)
		if first {
			top = 67
		}
		return ui.PageInset(gtx, top, bottom, func(gtx layout.Context) layout.Dimensions {
			return ui.Text(gtx, text, 24, ui.Bold, ui.ColText)
		})
	}
	if n == 0 && !(p.recents.Done && p.browse.Done) {
		return ui.Loading(gtx)
	}
	return p.list.Layout(gtx, n+1, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == recHead:
			return heading(gtx, "Recent searches", true, 4)
		case recHead >= 0 && i > recHead && i <= recHead+recRows:
			return p.recGrid.Row(gtx, i-recHead-1, rec, 176, ui.CardOpts{Overlay: p.removeButton})
		case i == catHead:
			return heading(gtx, "Browse all", recHead < 0, 20)
		case catHead >= 0 && i > catHead && i <= catHead+catRows:
			return p.catGrid.Row(gtx, i-catHead-1, cats)
		}
		return ui.Gap(0, 24)(gtx)
	})
}

// removeButton is the "x" on a hovered recent search.
func (p *page) removeButton(gtx layout.Context, c spotify.Card, art image.Rectangle) {
	if p.forget == nil {
		p.forget = map[string]*ui.IconButton{}
	}
	b := p.forget[c.URI]
	if b == nil {
		b = &ui.IconButton{}
		p.forget[c.URI] = b
	}
	if b.Click.Clicked(gtx) {
		ui.Go("Couldn't remove the recent search", func() error { return ui.B.RemoveRecentSearches([]string{c.URI}) }, func() {
			keep := p.recents.V[:0:0]
			for _, r := range p.recents.V {
				if r.URI != c.URI {
					keep = append(keep, r)
				}
			}
			p.recents.V = keep
		})
	}
	s := ui.Dp(gtx, 24)
	ui.Offset(gtx, art.Max.X-ui.Dp(gtx, 6)-s, art.Min.Y+ui.Dp(gtx, 6), func() {
		ui.FillCircle(gtx.Ops, s, ui.Alpha(ui.ColBg, .7))
		b.Layout(gtx, ui.IconStyle{Icon: "close", Size: 12, Box: 24, Label: "Remove " + c.Name})
	})
}
