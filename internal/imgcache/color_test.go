package imgcache

import (
	"image"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDominantPrefersColorOverWhite(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := range 100 {
		for x := range 100 {
			c := color.RGBA{255, 255, 255, 255} // 70% white
			if x < 30 {
				c = color.RGBA{200, 30, 30, 255}
			}
			img.Set(x, y, c)
		}
	}
	if got := dominant(img); got != "#c81e1e" {
		t.Fatalf("got %s", got)
	}
}

func TestDominantGrayscaleFallsBackToAverage(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 10, 10))
	for i := range img.Pix {
		img.Pix[i] = 100
	}
	if got := dominant(img); got != "#646464" {
		t.Fatalf("got %s", got)
	}
}

func TestTrimRemovesLeastRecentlyUsed(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{dir: dir, max: 250}
	now := time.Now()
	for i, name := range []string{"old", "mid", "new"} {
		p := filepath.Join(dir, name)
		os.WriteFile(p, make([]byte, 100), 0o600)
		ts := now.Add(time.Duration(i) * time.Minute)
		os.Chtimes(p, ts, ts)
	}
	c.trim() // 300 > 250: drop oldest until <= 225
	if _, err := os.Stat(filepath.Join(dir, "old")); !os.IsNotExist(err) {
		t.Error("oldest file should be removed")
	}
	for _, keep := range []string{"mid", "new"} {
		if _, err := os.Stat(filepath.Join(dir, keep)); err != nil {
			t.Errorf("%s should be kept: %v", keep, err)
		}
	}
}

func TestOnlySpotifyHostsAllowed(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://i.scdn.co/image/x":           true,
		"https://canvaz.scdn.co/upload/x.mp4": true,
		"https://image-cdn-ak.spotifycdn.com": true,
		"http://i.scdn.co/image/x":            false,
		"https://evil.example.com/x":          false,
		"https://scdn.co.evil.com/x":          false,
	} {
		u, _ := url.Parse(raw)
		if got := allowed(u); got != want {
			t.Errorf("%s: got %v", raw, got)
		}
	}
}
