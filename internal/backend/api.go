package backend

import (
	"context"
	"time"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// Internal API bindings. Each returns a typed result or an error the UI shows
// as "unavailable" for that section.

func (a *App) call() (context.Context, context.CancelFunc) {
	return context.WithTimeout(a.ctx, 30*time.Second)
}

func (a *App) Me() (*spotify.Me, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Me(ctx)
}

func (a *App) Library(q spotify.LibraryQuery) (*spotify.Library, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Library(ctx, q)
}

func (a *App) Playlist(uri string, offset, limit int) (*spotify.Playlist, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Playlist(ctx, uri, offset, limit)
}

func (a *App) Album(uri string) (*spotify.Album, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Album(ctx, uri)
}

func (a *App) LikedSongs(offset, limit int) (*spotify.TrackPage, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.LikedSongs(ctx, offset, limit)
}

func (a *App) SetSaved(uris []string, saved bool) error {
	if a.sandboxed("SetSaved", uris, saved) {
		return nil
	}
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.SetSaved(ctx, uris, saved)
}

func (a *App) AreSaved(uris []string) ([]bool, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.AreSaved(ctx, uris)
}

func (a *App) SetPinned(uri string, pinned bool) error {
	if a.sandboxed("SetPinned", uri, pinned) {
		return nil
	}
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.SetPinned(ctx, uri, pinned)
}

// CoverColor returns the header color Spotify extracts from a cover.
func (a *App) CoverColor(url string) (string, error) {
	if v, ok := a.coverColors.Load(url); ok {
		return v.(string), nil
	}
	ctx, cancel := a.call()
	defer cancel()
	hex, err := a.sp.ImageColor(ctx, url)
	if err == nil {
		a.coverColors.Store(url, hex)
	}
	return hex, err
}

// ImageColor returns the dominant color of an image for header gradients.
func (a *App) ImageColor(url string) (string, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.img.DominantColor(ctx, url)
}

func (a *App) Home(facet string) (*spotify.Home, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Home(ctx, facet)
}

func (a *App) HomeSection(uri string) (*spotify.Shelf, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.HomeSection(ctx, uri)
}

func (a *App) Profile(username string) (*spotify.Profile, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Profile(ctx, username)
}

// ProfileUsers lists a profile's followers or following.
func (a *App) ProfileUsers(username, kind string) ([]spotify.Card, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.ProfileUsers(ctx, username, kind)
}

func (a *App) Friends() ([]spotify.Friend, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Friends(ctx, time.Now().UnixMilli())
}

func (a *App) BrowseAll() ([]spotify.Category, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.BrowseAll(ctx)
}

func (a *App) Browse(uri string) (*spotify.BrowsePage, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Browse(ctx, uri)
}

func (a *App) Search(term string) (*spotify.SearchResults, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Search(ctx, term)
}

func (a *App) SearchSuggestions(term string) ([]spotify.Suggestion, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.SearchSuggestions(ctx, term)
}

func (a *App) SearchType(term, kind string, offset, limit int) (*spotify.SearchPage, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.SearchType(ctx, term, kind, offset, limit)
}

func (a *App) RecentSearches() ([]spotify.Card, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.RecentSearches(ctx)
}

func (a *App) SaveRecentSearch(uri string) error {
	if a.sandboxed("SaveRecentSearch", uri) {
		return nil
	}
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.SaveRecentSearch(ctx, uri)
}

func (a *App) RemoveRecentSearches(uris []string) error {
	if a.sandboxed("RemoveRecentSearches", uris) {
		return nil
	}
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.RemoveRecentSearches(ctx, uris)
}

func (a *App) Artist(uri string) (*spotify.ArtistPage, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Artist(ctx, uri)
}

// ArtistShelf is the "Show all" page of one of an artist's shelves.
func (a *App) ArtistShelf(uri, kind string) (*spotify.Shelf, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.ArtistShelf(ctx, uri, kind)
}

func (a *App) Discography(uri, kind string, byName bool, offset, limit int) (*spotify.ReleasePage, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Discography(ctx, uri, kind, byName, offset, limit)
}

// SetFollowing follows or unfollows a user profile.
func (a *App) SetFollowing(userURI string, follow bool) error {
	if a.sandboxed("SetFollowing", userURI, follow) {
		return nil
	}
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.SetFollowing(ctx, userURI, follow)
}

func (a *App) EditablePlaylists(uris []string) ([]spotify.EditablePlaylist, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.EditablePlaylists(ctx, uris)
}

// PlaylistTracks is every track URI in the user's own (editable) playlists,
// for the check the official client shows on songs saved in one.
func (a *App) PlaylistTracks() (map[string]bool, error) {
	pls, err := a.EditablePlaylists(nil)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, pl := range pls {
		if pl.Type != "playlist" {
			continue // Liked Songs: that's Saved
		}
		for off := 0; ; off += 100 {
			p, err := a.Playlist(pl.URI, off, 100)
			if err != nil {
				return nil, err
			}
			for _, t := range p.Tracks.Items {
				out[t.URI] = true
				if t.LinkedURI != "" {
					out[t.LinkedURI] = true
				}
			}
			if off+100 >= p.Tracks.Total || len(p.Tracks.Items) == 0 {
				break
			}
		}
	}
	return out, nil
}

// InviteLink creates a collaborator invite link for a playlist.
func (a *App) InviteLink(playlist string) (string, error) {
	if a.sandboxed("InviteLink", playlist) {
		return "https://open.spotify.com/playlist/sandbox?pt=sandbox", nil
	}
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.InviteLink(ctx, playlist)
}

func (a *App) AddToPlaylist(playlist string, uris []string) error {
	if a.sandboxed("AddToPlaylist", playlist, uris) {
		return nil
	}
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.AddToPlaylist(ctx, playlist, uris, spotify.Position{})
}

func (a *App) RemoveFromPlaylist(playlist string, uids []string) error {
	if a.sandboxed("RemoveFromPlaylist", playlist, uids) {
		return nil
	}
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.RemoveFromPlaylist(ctx, playlist, uids)
}

// MovePlaylistItems moves items (by uid) before or after another item.
func (a *App) MovePlaylistItems(playlist string, uids []string, pos spotify.Position) error {
	if a.sandboxed("MovePlaylistItems", playlist, uids, pos) {
		return nil
	}
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.MoveInPlaylist(ctx, playlist, uids, pos)
}

func (a *App) CreatePlaylist(name string) (string, error) {
	if a.sandboxed("CreatePlaylist", name) {
		return "spotify:playlist:sandbox", nil
	}
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.CreatePlaylist(ctx, name)
}

func (a *App) RenamePlaylist(uri, name, description string) error {
	if a.sandboxed("RenamePlaylist", uri, name, description) {
		return nil
	}
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.RenamePlaylist(ctx, uri, name, &description)
}

func (a *App) DeletePlaylist(uri string) error {
	if a.sandboxed("DeletePlaylist", uri) {
		return nil
	}
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.DeletePlaylist(ctx, uri)
}

func (a *App) Lyrics(trackURI string) (*spotify.Lyrics, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Lyrics(ctx, trackURI)
}

// Canvas returns the URL of the track's looping video ("" if none).
func (a *App) Canvas(trackURI string) (string, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Canvas(ctx, trackURI)
}

// ImageFile returns the local path of a cached image, downloading it on a miss.
func (a *App) ImageFile(ctx context.Context, url string) (string, error) {
	return a.img.File(ctx, url)
}

func (a *App) NowPlaying(trackURI, artistURI string) (*spotify.NowPlayingInfo, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.NowPlaying(ctx, trackURI, artistURI)
}

// StartRadio plays the radio playlist seeded from uri and returns its URI.
func (a *App) StartRadio(seedURI string) (string, error) {
	ctx, cancel := a.call()
	defer cancel()
	if a.sandboxed("StartRadio", seedURI) {
		return "", nil
	}
	pl, err := a.sp.Radio(ctx, seedURI)
	if err != nil {
		return "", err
	}
	return pl, a.Play(pl, "")
}

func (a *App) Show(uri string) (*spotify.Show, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Show(ctx, uri)
}

func (a *App) ShowEpisodes(uri string, offset, limit int) ([]spotify.Episode, error) {
	ctx, cancel := a.call()
	defer cancel()
	eps, _, err := a.sp.ShowEpisodes(ctx, uri, offset, limit)
	return eps, err
}

func (a *App) Episode(uri string) (*spotify.Episode, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Episode(ctx, uri)
}

func (a *App) YourEpisodes() ([]spotify.Episode, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.YourEpisodes(ctx)
}

func (a *App) RecentlyPlayed() ([]spotify.Track, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.RecentlyPlayed(ctx)
}

// PlaylistRecommendations is the "Recommended" list under a playlist.
func (a *App) PlaylistRecommendations(playlist string, skip []string, n int) ([]spotify.Track, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.PlaylistRecommendations(ctx, playlist, skip, n)
}

func (a *App) TasteExcluded(uris []string) ([]bool, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.TasteExcluded(ctx, uris)
}

func (a *App) SetTasteExcluded(uri string, excluded bool) error {
	if a.sandboxed("SetTasteExcluded", uri, excluded) {
		return nil
	}
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.SetTasteExcluded(ctx, uri, excluded)
}

// WhatsNew is the feed behind the top bar's bell (kinds: "ALBUM", "EPISODE").
func (a *App) WhatsNew(kinds []string) ([]spotify.FeedItem, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.WhatsNew(ctx, kinds)
}
