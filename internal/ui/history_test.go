package ui

import (
	"testing"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

func TestCollapseRepeats(t *testing.T) {
	tr := func(uri, name, artist string) spotify.Track {
		return spotify.Track{URI: uri, Name: name, Artists: []spotify.Artist{{Name: artist}}}
	}
	got := collapseRepeats([]spotify.Track{
		tr("a", "Song", "X"), tr("a2", "Song", "X"), // relinked: one play
		tr("b", "Other", "Y"),
		tr("c", "Song", "Z"), // same title, another artist
		tr("a", "Song", "X"), // played again later
	})
	var uris []string
	for _, t := range got {
		uris = append(uris, t.URI)
	}
	if want := []string{"a", "b", "c", "a"}; len(uris) != len(want) || uris[0] != "a" || uris[1] != "b" || uris[2] != "c" || uris[3] != "a" {
		t.Errorf("got %v, want %v", uris, want)
	}
}
