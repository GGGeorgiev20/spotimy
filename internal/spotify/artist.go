package spotify

import (
	"context"
	"encoding/json"
	"regexp"
	"time"
)

type ArtistPage struct {
	URI              string    `json:"uri"`
	Name             string    `json:"name"`
	Verified         bool      `json:"verified"`
	Avatar           []Image   `json:"avatar"`
	Header           []Image   `json:"header"` // banner; empty for artists without one
	Gallery          []Image   `json:"gallery"`
	MonthlyListeners int       `json:"monthlyListeners"`
	Followers        int       `json:"followers"`
	WorldRank        int       `json:"worldRank"`
	Bio              string    `json:"bio"`
	Following        bool      `json:"following"`
	TopTracks        []Track   `json:"topTracks"`
	Popular          []Card    `json:"popular"` // popular releases
	Albums           []Card    `json:"albums"`
	Singles          []Card    `json:"singles"`
	Compilations     []Card    `json:"compilations"`
	Totals           [3]int    `json:"totals"` // albums, singles, compilations
	Shelves          []Shelf   `json:"shelves"`
	Cities           []City    `json:"cities"`
	Links            []Link    `json:"links"` // social and external links
	Concerts         []Concert `json:"concerts"`
	ConcertTotal     int       `json:"concertTotal"`
	Merch            []Merch   `json:"merch"`
}

// Link is an external page of the artist; Name is the service in capitals
// (FACEBOOK, INSTAGRAM, TWITTER, WIKIPEDIA, TIKTOK...).
type Link struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Concert is an upcoming show; Start carries the venue's UTC offset.
type Concert struct {
	URI      string    `json:"uri"`
	Title    string    `json:"title"`
	City     string    `json:"city"`
	Venue    string    `json:"venue"`
	Start    time.Time `json:"start"`
	Festival bool      `json:"festival"`
}

// Merch is an item of the artist's shop.
type Merch struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       string  `json:"price"`
	URL         string  `json:"url"`
	Images      []Image `json:"images"`
}

// Release is an album, single, EP or compilation in a discography.
type Release struct {
	Card
	Kind   string `json:"kind"` // "Album", "Single", "EP", "Compilation"
	Year   string `json:"year"`
	Tracks int    `json:"tracks"`
}

// ReleasePage is a page of an artist's discography.
type ReleasePage struct {
	Total    int       `json:"total"`
	Releases []Release `json:"releases"`
}

// City is one of an artist's top listener cities.
type City struct {
	Name      string `json:"name"` // "London, GB"
	Listeners int    `json:"listeners"`
}

// paragraphRE matches the tags that end a paragraph of a biography.
var paragraphRE = regexp.MustCompile(`(?i)<br\s*/?>|</p>`)

type gqlRelease struct {
	URI      string     `json:"uri"`
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Date     gqlDate    `json:"date"`
	CoverArt gqlSources `json:"coverArt"`
	Tracks   struct {
		TotalCount int `json:"totalCount"`
	} `json:"tracks"`
}

func (r gqlRelease) release() Release {
	return Release{Card: r.card(), Kind: albumTypeLabel(r.Type), Year: r.Date.year(), Tracks: r.Tracks.TotalCount}
}

func (r gqlRelease) card() Card {
	return Card{URI: r.URI, Type: "album", Name: r.Name, Images: r.CoverArt.images(),
		Subtitle: r.Date.year() + " • " + albumTypeLabel(r.Type)}
}

// gqlReleaseGroups is discography.albums/singles/compilations/all:
// items[].releases.items[0] is the release.
type gqlReleaseGroups struct {
	TotalCount int `json:"totalCount"`
	Items      []struct {
		Releases struct {
			Items []gqlRelease `json:"items"`
		} `json:"releases"`
	} `json:"items"`
}

func (g gqlReleaseGroups) cards() []Card {
	out := []Card{}
	for _, r := range g.releases() {
		out = append(out, r.Card)
	}
	return out
}

func (g gqlReleaseGroups) releases() []Release {
	out := []Release{}
	for _, it := range g.Items {
		if len(it.Releases.Items) > 0 {
			out = append(out, it.Releases.Items[0].release())
		}
	}
	return out
}

// gqlGoods is artistUnion.goods: upcoming concerts and merch.
type gqlGoods struct {
	Concerts struct {
		TotalCount int `json:"totalCount"`
		Items      []struct {
			Data struct {
				URI       string `json:"uri"`
				Title     string `json:"title"`
				Festival  bool   `json:"festival"`
				StartDate string `json:"startDateIsoString"`
				Location  struct {
					City string `json:"city"`
					Name string `json:"name"`
				} `json:"location"`
			} `json:"data"`
		} `json:"items"`
	} `json:"concerts"`
	Merch struct {
		Items []struct {
			Name        string     `json:"nameV2"`
			Description string     `json:"description"`
			Price       string     `json:"price"`
			URL         string     `json:"url"`
			Image       gqlSources `json:"image"`
		} `json:"items"`
	} `json:"merch"`
}

func (g gqlGoods) concerts() []Concert {
	out := []Concert{}
	for _, it := range g.Concerts.Items {
		d := it.Data
		// The venue's local time: "2027-06-08T19:00+09:00".
		start, err := time.Parse("2006-01-02T15:04Z07:00", d.StartDate)
		if err != nil || d.URI == "" {
			continue
		}
		out = append(out, Concert{URI: d.URI, Title: d.Title, City: d.Location.City, Venue: d.Location.Name, Start: start, Festival: d.Festival})
	}
	return out
}

func (g gqlGoods) merch() []Merch {
	out := []Merch{}
	for _, m := range g.Merch.Items {
		out = append(out, Merch{Name: m.Name, Description: m.Description, Price: m.Price, URL: m.URL, Images: m.Image.images()})
	}
	return out
}

// relatedArtistCards decodes relatedContent.relatedArtists.items.
func relatedArtistCards(items []json.RawMessage) []Card {
	out := []Card{}
	for _, raw := range items {
		var e gqlEntity
		if json.Unmarshal(raw, &e) == nil {
			e.Typename = "Artist"
			if c, ok := e.card(""); ok {
				out = append(out, c)
			}
		}
	}
	return out
}

type gqlV2List struct {
	TotalCount int          `json:"totalCount"`
	Items      []gqlWrapper `json:"items"`
}

func (l gqlV2List) cards() []Card {
	out := []Card{}
	for _, w := range l.Items {
		if c, ok := cardFromWrapper(w); ok {
			out = append(out, c)
		}
	}
	return out
}

func (c *Client) Artist(ctx context.Context, uri string) (*ArtistPage, error) {
	var resp struct {
		ArtistUnion struct {
			Typename string `json:"__typename"`
			URI      string `json:"uri"`
			Saved    bool   `json:"saved"`
			Profile  struct {
				Name      string `json:"name"`
				Verified  bool   `json:"verified"`
				Biography struct {
					Text string `json:"text"`
				} `json:"biography"`
				ExternalLinks struct {
					Items []Link `json:"items"`
				} `json:"externalLinks"`
				PlaylistsV2 gqlV2List `json:"playlistsV2"`
			} `json:"profile"`
			Goods       gqlGoods `json:"goods"`
			HeaderImage *struct {
				Data gqlSources `json:"data"`
			} `json:"headerImage"`
			Visuals struct {
				AvatarImage *gqlSources `json:"avatarImage"`
				Gallery     gqlImages   `json:"gallery"`
			} `json:"visuals"`
			Stats struct {
				Followers        int `json:"followers"`
				MonthlyListeners int `json:"monthlyListeners"`
				WorldRank        int `json:"worldRank"`
				TopCities        struct {
					Items []struct {
						City      string `json:"city"`
						Country   string `json:"country"`
						Listeners int    `json:"numberOfListeners"`
					} `json:"items"`
				} `json:"topCities"`
			} `json:"stats"`
			Discography struct {
				TopTracks struct {
					Items []struct {
						UID   string   `json:"uid"`
						Track gqlTrack `json:"track"`
					} `json:"items"`
				} `json:"topTracks"`
				PopularReleasesAlbums struct {
					Items []gqlRelease `json:"items"`
				} `json:"popularReleasesAlbums"`
				Albums       gqlReleaseGroups `json:"albums"`
				Singles      gqlReleaseGroups `json:"singles"`
				Compilations gqlReleaseGroups `json:"compilations"`
			} `json:"discography"`
			RelatedContent struct {
				FeaturingV2    gqlV2List `json:"featuringV2"`
				DiscoveredOnV2 gqlV2List `json:"discoveredOnV2"`
				RelatedArtists struct {
					Items []json.RawMessage `json:"items"`
				} `json:"relatedArtists"`
				AppearsOn gqlReleaseGroups `json:"appearsOn"`
			} `json:"relatedContent"`
		} `json:"artistUnion"`
	}
	err := c.query(ctx, "queryArtistOverview", map[string]any{"uri": uri, "locale": "", "preReleaseV2": false}, 5*time.Minute, &resp)
	if err != nil {
		return nil, err
	}
	a := resp.ArtistUnion
	if a.Typename != "Artist" {
		return nil, &Error{Op: "queryArtistOverview", Err: errNotFound}
	}
	d := a.Discography
	out := &ArtistPage{
		URI: a.URI, Name: a.Profile.Name, Verified: a.Profile.Verified,
		Avatar: a.Visuals.AvatarImage.images(), Gallery: a.Visuals.Gallery.first(),
		MonthlyListeners: a.Stats.MonthlyListeners, Followers: a.Stats.Followers, WorldRank: a.Stats.WorldRank,
		Bio: stripHTML(paragraphRE.ReplaceAllString(a.Profile.Biography.Text, "\n")), Following: a.Saved,
		Albums: d.Albums.cards(), Singles: d.Singles.cards(), Compilations: d.Compilations.cards(),
		Totals:    [3]int{d.Albums.TotalCount, d.Singles.TotalCount, d.Compilations.TotalCount},
		TopTracks: []Track{}, Popular: []Card{}, Shelves: []Shelf{}, Cities: []City{},
		Links: []Link{}, Concerts: a.Goods.concerts(), ConcertTotal: a.Goods.Concerts.TotalCount, Merch: a.Goods.merch(),
	}
	for _, l := range a.Profile.ExternalLinks.Items {
		if l.Name != "" && l.URL != "" {
			out.Links = append(out.Links, l)
		}
	}
	if a.HeaderImage != nil {
		out.Header = a.HeaderImage.Data.images()
	}
	for _, t := range d.TopTracks.Items {
		tr := t.Track.track()
		tr.UID = t.UID
		out.TopTracks = append(out.TopTracks, tr)
	}
	for _, r := range d.PopularReleasesAlbums.Items {
		out.Popular = append(out.Popular, r.card())
	}
	for _, ci := range a.Stats.TopCities.Items {
		out.Cities = append(out.Cities, City{Name: ci.City + ", " + ci.Country, Listeners: ci.Listeners})
	}

	// The official order; the page puts About after the first shelf. URI is
	// the "Show all" page (/artist/<id>/<URI>).
	for _, s := range []Shelf{
		{URI: "featuring", Title: "Featuring " + out.Name, Items: a.RelatedContent.FeaturingV2.cards()},
		{URI: "discovered-on", Title: "Discovered on", Items: a.RelatedContent.DiscoveredOnV2.cards()},
		{URI: "playlists", Title: "Artist Playlists", Items: a.Profile.PlaylistsV2.cards()},
		{URI: "related", Title: "Fans also like", Items: relatedArtistCards(a.RelatedContent.RelatedArtists.Items)},
		{URI: "appears-on", Title: "Appears On", Items: a.RelatedContent.AppearsOn.cards()},
	} {
		if len(s.Items) > 0 {
			s.Kind = "shelf"
			s.Total = len(s.Items)
			out.Shelves = append(out.Shelves, s)
		}
	}
	return out, nil
}

// ArtistShelf is the "Show all" page of one of the artist's shelves: kind is
// "featuring", "discovered-on", "playlists", "related" or "appears-on".
func (c *Client) ArtistShelf(ctx context.Context, uri, kind string) (*Shelf, error) {
	op := map[string]string{"featuring": "queryArtistFeaturing", "discovered-on": "queryArtistDiscoveredOn",
		"playlists": "queryArtistPlaylists", "related": "queryArtistRelated", "appears-on": "queryArtistAppearsOn"}[kind]
	if op == "" {
		return nil, &Error{Op: "artist shelf:" + kind, Err: errNotFound}
	}
	var resp struct {
		ArtistUnion struct {
			Typename string `json:"__typename"`
			Profile  struct {
				Name        string    `json:"name"`
				PlaylistsV2 gqlV2List `json:"playlistsV2"`
			} `json:"profile"`
			RelatedContent struct {
				FeaturingV2    gqlV2List `json:"featuringV2"`
				DiscoveredOnV2 gqlV2List `json:"discoveredOnV2"`
				RelatedArtists struct {
					Items []json.RawMessage `json:"items"`
				} `json:"relatedArtists"`
				AppearsOn gqlReleaseGroups `json:"appearsOn"`
			} `json:"relatedContent"`
		} `json:"artistUnion"`
	}
	if err := c.query(ctx, op, map[string]any{"uri": uri}, 5*time.Minute, &resp); err != nil {
		return nil, err
	}
	a := resp.ArtistUnion
	if a.Typename != "Artist" {
		return nil, &Error{Op: op, Err: errNotFound}
	}
	s := &Shelf{URI: kind, Kind: "shelf"}
	switch kind {
	case "featuring":
		s.Title, s.Items = "Featuring "+a.Profile.Name, a.RelatedContent.FeaturingV2.cards()
	case "discovered-on":
		s.Title, s.Items = "Discovered on", a.RelatedContent.DiscoveredOnV2.cards()
	case "playlists":
		s.Title, s.Items = "Artist Playlists", a.Profile.PlaylistsV2.cards()
	case "related":
		s.Title, s.Items = "Fans also like", relatedArtistCards(a.RelatedContent.RelatedArtists.Items)
	default:
		s.Title, s.Items = "Appears On", a.RelatedContent.AppearsOn.cards()
	}
	s.Total = len(s.Items)
	return s, nil
}

// Discography returns the releases of an artist. kind is "all", "albums",
// "singles" or "compilations"; byName sorts A-Z instead of newest first.
func (c *Client) Discography(ctx context.Context, uri, kind string, byName bool, offset, limit int) (*ReleasePage, error) {
	op := map[string]string{"all": "queryArtistDiscographyAll", "albums": "queryArtistDiscographyAlbums",
		"singles": "queryArtistDiscographySingles", "compilations": "queryArtistDiscographyCompilations"}[kind]
	if op == "" {
		return nil, &Error{Op: "discography:" + kind, Err: errNotFound}
	}
	var resp struct {
		ArtistUnion struct {
			Discography map[string]gqlReleaseGroups `json:"discography"`
		} `json:"artistUnion"`
	}
	order := "DATE_DESC"
	if byName {
		order = "NAME_ASC"
	}
	err := c.query(ctx, op, map[string]any{"uri": uri, "offset": offset, "limit": limit, "order": order}, 5*time.Minute, &resp)
	if err != nil {
		return nil, err
	}
	g := resp.ArtistUnion.Discography[kind]
	return &ReleasePage{Total: g.TotalCount, Releases: g.releases()}, nil
}
