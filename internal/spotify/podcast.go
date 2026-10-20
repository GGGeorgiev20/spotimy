package spotify

import (
	"context"
	"strings"
	"time"

	collectionpb "github.com/devgianlu/go-librespot/proto/spotify/collection/v2"
	"google.golang.org/protobuf/proto"
)

type Episode struct {
	URI         string  `json:"uri"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Images      []Image `json:"images"`
	ShowURI     string  `json:"showUri"`
	ShowName    string  `json:"showName"`
	DurationMs  int     `json:"durationMs"`
	ReleaseDate string  `json:"releaseDate"`
	PlayedMs    int     `json:"playedMs"` // resume point
	State       string  `json:"state"`    // NOT_STARTED, IN_PROGRESS, COMPLETED
	Explicit    bool    `json:"explicit"`
	Playable    bool    `json:"playable"`
}

type Show struct {
	URI         string    `json:"uri"`
	Name        string    `json:"name"`
	Publisher   string    `json:"publisher"`
	Description string    `json:"description"`
	Images      []Image   `json:"images"`
	Saved       bool      `json:"saved"`
	Total       int       `json:"total"`
	Episodes    []Episode `json:"episodes"`
}

type gqlEpisode struct {
	URI           string                          `json:"uri"`
	Name          string                          `json:"name"`
	Description   string                          `json:"description"`
	CoverArt      *gqlSources                     `json:"coverArt"`
	Duration      struct{ TotalMilliseconds int } `json:"duration"`
	ReleaseDate   *gqlDate                        `json:"releaseDate"`
	ContentRating struct {
		Label string `json:"label"`
	} `json:"contentRating"`
	Playability struct {
		Playable bool `json:"playable"`
	} `json:"playability"`
	PlayedState *struct {
		PlayPositionMilliseconds int    `json:"playPositionMilliseconds"`
		State                    string `json:"state"`
	} `json:"playedState"`
	PodcastV2 *struct {
		Data struct {
			URI  string `json:"uri"`
			Name string `json:"name"`
		} `json:"data"`
	} `json:"podcastV2"`
}

func (e gqlEpisode) episode() Episode {
	ep := Episode{URI: e.URI, Name: e.Name, Description: e.Description, Images: e.CoverArt.images(),
		DurationMs: e.Duration.TotalMilliseconds, Explicit: e.ContentRating.Label == "EXPLICIT",
		Playable: e.Playability.Playable, State: "NOT_STARTED"}
	if e.ReleaseDate != nil {
		ep.ReleaseDate = e.ReleaseDate.ISOString
	}
	if p := e.PlayedState; p != nil {
		ep.PlayedMs, ep.State = p.PlayPositionMilliseconds, p.State
	}
	if e.PodcastV2 != nil {
		ep.ShowURI, ep.ShowName = e.PodcastV2.Data.URI, e.PodcastV2.Data.Name
	}
	return ep
}

// Show returns a podcast with its first page of episodes (newest first).
func (c *Client) Show(ctx context.Context, uri string) (*Show, error) {
	var meta struct {
		PodcastUnionV2 struct {
			Typename    string     `json:"__typename"`
			URI         string     `json:"uri"`
			Name        string     `json:"name"`
			Description string     `json:"description"`
			Saved       bool       `json:"saved"`
			CoverArt    gqlSources `json:"coverArt"`
			Publisher   struct {
				Name string `json:"name"`
			} `json:"publisher"`
		} `json:"podcastUnionV2"`
	}
	if err := c.query(ctx, "queryShowMetadataV2", map[string]any{"uri": uri}, 10*time.Minute, &meta); err != nil {
		return nil, err
	}
	m := meta.PodcastUnionV2
	if m.URI == "" {
		return nil, &Error{Op: "queryShowMetadataV2", Err: errNotFound}
	}
	page, total, err := c.ShowEpisodes(ctx, uri, 0, 50)
	if err != nil {
		return nil, err
	}
	return &Show{URI: m.URI, Name: m.Name, Publisher: m.Publisher.Name, Description: m.Description,
		Images: m.CoverArt.images(), Saved: m.Saved, Total: total, Episodes: page}, nil
}

func (c *Client) ShowEpisodes(ctx context.Context, uri string, offset, limit int) ([]Episode, int, error) {
	var resp struct {
		PodcastUnionV2 struct {
			EpisodesV2 struct {
				TotalCount int `json:"totalCount"`
				Items      []struct {
					Entity struct {
						Data gqlEpisode `json:"data"`
					} `json:"entity"`
				} `json:"items"`
			} `json:"episodesV2"`
		} `json:"podcastUnionV2"`
	}
	if err := c.query(ctx, "queryPodcastEpisodes", map[string]any{"uri": uri, "offset": offset, "limit": limit}, 2*time.Minute, &resp); err != nil {
		return nil, 0, err
	}
	out := []Episode{}
	for _, it := range resp.PodcastUnionV2.EpisodesV2.Items {
		if it.Entity.Data.URI != "" {
			out = append(out, it.Entity.Data.episode())
		}
	}
	return out, resp.PodcastUnionV2.EpisodesV2.TotalCount, nil
}

func (c *Client) Episode(ctx context.Context, uri string) (*Episode, error) {
	var resp struct {
		EpisodeUnionV2 gqlEpisode `json:"episodeUnionV2"`
	}
	if err := c.query(ctx, "getEpisodeOrChapter", map[string]any{"uri": uri}, time.Minute, &resp); err != nil {
		return nil, err
	}
	if resp.EpisodeUnionV2.URI == "" {
		return nil, &Error{Op: "getEpisodeOrChapter", Err: errNotFound}
	}
	ep := resp.EpisodeUnionV2.episode()
	return &ep, nil
}

// YourEpisodes lists saved episodes (the "listenlater" collection set),
// newest saves first.
func (c *Client) YourEpisodes(ctx context.Context) ([]Episode, error) {
	user, err := c.username()
	if err != nil {
		return nil, &Error{Op: "collection listenlater", Err: err}
	}
	var uris []string
	token := ""
	for range 10 { // ponytail: caps at 10 pages (~500 episodes)
		body, _ := proto.Marshal(&collectionpb.PageRequest{Username: user, Set: "listenlater", PaginationToken: token, Limit: 50})
		resp, err := c.do(ctx, request{op: "collection listenlater", method: "POST", url: c.spclientURL + "/collection/v2/paging", body: body,
			contentType: "application/vnd.collection-v2.spotify.proto", accept: "application/vnd.collection-v2.spotify.proto", idempotent: true})
		if err != nil {
			return nil, err
		}
		var page collectionpb.PageResponse
		if err := proto.Unmarshal(resp, &page); err != nil {
			return nil, &Error{Op: "collection listenlater", Err: err}
		}
		for _, it := range page.Items {
			if strings.HasPrefix(it.Uri, "spotify:episode:") {
				uris = append(uris, it.Uri)
			}
		}
		token = page.NextPageToken
		if token == "" {
			break
		}
	}
	out := []Episode{}
	for i := 0; i < len(uris); i += 50 {
		var resp struct {
			Episodes []gqlEpisode `json:"episodes"`
		}
		if err := c.query(ctx, "decorateContextEpisodesOrChapters", map[string]any{"uris": uris[i:min(i+50, len(uris))]}, time.Minute, &resp); err != nil {
			return out, err
		}
		for _, e := range resp.Episodes {
			out = append(out, e.episode())
		}
	}
	return out, nil
}
