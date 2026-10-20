package spotify

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
)

// gqlWrapper is the "XResponseWrapper" envelope pathfinder puts around
// entities in shelves, search results and artist pages.
type gqlWrapper struct {
	Typename string          `json:"__typename"`
	URI      string          `json:"_uri"`
	Data     json.RawMessage `json:"data"`
}

// gqlEntity is the union of every entity shape a card can be built from.
type gqlEntity struct {
	Typename    string      `json:"__typename"`
	URI         string      `json:"uri"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Images      gqlImages   `json:"images"`   // Playlist
	CoverArt    *gqlSources `json:"coverArt"` // Album, Podcast, Episode, Audiobook
	Artists     gqlArtists  `json:"artists"`  // Album, Track
	AlbumType   string      `json:"albumType"`
	Type        string      `json:"type"`
	Date        *gqlDate    `json:"date"`
	OwnerV2     *struct {
		Data gqlUser `json:"data"`
	} `json:"ownerV2"`
	Profile *struct {
		Name string `json:"name"`
	} `json:"profile"` // Artist
	Visuals *struct {
		AvatarImage *gqlSources `json:"avatarImage"`
	} `json:"visuals"` // Artist
	Publisher *struct {
		Name string `json:"name"`
	} `json:"publisher"` // Podcast
	PodcastV2 *struct {
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
	} `json:"podcastV2"` // Episode
	ReleaseDate *gqlDate `json:"releaseDate"` // Episode
	Duration    *struct {
		TotalMilliseconds int `json:"totalMilliseconds"`
	} `json:"duration"`
	AlbumOfTrack *struct {
		CoverArt *gqlSources `json:"coverArt"`
	} `json:"albumOfTrack"` // Track
	Avatar      *gqlSources `json:"avatar"`      // User
	DisplayName string      `json:"displayName"` // User
	Username    string      `json:"username"`

	// Trait-based "Entity" shape (used by Recents and newer sections).
	IdentityTrait *struct {
		Name         string `json:"name"`
		Contributors struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
		} `json:"contributors"`
	} `json:"identityTrait"`
	EntityTypeTrait *struct {
		Type string `json:"type"`
	} `json:"entityTypeTrait"`
	VisualIdentityTrait *struct {
		SquareCoverImage *struct {
			Image struct {
				Data gqlSources `json:"data"`
			} `json:"image"`
		} `json:"squareCoverImage"`
	} `json:"visualIdentityTrait"`
}

var tagRE = regexp.MustCompile(`<[^>]*>`)

// stripHTML turns Spotify's HTML descriptions into plain text.
func stripHTML(s string) string {
	return strings.TrimSpace(html.UnescapeString(tagRE.ReplaceAllString(s, "")))
}

// card maps an entity to a Card; ok is false for types the UI can't show.
func (e gqlEntity) card(uri string) (Card, bool) {
	if e.URI != "" {
		uri = e.URI
	}
	c := Card{URI: uri, Name: e.Name, Type: URIType(uri)}
	switch e.Typename {
	case "Playlist":
		c.Images = e.Images.first()
		c.Subtitle = stripHTML(e.Description)
		if c.Subtitle == "" && e.OwnerV2 != nil {
			c.Subtitle = "By " + e.OwnerV2.Data.user().Name
		}
	case "Album":
		c.Images = e.CoverArt.images()
		c.Subtitle = e.Artists.names()
		if e.Date != nil && e.Date.year() != "" {
			c.Subtitle = e.Date.year() + " • " + albumTypeLabel(e.AlbumType, e.Type)
		}
	case "Artist":
		if e.Profile != nil {
			c.Name = e.Profile.Name
		}
		if e.Visuals != nil {
			c.Images = e.Visuals.AvatarImage.images()
		}
		c.Subtitle = "Artist"
	case "Podcast", "Audiobook":
		c.Images = e.CoverArt.images()
		if e.Publisher != nil {
			c.Subtitle = e.Publisher.Name
		}
	case "Episode":
		c.Images = e.CoverArt.images()
		if e.PodcastV2 != nil {
			c.Subtitle = e.PodcastV2.Data.Name
		}
	case "Track":
		if e.AlbumOfTrack != nil {
			c.Images = e.AlbumOfTrack.CoverArt.images()
		}
		c.Subtitle = e.Artists.names()
	case "User":
		c.Name = e.DisplayName
		if c.Name == "" {
			c.Name = e.Name
		}
		c.Images = e.Avatar.images()
		c.Subtitle = "Profile"
	case "Entity":
		if e.IdentityTrait == nil {
			return c, false
		}
		c.Name = e.IdentityTrait.Name
		if len(e.IdentityTrait.Contributors.Items) > 0 {
			c.Subtitle = e.IdentityTrait.Contributors.Items[0].Name
		}
		if v := e.VisualIdentityTrait; v != nil && v.SquareCoverImage != nil {
			c.Images = v.SquareCoverImage.Image.Data.images()
		}
	default:
		return c, false
	}
	return c, c.URI != "" && c.Name != ""
}

func albumTypeLabel(types ...string) string {
	for _, t := range types {
		switch t {
		case "ALBUM":
			return "Album"
		case "SINGLE":
			return "Single"
		case "EP":
			return "EP"
		case "COMPILATION":
			return "Compilation"
		}
	}
	return "Album"
}

// cardFromWrapper decodes a wrapper's data into a Card.
func cardFromWrapper(w gqlWrapper) (Card, bool) {
	if len(w.Data) == 0 || string(w.Data) == "null" {
		return Card{}, false
	}
	var e gqlEntity
	if err := json.Unmarshal(w.Data, &e); err != nil {
		return Card{}, false
	}
	return e.card(w.URI)
}
