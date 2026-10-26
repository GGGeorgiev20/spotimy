package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
)

// The last playback (Prefs.Last) is kept as it changes and loaded again,
// paused where it was, when the app starts.

// resumed is set once the start-up resume was tried.
var resumed bool

// noteLast keeps what s plays as the playback to resume.
func noteLast(s *backend.PlayerStatus) {
	if s == nil || s.Track == nil {
		return
	}
	Prefs.Last = &backend.LastPlayback{
		Context: s.ContextURI, Track: s.Track.URI, PositionMs: s.PositionMs,
		Shuffle: s.Shuffle, Repeat: RepeatMode(),
	}
}

// keepPosition moves the kept position along while playing, in 5 second
// steps so the prefs are written at most that often.
func keepPosition(now time.Time) {
	if l, t := Prefs.Last, NowTrack(); l != nil && t != nil && Playing() && l.Track == t.URI {
		l.PositionMs = Position(now).Milliseconds() / 5000 * 5000
	}
}

// resumeLast loads the last playback once the player is ready.
func resumeLast() {
	if resumed || Prefs.Last == nil {
		return
	}
	resumed = true
	last := *Prefs.Last
	Async(func() (struct{}, error) { return struct{}{}, B.Resume(last) }, func(_ struct{}, err error) {
		if err != nil {
			Log.Warn("resume last playback", "err", err)
		}
		RefreshPlayer()
	})
}

// saveLastNow records the exact position and writes the prefs at once (the
// window is closing, the process is about to end).
func saveLastNow() {
	if l, t := Prefs.Last, NowTrack(); l != nil && t != nil && l.Track == t.URI {
		l.PositionMs = Position(time.Now()).Milliseconds()
	}
	if prefsPath == "" {
		return
	}
	data, _ := json.Marshal(Prefs)
	_ = os.MkdirAll(filepath.Dir(prefsPath), 0o700)
	_ = os.WriteFile(prefsPath, data, 0o600)
}
