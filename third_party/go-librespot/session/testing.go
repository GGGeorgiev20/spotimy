//go:build test_unit || test_integration

package session

import (
	"github.com/devgianlu/go-librespot/player"
	"github.com/devgianlu/go-librespot/spclient"
)

// NewWithEvents is a session holding only events, for tests of what is
// reported to them.
func NewWithEvents(events player.EventManager) *Session { return &Session{events: events} }

// NewWithSpclient is a session holding only events and a spclient, for tests
// of what is asked of Spotify.
func NewWithSpclient(events player.EventManager, sp *spclient.Spclient) *Session {
	return &Session{events: events, sp: sp}
}
