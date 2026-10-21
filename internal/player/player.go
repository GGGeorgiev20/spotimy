// Package player runs go-librespot's daemon in-process: login, session,
// access point, dealer, connect-state and audio output. The UI talks to it
// through daemon.ApiRequest values instead of go-librespot's HTTP server.
package player

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/devgianlu/go-librespot/daemon"
	"github.com/devgianlu/go-librespot/mpris"
	exprand "golang.org/x/exp/rand"
)

// State is the login/session state shown by the UI.
type State string

const (
	StateConnecting State = "connecting"
	StateLogin      State = "login" // waiting for the user in the browser
	StateReady      State = "ready"
	StateError      State = "error"
)

type Options struct {
	ConfigDir string
	CacheDir  string
	Log       *slog.Logger

	// ClientToken supplies the client-token so the daemon and the internal
	// API client present the same identity. On error the daemon fetches its own.
	ClientToken func(context.Context) (string, error)

	OnEvent    func(*daemon.ApiEvent) // player events, delivered in order off the player loop
	OnState    func(State, error)
	OnLoginURL func(string)
}

type Player struct {
	opts  Options
	store *stateStore

	mu     sync.Mutex
	reqs   chan daemon.ApiRequest
	cancel context.CancelFunc
	done   chan struct{}
}

func New(opts Options) *Player {
	// go-librespot draws device ids, playback ids and nonces from
	// golang.org/x/exp/rand, which only its own main() seeds; unseeded, every
	// install would generate the same "random" device id.
	seedOnce.Do(func() { exprand.Seed(uint64(time.Now().UnixNano())) })
	return &Player{opts: opts, store: newStateStore(opts.ConfigDir)}
}

// bridge implements daemon.ApiServer over channels.
type bridge struct {
	reqs   chan daemon.ApiRequest
	events chan *daemon.ApiEvent
}

// Emit is called on the player loop and must not block. If the UI falls far
// behind the oldest pending event is dropped; the UI re-reads full status anyway.
func (b *bridge) Emit(ev *daemon.ApiEvent) {
	for {
		select {
		case b.events <- ev:
			return
		default:
			select {
			case <-b.events:
			default:
			}
		}
	}
}
func (b *bridge) Receive() <-chan daemon.ApiRequest { return b.reqs }
func (b *bridge) SetAuthCode(*daemon.ApiDeviceAuth) {}
func (b *bridge) Close() error                      { return nil }

func (p *Player) config() *daemon.Config {
	cfg := &daemon.Config{
		DeviceName:       "Personal Spotify",
		DeviceType:       "computer",
		AudioBackend:     "pulseaudio", // PipeWire serves the pulse protocol too
		Bitrate:          320,
		VolumeSteps:      65535,
		InitialVolume:    65535,
		AudioBufferTime:  500000,
		AudioPeriodCount: 4,
	}
	cfg.Cache.Enabled = true
	cfg.Cache.Dir = filepath.Join(p.opts.CacheDir, "audio")
	cfg.Cache.SizeLimit = 1 << 30
	cfg.Metadata.Enabled = true
	cfg.Credentials.Type = "interactive"
	if p.opts.ClientToken != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if tok, err := p.opts.ClientToken(ctx); err == nil {
			cfg.ClientToken = tok
		} else {
			p.opts.Log.Warn("client-token unavailable, daemon will fetch its own", "err", err)
		}
	}
	return cfg
}

// Start launches the daemon in the background. It re-launches it with backoff
// if the session drops, until Stop is called.
func (p *Player) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done = cancel, make(chan struct{})
	b := &bridge{reqs: make(chan daemon.ApiRequest), events: make(chan *daemon.ApiEvent, 64)}
	p.reqs = b.reqs

	go func() {
		for {
			select {
			case ev := <-b.events:
				if p.opts.OnEvent != nil {
					p.opts.OnEvent(ev)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	go p.supervise(ctx, b)
}

func (p *Player) supervise(ctx context.Context, b *bridge) {
	defer close(p.done)
	lg := logger{p.opts.Log, p.opts.OnLoginURL}
	backoff := time.Second

	var mp mpris.Server = mpris.DummyServer{}
	if s, err := mpris.NewServer(lg); err != nil {
		p.opts.Log.Warn("mpris unavailable", "err", err)
	} else {
		mp = s
		defer s.Close()
	}

	for ctx.Err() == nil {
		app, err := daemon.New(&daemon.Options{
			Logger: lg, Config: p.config(), StateStore: p.store, APIServer: b, MediaPlayer: noClose{mp},
		})
		if err == nil {
			st, _ := p.store.Load()
			if st != nil && len(st.Credentials.Data) == 0 {
				p.setState(StateLogin, nil)
			} else {
				p.setState(StateConnecting, nil)
			}
			go p.waitReady(ctx)
			started := time.Now()
			err = app.Run(ctx)
			_ = app.Close()
			if time.Since(started) > time.Minute {
				backoff = time.Second
			}
		}
		if ctx.Err() != nil {
			return
		}
		p.opts.Log.Error("daemon stopped", "err", err)
		p.setState(StateError, err)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		backoff = min(backoff*2, time.Minute)
	}
}

// noClose keeps the MPRIS server alive across daemon restarts; daemon.App.Close
// would otherwise close it.
type noClose struct{ mpris.Server }

func (n noClose) Close() error { return nil }

// waitReady polls status until the session answers, then reports StateReady.
func (p *Player) waitReady(ctx context.Context) {
	for ctx.Err() == nil {
		c, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, err := p.Do(c, daemon.ApiRequestTypeStatus, nil)
		cancel()
		if err == nil {
			p.setState(StateReady, nil)
			return
		}
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
		}
	}
}

func (p *Player) setState(s State, err error) {
	if p.opts.OnState != nil {
		p.opts.OnState(s, err)
	}
}

// Stop shuts the daemon down and waits for it.
func (p *Player) Stop() {
	p.mu.Lock()
	cancel, done := p.cancel, p.done
	p.cancel, p.reqs = nil, nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

// DeviceID is the persisted Spotify device id ("" before the first run).
func (p *Player) DeviceID() string {
	st, err := p.store.Load()
	if err != nil {
		return ""
	}
	return st.DeviceId
}

// Logout stops the session, wipes credentials, state and the audio cache,
// then starts again at the login screen.
func (p *Player) Logout() error {
	p.Stop()
	err := p.store.wipe()
	err = errors.Join(err, os.RemoveAll(filepath.Join(p.opts.CacheDir, "audio")))
	p.Start()
	return err
}

var seedOnce sync.Once

var ErrNotRunning = errors.New("player not running")

// Do sends a request to the daemon and waits for its reply.
func (p *Player) Do(ctx context.Context, typ daemon.ApiRequestType, data any) (any, error) {
	p.mu.Lock()
	reqs := p.reqs
	p.mu.Unlock()
	if reqs == nil {
		return nil, ErrNotRunning
	}
	req, wait := daemon.NewApiRequest(typ, data)
	select {
	case reqs <- req:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return wait(ctx)
}
