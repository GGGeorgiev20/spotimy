package spotify

import (
	"context"
	"time"
)

// ArtistAbout is what the artist's About dialog shows.
type ArtistAbout struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
	Bio  string `json:"bio"`
	// Autobiography is set when the artist wrote the bio ("Posted By").
	Autobiography    bool      `json:"autobiography"`
	Avatar           []Image   `json:"avatar"`
	Photos           [][]Image `json:"photos"` // the gallery
	Followers        int       `json:"followers"`
	MonthlyListeners int       `json:"monthlyListeners"`
	WorldRank        int       `json:"worldRank"`
	Cities           []City    `json:"cities"`
	Links            []Link    `json:"links"`
	Verified         bool      `json:"verified"`   // "Verified by Spotify"
	Registered       bool      `json:"registered"` // claimed in Spotify for Artists
}

// ArtistAbout fetches the About dialog of an artist (queryArtistAboutModal).
func (c *Client) ArtistAbout(ctx context.Context, uri string) (*ArtistAbout, error) {
	var resp struct {
		ArtistUnion struct {
			Typename   string `json:"__typename"`
			Reputation struct {
				Verification struct {
					IsRegistered bool `json:"isRegistered"`
					IsVerified   bool `json:"isVerified"`
				} `json:"verification"`
			} `json:"onPlatformReputationTrait"`
			Profile struct {
				Name      string `json:"name"`
				Biography struct {
					Text string `json:"text"`
					Type string `json:"type"`
				} `json:"biography"`
				ExternalLinks struct {
					Items []Link `json:"items"`
				} `json:"externalLinks"`
			} `json:"profile"`
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
			Visuals struct {
				AvatarImage *gqlSources `json:"avatarImage"`
				Gallery     gqlImages   `json:"gallery"`
			} `json:"visuals"`
		} `json:"artistUnion"`
	}
	if err := c.query(ctx, "queryArtistAboutModal", map[string]any{"artistUri": uri}, 5*time.Minute, &resp); err != nil {
		return nil, err
	}
	a := resp.ArtistUnion
	if a.Typename != "Artist" {
		return nil, &Error{Op: "queryArtistAboutModal", Err: errNotFound}
	}
	out := &ArtistAbout{
		URI: uri, Name: a.Profile.Name,
		Bio:           stripHTML(paragraphRE.ReplaceAllString(a.Profile.Biography.Text, "\n")),
		Autobiography: a.Profile.Biography.Type == "AUTOBIOGRAPHY",
		Avatar:        a.Visuals.AvatarImage.images(), Photos: a.Visuals.Gallery.all(),
		Followers: a.Stats.Followers, MonthlyListeners: a.Stats.MonthlyListeners, WorldRank: a.Stats.WorldRank,
		Cities: []City{}, Links: []Link{},
		Verified: a.Reputation.Verification.IsVerified, Registered: a.Reputation.Verification.IsRegistered,
	}
	for _, ci := range a.Stats.TopCities.Items {
		out.Cities = append(out.Cities, City{Name: ci.City + ", " + ci.Country, Listeners: ci.Listeners})
	}
	for _, l := range a.Profile.ExternalLinks.Items {
		if l.Name != "" && l.URL != "" {
			out.Links = append(out.Links, l)
		}
	}
	return out, nil
}
