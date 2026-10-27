package impl

import (
	"testing"
	"time"

	librespot "github.com/devgianlu/go-librespot"
	eventsenderpb "github.com/devgianlu/go-librespot/proto/spotify/event_sender"
	metadatapb "github.com/devgianlu/go-librespot/proto/spotify/metadata"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// Pauses don't count as listening.
func TestListenCountsPlayingTimeOnly(t *testing.T) {
	t0 := time.Unix(1000, 0)
	l := &listen{}
	l.resume(t0)
	l.pause(t0.Add(30 * time.Second))
	l.pause(t0.Add(40 * time.Second)) // already paused
	l.resume(t0.Add(60 * time.Second))
	l.pause(t0.Add(70 * time.Second))
	if l.played != 40*time.Second || !l.started.Equal(t0) {
		t.Errorf("played %v from %v", l.played, l.started)
	}
}

func TestStreamMessage(t *testing.T) {
	r := report{
		id:      librespot.SpotifyIdFromGid(librespot.SpotifyIdTypeTrack, make([]byte, 16)),
		file:    &metadatapb.AudioFile{FileId: []byte{1, 2, 3}, Format: metadatapb.AudioFile_OGG_VORBIS_320.Enum()},
		started: time.UnixMilli(1_700_000_000_000), ended: time.UnixMilli(1_700_000_180_000),
		played: 32125 * time.Millisecond, reasonStart: "playbtn", reasonEnd: "trackdone",
	}
	data, err := streamMessage(r, "Vorbis 320 kbps", []byte{0xab, 0xcd}, "s")
	if err != nil {
		t.Fatal(err)
	}
	var msg eventsenderpb.RawCoreStream
	if err := proto.Unmarshal(data, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.GetMsPlayed() != 32125 || msg.GetContentUri() != r.id.Uri() || msg.GetPlayContext() != r.id.Uri() || msg.GetReasonEnd() != "trackdone" {
		t.Errorf("message %v", &msg)
	}
	// The appended fields survive as unknown ones: audio_id among them.
	var audioId string
	for b := msg.ProtoReflect().GetUnknown(); len(b) > 0; {
		num, typ, n := protowire.ConsumeTag(b)
		b = b[n:]
		n = protowire.ConsumeFieldValue(num, typ, b)
		if num == 63 {
			v, _ := protowire.ConsumeString(b)
			audioId = v
		}
		b = b[n:]
	}
	if audioId != "abcd" {
		t.Errorf("audio_id %q", audioId)
	}
}
