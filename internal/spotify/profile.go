package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Profile is a user's profile page.
type Profile struct {
	URI             string  `json:"uri"`
	Name            string  `json:"name"`
	Images          []Image `json:"images"`
	Color           string  `json:"color"`
	Followers       int     `json:"followers"`
	Following       int     `json:"following"`
	PublicPlaylists int     `json:"publicPlaylists"`
	IsMe            bool    `json:"isMe"`
	CanFollow       bool    `json:"canFollow"`   // not themselves and they allow followers
	IsFollowing     bool    `json:"isFollowing"` // the listener follows them
	Playlists       []Card  `json:"playlists"`
	// Only for the current user ("Top artists/tracks this month").
	TopArtists    []Card  `json:"topArtists"`
	TopTracks     []Track `json:"topTracks"`
	FollowerList  []Card  `json:"followerList"`
	FollowingList []Card  `json:"followingList"`
}

type profileUser struct {
	URI      string `json:"uri"`
	Name     string `json:"name"`
	ImageURL string `json:"image_url"`
}

func (u profileUser) card() Card {
	return Card{URI: u.URI, Type: "user", Name: u.Name, Subtitle: "Profile", Images: imageFromURL(u.ImageURL)}
}

// imageFromURL resolves the image forms user-profile-view returns: plain
// URLs, spotify:image:<id> and spotify:mosaic:<id>:<id>:...
func imageFromURL(s string) []Image {
	switch {
	case s == "":
		return []Image{}
	case strings.HasPrefix(s, "spotify:image:"):
		s = "https://i.scdn.co/image/" + strings.TrimPrefix(s, "spotify:image:")
	case strings.HasPrefix(s, "spotify:mosaic:"):
		s = "https://mosaic.scdn.co/300/" + strings.ReplaceAll(strings.TrimPrefix(s, "spotify:mosaic:"), ":", "")
	}
	return []Image{{URL: s}}
}

// Profile loads a user profile from user-profile-view, plus the listener's
// own top artists and tracks when it's their profile.
func (c *Client) Profile(ctx context.Context, username string) (*Profile, error) {
	var resp struct {
		URI             string `json:"uri"`
		Name            string `json:"name"`
		ImageURL        string `json:"image_url"`
		Color           int    `json:"color"`
		Followers       int    `json:"followers_count"`
		Following       int    `json:"following_count"`
		TotalPlaylists  int    `json:"total_public_playlists_count"`
		IsCurrentUser   bool   `json:"is_current_user"`
		AllowFollows    *bool  `json:"allow_follows"`
		PublicPlaylists []struct {
			URI       string `json:"uri"`
			Name      string `json:"name"`
			ImageURL  string `json:"image_url"`
			OwnerName string `json:"owner_name"`
			OwnerURI  string `json:"owner_uri"`
			Followers int    `json:"followers_count"`
		} `json:"public_playlists"`
	}
	path := "/user-profile-view/v3/profile/" + url.PathEscape(username) + "?playlist_limit=50&artist_limit=10&episode_limit=10&market=from_token"
	if err := c.getJSON(ctx, "profile", path, &resp); err != nil {
		return nil, err
	}
	p := &Profile{
		URI: resp.URI, Name: resp.Name, Color: fmt.Sprintf("#%06x", resp.Color&0xffffff),
		Followers: resp.Followers, Following: resp.Following, PublicPlaylists: resp.TotalPlaylists,
		IsMe: resp.IsCurrentUser, Images: imageFromURL(resp.ImageURL), Playlists: []Card{},
		TopArtists: []Card{}, TopTracks: []Track{}, FollowerList: []Card{}, FollowingList: []Card{},
	}
	for _, pl := range resp.PublicPlaylists {
		card := Card{URI: pl.URI, Type: "playlist", Name: pl.Name, Subtitle: "By " + pl.OwnerName, Images: imageFromURL(pl.ImageURL)}
		if pl.OwnerURI == resp.URI {
			card.Subtitle = groupDigits(pl.Followers) + " Followers"
			if pl.Followers == 1 {
				card.Subtitle = "1 Follower"
			}
		}
		p.Playlists = append(p.Playlists, card)
	}
	for kind, list := range map[string]*[]Card{"followers": &p.FollowerList, "following": &p.FollowingList} {
		if cards, err := c.ProfileUsers(ctx, username, kind); err != nil {
			c.log.Warn("profile list unavailable", "kind", kind, "err", err)
		} else {
			*list = cards
		}
	}
	if p.IsMe {
		// Optional: the page still works without it.
		if err := c.topContent(ctx, p); err != nil {
			c.log.Warn("top content unavailable", "err", err)
		}
	} else if p.CanFollow = resp.AllowFollows == nil || *resp.AllowFollows; p.CanFollow {
		if f, err := c.IsFollowing(ctx, resp.URI); err != nil {
			c.log.Warn("follow state unavailable", "err", err)
		} else {
			p.IsFollowing = f
		}
	}
	return p, nil
}

// IsFollowing reports whether the listener follows the user.
func (c *Client) IsFollowing(ctx context.Context, userURI string) (bool, error) {
	var resp struct {
		Users []struct {
			Following bool `json:"following"`
		} `json:"users"`
	}
	if err := c.query(ctx, "isFollowingUsers", map[string]any{"uris": []string{userURI}}, 0, &resp); err != nil {
		return false, err
	}
	return len(resp.Users) > 0 && resp.Users[0].Following, nil
}

// SetFollowing follows or unfollows a user (spotify:user:<name>).
func (c *Client) SetFollowing(ctx context.Context, userURI string, follow bool) error {
	op := "unfollowUsers"
	if follow {
		op = "followUsers"
	}
	var resp map[string]struct {
		Responses []struct {
			Result bool `json:"result"`
		} `json:"responses"`
	}
	vars := map[string]any{"usernames": []string{strings.TrimPrefix(userURI, "spotify:user:")}}
	if err := c.mutateInto(ctx, op, vars, &resp); err != nil {
		return err
	}
	if r := resp[op].Responses; len(r) == 0 || !r[0].Result {
		return &Error{Op: op, Err: errors.New("not applied")}
	}
	return nil
}

// ProfileUsers lists the profiles that follow username (kind "followers") or
// that they follow ("following").
func (c *Client) ProfileUsers(ctx context.Context, username, kind string) ([]Card, error) {
	var r struct {
		Profiles []profileUser `json:"profiles"`
	}
	if err := c.getJSON(ctx, "profile "+kind, "/user-profile-view/v3/profile/"+url.PathEscape(username)+"/"+kind+"?market=from_token", &r); err != nil {
		return nil, err
	}
	cards := make([]Card, len(r.Profiles))
	for i, u := range r.Profiles {
		cards[i] = u.card()
	}
	return cards, nil
}

func (c *Client) topContent(ctx context.Context, p *Profile) error {
	in := func(limit int) map[string]any {
		return map[string]any{"offset": 0, "limit": limit, "sortBy": "AFFINITY", "timeRange": "SHORT_TERM"}
	}
	var resp struct {
		Me struct {
			Profile struct {
				TopArtists struct {
					Items []gqlWrapper `json:"items"`
				} `json:"topArtists"`
				TopTracks struct {
					Items []struct {
						Data gqlTrack `json:"data"`
					} `json:"items"`
				} `json:"topTracks"`
			} `json:"profile"`
		} `json:"me"`
	}
	err := c.query(ctx, "userTopContent", map[string]any{
		"includeTopArtists": true, "topArtistsInput": in(10),
		"includeTopTracks": true, "topTracksInput": in(4),
	}, 30*time.Minute, &resp)
	if err != nil {
		return err
	}
	for _, w := range resp.Me.Profile.TopArtists.Items {
		if card, ok := cardFromWrapper(w); ok {
			p.TopArtists = append(p.TopArtists, card)
		}
	}
	for _, t := range resp.Me.Profile.TopTracks.Items {
		p.TopTracks = append(p.TopTracks, t.Data.track())
	}
	return nil
}

// groupDigits formats n with thousands separators: 1234567 -> 1,234,567.
func groupDigits(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0 && s[i-1] != '-'; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
