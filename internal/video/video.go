// Package video plays short looping videos (Spotify Canvas) as a stream of
// RGBA frames by running the ffmpeg binary as a subprocess.
package video

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxDownload = 20 << 20

// allowedHosts are the host suffixes a video may be downloaded from.
var allowedHosts = []string{"scdn.co", "spotifycdn.com"}

var availableOnce = sync.OnceValue(func() bool {
	_, e1 := exec.LookPath("ffmpeg")
	_, e2 := exec.LookPath("ffprobe")
	return e1 == nil && e2 == nil
})

// Available reports whether ffmpeg and ffprobe are on PATH (cached).
func Available() bool { return availableOnce() }

// ringSize is the number of frame buffers. Frame n lives in buffer n%ringSize
// and is only overwritten when frame n+ringSize is being decoded, i.e. it
// stays valid for at least the next two published frames.
const ringSize = 4

// Player decodes a looping video into RGBA frames until Close is called.
type Player struct {
	cancel context.CancelFunc
	done   chan struct{}

	mu    sync.Mutex
	cur   *image.RGBA
	count uint64
	err   error
}

// New downloads url into cacheDir (blocking; call it off the UI goroutine) and
// starts decoding it scaled to fit within maxW×maxH, keeping aspect ratio and
// never upscaling. onFrame (may be nil) is called from a background goroutine
// after each new frame. The player stops when ctx is cancelled or Close is called.
func New(ctx context.Context, url, cacheDir string, maxW, maxH int, onFrame func()) (*Player, error) {
	if !Available() {
		return nil, errors.New("video: ffmpeg/ffprobe not found on PATH")
	}
	path, err := download(ctx, url, cacheDir)
	if err != nil {
		return nil, err
	}
	return start(ctx, path, maxW, maxH, onFrame)
}

// Frame returns the latest frame and a counter that increases with every new
// frame; (nil, 0) before the first. The frame is immutable and must not be
// modified; it remains valid for at least the next two published frames
// (ring of 4 buffers, ~65 ms at 30 fps), so draw it promptly (paint.NewImageOp
// followed by the window frame) and re-call Frame on every layout.
func (p *Player) Frame() (*image.RGBA, uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cur, p.count
}

// Err returns the error that stopped decoding, if any. After an error Frame
// keeps returning the last frame.
func (p *Player) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

// Close stops ffmpeg and waits for it to exit. Idempotent.
func (p *Player) Close() {
	p.cancel()
	<-p.done
}

// start decodes a local file. It probes the size with ffprobe so frame buffers
// can be allocated before ffmpeg runs.
func start(ctx context.Context, path string, maxW, maxH int, onFrame func()) (*Player, error) {
	if maxW < 2 || maxH < 2 {
		return nil, errors.New("video: max size too small")
	}
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-of", "default=nw=1", path).Output()
	if err != nil {
		return nil, fmt.Errorf("video: ffprobe: %w", err)
	}
	var sw, sh int
	for _, l := range strings.Fields(string(out)) {
		k, v, _ := strings.Cut(l, "=")
		switch k {
		case "width":
			sw, _ = strconv.Atoi(v)
		case "height":
			sh, _ = strconv.Atoi(v)
		}
	}
	if sw <= 0 || sh <= 0 {
		return nil, errors.New("video: could not read video size")
	}
	w, h := fit(sw, sh, maxW, maxH)
	if path, err = shrink(ctx, path, w, h); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	// -re paces output at the native frame rate; -stream_loop -1 loops forever.
	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "error", "-nostdin",
		"-threads", "1", "-re", "-stream_loop", "-1", "-i", path, "-an",
		"-pix_fmt", "rgba", "-f", "rawvideo", "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("video: ffmpeg: %w", err)
	}

	p := &Player{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(p.done)
		var ring [ringSize]*image.RGBA
		for i := range ring {
			ring[i] = image.NewRGBA(image.Rect(0, 0, w, h))
		}
		var n uint64
		var rerr error
		for {
			img := ring[n%ringSize]
			if _, rerr = io.ReadFull(stdout, img.Pix); rerr != nil {
				break
			}
			n++
			p.mu.Lock()
			p.cur, p.count = img, n
			p.mu.Unlock()
			if onFrame != nil {
				onFrame()
			}
		}
		cancel()
		werr := cmd.Wait()
		if ctx.Err() == nil { // stopped on its own, not via Close/ctx
			if werr == nil {
				werr = rerr
			}
			p.mu.Lock()
			p.err = fmt.Errorf("video: ffmpeg stopped: %v: %s", werr, strings.TrimSpace(stderr.String()))
			p.mu.Unlock()
		}
	}()
	return p, nil
}

// shrink re-encodes path at w×h next to it (once) and returns the copy's path.
// Canvases are 1080×1920 H.264, which costs ~40% of a core to decode in
// software; an MPEG-4 copy at panel size loops for ~4%.
func shrink(ctx context.Context, path string, w, h int) (string, error) {
	out := fmt.Sprintf("%s-%dx%d.mkv", strings.TrimSuffix(path, filepath.Ext(path)), w, h)
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}
	tmp := out + ".tmp"
	defer os.Remove(tmp) // no-op after a successful rename
	if b, err := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-i", path, "-an", "-vf", fmt.Sprintf("scale=%d:%d:flags=area", w, h),
		"-c:v", "mpeg4", "-q:v", "2", "-f", "matroska", tmp).CombinedOutput(); err != nil {
		return "", fmt.Errorf("video: ffmpeg shrink: %v: %s", err, bytes.TrimSpace(b))
	}
	return out, os.Rename(tmp, out)
}

// fit scales sw×sh to fit in maxW×maxH, never upscaling, with even dimensions.
func fit(sw, sh, maxW, maxH int) (w, h int) {
	w, h = sw, sh
	if w > maxW || h > maxH {
		if w*maxH > h*maxW { // width-limited
			w, h = maxW, h*maxW/w
		} else {
			w, h = w*maxH/h, maxH
		}
	}
	w, h = max(2, w&^1), max(2, h&^1)
	return
}

func hostAllowed(u *url.URL) bool {
	if u.Scheme != "https" {
		return false
	}
	h := strings.ToLower(u.Hostname())
	for _, s := range allowedHosts {
		if h == s || strings.HasSuffix(h, "."+s) {
			return true
		}
	}
	return false
}

// download fetches rawURL into cacheDir (once) and returns the local path.
func download(ctx context.Context, rawURL, cacheDir string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || !hostAllowed(u) {
		return "", fmt.Errorf("video: url not allowed: %q", rawURL)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(cacheDir, hash(rawURL)+".mp4")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	client := &http.Client{CheckRedirect: func(r *http.Request, _ []*http.Request) error {
		if !hostAllowed(r.URL) {
			return fmt.Errorf("video: redirect not allowed: %s", r.URL.Host)
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("video: download: %s", resp.Status)
	}
	f, err := os.CreateTemp(cacheDir, "dl-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name()) // no-op after successful rename
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxDownload+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxDownload {
		err = errors.New("video: download too large")
	}
	if err != nil {
		return "", err
	}
	return path, os.Rename(f.Name(), path)
}
