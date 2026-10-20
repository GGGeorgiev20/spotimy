package spotify

import (
	"context"
	"time"
)

type gqlPlaylistItem struct {
	UID     string   `json:"uid"`
	AddedAt *gqlDate `json:"addedAt"`
	AddedBy *struct {
		Data gqlUser `json:"data"`
	} `json:"addedBy"`
	ItemV2 struct {
		Typename string   `json:"__typename"`
		Data     gqlTrack `json:"data"`
	} `json:"itemV2"`
}

type gqlPlaylist struct {
	URI         string    `json:"uri"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Followers   int       `json:"followers"`
	Following   bool      `json:"following"`
	RevisionID  string    `json:"revisionId"`
	Images      gqlImages `json:"images"`
	OwnerV2     struct {
		Data gqlUser `json:"data"`
	} `json:"ownerV2"`
	CurrentUserCapabilities struct {
		CanEditItems bool `json:"canEditItems"`
	} `json:"currentUserCapabilities"`
	Members struct {
		TotalCount int `json:"totalCount"`
		Items      []struct {
			IsOwner bool `json:"isOwner"`
			User    struct {
				Data gqlUser `json:"data"`
			} `json:"user"`
		} `json:"items"`
	} `json:"members"`
	Content struct {
		TotalCount int               `json:"totalCount"`
		Items      []gqlPlaylistItem `json:"items"`
	} `json:"content"`
}

func playlistTracks(items []gqlPlaylistItem) []Track {
	out := make([]Track, 0, len(items))
	for _, it := range items {
		t := it.ItemV2.Data.track()
		t.UID = it.UID
		if it.AddedAt != nil {
			t.AddedAt = it.AddedAt.ISOString
		}
		if it.AddedBy != nil && it.AddedBy.Data.URI != "" {
			u := it.AddedBy.Data.user()
			t.AddedBy = &u
		}
		if t.URI == "" { // unavailable / removed item: keep the row so uids stay aligned
			t.Name = "Unavailable"
		}
		out = append(out, t)
	}
	return out
}

// Playlist fetches a playlist with one page of its items.
func (c *Client) Playlist(ctx context.Context, uri string, offset, limit int) (*Playlist, error) {
	var resp struct {
		PlaylistV2 gqlPlaylist `json:"playlistV2"`
	}
	err := c.query(ctx, "fetchPlaylist", map[string]any{
		"uri": uri, "offset": offset, "limit": limit, "enableWatchFeedEntrypoint": false,
	}, 30*time.Second, &resp)
	if err != nil {
		return nil, err
	}
	p := resp.PlaylistV2
	if p.URI == "" {
		return nil, &Error{Op: "fetchPlaylist", Err: errNotFound}
	}
	owner := p.OwnerV2.Data.user()
	out := &Playlist{
		URI:         p.URI,
		Name:        p.Name,
		Description: p.Description,
		Owner:       owner,
		Images:      p.Images.first(),
		Followers:   p.Followers,
		Following:   p.Following,
		CanEdit:     p.CurrentUserCapabilities.CanEditItems,
		Collab:      p.Members.TotalCount > 1,
		RevisionID:  p.RevisionID,
		Tracks:      TrackPage{Total: p.Content.TotalCount, Offset: offset, Items: playlistTracks(p.Content.Items)},
	}
	out.Members = []User{}
	for _, m := range p.Members.Items {
		if !m.IsOwner {
			out.Members = append(out.Members, m.User.Data.user())
		}
	}
	out.IsOwner = owner.URI != "" && owner.URI == c.UserURI()
	return out, nil
}

// InvalidatePlaylists drops cached playlist pages after an edit.
func (c *Client) InvalidatePlaylists() { c.cache.invalidate("pf:fetchPlaylist:") }

// Position says where items go: "start", "end", or before/after a uid.
type Position struct {
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	Start  bool   `json:"start,omitempty"`
}

func (p Position) gql() map[string]any {
	switch {
	case p.Start:
		return map[string]any{"moveType": "TOP_OF_PLAYLIST", "fromUid": nil}
	case p.Before != "":
		return map[string]any{"moveType": "BEFORE_UID", "fromUid": p.Before}
	case p.After != "":
		return map[string]any{"moveType": "AFTER_UID", "fromUid": p.After}
	}
	return map[string]any{"moveType": "BOTTOM_OF_PLAYLIST", "fromUid": nil}
}

func (c *Client) AddToPlaylist(ctx context.Context, playlist string, uris []string, pos Position) error {
	defer c.InvalidatePlaylists()
	return c.mutate(ctx, "addToPlaylist", map[string]any{
		"playlistUri": playlist, "playlistItemUris": uris, "newPosition": pos.gql(), "interactionId": nil,
	})
}

func (c *Client) RemoveFromPlaylist(ctx context.Context, playlist string, uids []string) error {
	defer c.InvalidatePlaylists()
	return c.mutate(ctx, "removeFromPlaylist", map[string]any{"playlistUri": playlist, "uids": uids})
}

func (c *Client) MoveInPlaylist(ctx context.Context, playlist string, uids []string, pos Position) error {
	defer c.InvalidatePlaylists()
	return c.mutate(ctx, "moveItemsInPlaylist", map[string]any{"playlistUri": playlist, "uids": uids, "newPosition": pos.gql()})
}

// EditablePlaylist is a playlist the user can add to; Contains reports
// whether it already has every item that was asked about.
type EditablePlaylist struct {
	Card
	Contains bool `json:"contains"`
}

// EditablePlaylists lists the user's writable playlists for "Add to playlist".
func (c *Client) EditablePlaylists(ctx context.Context, uris []string) ([]EditablePlaylist, error) {
	if uris == nil {
		uris = []string{}
	}
	var resp struct {
		Me struct {
			EditablePlaylists struct {
				Items []struct {
					Curates bool       `json:"curates"`
					Item    gqlWrapper `json:"item"`
				} `json:"items"`
			} `json:"editablePlaylists"`
		} `json:"me"`
	}
	vars := map[string]any{"offset": 0, "limit": 100, "textFilter": "", "folderUri": nil, "uris": uris}
	if err := c.query(ctx, "editablePlaylists", vars, 0, &resp); err != nil {
		return nil, err
	}
	out := []EditablePlaylist{}
	for _, it := range resp.Me.EditablePlaylists.Items {
		if card, ok := cardFromWrapper(it.Item); ok {
			out = append(out, EditablePlaylist{Card: card, Contains: it.Curates})
		}
	}
	return out, nil
}
