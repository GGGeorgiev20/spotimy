package spotify

import (
	"testing"

	connectpb "github.com/devgianlu/go-librespot/proto/spotify/connectstate"
)

func TestConnectStateMapping(t *testing.T) {
	cn := &Connect{playerID: "local", self: "hobs_local", cluster: &connectpb.Cluster{
		ActiveDeviceId: "phone",
		Device: map[string]*connectpb.DeviceInfo{
			"local":      {Name: "This PC"},
			"phone":      {Name: "Phone", Volume: 30000},
			"hobs_local": {Name: "observer", Capabilities: &connectpb.Capabilities{Hidden: true}},
			"aspeaker":   {Name: "A Speaker"},
		},
		PlayerState: &connectpb.PlayerState{
			Track:                 &connectpb.ProvidedTrack{Uri: "spotify:track:now"},
			ContextUri:            "spotify:playlist:x",
			IsPlaying:             true,
			PositionAsOfTimestamp: 1000,
			Timestamp:             5,
			NextTracks: []*connectpb.ProvidedTrack{
				{Uri: "spotify:track:q", Uid: "q1", Provider: "queue"},
				{Uri: "spotify:delimiter", Provider: "context"},
				{Uri: "spotify:track:c", Uid: "c1", Provider: "context"},
			},
		},
	}}
	st := cn.State()
	if len(st.Devices) != 3 {
		t.Fatalf("hidden observer should be filtered: %+v", st.Devices)
	}
	if st.Devices[0].ID != "phone" || !st.Devices[0].Active || st.Devices[1].ID != "local" || !st.Devices[1].IsPlayer {
		t.Errorf("order should be active, this app, then by name: %+v", st.Devices)
	}
	if len(st.Next) != 2 || st.Next[0].Provider != "queue" || st.Next[1].URI != "spotify:track:c" {
		t.Errorf("queue: %+v", st.Next)
	}
	if st.TrackURI != "spotify:track:now" || !st.Playing || st.PositionMs != 1000 {
		t.Errorf("player: %+v", st)
	}
}

func TestPositionGQL(t *testing.T) {
	if m := (Position{Before: "u1"}).gql(); m["moveType"] != "BEFORE_UID" || m["fromUid"] != "u1" {
		t.Errorf("before: %v", m)
	}
	if m := (Position{}).gql(); m["moveType"] != "BOTTOM_OF_PLAYLIST" {
		t.Errorf("default: %v", m)
	}
}
