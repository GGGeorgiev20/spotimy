package spotify

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// LibraryQuery mirrors the official client's library filters. Filters are the
// ids Spotify returns in availableFilters ("Playlists", "Albums", "Artists",
// "Podcasts & Shows"...); Sort is one of "Recents", "Recently Added",
// "Alphabetical", "Creator".
type LibraryQuery struct {
	Filters    []string `json:"filters"`
	Sort       string   `json:"sort"`
	TextFilter string   `json:"textFilter"`
	Offset     int      `json:"offset"`
	Limit      int      `json:"limit"`
	FolderURI  string   `json:"folderUri"`
}

type libraryV3Resp struct {
	Me struct {
		LibraryV3 struct {
			TotalCount       int      `json:"totalCount"`
			AvailableFilters []idName `json:"availableFilters"`
			AvailableSorts   []idName `json:"availableSortOrders"`
			SelectedSort     idName   `json:"selectedSortOrder"`
			Items            []struct {
				AddedAt  *gqlDate `json:"addedAt"`
				PlayedAt *gqlDate `json:"playedAt"`
				Pinned   bool     `json:"pinned"`
				Item     struct {
					Typename string          `json:"__typename"`
					URI      string          `json:"_uri"`
					Data     json.RawMessage `json:"data"`
				} `json:"item"`
			} `json:"items"`
		} `json:"libraryV3"`
	} `json:"me"`
}

type idName struct {
	ID string `json:"id"`
}

func ids(xs []idName) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = x.ID
	}
	return out
}

// libraryEntity is the union of the entity shapes libraryV3 can return.
type libraryEntity struct {
	Typename string      `json:"__typename"`
	URI      string      `json:"uri"`
	Name     string      `json:"name"`
	Count    int         `json:"count"`    // PseudoPlaylist
	Image    *gqlSources `json:"image"`    // PseudoPlaylist
	Images   gqlImages   `json:"images"`   // Playlist
	CoverArt *gqlSources `json:"coverArt"` // Album, Podcast
	OwnerV2  *struct {
		Data gqlUser `json:"data"`
	} `json:"ownerV2"`
	Artists gqlArtists `json:"artists"` // Album
	Profile *struct {
		Name string `json:"name"`
	} `json:"profile"` // Artist
	Visuals *struct {
		AvatarImage *gqlSources `json:"avatarImage"`
	} `json:"visuals"` // Artist
	Publisher *struct {
		Name string `json:"name"`
	} `json:"publisher"` // Podcast
	PlaylistCount int `json:"playlistCount"` // Folder
	FolderCount   int `json:"folderCount"`   // Folder
}

func (e libraryEntity) item(uri string) LibraryItem {
	if e.URI != "" {
		uri = e.URI
	}
	it := LibraryItem{Card: Card{URI: uri, Name: e.Name, Type: URIType(uri)}}
	switch e.Typename {
	case "PseudoPlaylist":
		it.Images = e.Image.images()
		it.Count = e.Count
		it.Subtitle = "Playlist"
	case "Playlist":
		it.Images = e.Images.first()
		if e.OwnerV2 != nil {
			it.Owner = e.OwnerV2.Data.user().Name
		}
		it.Subtitle = "Playlist • " + it.Owner
	case "Album":
		it.Images = e.CoverArt.images()
		it.Owner = e.Artists.names()
		it.Subtitle = "Album • " + it.Owner
	case "Artist":
		if e.Profile != nil {
			it.Name = e.Profile.Name
		}
		if e.Visuals != nil {
			it.Images = e.Visuals.AvatarImage.images()
		}
		it.Subtitle = "Artist"
	case "Podcast", "Audiobook":
		it.Images = e.CoverArt.images()
		if e.Publisher != nil {
			it.Owner = e.Publisher.Name
		}
		it.Subtitle = "Podcast • " + it.Owner
	case "Folder":
		it.Type = "folder"
		it.Count = e.PlaylistCount + e.FolderCount
		it.Subtitle = plural(e.PlaylistCount, "playlist")
	}
	return it
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

// Library returns one page of Your Library, pinned items first, the way the
// left panel shows it.
func (c *Client) Library(ctx context.Context, q LibraryQuery) (*Library, error) {
	if q.Limit == 0 {
		q.Limit = 50
	}
	if q.Filters == nil {
		q.Filters = []string{}
	}
	vars := map[string]any{
		"filters":                      q.Filters,
		"order":                        nil,
		"textFilter":                   q.TextFilter,
		"features":                     []string{"LIKED_SONGS", "YOUR_EPISODES", "PRERELEASES"},
		"limit":                        q.Limit,
		"offset":                       q.Offset,
		"flatten":                      false,
		"expandedFolders":              []string{},
		"folderUri":                    nil,
		"includeFoldersWhenFlattening": true,
	}
	if q.Sort != "" {
		vars["order"] = q.Sort
	}
	if q.FolderURI != "" {
		vars["folderUri"] = q.FolderURI
	}
	var resp libraryV3Resp
	if err := c.query(ctx, "libraryV3", vars, 30*time.Second, &resp); err != nil {
		return nil, err
	}
	lib := resp.Me.LibraryV3
	out := &Library{
		Total: lib.TotalCount, Items: make([]LibraryItem, 0, len(lib.Items)),
		Filters: ids(lib.AvailableFilters), Sorts: ids(lib.AvailableSorts), Sort: lib.SelectedSort.ID,
	}
	for _, raw := range lib.Items {
		var e libraryEntity
		if len(raw.Item.Data) > 0 {
			_ = json.Unmarshal(raw.Item.Data, &e) // unknown shapes still get a URI row
		}
		it := e.item(raw.Item.URI)
		if it.URI == "" {
			continue
		}
		if strings.HasPrefix(raw.Item.Typename, "NotFound") || raw.Item.Typename == "GenericError" {
			continue
		}
		it.Pinned = raw.Pinned
		if raw.AddedAt != nil {
			it.AddedAt = raw.AddedAt.ISOString
		}
		if raw.PlayedAt != nil {
			it.PlayedAt = raw.PlayedAt.ISOString
		}
		out.Items = append(out.Items, it)
	}
	return out, nil
}

// InvalidateLibrary drops cached library pages after a change.
func (c *Client) InvalidateLibrary() { c.cache.invalidate("pf:libraryV3:") }

// SetSaved adds or removes items (tracks, albums, artists, shows, episodes,
// playlists) from the library.
func (c *Client) SetSaved(ctx context.Context, uris []string, saved bool) error {
	op := "removeFromLibrary"
	if saved {
		op = "addToLibrary"
	}
	err := c.mutate(ctx, op, map[string]any{"libraryItemUris": uris})
	c.InvalidateLibrary()
	c.cache.invalidate("pf:fetchLibraryTracks:")
	return err
}

// AreSaved reports whether each uri is in the library.
func (c *Client) AreSaved(ctx context.Context, uris []string) ([]bool, error) {
	var resp struct {
		Lookup []struct {
			Data struct {
				Saved bool `json:"saved"`
			} `json:"data"`
		} `json:"lookup"`
	}
	if err := c.query(ctx, "areEntitiesInLibrary", map[string]any{"uris": uris}, 0, &resp); err != nil {
		return nil, err
	}
	out := make([]bool, len(uris))
	for i := range out {
		if i < len(resp.Lookup) {
			out[i] = resp.Lookup[i].Data.Saved
		}
	}
	return out, nil
}

// SetPinned pins or unpins a library item.
func (c *Client) SetPinned(ctx context.Context, uri string, pinned bool) error {
	op := "unpinLibraryItem"
	if pinned {
		op = "pinLibraryItem"
	}
	err := c.mutate(ctx, op, map[string]any{"pinnableItemUri": uri})
	c.InvalidateLibrary()
	return err
}

// LikedSongs returns a page of the user's Liked Songs.
func (c *Client) LikedSongs(ctx context.Context, offset, limit int) (*TrackPage, error) {
	var resp struct {
		Me struct {
			Library struct {
				Tracks struct {
					TotalCount int `json:"totalCount"`
					Items      []struct {
						AddedAt gqlDate `json:"addedAt"`
						Track   struct {
							URI  string   `json:"_uri"`
							Data gqlTrack `json:"data"`
						} `json:"track"`
					} `json:"items"`
				} `json:"tracks"`
			} `json:"library"`
		} `json:"me"`
	}
	if err := c.query(ctx, "fetchLibraryTracks", map[string]any{"offset": offset, "limit": limit}, time.Minute, &resp); err != nil {
		return nil, err
	}
	t := resp.Me.Library.Tracks
	page := &TrackPage{Total: t.TotalCount, Offset: offset, Items: make([]Track, 0, len(t.Items))}
	for _, it := range t.Items {
		tr := it.Track.Data.track()
		if tr.URI == "" {
			tr.URI = it.Track.URI
		}
		tr.AddedAt = it.AddedAt.ISOString
		page.Items = append(page.Items, tr)
	}
	return page, nil
}
