package spotify

import (
	"context"
	"encoding/json"
	"time"
)

// Category is a "Browse all" tile (a genre or mood page).
type Category struct {
	URI    string  `json:"uri"`
	Name   string  `json:"name"`
	Color  string  `json:"color"`
	Images []Image `json:"images"`
}

// BrowsePage is a genre/mood page: shelves plus, for some, sub-categories.
type BrowsePage struct {
	Title      string     `json:"title"`
	Color      string     `json:"color"`
	Categories []Category `json:"categories"`
	Sections   []Shelf    `json:"sections"`
}

type gqlBrowseSection struct {
	URI  string `json:"uri"`
	Data struct {
		Typename string    `json:"__typename"`
		Title    *gqlLabel `json:"title"`
	} `json:"data"`
	SectionItems struct {
		TotalCount int `json:"totalCount"`
		Items      []struct {
			URI     string     `json:"uri"`
			Content gqlWrapper `json:"content"`
		} `json:"items"`
	} `json:"sectionItems"`
}

type gqlTile struct {
	Data struct {
		CardRepresentation struct {
			Artwork         *gqlSources `json:"artwork"`
			BackgroundColor struct {
				Hex string `json:"hex"`
			} `json:"backgroundColor"`
			Title gqlLabel `json:"title"`
		} `json:"cardRepresentation"`
	} `json:"data"`
}

func (s gqlBrowseSection) categories() []Category {
	out := []Category{}
	for _, it := range s.SectionItems.Items {
		if it.Content.Typename != "BrowseSectionContainerWrapper" {
			continue // feature links (Live Events) have no page of their own
		}
		var t gqlTile
		if json.Unmarshal(it.Content.Data, &t) != nil {
			continue
		}
		cr := t.Data.CardRepresentation
		out = append(out, Category{URI: it.URI, Name: cr.Title.TransformedLabel, Color: cr.BackgroundColor.Hex, Images: cr.Artwork.images()})
	}
	return out
}

func browseVars() map[string]any {
	return map[string]any{
		"pagePagination":           map[string]int{"offset": 0, "limit": 10},
		"sectionPagination":        map[string]int{"offset": 0, "limit": 99},
		"browseEndUserIntegration": "INTEGRATION_WEB_PLAYER",
	}
}

// BrowseAll returns the "Browse all" tiles shown on an empty search.
func (c *Client) BrowseAll(ctx context.Context) ([]Category, error) {
	var resp struct {
		BrowseStart struct {
			Sections struct {
				Items []gqlBrowseSection `json:"items"`
			} `json:"sections"`
		} `json:"browseStart"`
	}
	if err := c.query(ctx, "browseAll", browseVars(), time.Hour, &resp); err != nil {
		return nil, err
	}
	out := []Category{}
	for _, s := range resp.BrowseStart.Sections.Items {
		if s.Data.Typename == "BrowseGridSectionData" {
			out = append(out, s.categories()...)
		}
	}
	return out, nil
}

// Browse returns one genre/mood page.
func (c *Client) Browse(ctx context.Context, uri string) (*BrowsePage, error) {
	vars := browseVars()
	vars["uri"] = uri
	vars["sectionPagination"] = map[string]int{"offset": 0, "limit": 10}
	var resp struct {
		Browse struct {
			Header struct {
				Color struct {
					Hex string `json:"hex"`
				} `json:"color"`
				Title gqlLabel `json:"title"`
			} `json:"header"`
			Sections struct {
				Items []gqlBrowseSection `json:"items"`
			} `json:"sections"`
		} `json:"browse"`
	}
	if err := c.query(ctx, "browsePage", vars, 30*time.Minute, &resp); err != nil {
		return nil, err
	}
	b := resp.Browse
	out := &BrowsePage{Title: b.Header.Title.TransformedLabel, Color: b.Header.Color.Hex, Categories: []Category{}, Sections: []Shelf{}}
	for _, s := range b.Sections.Items {
		switch s.Data.Typename {
		case "BrowseGridSectionData":
			out.Categories = append(out.Categories, s.categories()...)
		case "BrowseGenericSectionData":
			sh := Shelf{URI: s.URI, Kind: "shelf", Total: s.SectionItems.TotalCount}
			if s.Data.Title != nil {
				sh.Title = s.Data.Title.TransformedLabel
			}
			for _, it := range s.SectionItems.Items {
				if card, ok := cardFromWrapper(it.Content); ok {
					sh.Items = append(sh.Items, card)
				}
			}
			if len(sh.Items) > 0 {
				out.Sections = append(out.Sections, sh)
			}
		}
	}
	return out, nil
}
