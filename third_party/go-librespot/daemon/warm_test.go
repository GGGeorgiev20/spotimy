//go:build test_unit

package daemon

import (
	"context"
	"fmt"
	"testing"

	librespot "github.com/devgianlu/go-librespot"
	"github.com/devgianlu/go-librespot/audio"
	"github.com/devgianlu/go-librespot/player"
	connectpb "github.com/devgianlu/go-librespot/proto/spotify/connectstate"
	metadatapb "github.com/devgianlu/go-librespot/proto/spotify/metadata"
	"github.com/devgianlu/go-librespot/tracks"
	"github.com/stretchr/testify/require"
)

// A warm stream goes to the load of its own track, once.
func TestTakeWarm(t *testing.T) {
	p := newTestAppPlayer(t)
	want, err := librespot.SpotifyIdFromUri("spotify:track:574y1r7o2tRA009FW0LE7v")
	require.NoError(t, err)
	other, err := librespot.SpotifyIdFromUri("spotify:track:1Vej0qeQ3ioKwpI6FUbRv1")
	require.NoError(t, err)

	w := &warmStream{uri: want.Uri(), done: make(chan struct{}), stream: &player.Stream{RequestedId: *want}}
	close(w.done)
	p.warm.cur = w

	take := func(id *librespot.SpotifyId) (*player.Stream, error) { return p.takeWarm(context.Background(), *id) }
	s, err := take(other)
	require.NoError(t, err)
	require.Nil(t, s, "another track's load leaves it")
	s, _ = take(want)
	require.Same(t, w.stream, s)
	s, _ = take(want)
	require.Nil(t, s, "taken once")

	// A refused key is passed on rather than asked for again.
	refused := &warmStream{uri: want.Uri(), done: make(chan struct{}), err: &audio.KeyProviderError{Code: 2}}
	close(refused.done)
	p.warm.cur = refused
	_, err = take(want)
	require.Error(t, err)
}

// A throttled key refusal stops playback instead of skipping through the
// context, which would ask for a key per track.
func TestThrottledKeyIsNotUnplayable(t *testing.T) {
	require.False(t, isUnplayableMedia(fmt.Errorf("load: %w", &audio.KeyProviderError{Code: 2})))
	require.True(t, isUnplayableMedia(fmt.Errorf("load: %w", &audio.KeyProviderError{Code: 1})))
}

// A queue edit keeps the prefetched stream when its track still plays next.
func TestInvalidateUpcomingForKeepsSameNext(t *testing.T) {
	p := newTestAppPlayer(t)
	id, err := librespot.SpotifyIdFromUri("spotify:track:574y1r7o2tRA009FW0LE7v")
	require.NoError(t, err)
	stream := &player.Stream{RequestedId: *id, Media: librespot.NewMediaFromTrack(&metadatapb.Track{Gid: id.Id()})}

	p.secondaryStream = stream
	p.invalidateUpcomingFor(&tracks.Snapshot{Next: []*connectpb.ProvidedTrack{{Uri: id.Uri()}}})
	require.Same(t, stream, p.secondaryStream)

	p.invalidateUpcomingFor(&tracks.Snapshot{Next: []*connectpb.ProvidedTrack{{Uri: "spotify:track:1Vej0qeQ3ioKwpI6FUbRv1"}}})
	require.Nil(t, p.secondaryStream)
}
