package spotify

import (
	"context"
	"encoding/json"
	"strings"
)

// Recommendations are the "Recommended" songs under a playlist
// (spclient playlistextender, what the web player calls).

type extTrack struct {
	ID         string `json:"id"`
	OriginalID string `json:"originalId"`
	Name       string `json:"name"`
	Artists    []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		ImageURL string `json:"imageUrl"` // 300px
		Large    string `json:"largeImageUrl"`
	} `json:"album"`
	Duration int  `json:"duration"`
	Explicit bool `json:"explicit"`
}

func (e extTrack) track() Track {
	uri := e.OriginalID
	if !strings.HasPrefix(uri, "spotify:track:") {
		uri = "spotify:track:" + e.ID
	}
	t := Track{URI: uri, Name: e.Name, DurationMs: e.Duration, Explicit: e.Explicit, Playable: true}
	for _, a := range e.Artists {
		t.Artists = append(t.Artists, Artist{URI: "spotify:artist:" + a.ID, Name: a.Name})
	}
	al := &AlbumRef{URI: "spotify:album:" + e.Album.ID, Name: e.Album.Name}
	if e.Album.ImageURL != "" {
		al.Images = append(al.Images, Image{e.Album.ImageURL, 300, 300})
	}
	if e.Album.Large != "" {
		al.Images = append(al.Images, Image{e.Album.Large, 640, 640})
	}
	t.Album = al
	return t
}

// PlaylistRecommendations returns n songs that fit playlist, never the ones
// in skip (track URIs, e.g. what the list already shows).
func (c *Client) PlaylistRecommendations(ctx context.Context, playlist string, skip []string, n int) ([]Track, error) {
	ids := make([]string, len(skip))
	for i, u := range skip {
		ids[i] = u[strings.LastIndex(u, ":")+1:]
	}
	body, _ := json.Marshal(map[string]any{"playlistURI": playlist, "trackSkipIDs": ids, "numResults": n})
	const op = "playlistextender"
	resp, err := c.do(ctx, request{
		op: op, method: "POST", url: c.spclientURL + "/playlistextender/extendp/", body: body,
		contentType: "application/json", accept: "application/json", idempotent: true,
	})
	if err != nil {
		return nil, err
	}
	var r struct {
		RecommendedTracks []extTrack `json:"recommendedTracks"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return nil, &Error{Op: op, Err: err}
	}
	out := make([]Track, 0, len(r.RecommendedTracks))
	for _, e := range r.RecommendedTracks {
		out = append(out, e.track())
	}
	return out, nil
}
