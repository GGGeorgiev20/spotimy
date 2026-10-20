package spotify

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Friend is one entry of Friend Activity: what a followed user last played.
type Friend struct {
	User      Card  `json:"user"`
	Track     Card  `json:"track"`  // subtitle is the artist
	Artist    Card  `json:"artist"` // name and uri only
	Album     Card  `json:"album"`
	Context   Card  `json:"context"`
	Timestamp int64 `json:"timestamp"` // unix ms
	Playing   bool  `json:"playing"`   // the friend is playing it right now (server flag)
}

// feedEntry is one row of listening-activity/v1/feed: bare URIs, no metadata.
type feedEntry struct {
	Username string
	Track    string
	Context  string
	Playing  bool
	TimeMs   int64
}

// parseFeed reads the feed response. Entries without activity or a user URI
// are dropped, the same as the official client does.
func parseFeed(body []byte) ([]feedEntry, error) {
	var resp struct {
		Entities []struct {
			UserEntity   *feedEntity `json:"userEntity"`
			FollowEntity *feedEntity `json:"followEntity"`
		} `json:"entities"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	var out []feedEntry
	for _, e := range resp.Entities {
		ent := e.UserEntity
		if ent == nil {
			ent = e.FollowEntity
		}
		if ent == nil || ent.Activity == nil || URIType(ent.URI) != "user" {
			continue
		}
		a := ent.Activity
		fe := feedEntry{Username: ent.URI[strings.LastIndex(ent.URI, ":")+1:], Track: a.EntityURI, Context: a.ContextURI, Playing: a.IsPlaying}
		if t, err := time.Parse(time.RFC3339Nano, a.Timestamp); err == nil {
			fe.TimeMs = t.UnixMilli()
		}
		out = append(out, fe)
	}
	return out, nil
}

type feedEntity struct {
	URI      string `json:"uri"`
	Activity *struct {
		EntityURI  string `json:"entityUri"`
		ContextURI string `json:"contextUri"`
		IsPlaying  bool   `json:"isPlaying"`
		Timestamp  string `json:"timestamp"`
	} `json:"activity"`
}

// Friends returns Friend Activity: playing now first, then most recent. Like
// the official client it reads spclient's listening-activity feed (the old
// presence-view/buddylist only knows some friends and goes stale), then
// resolves the bare URIs through pathfinder and user-profile-view. The clock
// argument is unused; the server says who is playing.
func (c *Client) Friends(ctx context.Context, _ int64) ([]Friend, error) {
	body, err := c.do(ctx, request{
		op: "listening-activity feed", method: "POST", url: c.spclientURL + "/listening-activity/v1/feed",
		body: []byte(`{"unused":true,"resultLimit":100}`), contentType: "application/json", accept: "application/json",
		idempotent: true, // a read, despite the verb
	})
	if err != nil {
		return nil, err
	}
	feed, err := parseFeed(body)
	if err != nil {
		return nil, &Error{Op: "listening-activity feed", Err: err}
	}
	if len(feed) == 0 {
		return []Friend{}, nil
	}

	var trackURIs []string
	for _, e := range feed {
		trackURIs = append(trackURIs, e.Track)
	}
	tracks, err := c.Tracks(ctx, trackURIs)
	if err != nil {
		return nil, err
	}
	byURI := map[string]Track{}
	for _, t := range tracks {
		byURI[t.URI] = t
	}

	// Users and playlist contexts are one call each; run them side by side.
	users := make([]Card, len(feed))
	ctxNames := make([]string, len(feed))
	var wg sync.WaitGroup
	for i, e := range feed {
		wg.Add(1)
		go func() {
			defer wg.Done()
			users[i] = c.feedUser(ctx, e.Username)
		}()
		if URIType(e.Context) == "playlist" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if p, err := c.Playlist(ctx, e.Context, 0, 1); err == nil {
					ctxNames[i] = p.Name
				}
			}()
		}
	}
	wg.Wait()

	out := make([]Friend, 0, len(feed))
	for i, e := range feed {
		t, ok := byURI[e.Track]
		if !ok {
			continue
		}
		f := Friend{User: users[i], Timestamp: e.TimeMs, Playing: e.Playing}
		f.Track = Card{URI: t.URI, Type: "track", Name: t.Name}
		if len(t.Artists) > 0 {
			f.Track.Subtitle = t.Artists[0].Name
			f.Artist = Card{URI: t.Artists[0].URI, Type: "artist", Name: t.Artists[0].Name}
		}
		if t.Album != nil {
			f.Track.Images = t.Album.Images
			f.Album = Card{URI: t.Album.URI, Type: "album", Name: t.Album.Name, Images: t.Album.Images}
		}
		f.Context = Card{URI: e.Context, Type: URIType(e.Context), Name: ctxNames[i]}
		switch {
		case e.Context == "":
		case e.Context == f.Album.URI:
			f.Context.Name = f.Album.Name
		case e.Context == f.Artist.URI:
			f.Context.Name = f.Artist.Name
		}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Playing != out[j].Playing {
			return out[i].Playing
		}
		return out[i].Timestamp > out[j].Timestamp
	})
	return out, nil
}

// feedUser resolves a username to a card; on failure the card has only the URI.
func (c *Client) feedUser(ctx context.Context, username string) Card {
	card := Card{URI: "spotify:user:" + username, Type: "user", Name: username}
	var r profileUser
	if err := c.getJSON(ctx, "friend profile", "/user-profile-view/v3/profile/"+url.PathEscape(username)+"?playlist_limit=0&artist_limit=0&episode_limit=0&market=from_token", &r); err == nil && r.Name != "" {
		card.Name, card.Images = r.Name, imageFromURL(r.ImageURL)
	}
	return card
}
