package spotify

import (
	"context"
	"encoding/json"
	"time"
)

// FeedItem is one entry of the What's New feed: a new release from a
// followed artist (Episode == nil) or a new episode of a followed show.
type FeedItem struct {
	ID        string   `json:"id"`
	New       bool     `json:"new"`
	URI       string   `json:"uri"`
	Name      string   `json:"name"`
	Images    []Image  `json:"images"`
	Date      string   `json:"date"` // ISO release date
	AlbumType string   `json:"albumType,omitempty"`
	Artists   []Artist `json:"artists,omitempty"`
	Episode   *Episode `json:"episode,omitempty"`
}

type gqlFeedItem struct {
	ID    string `json:"id"`
	State struct {
		State     string `json:"state"` // NEW, SEEN
		Timestamp *struct {
			ISOString string `json:"isoString"`
		} `json:"timestamp"`
	} `json:"state"`
	Content gqlWrapper `json:"content"`
}

// isNew follows the web player: new until seen, and for an hour after.
func (i gqlFeedItem) isNew(now time.Time) bool {
	if i.State.Timestamp == nil || i.State.Timestamp.ISOString == "" {
		return i.State.State == "NEW"
	}
	t, err := time.Parse(time.RFC3339, i.State.Timestamp.ISOString)
	return err == nil && now.Sub(t) <= time.Hour
}

func (i gqlFeedItem) item(now time.Time) (FeedItem, bool) {
	out := FeedItem{ID: i.ID, New: i.isNew(now)}
	switch i.Content.Typename {
	case "AlbumResponseWrapper":
		var a gqlEntity
		if json.Unmarshal(i.Content.Data, &a) != nil || a.Typename != "Album" {
			return out, false
		}
		out.URI, out.Name, out.Images, out.Artists = a.URI, a.Name, a.CoverArt.images(), a.Artists.list()
		out.AlbumType = albumTypeLabel(a.AlbumType, a.Type)
		if a.Date != nil {
			out.Date = a.Date.ISOString
		}
	case "EpisodeOrChapterResponseWrapper":
		var e gqlEpisode
		if json.Unmarshal(i.Content.Data, &e) != nil || e.URI == "" || e.PodcastV2 == nil {
			return out, false // chapters (audiobooks) aren't shown
		}
		ep := e.episode()
		out.URI, out.Name, out.Images, out.Date, out.Episode = ep.URI, ep.Name, ep.Images, ep.ReleaseDate, &ep
	default:
		return out, false
	}
	return out, out.URI != ""
}

// WhatsNew returns the feed (newest first). kinds filters it: "ALBUM",
// "EPISODE"; none = everything.
func (c *Client) WhatsNew(ctx context.Context, kinds []string) ([]FeedItem, error) {
	if kinds == nil {
		kinds = []string{}
	}
	var resp struct {
		Feed struct {
			Items []gqlFeedItem `json:"items"`
		} `json:"whatsNewFeedItems"`
	}
	err := c.query(ctx, "queryWhatsNewFeed", map[string]any{
		"offset": 0, "limit": 50, "onlyUnPlayedItems": false, "includedContentTypes": kinds,
		"includeEpisodeContentRatingsV2": true,
	}, time.Minute, &resp)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := []FeedItem{}
	for _, it := range resp.Feed.Items {
		if f, ok := it.item(now); ok {
			out = append(out, f)
		}
	}
	return out, nil
}
