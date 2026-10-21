package ui

import (
	"slices"
	"testing"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

func TestViewOrder(t *testing.T) {
	tr := func(name, artist, album, added string, ms int) *spotify.Track {
		return &spotify.Track{Name: name, Artists: []spotify.Artist{{Name: artist}}, Album: &spotify.AlbumRef{Name: album}, AddedAt: added, DurationMs: ms}
	}
	rows := []*spotify.Track{
		tr("beta", "Zed", "Gamma", "2026-01-02", 300),
		nil, // not loaded yet
		tr("Alpha", "Yan", "Delta", "2026-03-01", 100),
		tr("gamma", "Zed", "Alpha", "2026-02-01", 200),
	}
	for _, c := range []struct {
		query, key string
		desc       bool
		want       []int
	}{
		{"", SortCustom, false, []int{0, 2, 3}},
		{"", SortTitle, false, []int{2, 0, 3}}, // case-insensitive
		{"", SortTitle, true, []int{3, 0, 2}},
		{"", SortArtist, false, []int{2, 0, 3}}, // ties keep list order
		{"", SortAlbum, false, []int{3, 2, 0}},
		{"", SortAdded, false, []int{2, 3, 0}}, // newest first
		{"", SortAdded, true, []int{0, 3, 2}},
		{"", SortDuration, false, []int{2, 3, 0}},
		{"ZED", SortCustom, false, []int{0, 3}}, // artist
		{"delta", SortTitle, false, []int{2}},   // album
		{" a ", SortCustom, false, []int{0, 2, 3}},
		{"nothing", SortCustom, false, nil},
	} {
		if got := ViewOrder(rows, c.query, c.key, c.desc); !slices.Equal(got, c.want) {
			t.Errorf("ViewOrder(%q, %s, desc=%v) = %v, want %v", c.query, c.key, c.desc, got, c.want)
		}
	}
}

func TestHeaderSort(t *testing.T) {
	l := ListControls{Default: SortCustom, Sort: SortCustom}
	for _, want := range []struct {
		sort string
		desc bool
	}{{SortTitle, false}, {SortTitle, true}, {SortCustom, false}} {
		l.HeaderSort(SortTitle)
		if l.Sort != want.sort || l.Desc != want.desc {
			t.Fatalf("got %s desc=%v, want %s desc=%v", l.Sort, l.Desc, want.sort, want.desc)
		}
	}
	l.HeaderSort(SortTitle)
	l.HeaderSort(SortAlbum) // another column starts over
	if l.Sort != SortAlbum || l.Desc || !l.SortedBy(SortAlbum) || l.SortedBy(SortTitle) {
		t.Fatalf("got %s desc=%v", l.Sort, l.Desc)
	}
}
