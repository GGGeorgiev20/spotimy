package spotify

import "testing"

func TestImageURI(t *testing.T) {
	id := "ab67616d00001e02db216ca805faf5fe35df4ee6" // 40 hex digits
	for in, want := range map[string]string{
		"https://i.scdn.co/image/" + id:                        "spotify:image:" + id,
		"https://mosaic.scdn.co/60/" + id + id + id + id:       "spotify:mosaic:" + id + ":" + id + ":" + id + ":" + id,
		"https://misc.scdn.co/liked-songs/liked-songs-300.png": "",
	} {
		if got := imageURI(in); got != want {
			t.Errorf("imageURI(%q) = %q, want %q", in, got, want)
		}
	}
}
