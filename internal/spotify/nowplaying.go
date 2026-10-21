package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type LyricLine struct {
	StartMs int64  `json:"startMs"`
	Words   string `json:"words"`
}

type Lyrics struct {
	Synced   bool        `json:"synced"` // false: unsynced text, no highlighting
	Lines    []LyricLine `json:"lines"`
	Provider string      `json:"provider"`
}

func trackID(uri string) string { return uri[strings.LastIndex(uri, ":")+1:] }

// Lyrics returns time-synced lyrics from spclient color-lyrics, or nil when
// the track has none.
func (c *Client) Lyrics(ctx context.Context, trackURI string) (*Lyrics, error) {
	path := "/color-lyrics/v2/track/" + url.PathEscape(trackID(trackURI)) + "?format=json&vocalRemoval=false&market=from_token"
	key := "lyrics:" + trackURI
	body, ok := c.cache.get(key)
	if !ok {
		var err error
		body, err = c.do(ctx, request{op: "color-lyrics", method: "GET", url: c.spclientURL + path, accept: "application/json", idempotent: true})
		var e *Error
		if errors.As(err, &e) && e.Status == 404 {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
		if len(body) == 0 { // 204: no lyrics
			return nil, nil
		}
		c.cache.put(key, body, time.Hour)
	}
	var resp struct {
		Lyrics struct {
			SyncType string `json:"syncType"`
			Provider string `json:"providerDisplayName"`
			Lines    []struct {
				StartTimeMs string `json:"startTimeMs"`
				Words       string `json:"words"`
			} `json:"lines"`
		} `json:"lyrics"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &Error{Op: "color-lyrics", Err: err}
	}
	l := &Lyrics{Synced: resp.Lyrics.SyncType == "LINE_SYNCED", Provider: resp.Lyrics.Provider, Lines: []LyricLine{}}
	for _, ln := range resp.Lyrics.Lines {
		ms, _ := strconv.ParseInt(ln.StartTimeMs, 10, 64)
		l.Lines = append(l.Lines, LyricLine{StartMs: ms, Words: ln.Words})
	}
	return l, nil
}

// Canvas returns the looping video URL for a track, or "" if it has none.
func (c *Client) Canvas(ctx context.Context, trackURI string) (string, error) {
	var resp struct {
		TrackUnion struct {
			Canvas *struct {
				URL  string `json:"url"`
				Type string `json:"type"`
			} `json:"canvas"`
		} `json:"trackUnion"`
	}
	if err := c.query(ctx, "canvas", map[string]any{"trackUri": trackURI}, time.Hour, &resp); err != nil {
		return "", err
	}
	if cv := resp.TrackUnion.Canvas; cv != nil && strings.HasPrefix(cv.Type, "VIDEO") {
		return cv.URL, nil
	}
	return "", nil
}

type Credit struct {
	Name  string `json:"name"`
	URI   string `json:"uri"`
	Roles string `json:"roles"` // "Composer, Lyricist"
}

type NowPlayingInfo struct {
	Artist  *ArtistCard `json:"artist,omitempty"`
	Credits []Credit    `json:"credits"`
}

type ArtistCard struct {
	URI              string  `json:"uri"`
	Name             string  `json:"name"`
	Images           []Image `json:"images"` // avatar
	Gallery          []Image `json:"gallery"`
	Header           []Image `json:"header"` // the artist page's banner (the card's picture)
	MonthlyListeners int     `json:"monthlyListeners"`
	Bio              string  `json:"bio"`
	Verified         bool    `json:"verified"`
}

// NowPlaying fetches the Now Playing view extras: the main artist card and
// the track credits.
func (c *Client) NowPlaying(ctx context.Context, trackURI, artistURI string) (*NowPlayingInfo, error) {
	out := &NowPlayingInfo{Credits: []Credit{}}
	var npv struct {
		ArtistUnion struct {
			URI     string `json:"uri"`
			Profile struct {
				Name      string `json:"name"`
				Verified  bool   `json:"verified"`
				Biography struct {
					Text string `json:"text"`
				} `json:"biography"`
			} `json:"profile"`
			Stats struct {
				MonthlyListeners int `json:"monthlyListeners"`
			} `json:"stats"`
			Visuals struct {
				AvatarImage *gqlSources `json:"avatarImage"`
				Gallery     gqlImages   `json:"gallery"`
			} `json:"visuals"`
			HeaderImage *struct {
				Data gqlSources `json:"data"`
			} `json:"headerImage"`
			OnPlatformReputationTrait struct {
				Verification struct {
					IsVerified bool `json:"isVerified"`
				} `json:"verification"`
			} `json:"onPlatformReputationTrait"`
		} `json:"artistUnion"`
	}
	vars := map[string]any{"artistUri": artistURI, "trackUri": trackURI, "enableRelatedVideos": false, "enableRelatedAudioTracks": false}
	if artistURI != "" {
		if err := c.query(ctx, "queryNpvArtist", vars, 30*time.Minute, &npv); err == nil {
			a := npv.ArtistUnion
			out.Artist = &ArtistCard{URI: a.URI, Name: a.Profile.Name,
				Verified: a.Profile.Verified || a.OnPlatformReputationTrait.Verification.IsVerified,
				Images:   a.Visuals.AvatarImage.images(), Gallery: a.Visuals.Gallery.first(),
				MonthlyListeners: a.Stats.MonthlyListeners, Bio: stripHTML(a.Profile.Biography.Text)}
			if a.HeaderImage != nil {
				out.Artist.Header = a.HeaderImage.Data.images()
			}
		}
	}
	var cr struct {
		TrackUnion struct {
			CreditsTrait struct {
				Contributors struct {
					Items []struct {
						Name string `json:"name"`
						Role string `json:"role"`
						URI  string `json:"uri"`
					} `json:"items"`
				} `json:"contributors"`
			} `json:"creditsTrait"`
		} `json:"trackUnion"`
	}
	if err := c.query(ctx, "queryTrackCreditsGroupedModal", map[string]any{"trackUri": trackURI}, 30*time.Minute, &cr); err != nil {
		return out, err
	}
	idx := map[string]int{}
	for _, it := range cr.TrackUnion.CreditsTrait.Contributors.Items {
		if i, ok := idx[it.Name]; ok {
			out.Credits[i].Roles += ", " + it.Role
			continue
		}
		idx[it.Name] = len(out.Credits)
		out.Credits = append(out.Credits, Credit{Name: it.Name, URI: it.URI, Roles: it.Role})
	}
	return out, nil
}

// Radio returns the radio playlist Spotify generates for a seed (track,
// artist, album or playlist URI) — "Go to song radio".
func (c *Client) Radio(ctx context.Context, seedURI string) (string, error) {
	var resp struct {
		MediaItems []struct {
			URI string `json:"uri"`
		} `json:"mediaItems"`
	}
	if err := c.getJSON(ctx, "inspiredby-mix", "/inspiredby-mix/v2/seed_to_playlist/"+url.PathEscape(seedURI)+"?response-format=json", &resp); err != nil {
		return "", err
	}
	if len(resp.MediaItems) == 0 {
		return "", &Error{Op: "inspiredby-mix", Err: errNotFound}
	}
	return resp.MediaItems[0].URI, nil
}
