package main

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
