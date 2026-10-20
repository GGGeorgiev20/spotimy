package queue

import (
	"slices"
	"testing"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

func tracks(spec string) []backend.QueueTrack {
	var out []backend.QueueTrack
	for _, c := range spec { // lowercase = user queue, uppercase = context
		pr := "context"
		if c >= 'a' {
			pr = "queue"
		}
		out = append(out, backend.QueueTrack{Track: spotify.Track{URI: string(c)}, Provider: pr})
	}
	return out
}

func uris(next []backend.QueueTrack) string {
	var s []byte
	for _, q := range next {
		s = append(s, q.URI...)
	}
	return string(s)
}

func TestQueueEdits(t *testing.T) {
	next := tracks("abcXY")
	for _, c := range []struct{ got, want string }{
		{uris(removeAt(next, 1)), "acXY"},
		{uris(clearQueued(next)), "XY"},
		{uris(moveTo(next, 0, 2)), "bcaXY"},
		{uris(moveTo(next, 2, 0)), "cabXY"},
		{uris(moveTo(next, 1, 1)), "abcXY"},
	} {
		if c.got != c.want {
			t.Errorf("got %s want %s", c.got, c.want)
		}
	}
	if uris(next) != "abcXY" {
		t.Error("input modified")
	}
}

func TestDropIndex(t *testing.T) {
	for _, c := range []struct{ from, dy, want int }{
		{0, 0, 0}, {0, 30, 1}, {0, 27, 0}, {2, -60, 1}, {1, -500, 0}, {1, 500, 2},
	} {
		if got := dropIndex(c.from, c.dy, 56, 3); got != c.want {
			t.Errorf("dropIndex(%d,%d) = %d, want %d", c.from, c.dy, got, c.want)
		}
	}
}

func TestLayoutItems(t *testing.T) {
	items := layoutItems(true, tracks("abX"))
	var kinds []int
	for _, it := range items {
		kinds = append(kinds, it.kind)
	}
	want := []int{itemNowHead, itemNow, itemQueuedHead, itemTrack, itemTrack, itemNextHead, itemTrack}
	if !slices.Equal(kinds, want) {
		t.Errorf("kinds %v, want %v", kinds, want)
	}
	if n := len(layoutItems(false, nil)); n != 0 {
		t.Errorf("empty queue has %d items", n)
	}
}
