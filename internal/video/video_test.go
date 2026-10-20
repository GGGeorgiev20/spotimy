package video

import (
	"context"
	"net/url"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPlayLocalClip(t *testing.T) {
	if !Available() {
		t.Skip("ffmpeg not available")
	}
	clip := filepath.Join(t.TempDir(), "clip.mp4")
	// 1 second, 24 fps, 320x568 (portrait).
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x568:rate=24:duration=1",
		"-pix_fmt", "yuv420p", "-c:v", "mpeg4", clip).CombinedOutput(); err != nil {
		t.Skipf("cannot make clip: %v: %s", err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	got := make(chan struct{}, 100)
	p, err := start(ctx, clip, 160, 160, func() { got <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	// 30 frames at 24 fps spans more than the 1 s clip, so this also checks looping.
	for range 30 {
		select {
		case <-got:
		case <-ctx.Done():
			t.Fatalf("timed out waiting for frames; err=%v", p.Err())
		}
	}
	img, n := p.Frame()
	if img == nil || n < 30 {
		t.Fatalf("frame=%v n=%d", img, n)
	}
	if b := img.Bounds(); b.Dx() != 90 || b.Dy() != 160 { // 320x568 fit in 160x160 -> 90x160 (even)
		t.Fatalf("size %v", b)
	}
	p.Close()
	p.Close()
	if p.Err() != nil {
		t.Fatalf("unexpected err: %v", p.Err())
	}
}

func TestHostAllowed(t *testing.T) {
	for u, want := range map[string]bool{
		"https://canvaz.scdn.co/a.mp4":   true,
		"https://x.spotifycdn.com/a.mp4": true,
		"http://canvaz.scdn.co/a.mp4":    false,
		"https://evilscdn.co/a.mp4":      false,
		"https://scdn.co.evil.com/a.mp4": false,
	} {
		pu, _ := url.Parse(u)
		if hostAllowed(pu) != want {
			t.Errorf("%s: want allowed=%v", u, want)
		}
	}
}
