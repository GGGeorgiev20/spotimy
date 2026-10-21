// Package backend owns the session, the player and the internal API client.
// The UI calls its methods directly; each one returns a typed result or an
// error that the UI shows as "unavailable" for that section.
package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/GGGeorgiev20/personal-spotify/internal/imgcache"
	"github.com/GGGeorgiev20/personal-spotify/internal/player"
	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	librespot "github.com/devgianlu/go-librespot"
	"github.com/devgianlu/go-librespot/daemon"
	"github.com/devgianlu/go-librespot/login5"
	credentialspb "github.com/devgianlu/go-librespot/proto/spotify/login5/v3/credentials"
)

// Config holds paths plus the optional user overrides from config.json.
type Config struct {
	ConfigDir  string `json:"-"`
	CacheDir   string `json:"-"`
	AppVersion string `json:"appVersion"` // spotify-app-version; bump when Spotify rejects the old one
	Language   string `json:"language"`   // Accept-Language, e.g. "en" or "bg"
}

func LoadConfig(configDir, cacheDir string, log *slog.Logger) Config {
	cfg := Config{ConfigDir: configDir, CacheDir: cacheDir}
	data, err := os.ReadFile(filepath.Join(configDir, "config.json"))
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			log.Warn("ignoring bad config.json", "err", err)
		}
	}
	cfg.ConfigDir, cfg.CacheDir = configDir, cacheDir
	return cfg
}

type Options struct {
	Config Config
	Log    *slog.Logger
	// Sandbox is for UI testing: no player session or audio, and every call
	// that would change the account or playback is recorded instead of sent.
	Sandbox bool
	// OnPlayer runs (off the UI goroutine) whenever playback, queue or
	// devices may have changed.
	OnPlayer func()
	// OnPlaybackError runs (off the UI goroutine) with a message for the user
	// when a track couldn't be played.
	OnPlaybackError func(string)
	// OnState runs when the login/session state changes.
	OnState func(AppState)
}

type App struct {
	ctx    context.Context
	cancel context.CancelFunc
	opts   Options
	log    *slog.Logger
	player *player.Player
	sp     *spotify.Client
	img    *imgcache.Cache
	// coverColors caches CoverColor by image URL.
	coverColors sync.Map

	mu       sync.Mutex
	settings Settings
	connect  *spotify.Connect
	state    player.State
	stateErr string
	loginURL string

	// Sandbox only.
	deviceID string
	fake     *PlayerStatus
	calls    []string
}

type AppState struct {
	State    player.State `json:"state"`
	Error    string       `json:"error,omitempty"`
	LoginURL string       `json:"loginUrl,omitempty"`
}

func New(opts Options) *App {
	cfg, log := opts.Config, opts.Log
	a := &App{opts: opts, log: log, state: player.StateConnecting}
	a.ctx, a.cancel = context.WithCancel(context.Background())
	a.img = imgcache.New(filepath.Join(cfg.CacheDir, "images"), 300<<20, log)
	a.settings = a.loadSettings()
	if cfg.Language == "" {
		cfg.Language = a.settings.Language
	}
	a.player = player.New(player.Options{
		ConfigDir: cfg.ConfigDir,
		CacheDir:  cfg.CacheDir,
		Log:       log,
		ClientToken: func(ctx context.Context) (string, error) {
			return a.sp.ClientToken(ctx)
		},
		OnEvent: func(ev *daemon.ApiEvent) {
			if ev.Type == daemon.ApiEventTypePlaybackError && a.opts.OnPlaybackError != nil {
				if d, ok := ev.Data.(daemon.ApiEventDataPlaybackError); ok {
					a.opts.OnPlaybackError(playbackErrorText(d))
				}
			}
			a.emitPlayer()
		},
		OnState: func(s player.State, err error) {
			a.mu.Lock()
			a.state, a.stateErr = s, ""
			if err != nil {
				a.stateErr = err.Error()
			}
			if s == player.StateReady {
				a.loginURL = ""
				go a.startConnect()
			}
			a.mu.Unlock()
			a.emitState()
		},
		OnLoginURL: func(url string) {
			a.mu.Lock()
			a.loginURL = url
			a.mu.Unlock()
			OpenURL(url)
			a.emitState()
		},
	})
	a.player.SetAudio(a.settings.audio())

	id := spotify.DefaultIdentity()
	id.DeviceID = a.player.DeviceID()
	if cfg.AppVersion != "" {
		id.AppVersion = cfg.AppVersion
	}
	if cfg.Language != "" {
		id.Language = cfg.Language
	}
	token := a.token
	if opts.Sandbox {
		b := make([]byte, 20)
		rand.Read(b)
		a.deviceID = hex.EncodeToString(b)
		id.DeviceID = a.deviceID
		token = a.sandboxToken()
	}
	a.sp = spotify.New(id, token, log, filepath.Join(cfg.ConfigDir, "operations.json"))
	return a
}

// playbackErrorText words a failed play for the user.
func playbackErrorText(d daemon.ApiEventDataPlaybackError) string {
	// ponytail: matched on go-librespot's message; its KeyProviderError isn't in the event.
	if strings.Contains(d.Message, "aes key with code 2") {
		return "Spotify is refusing playback for a moment (too many requests). Try again in a minute."
	}
	return "Couldn't play this song"
}

func (a *App) emitPlayer() {
	if a.opts.OnPlayer != nil {
		a.opts.OnPlayer()
	}
}

func (a *App) emitState() {
	if a.opts.OnState != nil {
		a.opts.OnState(a.GetState())
	}
}

// token asks the session for a login5 bearer token.
func (a *App) token(ctx context.Context) (string, error) {
	res, err := a.player.Do(ctx, daemon.ApiRequestTypeToken, nil)
	if err != nil {
		return "", err
	}
	return res.(*daemon.ApiToken).Token, nil
}

// sandboxToken logs in to login5 directly with the stored credentials, the way
// cmd/spdev does, so no player session (and no Connect device) is created.
func (a *App) sandboxToken() func(context.Context) (string, error) {
	var (
		mu sync.Mutex
		l5 *login5.Login5
	)
	return func(ctx context.Context) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if l5 == nil {
			user, cred, _, err := player.StoredCredentials(a.opts.Config.ConfigDir)
			if err != nil {
				return "", err
			}
			ct, err := a.sp.ClientToken(ctx)
			if err != nil {
				return "", err
			}
			l := login5.NewLogin5(&librespot.NullLogger{}, http.DefaultClient, a.deviceID, ct)
			if err := l.Login(ctx, &credentialspb.StoredCredential{Username: user, Data: cred}); err != nil {
				return "", err
			}
			l5 = l
		}
		return l5.AccessToken()(ctx, false)
	}
}

// Start begins logging in. In sandbox mode the session is ready at once.
func (a *App) Start() {
	if a.opts.Sandbox {
		a.mu.Lock()
		a.state = player.StateReady
		a.mu.Unlock()
		go a.startConnect()
		a.emitState()
		return
	}
	a.player.Start()
}

func (a *App) Stop() {
	a.stopConnect()
	if !a.opts.Sandbox {
		a.player.Stop()
	}
	a.cancel()
}

func (a *App) Sandbox() bool { return a.opts.Sandbox }

// sandboxed records a mutating call in sandbox mode and reports whether the
// caller must skip the real work.
func (a *App) sandboxed(name string, args ...any) bool {
	if !a.opts.Sandbox {
		return false
	}
	call := fmt.Sprintf("%s%v", name, args)
	a.mu.Lock()
	a.calls = append(a.calls, call)
	a.mu.Unlock()
	a.log.Info("sandbox: skipped", "call", call)
	return true
}

// SandboxCalls returns the calls recorded in sandbox mode.
func (a *App) SandboxCalls() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.calls...)
}

func (a *App) GetState() AppState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return AppState{a.state, a.stateErr, a.loginURL}
}

// OpenLoginURL re-opens the login page if the browser tab was closed.
func (a *App) OpenLoginURL() {
	if s := a.GetState(); s.LoginURL != "" {
		OpenURL(s.LoginURL)
	}
}

func (a *App) Logout() error {
	if a.sandboxed("Logout") {
		return nil
	}
	a.stopConnect()
	a.sp.Reset()
	return errors.Join(a.player.Logout(), a.img.Clear())
}

// do forwards a request to the daemon. Payload types must match what the
// daemon's handler type-asserts (see go-librespot daemon/player.go), or its
// player loop panics.
func (a *App) do(typ daemon.ApiRequestType, data any) (any, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	res, err := a.player.Do(ctx, typ, data)
	if err != nil && !errors.Is(err, daemon.ErrNoSession) {
		a.log.Warn("player request failed", "type", typ, "err", err)
	} else if typ != daemon.ApiRequestTypeStatus {
		a.log.Debug("player request", "type", typ, "data", data)
	}
	return res, err
}

func (a *App) OpenAccountPage() {
	OpenURL("https://www.spotify.com/account/overview/")
}

// OpenURL opens a link in the default browser.
func OpenURL(url string) {
	cmd := exec.Command("xdg-open", url)
	if err := cmd.Start(); err == nil {
		go cmd.Wait()
	}
}
