package backend

import (
	"context"
	"errors"
	"time"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/devgianlu/go-librespot/daemon"
)

// Playback bindings. When this app's own device is playing (or nothing is),
// commands go straight to the embedded go-librespot daemon. When another
// Connect device is active they go through connect-state to that device.

type NowTrack struct {
	URI        string           `json:"uri"`
	Name       string           `json:"name"`
	Artists    []spotify.Artist `json:"artists"`
	AlbumURI   string           `json:"albumUri"`
	AlbumName  string           `json:"albumName"`
	Images     []spotify.Image  `json:"images"`
	DurationMs int64            `json:"durationMs"`
}

type PlayerStatus struct {
	Remote        bool      `json:"remote"` // playing on another device
	DeviceID      string    `json:"deviceId"`
	DeviceName    string    `json:"deviceName"`
	Track         *NowTrack `json:"track"`
	ContextURI    string    `json:"contextUri"`
	Playing       bool      `json:"playing"` // audio is running (not paused/stopped)
	Buffering     bool      `json:"buffering"`
	PositionMs    int64     `json:"positionMs"`
	Shuffle       bool      `json:"shuffle"`
	RepeatContext bool      `json:"repeatContext"`
	RepeatTrack   bool      `json:"repeatTrack"`
	Volume        int       `json:"volume"`
	VolumeMax     int       `json:"volumeMax"`
}

func (a *App) startConnect() {
	a.mu.Lock()
	running := a.connect != nil
	a.mu.Unlock()
	if running {
		return
	}
	if _, err := a.sp.Me(a.ctx); err != nil { // ownership checks and playlist edits need the user
		a.log.Warn("profile unavailable", "err", err)
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	id := a.player.DeviceID()
	if a.opts.Sandbox {
		id = a.deviceID
	}
	cn, err := a.sp.StartConnect(ctx, id, a.emitPlayer)
	if err != nil {
		a.log.Warn("connect unavailable; remote devices won't show", "err", err)
		return
	}
	a.mu.Lock()
	a.connect = cn
	a.mu.Unlock()
}

func (a *App) stopConnect() {
	a.mu.Lock()
	cn := a.connect
	a.connect = nil
	a.mu.Unlock()
	if cn != nil {
		cn.Close()
	}
}

func (a *App) conn() *spotify.Connect {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.connect
}

// remote returns the Connect handle when another device is the active one.
func (a *App) remote() (*spotify.Connect, *spotify.ConnectState) {
	cn := a.conn()
	if cn == nil {
		return nil, nil
	}
	st := cn.State()
	if st.ActiveDevice == "" || st.ActiveDevice == a.player.DeviceID() {
		return nil, nil
	}
	return cn, st
}

var errNoConnect = errors.New("spotify connect isn't available yet")

func (a *App) Status() (*PlayerStatus, error) {
	if a.opts.Sandbox {
		a.mu.Lock()
		fake := a.fake
		a.mu.Unlock()
		if fake != nil {
			cp := *fake
			return &cp, nil
		}
		if _, st := a.remote(); st != nil {
			return a.remoteStatus(st), nil
		}
		return &PlayerStatus{VolumeMax: 65535, Volume: 65535}, nil
	}
	if _, st := a.remote(); st != nil {
		return a.remoteStatus(st), nil
	}
	res, err := a.do(daemon.ApiRequestTypeStatus, nil)
	if err != nil {
		return nil, err
	}
	s := res.(*daemon.ApiStatus)
	out := &PlayerStatus{
		DeviceID: s.DeviceId, DeviceName: s.DeviceName,
		Playing: !s.Stopped && !s.Paused, Buffering: s.Buffering,
		Shuffle: s.ShuffleContext, RepeatContext: s.RepeatContext, RepeatTrack: s.RepeatTrack,
		Volume: int(s.Volume), VolumeMax: int(s.VolumeSteps),
	}
	if s.ContextUri != nil {
		out.ContextURI = *s.ContextUri
	}
	if t := s.Track; t != nil {
		nt := &NowTrack{URI: t.Uri, Name: t.Name, AlbumURI: t.AlbumUri, AlbumName: t.AlbumName, DurationMs: int64(t.Duration)}
		for i, n := range t.ArtistNames {
			ar := spotify.Artist{Name: n}
			if i < len(t.ArtistUris) {
				ar.URI = t.ArtistUris[i]
			}
			nt.Artists = append(nt.Artists, ar)
		}
		if t.AlbumCoverUrl != nil {
			nt.Images = []spotify.Image{{URL: *t.AlbumCoverUrl, Width: 300, Height: 300}}
		}
		out.Track, out.PositionMs = nt, t.Position
	}
	return out, nil
}

func (a *App) remoteStatus(st *spotify.ConnectState) *PlayerStatus {
	out := &PlayerStatus{
		Remote: true, DeviceID: st.ActiveDevice, ContextURI: st.ContextURI,
		Playing: st.Playing && !st.Paused, PositionMs: st.PositionMs,
		Shuffle: st.Shuffle, RepeatContext: st.RepeatCtx, RepeatTrack: st.RepeatTrack, VolumeMax: 65535,
	}
	for _, d := range st.Devices {
		if d.Active {
			out.DeviceName, out.Volume = d.Name, d.Volume
		}
	}
	if out.Playing && st.Timestamp > 0 {
		out.PositionMs += time.Now().UnixMilli() - st.Timestamp
	}
	if st.TrackURI != "" {
		out.Track = &NowTrack{URI: st.TrackURI, DurationMs: st.DurationMs}
		ctx, cancel := a.call()
		defer cancel()
		if ts, err := a.sp.Tracks(ctx, []string{st.TrackURI}); err == nil && len(ts) == 1 {
			t := ts[0]
			out.Track.Name, out.Track.Artists = t.Name, t.Artists
			if t.Album != nil {
				out.Track.AlbumURI, out.Track.AlbumName, out.Track.Images = t.Album.URI, t.Album.Name, t.Album.Images
			}
		}
	}
	if out.Track != nil && out.Track.DurationMs > 0 {
		out.PositionMs = min(out.PositionMs, out.Track.DurationMs)
	}
	return out
}

func (a *App) cmd(endpoint string, fields map[string]any) error {
	cn := a.conn()
	if cn == nil {
		return errNoConnect
	}
	ctx, cancel := a.call()
	defer cancel()
	return cn.Command(ctx, endpoint, fields)
}

func (a *App) Play(uri, skipToURI string) error {
	if a.sandboxed("Play", uri, skipToURI) {
		a.fakePlay(uri, skipToURI)
		return nil
	}
	if cn, _ := a.remote(); cn != nil {
		cmd := map[string]any{"context": map[string]any{"uri": uri, "url": "context://" + uri}}
		if skipToURI != "" {
			cmd["options"] = map[string]any{"skip_to": map[string]any{"track_uri": skipToURI}}
		}
		return a.cmd("play", cmd)
	}
	_, err := a.do(daemon.ApiRequestTypePlay, daemon.ApiPlay{Uri: uri, SkipToUri: skipToURI})
	return err
}

func (a *App) PlayPause() error {
	if a.sandboxed("PlayPause") {
		a.fakeSet(func(s *PlayerStatus) { s.Playing = !s.Playing })
		return nil
	}
	if _, st := a.remote(); st != nil {
		if st.Playing && !st.Paused {
			return a.cmd("pause", nil)
		}
		return a.cmd("resume", nil)
	}
	_, err := a.do(daemon.ApiRequestTypePlayPause, nil)
	return err
}

func (a *App) Next() error {
	if a.sandboxed("Next") {
		return nil
	}
	if cn, _ := a.remote(); cn != nil {
		return a.cmd("skip_next", nil)
	}
	_, err := a.do(daemon.ApiRequestTypeNext, daemon.ApiNext{})
	return err
}

// SkipTo jumps to an upcoming track, queued or in the context (uid tells
// apart copies of the same track), like clicking it in the queue.
func (a *App) SkipTo(uri, uid string) error {
	if a.sandboxed("SkipTo", uri, uid) {
		return nil
	}
	if cn, _ := a.remote(); cn != nil {
		track := map[string]any{"uri": uri}
		if uid != "" {
			track["uid"] = uid
		}
		return a.cmd("skip_next", map[string]any{"track": track})
	}
	next := daemon.ApiNext{Uri: &uri}
	if uid != "" {
		next.Uid = &uid
	}
	_, err := a.do(daemon.ApiRequestTypeNext, next)
	return err
}

func (a *App) Prev() error {
	if a.sandboxed("Prev") {
		return nil
	}
	if cn, _ := a.remote(); cn != nil {
		return a.cmd("skip_prev", nil)
	}
	_, err := a.do(daemon.ApiRequestTypePrev, nil)
	return err
}

func (a *App) SeekTo(ms int64) error {
	if a.sandboxed("SeekTo", ms) {
		a.fakeSet(func(s *PlayerStatus) { s.PositionMs = ms })
		return nil
	}
	if cn, _ := a.remote(); cn != nil {
		return a.cmd("seek_to", map[string]any{"value": ms})
	}
	_, err := a.do(daemon.ApiRequestTypeSeek, daemon.ApiSeek{Position: ms})
	return err
}

func (a *App) SetVolume(v int32) error {
	if a.sandboxed("SetVolume", v) {
		a.fakeSet(func(s *PlayerStatus) { s.Volume = int(v) })
		return nil
	}
	if cn, st := a.remote(); cn != nil {
		ctx, cancel := a.call()
		defer cancel()
		return cn.SetVolume(ctx, st.ActiveDevice, int(v))
	}
	_, err := a.do(daemon.ApiRequestTypeSetVolume, daemon.ApiSetVolume{Volume: v})
	return err
}

func (a *App) SetShuffle(on bool) error {
	if a.sandboxed("SetShuffle", on) {
		a.fakeSet(func(s *PlayerStatus) { s.Shuffle = on })
		return nil
	}
	if cn, _ := a.remote(); cn != nil {
		return a.cmd("set_shuffling_context", map[string]any{"value": on})
	}
	_, err := a.do(daemon.ApiRequestTypeSetShufflingContext, on)
	return err
}

// SetRepeat takes "off", "context" or "track".
func (a *App) SetRepeat(mode string) error {
	if a.sandboxed("SetRepeat", mode) {
		a.fakeSet(func(s *PlayerStatus) { s.RepeatContext, s.RepeatTrack = mode != "off", mode == "track" })
		return nil
	}
	if cn, _ := a.remote(); cn != nil {
		if err := a.cmd("set_repeating_context", map[string]any{"value": mode != "off"}); err != nil {
			return err
		}
		return a.cmd("set_repeating_track", map[string]any{"value": mode == "track"})
	}
	if _, err := a.do(daemon.ApiRequestTypeSetRepeatingContext, mode != "off"); err != nil {
		return err
	}
	_, err := a.do(daemon.ApiRequestTypeSetRepeatingTrack, mode == "track")
	return err
}

// --- Queue ---

type QueueTrack struct {
	spotify.Track
	Provider string `json:"provider"`
}

type Queue struct {
	Next     []QueueTrack `json:"next"`
	Revision string       `json:"revision"`
}

// Queue returns the upcoming tracks of the active device (user-queued first,
// then the rest of the context), with metadata.
func (a *App) Queue() (*Queue, error) {
	cn := a.conn()
	if cn == nil {
		return nil, errNoConnect
	}
	st := cn.State()
	items := st.Next
	if len(items) > 80 {
		items = items[:80]
	}
	uris := make([]string, len(items))
	for i, it := range items {
		uris[i] = it.URI
	}
	ctx, cancel := a.call()
	defer cancel()
	tracks, err := a.sp.Tracks(ctx, uris)
	if err != nil {
		return nil, err
	}
	meta := make(map[string]spotify.Track, len(tracks))
	for _, t := range tracks {
		meta[t.URI] = t
	}
	q := &Queue{Revision: st.QueueRev, Next: []QueueTrack{}}
	for _, it := range items {
		t, ok := meta[it.URI]
		if !ok {
			t = spotify.Track{URI: it.URI, Name: it.URI}
		}
		t.UID = it.UID
		q.Next = append(q.Next, QueueTrack{Track: t, Provider: it.Provider})
	}
	return q, nil
}

func (a *App) AddToQueue(uris []string) error {
	if a.sandboxed("AddToQueue", uris) {
		return nil
	}
	if cn, _ := a.remote(); cn != nil {
		ctx, cancel := a.call()
		defer cancel()
		for _, u := range uris {
			if err := cn.AddToQueue(ctx, u); err != nil {
				return err
			}
		}
		return nil
	}
	for _, u := range uris {
		if _, err := a.do(daemon.ApiRequestTypeAddToQueue, u); err != nil {
			return err
		}
	}
	return nil
}

// SetQueue replaces the upcoming tracks; the UI uses it to remove and reorder.
func (a *App) SetQueue(next []spotify.QueueItem) error {
	if a.sandboxed("SetQueue", len(next)) {
		return nil
	}
	cn := a.conn()
	if cn == nil {
		return errNoConnect
	}
	ctx, cancel := a.call()
	defer cancel()
	return cn.SetQueue(ctx, next)
}

// --- Devices ---

func (a *App) Devices() ([]spotify.Device, error) {
	cn := a.conn()
	if cn == nil {
		return nil, errNoConnect
	}
	return cn.State().Devices, nil
}

func (a *App) TransferPlayback(deviceID string) error {
	if a.sandboxed("TransferPlayback", deviceID) {
		return nil
	}
	cn := a.conn()
	if cn == nil {
		return errNoConnect
	}
	ctx, cancel := a.call()
	defer cancel()
	return cn.Transfer(ctx, deviceID)
}

// fakeSet changes the sandbox's pretend playback state.
func (a *App) fakeSet(f func(*PlayerStatus)) {
	a.mu.Lock()
	if a.fake == nil {
		a.fake = &PlayerStatus{VolumeMax: 65535, Volume: 65535}
	}
	f(a.fake)
	a.mu.Unlock()
	a.emitPlayer()
}

// fakePlay pretends to start a context so the UI's playing states can be
// tested in the sandbox.
func (a *App) fakePlay(uri, skipTo string) {
	track := skipTo
	if track == "" && spotify.URIType(uri) == "track" {
		track = uri
	}
	var nt *NowTrack
	if track != "" {
		ctx, cancel := a.call()
		defer cancel()
		if ts, err := a.sp.Tracks(ctx, []string{track}); err == nil && len(ts) == 1 {
			t := ts[0]
			nt = &NowTrack{URI: t.URI, Name: t.Name, Artists: t.Artists, DurationMs: int64(t.DurationMs)}
			if t.Album != nil {
				nt.AlbumURI, nt.AlbumName, nt.Images = t.Album.URI, t.Album.Name, t.Album.Images
			}
		}
	}
	a.fakeSet(func(s *PlayerStatus) {
		s.ContextURI, s.Playing, s.PositionMs = uri, true, 0
		if nt != nil {
			s.Track = nt
		}
	})
}
