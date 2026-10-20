package lyrics

import (
	"testing"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

func TestActiveLine(t *testing.T) {
	lines := []spotify.LyricLine{{StartMs: 1000}, {StartMs: 2000}, {StartMs: 2000}, {StartMs: 5000}}
	for ms, want := range map[int64]int{0: -1, 999: -1, 1000: 0, 1999: 0, 2000: 2, 4999: 2, 5000: 3, 1 << 40: 3} {
		if got := activeLine(lines, ms); got != want {
			t.Errorf("activeLine(%d) = %d, want %d", ms, got, want)
		}
	}
	if got := activeLine(nil, 10); got != -1 {
		t.Errorf("no lines: %d", got)
	}
}
