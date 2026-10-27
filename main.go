package main

import (
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"syscall"
	"time"

	"gioui.org/app"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/pages"
)

func main() {
	level := slog.LevelInfo
	if os.Getenv("SPOTIMY_LOG") == "debug" {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	// Trade a little CPU for a smaller heap; the app is mostly idle.
	debug.SetGCPercent(50)

	setCursorTheme()

	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Error("no config dir", "err", err)
		os.Exit(1)
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Error("no cache dir", "err", err)
		os.Exit(1)
	}
	// SPOTIMY_HARNESS=127.0.0.1:port runs the UI headless in sandbox
	// mode for automated testing (see internal/ui/harness.go).
	harness := os.Getenv("SPOTIMY_HARNESS")
	if harness == "" && forwardToRunning(os.Args[1:]) {
		log.Info("already running; passed the arguments on")
		return
	}
	oldConfig, oldCache := filepath.Join(configDir, "personal-spotify"), filepath.Join(cacheDir, "personal-spotify")
	configDir, cacheDir = filepath.Join(configDir, "spotimy"), filepath.Join(cacheDir, "spotimy")
	if harness == "" { // the harness may run next to the real app: it never moves its folders
		configDir, cacheDir = moveDir(oldConfig, configDir, log), moveDir(oldCache, cacheDir, log)
	}
	b := backend.New(backend.Options{
		Config:          backend.LoadConfig(configDir, cacheDir, log),
		Log:             log,
		Sandbox:         harness != "",
		OnPlayer:        ui.OnPlayer,
		OnPlaybackError: func(msg string) { ui.Post(func() { ui.Toast(msg) }) },
		OnWentBack:      func() { ui.Post(func() { ui.WentBack = time.Now() }) },
		OnState:         ui.OnState,
	})
	ui.Log = log
	ui.StartMinimized = slices.Contains(os.Args[1:], "--minimized") // the autostart entry
	prefsDir := configDir
	if harness != "" {
		prefsDir = filepath.Join(cacheDir, "harness") // don't touch the real prefs
	}
	ui.Init(b, prefsDir)
	b.Start()

	if harness != "" {
		w, h, scale := envInt("SPOTIMY_HARNESS_W", 1280), envInt("SPOTIMY_HARNESS_H", 800), float32(1)
		if err := ui.RunHarness(harness, w, h, scale); err != nil {
			log.Error("harness", "err", err)
			os.Exit(1)
		}
		return
	}

	open := func(arg string) {
		if uri := spotifyURI(arg); uri != "" {
			ui.Post(func() { ui.OpenURI(uri) })
		}
	}
	stop, err := listenForInstances(open)
	if err != nil {
		log.Warn("single-instance socket unavailable", "err", err)
		stop = func() {}
	}
	for _, a := range os.Args[1:] {
		open(a)
	}

	quit := func() {
		stop()
		b.Stop()
		os.Exit(0)
	}
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		quit()
	}()
	go func() {
		ui.Run()
		quit()
	}()
	app.Main()
}

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

// moveDir renames the folder of the app's old name (personal-spotify) to
// its new one the first time Spotimy runs, keeping the settings, play
// history and cache, and returns the new one.
func moveDir(old, dir string, log *slog.Logger) string {
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	if _, err := os.Stat(old); err != nil {
		return dir
	}
	if err := os.Rename(old, dir); err != nil {
		log.Warn("couldn't move the old folder", "from", old, "to", dir, "err", err)
		return old
	}
	return dir
}
