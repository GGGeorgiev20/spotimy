package spotify

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"
)

type Shelf struct {
	URI      string `json:"uri"`
	Title    string `json:"title"`
	Kind     string `json:"kind"` // "shortcuts" (top grid), "recents" (covers only) or "shelf"
	Items    []Card `json:"items"`
	Total    int    `json:"total"`
	Subtitle string `json:"subtitle,omitempty"`
	// Label is the small line above the title ("Made For", "For fans of").
	Label string `json:"label,omitempty"`
	// Header is the entity the title names, e.g. the artist in "For fans of".
	Header *Card `json:"header,omitempty"`
}

type HomeChip struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Home struct {
	Greeting string     `json:"greeting"`
	Chips    []HomeChip `json:"chips"`
	Sections []Shelf    `json:"sections"`
}

type gqlLabel struct {
	TransformedLabel   string `json:"transformedLabel"`
	TranslatedBaseText string `json:"translatedBaseText"`
}

// split turns "Made For Galin" (base "Made For {0}") into ("Made For", "Galin"),
// the way the official client stacks a small label over the title.
func (l gqlLabel) split() (label, title string) {
	prefix, ok := strings.CutSuffix(l.TranslatedBaseText, "{0}")
	if !ok || strings.Contains(prefix, "{") || strings.TrimSpace(prefix) == "" || !strings.HasPrefix(l.TransformedLabel, prefix) {
		return "", l.TransformedLabel
	}
	return strings.TrimSpace(prefix), strings.TrimPrefix(l.TransformedLabel, prefix)
}

var likedSongsCard = Card{
	URI: "spotify:collection:tracks", Type: "collection", Name: "Liked Songs", Subtitle: "Playlist",
	Images: []Image{{URL: "https://misc.scdn.co/liked-songs/liked-songs-300.png", Width: 300, Height: 300}},
}

type gqlHomeSection struct {
	URI  string `json:"uri"`
	Data struct {
		Typename     string     `json:"__typename"`
		Title        *gqlLabel  `json:"title"`
		Subtitle     *gqlLabel  `json:"subtitle"`
		HeaderEntity gqlWrapper `json:"headerEntity"`
	} `json:"data"`
	SectionItems struct {
		TotalCount int `json:"totalCount"`
		Items      []struct {
			URI     string     `json:"uri"`
			Content gqlWrapper `json:"content"`
		} `json:"items"`
	} `json:"sectionItems"`
}

func (s gqlHomeSection) shelf() (Shelf, bool) {
	sh := Shelf{URI: s.URI, Kind: "shelf", Total: s.SectionItems.TotalCount}
	switch s.Data.Typename {
	case "HomeShortsSectionData":
		sh.Kind = "shortcuts"
	case "HomeRecentlyPlayedSectionData":
		sh.Kind = "recents"
	case "HomeGenericSectionData", "HomeSpotlightSectionData":
	default:
		// Mobile-only baseline cards, etc.
		return sh, false
	}
	if s.Data.Title != nil {
		sh.Label, sh.Title = s.Data.Title.split()
	}
	if s.Data.Subtitle != nil {
		sh.Subtitle = s.Data.Subtitle.TransformedLabel
	}
	if sh.Label == "" {
		sh.Label = sh.Subtitle // shown in the same spot
	}
	if h, ok := cardFromWrapper(s.Data.HeaderEntity); ok {
		sh.Header = &h
	}
	for _, it := range s.SectionItems.Items {
		if it.Content.URI == "" {
			it.Content.URI = it.URI
		}
		switch {
		case it.Content.Typename == "ListResponseWrapper":
			// Recents: one list of trait entities.
			var l struct {
				Items struct {
					Items []struct {
						Entity gqlWrapper `json:"entity"`
					} `json:"items"`
				} `json:"items"`
			}
			if json.Unmarshal(it.Content.Data, &l) == nil {
				for _, e := range l.Items.Items {
					if c, ok := cardFromWrapper(e.Entity); ok {
						sh.Items = append(sh.Items, c)
					}
				}
			}
		case strings.HasSuffix(it.URI, ":collection"):
			sh.Items = append(sh.Items, likedSongsCard)
		default:
			if c, ok := cardFromWrapper(it.Content); ok {
				sh.Items = append(sh.Items, c)
			}
		}
	}
	return sh, len(sh.Items) > 0
}

func (c *Client) homeVars() map[string]any {
	return map[string]any{
		"timeZone":                       localTimeZone(),
		"sp_t":                           "",
		"homeEndUserIntegration":         "INTEGRATION_DESKTOP",
		"includeHomeChapterVideoCards":   false,
		"includeEpisodeContentRatingsV2": false,
	}
}

// Home returns the personalized home feed. facet is "" or a chip id
// ("music-chip", "podcasts-chip"...).
func (c *Client) Home(ctx context.Context, facet string) (*Home, error) {
	vars := c.homeVars()
	vars["facet"] = facet
	vars["sectionItemsLimit"] = 10
	var resp struct {
		Home struct {
			Greeting         *gqlLabel `json:"greeting"`
			SectionContainer struct {
				Sections struct {
					Items []gqlHomeSection `json:"items"`
				} `json:"sections"`
			} `json:"sectionContainer"`
			HomeChips []struct {
				ID    string   `json:"id"`
				Label gqlLabel `json:"label"`
			} `json:"homeChips"`
		} `json:"home"`
	}
	if err := c.query(ctx, "home", vars, 5*time.Minute, &resp); err != nil {
		return nil, err
	}
	h := resp.Home
	out := &Home{Chips: []HomeChip{}, Sections: []Shelf{}}
	if h.Greeting != nil {
		out.Greeting = h.Greeting.TransformedLabel
	}
	for _, ch := range h.HomeChips {
		out.Chips = append(out.Chips, HomeChip{ID: ch.ID, Label: ch.Label.TransformedLabel})
	}
	for _, s := range h.SectionContainer.Sections.Items {
		if sh, ok := s.shelf(); ok {
			out.Sections = append(out.Sections, sh)
		}
	}
	return out, nil
}

// HomeSection returns all items of one home shelf ("Show all").
func (c *Client) HomeSection(ctx context.Context, uri string) (*Shelf, error) {
	vars := c.homeVars()
	vars["uri"] = uri
	vars["sectionItemsOffset"] = 0
	vars["sectionItemsLimit"] = 50
	var resp struct {
		HomeSections struct {
			Sections []gqlHomeSection `json:"sections"`
		} `json:"homeSections"`
	}
	if err := c.query(ctx, "homeSection", vars, 5*time.Minute, &resp); err != nil {
		return nil, err
	}
	for _, s := range resp.HomeSections.Sections {
		if sh, ok := s.shelf(); ok {
			return &sh, nil
		}
	}
	return nil, &Error{Op: "homeSection", Err: errNotFound}
}

// localTimeZone returns the IANA zone name (pathfinder rejects "Local").
func localTimeZone() string {
	if tz := strings.TrimPrefix(os.Getenv("TZ"), ":"); tz != "" {
		return tz
	}
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		if _, name, ok := strings.Cut(target, "zoneinfo/"); ok {
			return name
		}
	}
	return "UTC"
}
