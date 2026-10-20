package spotify

import (
	"context"
	"os"
	"strings"
	"time"
)

type Shelf struct {
	URI      string `json:"uri"`
	Title    string `json:"title"`
	Kind     string `json:"kind"` // "shortcuts" (top grid) or "shelf"
	Items    []Card `json:"items"`
	Total    int    `json:"total"`
	Subtitle string `json:"subtitle,omitempty"`
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
	TransformedLabel string `json:"transformedLabel"`
}

type gqlHomeSection struct {
	URI  string `json:"uri"`
	Data struct {
		Typename string    `json:"__typename"`
		Title    *gqlLabel `json:"title"`
		Subtitle *gqlLabel `json:"subtitle"`
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
	case "HomeGenericSectionData", "HomeSpotlightSectionData":
	default:
		// Recents (trait entities in a list), mobile-only baseline cards, etc.
		return sh, false
	}
	if s.Data.Title != nil {
		sh.Title = s.Data.Title.TransformedLabel
	}
	if s.Data.Subtitle != nil {
		sh.Subtitle = s.Data.Subtitle.TransformedLabel
	}
	for _, it := range s.SectionItems.Items {
		if it.Content.URI == "" {
			it.Content.URI = it.URI
		}
		if c, ok := cardFromWrapper(it.Content); ok {
			sh.Items = append(sh.Items, c)
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
