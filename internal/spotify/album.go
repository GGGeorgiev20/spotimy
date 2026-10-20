package spotify

import (
	"strings"
	"context"
	"time"
)

// Album fetches an album with up to 300 tracks (more than any real album).
func (c *Client) Album(ctx context.Context, uri string) (*Album, error) {
	var resp struct {
		AlbumUnion struct {
			Typename  string     `json:"__typename"`
			URI       string     `json:"uri"`
			Name      string     `json:"name"`
			Type      string     `json:"type"`
			Label     string     `json:"label"`
			Saved     bool       `json:"saved"`
			Date      gqlDate    `json:"date"`
			Artists   gqlArtists `json:"artists"`
			CoverArt  gqlSources `json:"coverArt"`
			Copyright struct {
				Items []struct {
					Text string `json:"text"`
					Type string `json:"type"`
				} `json:"items"`
			} `json:"copyright"`
			TracksV2 struct {
				TotalCount int `json:"totalCount"`
				Items      []struct {
					UID   string   `json:"uid"`
					Track gqlTrack `json:"track"`
				} `json:"items"`
			} `json:"tracksV2"`
			MoreAlbumsByArtist struct {
				Items []struct {
					Discography struct {
						PopularReleasesAlbums struct {
							Items []struct {
								URI      string     `json:"uri"`
								Name     string     `json:"name"`
								Type     string     `json:"type"`
								Date     gqlDate    `json:"date"`
								CoverArt gqlSources `json:"coverArt"`
							} `json:"items"`
						} `json:"popularReleasesAlbums"`
					} `json:"discography"`
				} `json:"items"`
			} `json:"moreAlbumsByArtist"`
		} `json:"albumUnion"`
	}
	err := c.query(ctx, "getAlbum", map[string]any{"uri": uri, "locale": "", "offset": 0, "limit": 300}, 5*time.Minute, &resp)
	if err != nil {
		return nil, err
	}
	a := resp.AlbumUnion
	if a.Typename != "Album" {
		return nil, &Error{Op: "getAlbum", Err: errNotFound}
	}
	out := &Album{
		URI: a.URI, Name: a.Name, Type: a.Type, Label: a.Label, Saved: a.Saved,
		Date: a.Date.ISOString, Artists: a.Artists.list(), Images: a.CoverArt.images(),
		Total: a.TracksV2.TotalCount, Tracks: []Track{}, More: []Card{}, Copyright: []string{},
	}
	for _, c := range a.Copyright.Items {
		sym := "©"
		if c.Type == "P" {
			sym = "℗"
		}
		text := c.Text
		if !strings.HasPrefix(text, sym) { // some labels already include it
			text = sym + " " + text
		}
		out.Copyright = append(out.Copyright, text)
	}
	ref := &AlbumRef{URI: a.URI, Name: a.Name, Images: out.Images}
	for _, it := range a.TracksV2.Items {
		t := it.Track.track()
		t.UID, t.Album = it.UID, ref
		out.Tracks = append(out.Tracks, t)
	}
	for _, m := range a.MoreAlbumsByArtist.Items {
		for _, x := range m.Discography.PopularReleasesAlbums.Items {
			if x.URI == a.URI {
				continue
			}
			out.More = append(out.More, Card{URI: x.URI, Type: "album", Name: x.Name, Subtitle: year(x.Date.ISOString), Images: x.CoverArt.images()})
		}
	}
	return out, nil
}

func year(iso string) string {
	if len(iso) >= 4 {
		return iso[:4]
	}
	return ""
}

// Tracks returns metadata for track URIs, in order (missing ones are skipped).
func (c *Client) Tracks(ctx context.Context, uris []string) ([]Track, error) {
	out := []Track{}
	for i := 0; i < len(uris); i += 50 {
		chunk := uris[i:min(i+50, len(uris))]
		var resp struct {
			Tracks []gqlTrack `json:"tracks"`
		}
		if err := c.query(ctx, "decorateContextTracks", map[string]any{"uris": chunk}, 10*time.Minute, &resp); err != nil {
			return out, err
		}
		for _, t := range resp.Tracks {
			if tr := t.track(); tr.URI != "" {
				out = append(out, tr)
			}
		}
	}
	return out, nil
}
