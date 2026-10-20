package main

import "testing"

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
