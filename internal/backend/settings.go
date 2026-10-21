package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/devgianlu/go-librespot/daemon"
)

// Settings are the Settings page's values, kept in settings.json in the
// config dir. The UI owns the display ones; the backend applies the audio
// ones to the player and the language to the API client.
type Settings struct {
	Language string `json:"language"` // Accept-Language ("" = the system's); applied on restart

	Quality     string `json:"quality"`     // automatic, normal, high, very-high
	Normalize   bool   `json:"normalize"`   // volume normalisation
	VolumeLevel string `json:"volumeLevel"` // loud, normal, quiet (with Normalize)
	Crossfade   bool   `json:"crossfade"`
	CrossfadeS  int    `json:"crossfadeSeconds"` // 1..12
	Autoplay    bool   `json:"autoplay"`
	Mono        bool   `json:"mono"`

	NPVOnPlay  bool   `json:"npvOnPlay"`  // show the Now Playing view on click of play
	FriendFeed bool   `json:"friendFeed"` // the top bar's Friend Activity button
	Canvas     bool   `json:"canvas"`     // looping visuals in the Now Playing view
	Zoom       string `json:"zoom"`       // dense, default, spacious
	Autostart  string `json:"autostart"`  // off, on, minimized
}

// DefaultSettings are Spotify's defaults.
var DefaultSettings = Settings{
	Quality: "automatic", Normalize: true, VolumeLevel: "normal", CrossfadeS: 5, Autoplay: true,
	NPVOnPlay: true, FriendFeed: true, Canvas: true, Zoom: "default", Autostart: "off",
}

func (a *App) settingsPath() string { return filepath.Join(a.opts.Config.ConfigDir, "settings.json") }

// loadSettings reads settings.json over the defaults.
func (a *App) loadSettings() Settings {
	s := DefaultSettings
	if data, err := os.ReadFile(a.settingsPath()); err == nil {
		if err := json.Unmarshal(data, &s); err != nil {
			a.log.Warn("ignoring bad settings.json", "err", err)
			s = DefaultSettings
		}
	}
	return s
}

// Settings returns the current settings.
func (a *App) Settings() Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings
}

// SetSettings saves s and applies what changed.
func (a *App) SetSettings(s Settings) error {
	a.mu.Lock()
	old := a.settings
	a.settings = s
	a.mu.Unlock()
	if s.audio() != old.audio() {
		a.player.SetAudio(s.audio())
	}
	if a.sandboxed("SetSettings", s) {
		return nil
	}
	var errs []error
	if s.Autostart != old.Autostart {
		errs = append(errs, setAutostart(s.Autostart))
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	if err := os.MkdirAll(filepath.Dir(a.settingsPath()), 0o700); err != nil {
		errs = append(errs, err)
	} else {
		errs = append(errs, os.WriteFile(a.settingsPath(), data, 0o600))
	}
	return errors.Join(errs...)
}

// audio is what the settings ask of the player.
func (s Settings) audio() daemon.ApiAudioOptions {
	o := daemon.ApiAudioOptions{Bitrate: 320, Autoplay: s.Autoplay, Normalisation: s.Normalize, Mono: s.Mono}
	switch s.Quality {
	case "normal":
		o.Bitrate = 96
	case "high":
		o.Bitrate = 160
	}
	// Spotify's levels: loud -11 LUFS, normal -14, quiet -19.
	switch s.VolumeLevel {
	case "loud":
		o.Pregain = 3
	case "quiet":
		o.Pregain = -5
	}
	if s.Crossfade {
		o.CrossfadeMs = max(1, min(12, s.CrossfadeS)) * 1000
	}
	return o
}

// autostartPath is the XDG autostart entry.
func autostartPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart", "personal-spotify.desktop"), nil
}

// setAutostart writes or removes the autostart entry ("minimized" starts the
// window minimized).
func setAutostart(mode string) error {
	path, err := autostartPath()
	if err != nil {
		return err
	}
	if mode != "on" && mode != "minimized" {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return writeAutostart(path, exe, mode == "minimized")
}

func writeAutostart(path, exe string, minimized bool) error {
	cmd := quoteExec(exe)
	if minimized {
		cmd += " --minimized"
	}
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Spotify\nExec=%s\nIcon=spotify\nX-GNOME-Autostart-enabled=true\n", cmd)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(entry), 0o644)
}

// quoteExec quotes a path for a desktop entry's Exec key.
func quoteExec(s string) string {
	if !strings.ContainsAny(s, " \t\"'\\$`") {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`)
	return `"` + r.Replace(s) + `"`
}

// CacheSize is how much the image and audio caches take, in bytes.
func (a *App) CacheSize() int64 {
	var n int64
	for _, d := range []string{"images", "audio"} {
		_ = filepath.WalkDir(filepath.Join(a.opts.Config.CacheDir, d), func(_ string, e fs.DirEntry, err error) error {
			if err == nil && !e.IsDir() {
				if fi, err := e.Info(); err == nil {
					n += fi.Size()
				}
			}
			return nil
		})
	}
	return n
}

// ClearCache removes the cached pictures and audio files; the player fetches
// what it needs again.
func (a *App) ClearCache() error {
	if a.sandboxed("ClearCache") {
		return nil
	}
	errs := []error{a.img.Clear()}
	audio := filepath.Join(a.opts.Config.CacheDir, "audio")
	entries, err := os.ReadDir(audio)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		errs = append(errs, err)
	}
	for _, e := range entries {
		errs = append(errs, os.RemoveAll(filepath.Join(audio, e.Name())))
	}
	return errors.Join(errs...)
}
