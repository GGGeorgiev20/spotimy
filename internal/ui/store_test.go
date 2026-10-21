package ui

import (
	"testing"
	"time"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
)

func TestIsContext(t *testing.T) {
	defer func() { Player.Status, contextAliases = nil, map[string]string{} }()
	set := func(ctx string) { Player.Status = &backend.PlayerStatus{ContextURI: ctx} }

	set("spotify:user:bob:playlist:abc")
	if !IsContext("spotify:playlist:abc") {
		t.Error("legacy playlist uri should match")
	}
	set("spotify:user:bob:collection")
	if !IsContext("spotify:collection:tracks") || IsContext("spotify:playlist:liked") {
		t.Error("collection")
	}
	// A playlist the player resolved to another context.
	playRequest, playRequestAt = "spotify:playlist:liked", time.Now()
	noteAlias("spotify:user:bob:collection")
	if !IsContext("spotify:playlist:liked") {
		t.Error("alias of the requested context should match")
	}
}
