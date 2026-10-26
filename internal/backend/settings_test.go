package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsAudio(t *testing.T) {
	s := DefaultSettings
	if o := s.audio(); o.Bitrate != 320 || !o.Autoplay || !o.Normalisation || o.Pregain != 0 || o.CrossfadeMs != 0 || o.Mono {
		t.Errorf("defaults: %+v", o)
	}
	s.Quality, s.VolumeLevel, s.Crossfade, s.CrossfadeS, s.Autoplay, s.Mono = "high", "quiet", true, 20, false, true
	if o := s.audio(); o.Bitrate != 160 || o.Autoplay || o.Pregain != -5 || o.CrossfadeMs != 12000 || !o.Mono {
		t.Errorf("changed: %+v", o)
	}
}

func TestWriteAutostart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "autostart", "x.desktop")
	if err := writeAutostart(path, "/opt/my apps/personal-spotify", true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `Exec="/opt/my apps/personal-spotify" --minimized`) {
		t.Errorf("entry:\n%s", data)
	}
}

func TestResumePlay(t *testing.T) {
	p := resumePlay(LastPlayback{Context: "spotify:album:a", Track: "spotify:track:t", PositionMs: 61000})
	if p.Uri != "spotify:album:a" || p.SkipToUri != "spotify:track:t" || p.Position != 61000 || !p.Paused {
		t.Errorf("in a context: %+v", p)
	}
	p = resumePlay(LastPlayback{Context: "spotify:track:t", Track: "spotify:track:t"})
	if p.Uri != "spotify:track:t" || p.SkipToUri != "" || !p.Paused {
		t.Errorf("on its own: %+v", p)
	}
}
