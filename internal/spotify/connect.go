package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	librespot "github.com/devgianlu/go-librespot"
	"github.com/devgianlu/go-librespot/apresolve"
	"github.com/devgianlu/go-librespot/dealer"
	connectpb "github.com/devgianlu/go-librespot/proto/spotify/connectstate"
	devicespb "github.com/devgianlu/go-librespot/proto/spotify/connectstate/devices"
	"google.golang.org/protobuf/proto"
)

// Connect follows Spotify Connect the way the official clients do: a dealer
// websocket plus a hidden observer device ("hobs_" prefix) registered in
// connect-state, which makes Spotify push cluster updates (all devices, the
// active player state and its queue). Commands go to whichever device is
// active through the connect-state command endpoints.
type Connect struct {
	c        *Client
	log      *slog.Logger
	self     string // our observer id, hobs_<device id>
	playerID string // the local go-librespot device id
	onChange func()

	mu      sync.Mutex
	cluster *connectpb.Cluster
	dealer  *dealer.Dealer
	cancel  context.CancelFunc
}

// StartConnect connects the dealer and registers the observer. playerID is the
// local device's id; onChange runs (off the dealer goroutine) after each
// cluster update.
func (c *Client) StartConnect(ctx context.Context, playerID string, onChange func()) (*Connect, error) {
	addr, err := apresolve.NewApResolver(&librespot.NullLogger{}, c.http, false).GetDealer(ctx)
	if err != nil {
		return nil, &Error{Op: "dealer resolve", Err: err}
	}
	runCtx, cancel := context.WithCancel(context.Background())
	cn := &Connect{
		c: c, log: c.log, self: "hobs_" + playerID, playerID: playerID, onChange: onChange, cancel: cancel,
		dealer: dealer.NewDealer(&librespot.NullLogger{}, c.http, addr, func(ctx context.Context, force bool) (string, error) {
			return c.bearerToken(ctx, force)
		}),
	}
	if err := cn.dealer.Connect(ctx); err != nil {
		cancel()
		return nil, &Error{Op: "dealer connect", Err: err}
	}
	go cn.run(runCtx)
	return cn, nil
}

func (cn *Connect) Close() {
	cn.cancel()
	cn.dealer.Close()
}

func (cn *Connect) run(ctx context.Context) {
	msgs := cn.dealer.ReceiveMessage("hm://pusher/v1/connections/", "hm://connect-state/v1/cluster")
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				return
			}
			switch {
			case strings.HasPrefix(msg.Uri, "hm://pusher/v1/connections/"):
				// A (re)connection: register so cluster pushes start (again).
				connID := msg.Headers["Spotify-Connection-Id"]
				go func() {
					if err := cn.register(ctx, connID); err != nil {
						cn.log.Warn("connect-state register failed", "err", err)
					}
				}()
			case strings.HasPrefix(msg.Uri, "hm://connect-state/v1/cluster"):
				var upd connectpb.ClusterUpdate
				if err := proto.Unmarshal(msg.Payload, &upd); err != nil {
					cn.log.Warn("bad cluster update", "err", err)
					continue
				}
				cn.setCluster(upd.Cluster)
			}
		}
	}
}

func (cn *Connect) setCluster(cl *connectpb.Cluster) {
	if cl == nil {
		return
	}
	cn.mu.Lock()
	cn.cluster = cl
	cn.mu.Unlock()
	if cn.onChange != nil {
		cn.onChange()
	}
}

func (cn *Connect) register(ctx context.Context, connID string) error {
	req := &connectpb.PutStateRequest{
		MemberType:     connectpb.MemberType_CONNECT_STATE,
		PutStateReason: connectpb.PutStateReason_NEW_DEVICE,
		Device: &connectpb.Device{DeviceInfo: &connectpb.DeviceInfo{
			CanPlay:    false,
			Name:       "Spotimy (observer)",
			DeviceId:   cn.self,
			DeviceType: devicespb.DeviceType_COMPUTER,
			ClientId:   cn.c.id.ClientID,
			Capabilities: &connectpb.Capabilities{
				Hidden: true, IsObservable: true, CanBePlayer: false, IsControllable: false,
				SupportsCommandRequest: true, SupportsGzipPushes: true,
			},
		}},
		ClientSideTimestamp: uint64(time.Now().UnixMilli()),
	}
	body, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	resp, err := cn.c.do(ctx, request{
		op: "connect-state register", method: "PUT", url: cn.c.spclientURL + "/connect-state/v1/devices/" + cn.self,
		body: body, contentType: "application/x-protobuf", accept: "application/x-protobuf", idempotent: true,
		headers: http.Header{"X-Spotify-Connection-Id": {connID}},
	})
	if err != nil {
		return err
	}
	var cl connectpb.Cluster
	if err := proto.Unmarshal(resp, &cl); err != nil {
		return fmt.Errorf("decoding cluster: %w", err)
	}
	cn.setCluster(&cl)
	return nil
}

// --- State for the UI ---

type Device struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // COMPUTER, SMARTPHONE, SPEAKER...
	Volume   int    `json:"volume"`
	Active   bool   `json:"active"`
	IsPlayer bool   `json:"isPlayer"` // this app's own audio device
	CanVol   bool   `json:"canVolume"`
}

type QueueItem struct {
	URI      string `json:"uri"`
	UID      string `json:"uid"`
	Provider string `json:"provider"` // "queue" (user added), "context", "autoplay"
}

type ConnectState struct {
	ActiveDevice string      `json:"activeDevice"` // "" when nothing is playing anywhere
	Devices      []Device    `json:"devices"`
	TrackURI     string      `json:"trackUri"`
	ContextURI   string      `json:"contextUri"`
	Playing      bool        `json:"playing"`
	Paused       bool        `json:"paused"`
	PositionMs   int64       `json:"positionMs"` // at Timestamp
	Timestamp    int64       `json:"timestamp"`  // unix ms when PositionMs was valid
	DurationMs   int64       `json:"durationMs"`
	Shuffle      bool        `json:"shuffle"`
	RepeatCtx    bool        `json:"repeatContext"`
	RepeatTrack  bool        `json:"repeatTrack"`
	Next         []QueueItem `json:"next"`
	QueueRev     string      `json:"queueRevision"`
}

// Known reports whether the cluster (the account's devices and what plays
// where) has arrived.
func (cn *Connect) Known() bool {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	return cn.cluster != nil
}

// State returns a snapshot of the cluster.
func (cn *Connect) State() *ConnectState {
	cn.mu.Lock()
	cl := cn.cluster
	cn.mu.Unlock()
	st := &ConnectState{Devices: []Device{}, Next: []QueueItem{}}
	if cl == nil {
		return st
	}
	st.ActiveDevice = cl.ActiveDeviceId
	for id, d := range cl.Device {
		if d.Capabilities.GetHidden() || strings.HasPrefix(id, "hobs_") {
			continue
		}
		st.Devices = append(st.Devices, Device{
			ID: id, Name: d.Name, Type: d.DeviceType.String(), Volume: int(d.Volume),
			Active: id == cl.ActiveDeviceId, IsPlayer: id == cn.playerID,
			CanVol: !d.Capabilities.GetDisableVolume(),
		})
	}
	sortDevices(st.Devices)
	ps := cl.PlayerState
	if ps == nil || cl.ActiveDeviceId == "" {
		return st
	}
	st.TrackURI = ps.Track.GetUri()
	st.ContextURI = ps.ContextUri
	st.Playing, st.Paused = ps.IsPlaying, ps.IsPaused
	st.PositionMs, st.Timestamp, st.DurationMs = ps.PositionAsOfTimestamp, ps.Timestamp, ps.Duration
	if o := ps.Options; o != nil {
		st.Shuffle, st.RepeatCtx, st.RepeatTrack = o.ShufflingContext, o.RepeatingContext, o.RepeatingTrack
	}
	st.QueueRev = ps.QueueRevision
	for _, t := range ps.NextTracks {
		if t.Uri == "" || strings.HasPrefix(t.Uri, "spotify:delimiter") || strings.HasPrefix(t.Uri, "spotify:meta:") {
			continue
		}
		st.Next = append(st.Next, QueueItem{URI: t.Uri, UID: t.Uid, Provider: t.Provider})
	}
	return st
}

func sortDevices(ds []Device) {
	// Active first, then this app, then by name.
	for i := 1; i < len(ds); i++ {
		for j := i; j > 0 && lessDevice(ds[j], ds[j-1]); j-- {
			ds[j], ds[j-1] = ds[j-1], ds[j]
		}
	}
}

func lessDevice(a, b Device) bool {
	if a.Active != b.Active {
		return a.Active
	}
	if a.IsPlayer != b.IsPlayer {
		return a.IsPlayer
	}
	return a.Name < b.Name
}

// --- Commands ---

func (cn *Connect) target() (string, error) {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	if cn.cluster == nil || cn.cluster.ActiveDeviceId == "" {
		return "", errors.New("no active device")
	}
	return cn.cluster.ActiveDeviceId, nil
}

// Command sends a player command (pause, resume, skip_next, seek_to,
// set_queue, add_to_queue...) to the active device.
func (cn *Connect) Command(ctx context.Context, endpoint string, fields map[string]any) error {
	to, err := cn.target()
	if err != nil {
		return &Error{Op: "command " + endpoint, Err: err}
	}
	return cn.CommandTo(ctx, to, endpoint, fields)
}

// CommandTo sends a player command to a specific device.
func (cn *Connect) CommandTo(ctx context.Context, to, endpoint string, fields map[string]any) error {
	cmd := map[string]any{"endpoint": endpoint}
	for k, v := range fields {
		cmd[k] = v
	}
	return cn.postJSON(ctx, "command "+endpoint,
		fmt.Sprintf("/connect-state/v1/player/command/from/%s/to/%s", url.PathEscape(cn.self), url.PathEscape(to)),
		map[string]any{"command": cmd})
}

// Transfer moves playback to device id.
func (cn *Connect) Transfer(ctx context.Context, to string) error {
	from, _ := cn.target()
	if from == "" {
		from = cn.self
	}
	return cn.postJSON(ctx, "transfer",
		fmt.Sprintf("/connect-state/v1/connect/transfer/from/%s/to/%s", url.PathEscape(from), url.PathEscape(to)),
		map[string]any{"transfer_options": map[string]any{"restore_paused": "restore"}})
}

// SetVolume sets a device's volume (0..65535).
func (cn *Connect) SetVolume(ctx context.Context, device string, volume int) error {
	body, _ := json.Marshal(map[string]any{"volume": volume})
	_, err := cn.c.do(ctx, request{
		op: "volume", method: "PUT", body: body, contentType: "application/json",
		url: cn.c.spclientURL + fmt.Sprintf("/connect-state/v1/connect/volume/from/%s/to/%s", url.PathEscape(cn.self), url.PathEscape(device)),
	})
	return err
}

func (cn *Connect) postJSON(ctx context.Context, op, path string, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return &Error{Op: op, Err: err}
	}
	_, err = cn.c.do(ctx, request{op: op, method: "POST", url: cn.c.spclientURL + path, body: bytes.Clone(body), contentType: "application/json"})
	return err
}

// AddToQueue appends a track to the user queue of the active device.
func (cn *Connect) AddToQueue(ctx context.Context, uri string) error {
	return cn.Command(ctx, "add_to_queue", map[string]any{
		"track": map[string]any{"uri": uri, "metadata": map[string]string{"is_queued": "true"}, "provider": "queue"},
	})
}

// SetQueue replaces the upcoming tracks (used for remove and reorder).
func (cn *Connect) SetQueue(ctx context.Context, next []QueueItem) error {
	track := func(uri, uid, provider string, meta map[string]string) map[string]any {
		m := map[string]any{"uri": uri, "uid": uid, "provider": provider}
		if provider == "queue" {
			if meta == nil {
				meta = map[string]string{}
			}
			meta["is_queued"] = "true"
		}
		if len(meta) > 0 {
			m["metadata"] = meta
		}
		return m
	}
	prev := []map[string]any{} // must not marshal as null
	cn.mu.Lock()
	if cn.cluster != nil && cn.cluster.PlayerState != nil {
		for _, t := range cn.cluster.PlayerState.PrevTracks {
			prev = append(prev, track(t.Uri, t.Uid, t.Provider, t.Metadata))
		}
	}
	cn.mu.Unlock()
	tracks := make([]map[string]any, 0, len(next))
	for _, t := range next {
		tracks = append(tracks, track(t.URI, t.UID, t.Provider, nil))
	}
	return cn.Command(ctx, "set_queue", map[string]any{"next_tracks": tracks, "prev_tracks": prev})
}
