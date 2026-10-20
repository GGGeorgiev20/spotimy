package spotify

import (
	"context"
	"encoding/json"
	"regexp"
	"time"
)

type ArtistPage struct {
	URI              string  `json:"uri"`
	Name             string  `json:"name"`
	Verified         bool    `json:"verified"`
	Avatar           []Image `json:"avatar"`
	Header           []Image `json:"header"` // banner; empty for artists without one
	Gallery          []Image `json:"gallery"`
	MonthlyListeners int     `json:"monthlyListeners"`
	Followers        int     `json:"followers"`
	WorldRank        int     `json:"worldRank"`
	Bio              string  `json:"bio"`
	Following        bool    `json:"following"`
	TopTracks        []Track `json:"topTracks"`
	Popular          []Card  `json:"popular"` // popular releases
	Albums           []Card  `json:"albums"`
	Singles          []Card  `json:"singles"`
	Compilations     []Card  `json:"compilations"`
	Totals           [3]int  `json:"totals"` // albums, singles, compilations
	Shelves          []Shelf `json:"shelves"`
	Cities           []City  `json:"cities"`
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
	for _, it := range g.Items {
		if len(it.Releases.Items) > 0 {
			out = append(out, it.Releases.Items[0].card())
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
				PlaylistsV2 gqlV2List `json:"playlistsV2"`
			} `json:"profile"`
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

	var related []Card
	for _, raw := range a.RelatedContent.RelatedArtists.Items {
		var e gqlEntity
		if json.Unmarshal(raw, &e) == nil {
			e.Typename = "Artist"
			if c, ok := e.card(""); ok {
				related = append(related, c)
			}
		}
	}
	for _, s := range []Shelf{
		// The official order; the page puts About after the first shelf.
		{Title: "Featuring " + out.Name, Items: a.RelatedContent.FeaturingV2.cards()},
		{Title: "Discovered on", Items: a.RelatedContent.DiscoveredOnV2.cards()},
		{Title: "Artist Playlists", Items: a.Profile.PlaylistsV2.cards()},
		{Title: "Fans also like", Items: related},
		{Title: "Appears On", Items: a.RelatedContent.AppearsOn.cards()},
	} {
		if len(s.Items) > 0 {
			s.Kind = "shelf"
			s.Total = len(s.Items)
			out.Shelves = append(out.Shelves, s)
		}
	}
	return out, nil
}

// Discography returns all releases of an artist, newest first. kind is "all",
// "albums", "singles" or "compilations".
func (c *Client) Discography(ctx context.Context, uri, kind string, offset, limit int) (*SearchPage, error) {
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
	err := c.query(ctx, op, map[string]any{"uri": uri, "offset": offset, "limit": limit, "order": "DATE_DESC"}, 5*time.Minute, &resp)
	if err != nil {
		return nil, err
	}
	g := resp.ArtistUnion.Discography[kind]
	return &SearchPage{Total: g.TotalCount, Cards: g.cards()}, nil
}
