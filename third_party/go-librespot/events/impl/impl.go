// Package impl reports what was listened to, as the desktop client does: one
// RawCoreStream event per track played, sent to the event service when the
// track ends, is skipped or is replaced. That is what Spotify's listening
// history, stats and Friend Activity are built from.
//
// The event's shape and the desktop identifiers come from librespot's
// listening reporter (librespot-org/librespot#1759), observed in the Linux
// desktop client 1.2.96.518.
package impl

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	librespot "github.com/devgianlu/go-librespot"
	"github.com/devgianlu/go-librespot/audio"
	"github.com/devgianlu/go-librespot/mercury"
	"github.com/devgianlu/go-librespot/player"
	connectpb "github.com/devgianlu/go-librespot/proto/spotify/connectstate"
	eventsenderpb "github.com/devgianlu/go-librespot/proto/spotify/event_sender"
	extmetadatapb "github.com/devgianlu/go-librespot/proto/spotify/extendedmetadata"
	audiofilespb "github.com/devgianlu/go-librespot/proto/spotify/extendedmetadata/audiofiles"
	metadatapb "github.com/devgianlu/go-librespot/proto/spotify/metadata"
	"github.com/devgianlu/go-librespot/spclient"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// The desktop client these events claim to come from. Spotify accepts other
// values, but its history ingestion may drop events it doesn't recognise.
const (
	reportingVersion     = "1.2.96.518"
	reportingVersionCode = 129600518
	reportingCoreVersion = 6005400000002005
	reportingSdk         = "0.9.4-rl-essopt-loginsend-onlinesend-bcdsend-heartbeat300.0s/30.0s-modern-payload125kB-batch100"
	deviceName           = "Spotimy"
)

type Impl struct{}

func (Impl) NewEventManager(log librespot.Logger, store librespot.StateStore, _ *mercury.Client, sp *spclient.Spclient, _ string) (player.EventManager, error) {
	m := &manager{log: log, sp: sp, sessionId: random(16), sequenceId: random(20), clock: time.Now()}
	if st, err := store.Load(); err == nil && st != nil {
		m.deviceId = st.DeviceId
	}
	return m, nil
}

func random(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

type manager struct {
	log      librespot.Logger
	sp       *spclient.Spclient
	deviceId string

	sessionId, sequenceId []byte
	clock                 time.Time // the monotonic clock's zero

	mu       sync.Mutex
	sequence int64
	cur      *listen
	// lastEnd is how the previous track ended, which is how the next started.
	lastEnd string
	sending sync.WaitGroup
}

// listen is the playing track's listening so far: played is the time it has
// been playing, not counting pauses, from started until now.
type listen struct {
	stream         *player.Stream
	context        string
	shuffle        bool
	reasonStart    string
	reasonEnd      string // set by a skip; "endplay" when nothing did
	started, since time.Time
	played         time.Duration
}

func (l *listen) pause(now time.Time) {
	if !l.since.IsZero() {
		l.played += now.Sub(l.since)
		l.since = time.Time{}
	}
}

func (l *listen) resume(now time.Time) {
	if l.since.IsZero() {
		l.since = now
		if l.started.IsZero() {
			l.started = now
		}
	}
}

// track makes stream the one being listened to, ending the previous one.
func (m *manager) track(stream *player.Stream, ctxUri string, shuffle bool) *listen {
	if m.cur != nil && m.cur.stream == stream {
		if ctxUri != "" {
			m.cur.context, m.cur.shuffle = ctxUri, shuffle
		}
		return m.cur
	}
	m.finish(m.cur, "endplay")
	start := m.lastEnd
	if start == "" {
		start = "playbtn"
	}
	m.cur = &listen{stream: stream, context: ctxUri, shuffle: shuffle, reasonStart: start}
	return m.cur
}

// finish reports l, unless nothing of it was heard.
func (m *manager) finish(l *listen, reason string) {
	if l == nil {
		return
	}
	if l == m.cur {
		m.cur = nil
	}
	if l.reasonEnd != "" {
		reason = l.reasonEnd
	}
	m.lastEnd = reason
	now := time.Now()
	l.pause(now)
	if l.played <= 0 || l.stream == nil || l.stream.Media == nil || l.stream.File == nil {
		return
	}
	m.sequence++
	ev := report{
		id: l.stream.Media.Id(), file: l.stream.File, context: l.context, shuffle: l.shuffle,
		started: l.started, ended: now, played: l.played, reasonStart: l.reasonStart, reasonEnd: reason,
		sequence: m.sequence,
	}
	m.sending.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := m.send(ctx, ev); err != nil {
			m.log.WithError(err).Warnf("failed reporting the listen of %s", ev.id.Uri())
		} else {
			m.log.Infof("reported %dms listened of %s", ev.played.Milliseconds(), ev.id.Uri())
		}
	})
}

func (m *manager) OnPlayerPlay(stream *player.Stream, ctxUri string, shuffle bool, _ *connectpb.PlayOrigin, _ *connectpb.ProvidedTrack, _ int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.track(stream, ctxUri, shuffle).resume(time.Now())
}

func (m *manager) OnPlayerResume(stream *player.Stream, _ int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.track(stream, "", false).resume(time.Now())
}

func (m *manager) OnPlayerPause(stream *player.Stream, ctxUri string, shuffle bool, _ *connectpb.PlayOrigin, _ *connectpb.ProvidedTrack, _ int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.track(stream, ctxUri, shuffle).pause(time.Now())
}

func (m *manager) OnPlayerSkipForward(stream *player.Stream, _ int64, _ bool) {
	m.skipped(stream, "fwdbtn")
}

func (m *manager) OnPlayerSkipBackward(stream *player.Stream, _ int64) {
	m.skipped(stream, "backbtn")
}

func (m *manager) skipped(stream *player.Stream, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cur != nil && m.cur.stream == stream {
		m.cur.reasonEnd = reason
	}
}

func (m *manager) OnPlayerEnd(stream *player.Stream, _ int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cur != nil && m.cur.stream == stream {
		m.finish(m.cur, "trackdone")
	}
}

func (m *manager) OnPrimaryStreamUnload(stream *player.Stream, _ int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cur != nil && m.cur.stream == stream {
		m.finish(m.cur, "endplay")
	}
}

// Close reports the track playing, and waits a little for the reports
// still on their way.
func (m *manager) Close() {
	m.mu.Lock()
	m.finish(m.cur, "endplay")
	m.mu.Unlock()
	done := make(chan struct{})
	go func() { m.sending.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

func (m *manager) PreStreamLoadNew([]byte, librespot.SpotifyId, int64) {}
func (m *manager) PostStreamResolveAudioFile([]byte, int32, *librespot.Media, *metadatapb.AudioFile) {
}
func (m *manager) PostStreamRequestAudioKey([]byte)                               {}
func (m *manager) PostStreamResolveStorage([]byte)                                {}
func (m *manager) PostStreamInitHttpChunkReader([]byte, *audio.HttpChunkedReader) {}
func (m *manager) PostPrimaryStreamLoad(*player.Stream, bool)                     {}
func (m *manager) OnPlayerSeek(*player.Stream, int64, int64)                      {}

// report is a finished listen, ready to send.
type report struct {
	id                     librespot.SpotifyId
	file                   *metadatapb.AudioFile
	context                string
	shuffle                bool
	started, ended         time.Time
	played                 time.Duration
	reasonStart, reasonEnd string
	sequence               int64
}

// audioFormat is the desktop client's name for f, "" for one it doesn't play.
func audioFormat(f metadatapb.AudioFile_Format) string {
	switch f {
	case metadatapb.AudioFile_OGG_VORBIS_96:
		return "Vorbis 96 kbps"
	case metadatapb.AudioFile_OGG_VORBIS_160:
		return "Vorbis 160 kbps"
	case metadatapb.AudioFile_OGG_VORBIS_320:
		return "Vorbis 320 kbps"
	case metadatapb.AudioFile_MP3_96:
		return "MP3 96 kbps"
	case metadatapb.AudioFile_MP3_160:
		return "MP3 160 kbps"
	case metadatapb.AudioFile_MP3_256:
		return "MP3 256 kbps"
	case metadatapb.AudioFile_MP3_320:
		return "MP3 320 kbps"
	}
	return ""
}

func (m *manager) send(ctx context.Context, r report) error {
	format := audioFormat(r.file.GetFormat())
	if format == "" {
		return fmt.Errorf("unreported audio format %s", r.file.GetFormat())
	}
	var files audiofilespb.AudioFilesExtensionResponse
	if err := m.sp.ExtendedMetadataSimple(ctx, r.id, extmetadatapb.ExtensionKind_AUDIO_FILES, &files); err != nil {
		return fmt.Errorf("audio id: %w", err)
	}
	if len(files.AudioId) == 0 {
		return fmt.Errorf("no audio id")
	}
	body, err := m.request(r, format, files.AudioId)
	if err != nil {
		return err
	}
	resp, err := m.sp.Request(ctx, "POST", "/gabo-receiver-service/v3/events/", nil, http.Header{
		"Content-Type":        {"application/x-protobuf"},
		"Content-Encoding":    {"gzip"},
		"App-Platform":        {"Linux"},
		"Spotify-App-Version": {fmt.Sprint(reportingVersionCode)},
	}, body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("event service answered %d", resp.StatusCode)
	}
	var out eventsenderpb.PublishEventsResponse
	if err := proto.Unmarshal(data, &out); err != nil {
		return fmt.Errorf("event service response: %w", err)
	}
	if len(out.Error) > 0 {
		e := out.Error[0]
		return fmt.Errorf("event rejected: reason %d (transient %v)", e.Reason, e.Transient)
	}
	return nil
}

// request is the gzipped PublishEventsRequest carrying r.
func (m *manager) request(r report, format string, audioId []byte) ([]byte, error) {
	msg, err := streamMessage(r, format, audioId, hex.EncodeToString(m.sessionId))
	if err != nil {
		return nil, err
	}
	clientId, _ := hex.DecodeString(librespot.ClientIdHex)
	installation := sha1.Sum([]byte(m.deviceId))
	frags := []*eventsenderpb.EventEnvelope_EventFragment{{Name: "message", Data: msg}}
	for _, f := range []struct {
		name string
		msg  proto.Message
	}{
		{"context_client_id", &eventsenderpb.ClientId{Value: clientId}},
		{"context_installation_id", &eventsenderpb.InstallationId{Value: installation[:16]}},
		{"context_application_desktop", &eventsenderpb.ApplicationDesktop{VersionString: reportingVersion, VersionCode: reportingVersionCode, SessionId: m.sessionId}},
		{"context_device_desktop", &eventsenderpb.DeviceDesktop{PlatformType: "linux", DeviceManufacturer: deviceName, DeviceModel: deviceName, DeviceId: m.deviceId, OsVersion: osVersion()}},
		{"context_time", &eventsenderpb.Time{Value: r.ended.UnixMilli()}},
		{"context_monotonic_clock", &eventsenderpb.MonotonicClock{Id: 1, Value: r.ended.Sub(m.clock).Milliseconds()}},
		{"context_sdk", &eventsenderpb.Sdk{VersionName: reportingSdk, Type: "cpp"}},
	} {
		data, err := proto.Marshal(f.msg)
		if err != nil {
			return nil, err
		}
		frags = append(frags, &eventsenderpb.EventEnvelope_EventFragment{Name: f.name, Data: data})
	}
	frags = append(frags, &eventsenderpb.EventEnvelope_EventFragment{Name: "context_client_context_id"})
	data, err := proto.Marshal(&eventsenderpb.PublishEventsRequest{Event: []*eventsenderpb.EventEnvelope{{
		EventName: "RawCoreStream", EventFragment: frags, SequenceId: m.sequenceId, SequenceNumber: r.sequence,
	}}})
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write(data)
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// streamMessage is r's RawCoreStream. The fields newer than the generated
// message are appended in their wire form.
func streamMessage(r report, format string, audioId []byte, sessionId string) ([]byte, error) {
	str, i32, b := proto.String, proto.Int32, proto.Bool
	played := int32(min(r.played.Milliseconds(), 1<<31-1))
	uri := r.id.Uri()
	ctx := r.context
	if ctx == "" {
		ctx = uri
	}
	mediaType := "audio"
	msg := &eventsenderpb.RawCoreStream{
		PlaybackId: random(16), ParentPlaybackId: make([]byte, 16), StreamId: random(16),
		MediaId: r.file.FileId, MediaType: &mediaType, VideoSessionId: str(""),
		ContentUri: &uri, PlayContext: &ctx, DisplayedContentUri: str(""), AudioFormat: &format,
		PlaybackStartTime: proto.Int64(r.started.UnixMilli()),
		MsPlayed:          &played, MsPlayedNominal: &played,
		MsPlayedOverlapping: i32(0), MsPlayedVideo: i32(0), MsPlayedFullscreen: i32(0), MsPlayedExternal: i32(0),
		MsNarrationOverlapping: i32(0), MsTrimmed: i32(0), MsNominalTrimmed: i32(0),
		ReasonStart: &r.reasonStart, ReasonEnd: &r.reasonEnd,
		SourceStart: str(strings.ToLower(deviceName)), SourceEnd: str(strings.ToLower(deviceName)),
		Referrer: str("unknown"), Provider: str("context"), StreamingRule: str("none"),
		Shuffle: b(r.shuffle), Live: b(false), ContentIsDownloaded: b(false), IncognitoMode: b(false),
		ClientOfflineAtStreamStart: b(false), SocialListening: b(false), IsAssumedPremium: b(true),
		ConnectControllerDeviceIdV2: str("local"),
		CoreVersion:                 proto.Int64(reportingCoreVersion), CoreBundle: str("full"),
		PlaybackStack: str("boombox"),
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	data = protowire.AppendTag(data, 56, protowire.VarintType) // end_timestamp
	data = protowire.AppendVarint(data, uint64(r.ended.UnixMilli()))
	data = protowire.AppendTag(data, 59, protowire.BytesType) // player_session_id
	data = protowire.AppendString(data, sessionId)
	data = protowire.AppendTag(data, 63, protowire.BytesType) // audio_id, as hex
	data = protowire.AppendString(data, hex.EncodeToString(audioId))
	data = protowire.AppendTag(data, 67, protowire.VarintType) // ms_played_nominal_overlapping
	data = protowire.AppendVarint(data, 0)
	data = protowire.AppendTag(data, 69, protowire.BytesType) // playback_stacks
	data = protowire.AppendString(data, "boombox")
	data = protowire.AppendTag(data, 71, protowire.BytesType) // orchestration_stack
	data = protowire.AppendString(data, "context-player")
	for _, f := range []struct {
		num protowire.Number
		v   string
	}{{78, deviceName}, {79, deviceName}, {80, "computer"}} { // controlling_device_brand, _model, _type
		data = protowire.AppendTag(data, f.num, protowire.BytesType)
		data = protowire.AppendString(data, f.v)
	}
	return data, nil
}

func osVersion() string {
	v, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	return strings.TrimSpace(string(v))
}
