package ui

import (
	"strings"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// The play history is the queue's "Recently played": like the desktop
// client's player, the tracks that started playing, newest first, kept
// locally (in the prefs, cleared on log out).

// historyMax is how many played tracks are kept.
const historyMax = 50

// HistoryVersion changes whenever a track is added to the history.
var HistoryVersion int

// notePlayed adds the track s is playing, unless it's the latest one already.
func notePlayed(s *backend.PlayerStatus) {
	if s == nil || s.Track == nil || !s.Playing {
		return
	}
	uri := s.Track.URI
	if !strings.HasPrefix(uri, "spotify:track:") && !strings.HasPrefix(uri, "spotify:episode:") {
		return
	}
	if len(Prefs.History) > 0 && Prefs.History[0] == uri {
		return
	}
	Prefs.History = append([]string{uri}, Prefs.History[:min(len(Prefs.History), historyMax-1)]...)
	HistoryVersion++
}

// RecentlyPlayed is the history before the playing track.
func RecentlyPlayed() []string {
	h := Prefs.History
	if t := NowTrack(); t != nil && len(h) > 0 && h[0] == t.URI {
		h = h[1:]
	}
	return append([]string(nil), h...)
}

// HistoryTracks looks up the tracks of uris (repeats included), in order.
// It may run off the UI goroutine.
func HistoryTracks(uris []string) ([]spotify.Track, error) {
	seen := map[string]bool{}
	var uniq []string
	for _, u := range uris {
		if !seen[u] {
			seen[u] = true
			uniq = append(uniq, u)
		}
	}
	ts, err := B.Tracks(uniq)
	if err != nil {
		return nil, err
	}
	byURI := make(map[string]spotify.Track, len(ts))
	for _, t := range ts {
		byURI[t.URI] = t
	}
	out := []spotify.Track{}
	for _, u := range uris {
		if t, ok := byURI[u]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}
