package ui

import (
	"strconv"
	"strings"
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
	defer func() { starting.track = nil }()

	// Mid-load the player reports no track: the outgoing one stays.
	s := &backend.PlayerStatus{Buffering: true}
	loadingTrack(s, old, now)
	if s.Track != old.Track || s.PositionMs != 5000 {
		t.Fatalf("kept %+v", s.Track)
	}
	// A started track shows, even while the player still reports the old one
	// as playing (it hasn't begun loading yet): no flash back to it.
	starting.track, starting.at = started, now
	s = &backend.PlayerStatus{Track: old.Track, Playing: true}
	loadingTrack(s, old, now.Add(time.Second))
	if s.Track != started || !s.Buffering {
		t.Fatalf("got %+v, want the started track", s.Track)
	}
	// Once loaded, the player's own track wins and the guess is dropped.
	s = &backend.PlayerStatus{Track: &backend.NowTrack{URI: "spotify:track:new"}}
	loadingTrack(s, old, now.Add(time.Second))
	if starting.track != nil || s.Track.URI != "spotify:track:new" {
		t.Fatalf("after load: %+v, starting %+v", s.Track, starting.track)
	}
	// Another track playing well after the click means it didn't start.
	starting.track, starting.at = started, now
	s = &backend.PlayerStatus{Track: old.Track, Playing: true}
	loadingTrack(s, old, now.Add(3*time.Second))
	if s.Track != old.Track || starting.track != nil {
		t.Fatalf("failed start: %+v", s.Track)
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

func TestNotePlayed(t *testing.T) {
	defer func() { Prefs.History = nil }()
	Prefs.History = nil
	play := func(uri string, playing bool) {
		notePlayed(&backend.PlayerStatus{Playing: playing, Track: &backend.NowTrack{URI: uri}})
	}
	play("spotify:track:a", true)
	play("spotify:track:a", true) // still the same play
	play("spotify:track:b", false)
	play("spotify:track:b", true)
	play("spotify:ad:x", true)
	play("spotify:track:a", true)
	if got := strings.Join(Prefs.History, " "); got != "spotify:track:a spotify:track:b spotify:track:a" {
		t.Fatalf("history %q", got)
	}
	for i := 0; i < historyMax+10; i++ {
		play("spotify:track:"+strconv.Itoa(i), true)
	}
	if len(Prefs.History) != historyMax {
		t.Fatalf("history kept %d", len(Prefs.History))
	}
}
