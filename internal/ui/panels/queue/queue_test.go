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

func uriString(next []backend.QueueTrack) string {
	var s []byte
	for _, q := range next {
		s = append(s, q.URI...)
	}
	return string(s)
}

func TestQueueEdits(t *testing.T) {
	next := tracks("abcXY")
	for _, c := range []struct{ got, want string }{
		{uriString(removeAt(next, 1)), "acXY"},
		{uriString(clearQueued(next)), "XY"},
		{uriString(moveTo(next, 0, 2)), "bcaXY"},
		{uriString(moveTo(next, 2, 0)), "cabXY"},
		{uriString(moveTo(next, 1, 1)), "abcXY"},
	} {
		if c.got != c.want {
			t.Errorf("got %s want %s", c.got, c.want)
		}
	}
	if uriString(next) != "abcXY" {
		t.Error("input modified")
	}
}

func TestDropInto(t *testing.T) {
	next := tracks("abcXY") // a b c queued, X Y the context
	uid := func() string { return "new" }
	for _, c := range []struct {
		name     string
		tracks   string // dragged in from elsewhere (from = -1)
		from     int
		at       spot
		want     string
		provider string // of the moved or added track, when checked
		appended bool
	}{
		{"queue row down", "", 0, spot{2, false}, "bacXY", "queue", false},
		{"queue row up", "", 2, spot{0, false}, "cabXY", "", false},
		{"queue row to the queue's end", "", 0, spot{3, false}, "bcaXY", "", false},
		{"queue row onto itself", "", 1, spot{1, false}, "", "", false},
		{"queue row into Next from", "", 0, spot{4, true}, "bcXaY", "context", false},
		{"Next from row into the queue", "", 3, spot{1, false}, "aXbcY", "queue", false},
		{"Next from row reordered", "", 4, spot{3, true}, "abcYX", "context", false},
		{"from a list, between queue rows", "Z", -1, spot{1, false}, "aZbcXY", "queue", false},
		{"from a list, at the queue's end", "Z", -1, spot{3, false}, "abcZXY", "queue", true},
		{"from a list, into Next from", "Z", -1, spot{4, true}, "abcXZY", "context", false},
	} {
		var ts []spotify.Track
		for _, r := range c.tracks {
			ts = append(ts, spotify.Track{URI: string(r)})
		}
		out, appended := dropInto(next, 3, ts, c.from, c.at, uid)
		if uriString(out) != c.want || appended != c.appended {
			t.Errorf("%s: got %q appended=%v, want %q appended=%v", c.name, uriString(out), appended, c.want, c.appended)
			continue
		}
		if c.provider != "" {
			moved := "Z"
			if c.from >= 0 {
				moved = next[c.from].URI
			}
			for _, q := range out {
				if q.URI == moved && q.Provider != c.provider {
					t.Errorf("%s: %s is %q, want %q", c.name, moved, q.Provider, c.provider)
				}
			}
		}
	}
	if uriString(next) != "abcXY" {
		t.Error("input modified")
	}
}

func TestLineFor(t *testing.T) {
	for _, c := range []struct {
		s     spot
		row   int
		below bool
	}{
		{spot{0, false}, 0, false},
		{spot{3, false}, 2, true}, // under the last queued row
		{spot{3, true}, 3, false}, // over the first Next from row
		{spot{5, true}, 4, true},  // under the last row
	} {
		row, below, ok := lineFor(c.s, 5, 3)
		if !ok || row != c.row || below != c.below {
			t.Errorf("lineFor(%+v) = %d %v %v, want %d %v", c.s, row, below, ok, c.row, c.below)
		}
	}
	if _, _, ok := lineFor(spot{0, false}, 2, 0); ok { // empty queue: above the Next from heading instead
		t.Error("empty queue got a row line")
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

func TestUpcomingIndex(t *testing.T) {
	next := tracks("abcab")
	next[2].LinkedURI = "z"
	for uri, want := range map[string]int{"a": 0, "b": 1, "c": 2, "z": 2, "q": -1} {
		if got := upcomingIndex(next, uri); got != want {
			t.Errorf("%s: got %d want %d", uri, got, want)
		}
	}
}
