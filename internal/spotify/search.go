package spotify

import (
	"context"
	"encoding/json"
	"time"
)

type SearchResults struct {
	Query  string  `json:"query"`
	Top    *Card   `json:"top,omitempty"`
	Tracks []Track `json:"tracks"`
	Totals Counts  `json:"totals"`
	Groups []Shelf `json:"groups"` // Artists, Albums, Playlists, Podcasts, Episodes, Profiles
}

type Counts struct {
	Tracks    int `json:"tracks"`
	Artists   int `json:"artists"`
	Albums    int `json:"albums"`
	Playlists int `json:"playlists"`
	Podcasts  int `json:"podcasts"`
	Episodes  int `json:"episodes"`
	Users     int `json:"users"`
}

// SearchPage is one page of a single-type search tab.
type SearchPage struct {
	Total  int     `json:"total"`
	Cards  []Card  `json:"cards,omitempty"`
	Tracks []Track `json:"tracks,omitempty"`
}

type gqlCardList struct {
	TotalCount int          `json:"totalCount"`
	Items      []gqlWrapper `json:"items"`
}

func (l gqlCardList) cards() []Card {
	out := []Card{}
	for _, w := range l.Items {
		if c, ok := cardFromWrapper(w); ok {
			out = append(out, c)
		}
	}
	return out
}

type gqlTrackHits struct {
	TotalCount int `json:"totalCount"`
	Items      []struct {
		Item struct {
			Data gqlTrack `json:"data"`
		} `json:"item"`
	} `json:"items"`
}

func (h gqlTrackHits) tracks() []Track {
	out := []Track{}
	for _, it := range h.Items {
		if t := it.Item.Data.track(); t.URI != "" {
			out = append(out, t)
		}
	}
	return out
}

type gqlSearch struct {
	TopResultsV2 struct {
		ItemsV2 []struct {
			Item gqlWrapper `json:"item"`
		} `json:"itemsV2"`
	} `json:"topResultsV2"`
	TracksV2  gqlTrackHits `json:"tracksV2"`
	Artists   gqlCardList  `json:"artists"`
	AlbumsV2  gqlCardList  `json:"albumsV2"`
	Playlists gqlCardList  `json:"playlists"`
	Podcasts  gqlCardList  `json:"podcasts"`
	Episodes  gqlCardList  `json:"episodes"`
	Users     gqlCardList  `json:"users"`
}

func searchVars(term string, offset, limit int) map[string]any {
	return map[string]any{
		"searchTerm": term, "offset": offset, "limit": limit, "numberOfTopResults": 5,
		"includeAudiobooks": true, "includeArtistHasConcertsField": false, "includePreReleases": true,
		"includeAuthors": false, "includeEpisodeContentRatingsV2": false, "includeAlbumPreReleases": false,
		"sectionFilters": []string{"GENERIC", "VIDEO_CONTENT"},
	}
}

// Search runs the "All" search shown while typing.
func (c *Client) Search(ctx context.Context, term string) (*SearchResults, error) {
	var resp struct {
		SearchV2 gqlSearch `json:"searchV2"`
	}
	if err := c.query(ctx, "searchDesktop", searchVars(term, 0, 10), 2*time.Minute, &resp); err != nil {
		return nil, err
	}
	s := resp.SearchV2
	out := &SearchResults{
		Query:  term,
		Tracks: s.TracksV2.tracks(),
		Groups: []Shelf{},
		Totals: Counts{
			Tracks: s.TracksV2.TotalCount, Artists: s.Artists.TotalCount, Albums: s.AlbumsV2.TotalCount,
			Playlists: s.Playlists.TotalCount, Podcasts: s.Podcasts.TotalCount, Episodes: s.Episodes.TotalCount,
			Users: s.Users.TotalCount,
		},
	}
	for _, it := range s.TopResultsV2.ItemsV2 {
		if card, ok := cardFromWrapper(it.Item); ok {
			out.Top = &card
			break
		}
	}
	for _, g := range []struct {
		title string
		list  gqlCardList
	}{
		{"Artists", s.Artists}, {"Albums", s.AlbumsV2}, {"Playlists", s.Playlists},
		{"Podcasts", s.Podcasts}, {"Episodes", s.Episodes}, {"Profiles", s.Users},
	} {
		if cards := g.list.cards(); len(cards) > 0 {
			out.Groups = append(out.Groups, Shelf{Title: g.title, Kind: "shelf", Items: cards, Total: g.list.TotalCount})
		}
	}
	return out, nil
}

// Suggestion is one row of the search box's dropdown: a query completion
// (Query) or an entity (Card).
type Suggestion struct {
	Query string `json:"query,omitempty"`
	Card  *Card  `json:"card,omitempty"`
}

// SearchSuggestions returns the dropdown's rows while typing: query
// completions first, then artists, songs and so on.
func (c *Client) SearchSuggestions(ctx context.Context, term string) ([]Suggestion, error) {
	var resp struct {
		SearchV2 struct {
			TopResultsV2 struct {
				ItemsV2 []struct {
					Item gqlWrapper `json:"item"`
				} `json:"itemsV2"`
			} `json:"topResultsV2"`
		} `json:"searchV2"`
	}
	vars := map[string]any{
		"query": term, "limit": 30, "numberOfTopResults": 30, "offset": 0,
		"includeAuthors": false, "includeEpisodeContentRatingsV2": false, "includeAudiobooks": true,
		"includePreReleases": false, "includeArtistHasConcertsField": false,
	}
	if err := c.query(ctx, "searchSuggestions", vars, 2*time.Minute, &resp); err != nil {
		return nil, err
	}
	out := []Suggestion{}
	for _, it := range resp.SearchV2.TopResultsV2.ItemsV2 {
		if it.Item.Typename == "SearchAutoCompleteEntity" {
			var auto struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(it.Item.Data, &auto) == nil && auto.Text != "" {
				out = append(out, Suggestion{Query: auto.Text})
			}
			continue
		}
		if card, ok := cardFromWrapper(it.Item); ok {
			out = append(out, Suggestion{Card: &card})
		}
	}
	return out, nil
}

// searchOps maps a search tab to its pathfinder operation and result field.
var searchOps = map[string][2]string{
	"tracks":    {"searchTracks", "tracksV2"},
	"artists":   {"searchArtists", "artists"},
	"albums":    {"searchAlbums", "albumsV2"},
	"playlists": {"searchPlaylists", "playlists"},
	"podcasts":  {"searchPodcasts", "podcasts"},
	"episodes":  {"searchFullEpisodes", "episodes"},
	"users":     {"searchUsers", "users"},
	"genres":    {"searchGenres", "genres"},
}

// SearchType returns one page of a single search tab.
func (c *Client) SearchType(ctx context.Context, term, kind string, offset, limit int) (*SearchPage, error) {
	op, ok := searchOps[kind]
	if !ok {
		return nil, &Error{Op: "search:" + kind, Err: errNotFound}
	}
	var resp struct {
		SearchV2 map[string]json.RawMessage `json:"searchV2"`
	}
	if err := c.query(ctx, op[0], searchVars(term, offset, limit), 2*time.Minute, &resp); err != nil {
		return nil, err
	}
	raw := resp.SearchV2[op[1]]
	page := &SearchPage{}
	if kind == "tracks" {
		var h gqlTrackHits
		if err := json.Unmarshal(raw, &h); err != nil {
			return nil, &Error{Op: op[0], Err: err}
		}
		page.Total, page.Tracks = h.TotalCount, h.tracks()
	} else {
		var l gqlCardList
		if err := json.Unmarshal(raw, &l); err != nil {
			return nil, &Error{Op: op[0], Err: err}
		}
		page.Total, page.Cards = l.TotalCount, l.cards()
	}
	return page, nil
}

// RecentSearches returns the entities the user opened from search.
func (c *Client) RecentSearches(ctx context.Context) ([]Card, error) {
	var resp struct {
		RecentSearches struct {
			RecentSearchesItems gqlCardList `json:"recentSearchesItems"`
		} `json:"recentSearches"`
	}
	if err := c.query(ctx, "recentSearches", map[string]any{"offset": 0, "limit": 50}, 0, &resp); err != nil {
		return nil, err
	}
	return resp.RecentSearches.RecentSearchesItems.cards(), nil
}

func (c *Client) SaveRecentSearch(ctx context.Context, uri string) error {
	return c.mutate(ctx, "saveRecentSearches", map[string]any{"uris": []string{uri}})
}

func (c *Client) RemoveRecentSearches(ctx context.Context, uris []string) error {
	return c.mutate(ctx, "removeRecentSearches", map[string]any{"uris": uris})
}
