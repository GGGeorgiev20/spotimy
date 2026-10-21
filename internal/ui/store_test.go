package ui

import (
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

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

func TestLoadingTrack(t *testing.T) {
	now := time.Now()
	old := &backend.PlayerStatus{Track: &backend.NowTrack{URI: "spotify:track:old"}, PositionMs: 5000}
	started := &backend.NowTrack{URI: "spotify:track:new"}

	// Mid-load the player reports no track: the outgoing one stays.
	s := &backend.PlayerStatus{Buffering: true}
	loadingTrack(s, old, now)
	if s.Track != old.Track || s.PositionMs != 5000 {
		t.Fatalf("kept %+v", s.Track)
	}
	// Unless the user just started another: that one shows.
	starting.track, starting.at = started, now
	s = &backend.PlayerStatus{Buffering: true}
	loadingTrack(s, old, now)
	if s.Track != started {
		t.Fatalf("got %+v, want the started track", s.Track)
	}
	// Once loaded, the player's own track wins and the guess is dropped.
	s = &backend.PlayerStatus{Track: &backend.NowTrack{URI: "spotify:track:new"}}
	loadingTrack(s, old, now)
	if starting.track != nil || s.Track.URI != "spotify:track:new" {
		t.Fatalf("after load: %+v, starting %+v", s.Track, starting.track)
	}
}

func TestSettled(t *testing.T) {
	defer func() { Player.Status = nil }()
	gtx := layout.Context{Ops: new(op.Ops), Now: time.Now()}
	Player.Status = &backend.PlayerStatus{Track: &backend.NowTrack{URI: "spotify:track:a"}}
	if Settled(gtx) {
		t.Fatal("a track that just started isn't settled")
	}
	gtx.Now = gtx.Now.Add(settleDelay)
	if !Settled(gtx) {
		t.Fatal("settled after the delay")
	}
	Player.Status.Track = &backend.NowTrack{URI: "spotify:track:b"}
	if Settled(gtx) {
		t.Fatal("a skip starts the wait over")
	}
}
