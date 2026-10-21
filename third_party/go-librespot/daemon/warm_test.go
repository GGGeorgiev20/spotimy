//go:build test_unit

package daemon

import (
	"context"
	"testing"

	librespot "github.com/devgianlu/go-librespot"
	"github.com/devgianlu/go-librespot/player"
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

	require.Nil(t, p.takeWarm(context.Background(), *other), "another track's load leaves it")
	require.Same(t, w.stream, p.takeWarm(context.Background(), *want))
	require.Nil(t, p.takeWarm(context.Background(), *want), "taken once")
}
