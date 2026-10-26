package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestSpotifyURI(t *testing.T) {
	for in, want := range map[string]string{
		"spotify:album:abc":                          "spotify:album:abc",
		"https://open.spotify.com/track/xyz?si=123":  "spotify:track:xyz",
		"https://open.spotify.com/intl-de/artist/a1": "spotify:artist:a1",
		"https://open.spotify.com/user/bob":          "spotify:user:bob",
		"https://example.com/track/x":                "",
		"--flag":                                     "",
	} {
		if got := spotifyURI(in); got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestMoveDir(t *testing.T) {
	base := t.TempDir()
	old, dir := filepath.Join(base, "personal-spotify"), filepath.Join(base, "spotimy")
	os.MkdirAll(old, 0o700)
	os.WriteFile(filepath.Join(old, "ui.json"), []byte("{}"), 0o600)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if got := moveDir(old, dir, log); got != dir {
		t.Fatalf("moved to %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "ui.json")); err != nil {
		t.Fatal("the old folder's files didn't move:", err)
	}
	os.MkdirAll(old, 0o700) // both exist: the new one wins, the old one is left alone
	if got := moveDir(old, dir, log); got != dir {
		t.Fatalf("got %s", got)
	}
}
