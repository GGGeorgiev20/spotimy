package main

import (
	"embed"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	level := slog.LevelInfo
	if os.Getenv("PERSONAL_SPOTIFY_LOG") == "debug" {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	// Trade a little CPU for a smaller heap; the app is mostly idle.
	debug.SetGCPercent(50)

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
	configDir = filepath.Join(configDir, "personal-spotify")
	cacheDir = filepath.Join(cacheDir, "personal-spotify")

	app := NewApp(log, configDir, cacheDir)
	err = wails.Run(&options.App{
		Title:            "Spotify",
		Width:            1280,
		Height:           800,
		MinWidth:         800,
		MinHeight:        600,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
		Linux:            &linux.Options{ProgramName: "personal-spotify", WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand},
	})
	if err != nil {
		log.Error("wails", "err", err)
		os.Exit(1)
	}
}
