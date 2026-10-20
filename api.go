package main

import (
	"context"
	"net/url"
	"time"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.SetPinned(ctx, uri, pinned)
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

func (a *App) Search(term string) (*spotify.SearchResults, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Search(ctx, term)
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
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.SaveRecentSearch(ctx, uri)
}

func (a *App) RemoveRecentSearches(uris []string) error {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.RemoveRecentSearches(ctx, uris)
}

func (a *App) Artist(uri string) (*spotify.ArtistPage, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Artist(ctx, uri)
}

func (a *App) Discography(uri, kind string, offset, limit int) (*spotify.SearchPage, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Discography(ctx, uri, kind, offset, limit)
}

func (a *App) EditablePlaylists(uris []string) ([]spotify.EditablePlaylist, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.EditablePlaylists(ctx, uris)
}

func (a *App) AddToPlaylist(playlist string, uris []string) error {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.AddToPlaylist(ctx, playlist, uris, spotify.Position{})
}

func (a *App) RemoveFromPlaylist(playlist string, uids []string) error {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.RemoveFromPlaylist(ctx, playlist, uids)
}

// MovePlaylistItems moves items (by uid) before or after another item.
func (a *App) MovePlaylistItems(playlist string, uids []string, pos spotify.Position) error {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.MoveInPlaylist(ctx, playlist, uids, pos)
}

func (a *App) CreatePlaylist(name string) (string, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.CreatePlaylist(ctx, name)
}

func (a *App) RenamePlaylist(uri, name, description string) error {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.RenamePlaylist(ctx, uri, name, &description)
}

func (a *App) DeletePlaylist(uri string) error {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.DeletePlaylist(ctx, uri)
}

func (a *App) CopyText(text string) {
	runtime.ClipboardSetText(a.ctx, text)
}

func (a *App) Lyrics(trackURI string) (*spotify.Lyrics, error) {
	ctx, cancel := a.call()
	defer cancel()
	return a.sp.Lyrics(ctx, trackURI)
}

// Canvas returns a local URL for the track's looping video ("" if none).
// The video is served through the disk cache like images.
func (a *App) Canvas(trackURI string) (string, error) {
	ctx, cancel := a.call()
	defer cancel()
	u, err := a.sp.Canvas(ctx, trackURI)
	if err != nil || u == "" {
		return "", err
	}
	return "/img?u=" + url.QueryEscape(u), nil
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
