// Package imgcache serves Spotify images to the webview from a size-capped
// disk cache (GET /img?u=<url>), and computes dominant colors for headers.
package imgcache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type Cache struct {
	dir   string
	max   int64
	log   *slog.Logger
	http  *http.Client
	sem   chan struct{} // caps concurrent downloads so a fast scroll can't storm the CDN
	mu    sync.Mutex
	puts  int
	color sync.Map // url -> hex
}

func New(dir string, maxBytes int64, log *slog.Logger) *Cache {
	_ = os.MkdirAll(dir, 0o700)
	c := &Cache{dir: dir, max: maxBytes, log: log, http: &http.Client{Timeout: 20 * time.Second}, sem: make(chan struct{}, 6)}
	go c.trim()
	return c
}

func allowed(u *url.URL) bool {
	h := u.Hostname()
	return u.Scheme == "https" && (strings.HasSuffix(h, ".scdn.co") || strings.HasSuffix(h, ".spotifycdn.com"))
}

func (c *Cache) path(rawURL string) string {
	sum := sha1.Sum([]byte(rawURL))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:]))
}

// Middleware serves /img?u=<url> ahead of the frontend assets (the Vite dev
// server would otherwise answer it with index.html).
func (c *Cache) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/img" {
			c.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ServeHTTP handles /img?u=<url>.
func (c *Cache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, err := c.file(r.Context(), r.URL.Query().Get("u"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Cache-Control", "max-age=31536000, immutable")
	http.ServeFile(w, r, p)
}

// file returns the local path of the image, downloading it on a miss.
func (c *Cache) file(ctx context.Context, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || !allowed(u) {
		return "", errors.New("image host not allowed")
	}
	p := c.path(raw)
	if _, err := os.Stat(p); err == nil {
		now := time.Now()
		_ = os.Chtimes(p, now, now) // LRU by mtime
		return p, nil
	}

	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-c.sem }()

	req, _ := http.NewRequestWithContext(ctx, "GET", raw, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("image: HTTP %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp(c.dir, "dl-*")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, 10<<20)); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	tmp.Close()
	if err := os.Rename(tmp.Name(), p); err != nil {
		return "", err
	}

	c.mu.Lock()
	c.puts++
	trim := c.puts%200 == 0
	c.mu.Unlock()
	if trim {
		go c.trim()
	}
	return p, nil
}

// trim deletes least recently used files until the cache fits its cap.
func (c *Cache) trim() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	type f struct {
		path string
		size int64
		mod  time.Time
	}
	var files []f
	var total int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || info.IsDir() {
			continue
		}
		files = append(files, f{filepath.Join(c.dir, e.Name()), info.Size(), info.ModTime()})
		total += info.Size()
	}
	if total <= c.max {
		return
	}
	slices.SortFunc(files, func(a, b f) int { return a.mod.Compare(b.mod) })
	for _, x := range files {
		if total <= c.max*9/10 {
			break
		}
		if os.Remove(x.path) == nil {
			total -= x.size
		}
	}
}

// Clear removes every cached image (on logout).
func (c *Cache) Clear() error {
	c.color.Clear()
	if err := os.RemoveAll(c.dir); err != nil {
		return err
	}
	return os.MkdirAll(c.dir, 0o700)
}
