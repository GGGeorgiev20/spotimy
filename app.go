package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/GGGeorgiev20/personal-spotify/internal/player"
	"github.com/devgianlu/go-librespot/daemon"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is bound to the frontend. Methods here stay thin: they forward to the
// player (and, from milestone 2, to internal/spotify).
type App struct {
	ctx    context.Context
	log    *slog.Logger
	player *player.Player

	mu       sync.Mutex
	state    player.State
	stateErr string
	loginURL string
}

type AppState struct {
	State    player.State `json:"state"`
	Error    string       `json:"error,omitempty"`
	LoginURL string       `json:"loginUrl,omitempty"`
}

func NewApp(log *slog.Logger, configDir, cacheDir string) *App {
	a := &App{log: log, state: player.StateConnecting}
	a.player = player.New(player.Options{
		ConfigDir: configDir,
		CacheDir:  cacheDir,
		Log:       log,
		OnEvent: func(ev *daemon.ApiEvent) {
			runtime.EventsEmit(a.ctx, "player:event", ev)
		},
		OnState: func(s player.State, err error) {
			a.mu.Lock()
			a.state, a.stateErr = s, ""
			if err != nil {
				a.stateErr = err.Error()
			}
			if s == player.StateReady {
				a.loginURL = ""
			}
			a.mu.Unlock()
			runtime.EventsEmit(a.ctx, "app:state", a.GetState())
		},
		OnLoginURL: func(url string) {
			a.mu.Lock()
			a.loginURL = url
			a.mu.Unlock()
			runtime.BrowserOpenURL(a.ctx, url)
			runtime.EventsEmit(a.ctx, "app:state", a.GetState())
		},
	})
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.player.Start()
}

func (a *App) shutdown(context.Context) { a.player.Stop() }

func (a *App) GetState() AppState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return AppState{a.state, a.stateErr, a.loginURL}
}

// OpenLoginURL re-opens the login page if the browser tab was closed.
func (a *App) OpenLoginURL() {
	if s := a.GetState(); s.LoginURL != "" {
		runtime.BrowserOpenURL(a.ctx, s.LoginURL)
	}
}

func (a *App) Logout() error { return a.player.Logout() }

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

func (a *App) Status() (*daemon.ApiStatus, error) {
	res, err := a.do(daemon.ApiRequestTypeStatus, nil)
	if err != nil {
		return nil, err
	}
	return res.(*daemon.ApiStatus), nil
}

func (a *App) Play(uri, skipToURI string) error {
	_, err := a.do(daemon.ApiRequestTypePlay, daemon.ApiPlay{Uri: uri, SkipToUri: skipToURI})
	return err
}

func (a *App) PlayPause() error { _, err := a.do(daemon.ApiRequestTypePlayPause, nil); return err }
func (a *App) Next() error      { _, err := a.do(daemon.ApiRequestTypeNext, nil); return err }
func (a *App) Prev() error      { _, err := a.do(daemon.ApiRequestTypePrev, nil); return err }

func (a *App) Seek(ms int64) error {
	_, err := a.do(daemon.ApiRequestTypeSeek, daemon.ApiSeek{Position: ms})
	return err
}

func (a *App) SetVolume(v int32) error {
	_, err := a.do(daemon.ApiRequestTypeSetVolume, daemon.ApiSetVolume{Volume: v})
	return err
}

func (a *App) SetShuffle(on bool) error {
	_, err := a.do(daemon.ApiRequestTypeSetShufflingContext, daemon.ApiShuffleContext{ShuffleContext: on})
	return err
}

// SetRepeat takes "off", "context" or "track".
func (a *App) SetRepeat(mode string) error {
	if _, err := a.do(daemon.ApiRequestTypeSetRepeatingContext, daemon.ApiRepeatContext{RepeatContext: mode != "off"}); err != nil {
		return err
	}
	_, err := a.do(daemon.ApiRequestTypeSetRepeatingTrack, daemon.ApiRepeatTrack{RepeatTrack: mode == "track"})
	return err
}
