package spotify

import (
	"context"
	"net/url"
	"strings"
)

type recentContexts struct {
	PlayContexts []struct {
		URI           string `json:"uri"`
		LastPlayedURI string `json:"lastPlayedTrackUri"`
	} `json:"playContexts"`
}

// lastTracks lists each context's last played track, newest first, once each.
func (r recentContexts) lastTracks() []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range r.PlayContexts {
		u := c.LastPlayedURI
		if strings.HasPrefix(u, "spotify:track:") && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// RecentlyPlayed returns recently played tracks (the queue's "Recently
// played" tab): the last track of each recently played context.
func (c *Client) RecentlyPlayed(ctx context.Context) ([]Track, error) {
	me := c.UserURI()
	if me == "" {
		m, err := c.Me(ctx)
		if err != nil {
			return nil, err
		}
		me = m.URI
	}
	user := me[strings.LastIndex(me, ":")+1:]
	var resp recentContexts
	path := "/recently-played/v3/user/" + url.PathEscape(user) + "/recently-played?format=json&offset=0&limit=50&filter=default,collection-new-episodes"
	if err := c.getJSON(ctx, "recently-played", path, &resp); err != nil {
		return nil, err
	}
	return c.Tracks(ctx, resp.lastTracks())
}
