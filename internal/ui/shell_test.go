package ui

import "testing"

func TestCenterBlock(t *testing.T) {
	for _, c := range []struct{ win, x, w int }{
		{1280, 367, 546}, // fits: centered
		{1100, 277, 546},
		{1000, 227, 546},
		{800, 127, 546},
		{700, 96, 500}, // too narrow to center: next to the nav buttons, shrunk
	} {
		if x, w := centerBlock(c.win, 546, 96, c.win-104); x != c.x || w != c.w {
			t.Errorf("win %d: got %d,%d want %d,%d", c.win, x, w, c.x, c.w)
		}
	}
}
