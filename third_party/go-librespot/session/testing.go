//go:build test_unit || test_integration

package session

import "github.com/devgianlu/go-librespot/player"

// NewWithEvents is a session holding only events, for tests of what is
// reported to them.
func NewWithEvents(events player.EventManager) *Session { return &Session{events: events} }
