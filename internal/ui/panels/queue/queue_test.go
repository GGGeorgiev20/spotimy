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
	next := tracks("abcXY")
	uid := func() string { return "new" }
	for _, c := range []struct {
		name     string
		tracks   string // dragged in from elsewhere (from = -1)
		from     int
		spot     int
		want     string
		appended bool
	}{
		{"queue row down", "", 0, 2, "bacXY", false},
		{"queue row up", "", 2, 0, "cabXY", false},
		{"queue row past the queue lands at its end", "", 0, 5, "bcaXY", false},
		{"queue row onto itself", "", 1, 1, "", false},
		{"queue row just below itself", "", 1, 2, "", false},
		{"Next up row into the queue", "", 3, 1, "aXbcXY", false},
		{"from a list, between queue rows", "Z", -1, 1, "aZbcXY", false},
		{"from a list, below the queue", "Z", -1, 4, "abcZXY", true},
	} {
		var ts []spotify.Track
		for _, r := range c.tracks {
			ts = append(ts, spotify.Track{URI: string(r)})
		}
		if c.from >= 0 {
			ts = []spotify.Track{next[c.from].Track}
		}
		out, appended := dropInto(next, 3, ts, c.from, c.spot, uid)
		if uriString(out) != c.want || appended != c.appended {
			t.Errorf("%s: got %q appended=%v, want %q appended=%v", c.name, uriString(out), appended, c.want, c.appended)
		}
	}
	out, _ := dropInto(next, 3, []spotify.Track{{URI: "Z"}}, -1, 0, uid)
	if out[0].Provider != "queue" || out[0].UID != "new" {
		t.Errorf("dropped track is queued with a uid: %+v", out[0])
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
