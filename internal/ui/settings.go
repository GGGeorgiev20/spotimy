package ui

import (
	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
)

// Settings is the Settings page's values (read by the parts they change).
var Settings = backend.DefaultSettings

// StartMinimized opens the window minimized (the autostart entry's
// --minimized).
var StartMinimized bool

// SaveSettings stores and applies Settings after the page changed them.
func SaveSettings() {
	s := Settings
	Go("Couldn't save the settings", func() error { return B.SetSettings(s) }, nil)
}

// zoom is the interface scale the zoom level asks for.
func zoom() float32 {
	switch Settings.Zoom {
	case "dense":
		return .9
	case "spacious":
		return 1.15
	}
	return 1
}

// showNPVOnPlay opens the Now Playing view after a click of play, if the
// setting asks for it and no other panel takes its place.
func showNPVOnPlay() {
	if Settings.NPVOnPlay && Prefs.RightPanel == "" {
		Prefs.NPVHidden = false
	}
}
