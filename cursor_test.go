package main

import (
	"strings"
	"testing"
)

func TestKDECursor(t *testing.T) {
	theme, size := kdeCursor(strings.NewReader("[Keyboard]\ncursorTheme=no\n\n[Mouse]\ncursorSize=32\ncursorTheme=Breeze_Light\n"))
	if theme != "Breeze_Light" || size != "32" {
		t.Fatalf("got %q %q", theme, size)
	}
}
