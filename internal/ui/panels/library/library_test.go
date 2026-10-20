package library

import (
	"testing"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

func TestSubtitles(t *testing.T) {
	pl := &spotify.LibraryItem{Card: spotify.Card{Type: "playlist", Subtitle: "Playlist • Me"}}
	liked := &spotify.LibraryItem{Card: spotify.Card{Type: "collection", Subtitle: "Playlist"}, Count: 1234}
	artist := &spotify.LibraryItem{Card: spotify.Card{Type: "artist", Subtitle: "Artist"}}
	for _, c := range []struct {
		it            *spotify.LibraryItem
		full, compact string
	}{
		{pl, "Playlist • Me", "Me"},
		{liked, "Playlist • 1,234 songs", "1,234 songs"},
		{artist, "Artist", "Artist"},
	} {
		if got := subtitle(c.it); got != c.full {
			t.Errorf("subtitle = %q, want %q", got, c.full)
		}
		if got := compactSub(c.it); got != c.compact {
			t.Errorf("compactSub = %q, want %q", got, c.compact)
		}
	}
}

func TestGridCols(t *testing.T) {
	for w, want := range map[int]int{72: 1, 280: 1, 340: 2, 480: 2, 600: 3} {
		if got := gridCols(w, 1); got != want {
			t.Errorf("gridCols(%d) = %d, want %d", w, got, want)
		}
	}
}

func TestEnsureRequestsEachPageOnce(t *testing.T) {
	p := &page{items: make([]*spotify.LibraryItem, 120), asked: map[int]bool{0: true, 1: true, 2: true}}
	p.ensure(0, 120) // every page already asked: nothing to fetch (would need the backend)
	if len(p.asked) != 3 {
		t.Errorf("asked %v", p.asked)
	}
}
