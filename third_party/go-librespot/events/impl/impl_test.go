package impl

import (
	"fmt"
	"testing"
	"time"

	librespot "github.com/devgianlu/go-librespot"
	"github.com/devgianlu/go-librespot/player"
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

// listening drives a manager the way the daemon does, on a clock the test
// moves, and keeps the listens it reports.
type listening struct {
	m    *manager
	t    time.Time
	sent []report
}

func newListening() *listening {
	l := &listening{t: time.Unix(1_700_000_000, 0)}
	l.m = &manager{now: func() time.Time { return l.t }, post: func(r report) { l.sent = append(l.sent, r) }}
	return l
}

func (l *listening) wait(d time.Duration) { l.t = l.t.Add(d) }

func stream(n byte) *player.Stream {
	gid := make([]byte, 16)
	gid[15] = n
	return &player.Stream{
		Media: librespot.NewMediaFromTrack(&metadatapb.Track{Gid: gid}),
		File:  &metadatapb.AudioFile{Format: metadatapb.AudioFile_OGG_VORBIS_320.Enum()},
	}
}

// want checks the listens reported, as "played reasonStart-reasonEnd".
func (l *listening) want(t *testing.T, want ...string) {
	t.Helper()
	var got []string
	for _, r := range l.sent {
		got = append(got, fmt.Sprintf("%v %s-%s", r.played, r.reasonStart, r.reasonEnd))
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("reported %q, want %q", got, want)
	}
}

// Each scenario calls the manager in the order the daemon does.
func TestListeningScenarios(t *testing.T) {
	a, b, c := stream(1), stream(2), stream(3)
	for _, tc := range []struct {
		name string
		run  func(l *listening)
		want []string
	}{
		{"picking another track near the end counts what was heard", func(l *listening) {
			l.m.OnPlayerPlay(a, "ctx", false, nil, nil, 0)
			l.wait(230 * time.Second)
			l.m.OnPrimaryStreamUnload(a, 230000) // loading the picked track
			l.m.OnPlayerPlay(b, "ctx", false, nil, nil, 0)
			l.wait(40 * time.Second)
			l.m.Close()
		}, []string{"3m50s playbtn-endplay", "40s clickrow-endplay"}},
		{"a track played to the end, then the next skipped", func(l *listening) {
			l.m.OnPlayerPlay(a, "ctx", false, nil, nil, 0)
			l.wait(200 * time.Second)
			l.m.OnPlayerEnd(a, 200000)
			l.m.OnPrimaryStreamUnload(a, 0) // advancing to the next
			l.m.OnPlayerPlay(b, "ctx", false, nil, nil, 0)
			l.wait(10 * time.Second)
			l.m.OnPlayerSkipForward(b, 10000, false)
			l.m.OnPrimaryStreamUnload(b, 10000)
			l.m.OnPlayerPlay(c, "ctx", false, nil, nil, 0)
			l.m.Close()
		}, []string{"3m20s playbtn-trackdone", "10s trackdone-fwdbtn"}},
		{"pauses don't count, nor does a skip made while paused", func(l *listening) {
			l.m.OnPlayerPlay(a, "ctx", false, nil, nil, 0)
			l.wait(10 * time.Second)
			l.m.OnPlayerPause(a, "ctx", false, nil, nil, 10000)
			l.wait(time.Hour)
			l.m.OnPlayerResume(a, 10000)
			l.wait(20 * time.Second)
			l.m.OnPlayerPause(a, "ctx", false, nil, nil, 30000)
			l.wait(time.Hour)
			l.m.OnPlayerSkipForward(a, 30000, false)
			l.m.OnPrimaryStreamUnload(a, 30000)
			l.m.OnPlayerPause(b, "ctx", false, nil, nil, 0)
			l.m.Close()
		}, []string{"30s playbtn-fwdbtn"}},
		{"restarting with previous is a seek, one listen as in the official clients", func(l *listening) {
			l.m.OnPlayerPlay(a, "ctx", false, nil, nil, 0)
			l.wait(100 * time.Second)
			l.m.OnPlayerSeek(a, 100000, 0)
			l.wait(200 * time.Second)
			l.m.OnPlayerEnd(a, 200000)
		}, []string{"5m0s playbtn-trackdone"}},
		{"previous within the first seconds goes back a track", func(l *listening) {
			l.m.OnPlayerPlay(a, "ctx", false, nil, nil, 0)
			l.wait(2 * time.Second)
			l.m.OnPlayerSkipBackward(a, 2000)
			l.m.OnPrimaryStreamUnload(a, 2000)
			l.m.OnPlayerPlay(b, "ctx", false, nil, nil, 0)
			l.wait(time.Second)
			l.m.Close()
		}, []string{"2s playbtn-backbtn", "1s backbtn-endplay"}},
		{"repeating a track counts each time round", func(l *listening) {
			l.m.OnPlayerPlay(a, "ctx", false, nil, nil, 0)
			l.wait(200 * time.Second)
			l.m.OnPlayerEnd(a, 200000)
			l.m.OnPrimaryStreamUnload(a, 0)
			again := stream(1)
			l.m.OnPlayerPlay(again, "ctx", false, nil, nil, 0)
			l.wait(200 * time.Second)
			l.m.OnPlayerEnd(again, 200000)
		}, []string{"3m20s playbtn-trackdone", "3m20s trackdone-trackdone"}},
		{"handing playback to another device ends the listen there", func(l *listening) {
			l.m.OnPlayerPlay(a, "ctx", false, nil, nil, 0)
			l.wait(50 * time.Second)
			l.m.OnPrimaryStreamUnload(a, 50000) // stopPlayback
			l.wait(time.Hour)
			l.m.Close()
		}, []string{"50s playbtn-endplay"}},
		{"quitting reports the track playing", func(l *listening) {
			l.m.OnPlayerPlay(a, "ctx", false, nil, nil, 0)
			l.wait(40 * time.Second)
			l.m.Close()
		}, []string{"40s playbtn-endplay"}},
		{"a track restored paused counts from when it is played", func(l *listening) {
			l.m.OnPlayerPause(a, "ctx", false, nil, nil, 60000)
			l.wait(5 * time.Minute)
			l.m.OnPlayerResume(a, 60000)
			l.wait(50 * time.Second)
			l.m.Close()
		}, []string{"50s playbtn-endplay"}},
		{"a track never played isn't reported", func(l *listening) {
			l.m.OnPlayerPause(a, "ctx", false, nil, nil, 0)
			l.wait(time.Minute)
			l.m.OnPrimaryStreamUnload(a, 0)
			l.m.OnPlayerPlay(b, "ctx", false, nil, nil, 0)
			l.wait(time.Minute)
			l.m.Close()
		}, []string{"1m0s clickrow-endplay"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := newListening()
			tc.run(l)
			l.want(t, tc.want...)
		})
	}
}

// A relinked track counts as the one asked for, with the audio that played.
func TestRelinkedTrackReportsTheRequestedOne(t *testing.T) {
	asked, played := stream(1), stream(2)
	played.RequestedId = asked.Media.Id()
	l := newListening()
	l.m.OnPlayerPlay(played, "ctx", false, nil, nil, 0)
	l.wait(time.Minute)
	l.m.Close()
	if len(l.sent) != 1 || l.sent[0].id.Uri() != asked.Media.Id().Uri() || l.sent[0].audio.Uri() != played.Media.Id().Uri() {
		t.Errorf("reported %+v", l.sent)
	}
}
