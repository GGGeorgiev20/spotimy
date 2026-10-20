package spotify

import "strings"

// Types returned to the UI. They are flattened from pathfinder's GraphQL
// shapes so nothing outside this package sees Spotify's schema.

type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Artist struct {
	URI    string  `json:"uri"`
	Name   string  `json:"name"`
	Images []Image `json:"images,omitempty"`
}

type AlbumRef struct {
	URI    string  `json:"uri"`
	Name   string  `json:"name"`
	Images []Image `json:"images,omitempty"`
}

type User struct {
	URI    string  `json:"uri"`
	Name   string  `json:"name"`
	Images []Image `json:"images,omitempty"`
}

type Track struct {
	URI         string    `json:"uri"`
	UID         string    `json:"uid,omitempty"`       // position-specific id inside a playlist/album
	LinkedURI   string    `json:"linkedUri,omitempty"` // the relinked original; Liked Songs may hold this one
	Name        string    `json:"name"`
	Artists     []Artist  `json:"artists"`
	Album       *AlbumRef `json:"album,omitempty"`
	DurationMs  int       `json:"durationMs"`
	Explicit    bool      `json:"explicit"`
	Playable    bool      `json:"playable"`
	AddedAt     string    `json:"addedAt,omitempty"`
	AddedBy     *User     `json:"addedBy,omitempty"`
	Playcount   string    `json:"playcount,omitempty"`
	TrackNumber int       `json:"trackNumber,omitempty"`
	DiscNumber  int       `json:"discNumber,omitempty"`
	Episode     bool      `json:"episode,omitempty"`
}

// TrackPage is one page of a long track list.
type TrackPage struct {
	Total  int     `json:"total"`
	Offset int     `json:"offset"`
	Items  []Track `json:"items"`
}

type Playlist struct {
	URI         string    `json:"uri"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Owner       User      `json:"owner"`
	Images      []Image   `json:"images"`
	Color       string    `json:"color,omitempty"` // header color extracted from the cover
	Followers   int       `json:"followers"`
	Following   bool      `json:"following"`
	CanEdit     bool      `json:"canEdit"`
	IsOwner     bool      `json:"isOwner"`
	Collab      bool      `json:"collaborative"`
	Members     []User    `json:"members"` // collaborators other than the owner
	RevisionID  string    `json:"revisionId"`
	Tracks      TrackPage `json:"tracks"`
}

type Album struct {
	URI       string   `json:"uri"`
	Name      string   `json:"name"`
	Type      string   `json:"type"` // ALBUM, SINGLE, EP, COMPILATION
	Artists   []Artist `json:"artists"`
	Images    []Image  `json:"images"`
	Color     string   `json:"color,omitempty"` // header color extracted from the cover
	Date      string   `json:"date"`            // ISO; precision may be year only
	Label     string   `json:"label"`
	Copyright []string `json:"copyright"`
	Saved     bool     `json:"saved"`
	Tracks    []Track  `json:"tracks"`
	Total     int      `json:"total"`
	More      []Card   `json:"more,omitempty"` // "More by <artist>"
}

// Card is a generic tile: playlists, albums, artists, shows in shelves.
type Card struct {
	URI      string  `json:"uri"`
	Type     string  `json:"type"` // playlist, album, artist, show, episode, user, collection, folder
	Name     string  `json:"name"`
	Subtitle string  `json:"subtitle"`
	Images   []Image `json:"images"`
}

type LibraryItem struct {
	Card
	Pinned   bool   `json:"pinned"`
	AddedAt  string `json:"addedAt,omitempty"`
	PlayedAt string `json:"playedAt,omitempty"`
	Owner    string `json:"owner,omitempty"` // for the Creator sort
	Count    int    `json:"count,omitempty"` // tracks in a pseudo-playlist, items in a folder
}

type Library struct {
	Total   int           `json:"total"`
	Items   []LibraryItem `json:"items"`
	Filters []string      `json:"filters"` // what this user's library can be filtered by
	Sorts   []string      `json:"sorts"`
	Sort    string        `json:"sort"` // the sort applied
}

// URIType returns the entity type of a spotify: URI ("playlist", "album"...).
// spotify:user:x:collection maps to "collection".
func URIType(uri string) string {
	parts := strings.Split(uri, ":")
	switch {
	case len(parts) < 2:
		return ""
	case parts[len(parts)-1] == "collection" || parts[1] == "collection":
		return "collection"
	case len(parts) == 3:
		return parts[1]
	case len(parts) > 3 && parts[1] == "user":
		return parts[3] // spotify:user:x:playlist:id (legacy)
	}
	return parts[1]
}
