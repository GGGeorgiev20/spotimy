// Package spotify is the only place that knows Spotify's internal API: URLs,
// persisted-query hashes, headers and protobufs. Everything else calls the
// typed methods here.
package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	spclientBase   = "https://spclient.wg.spotify.com"
	pathfinderURL  = "https://api-partner.spotify.com/pathfinder/v2/query"
	clientTokenURL = "https://clienttoken.spotify.com/v1/clienttoken"
)

// TokenFunc returns a fresh login5 bearer token from the session.
type TokenFunc func(ctx context.Context) (string, error)

type Client struct {
	http  *http.Client
	id    Identity
	log   *slog.Logger
	token TokenFunc
	ops   map[string]string // pathfinder operation name -> sha256 hash

	mu         sync.Mutex
	pauseUntil time.Time // set by 429s; every request waits for it
	userURI    string

	tokMu     sync.Mutex
	bearer    string
	bearerExp time.Time

	ctMu        sync.Mutex
	clientTok   string
	clientTokEx time.Time

	cache *cache

	// Base URLs; tests point them at httptest servers.
	pathfinderURL string
	spclientURL   string
}

func New(id Identity, token TokenFunc, log *slog.Logger, opsOverride string) *Client {
	return &Client{
		http:  &http.Client{Timeout: 20 * time.Second},
		id:    id,
		log:   log,
		token: token,
		ops:   loadOperations(opsOverride, log),
		cache: newCache(8 << 20),

		pathfinderURL: pathfinderURL,
		spclientURL:   spclientBase,
	}
}

// Error is returned for every failed call. Op names the pathfinder operation
// or spclient endpoint so a broken hash or schema is obvious from the log.
type Error struct {
	Op     string
	Status int
	Body   string // short excerpt
	Err    error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("%s: HTTP %d: %s", e.Op, e.Status, e.Body)
}

func (e *Error) Unwrap() error { return e.Err }

var ErrUnauthorized = errors.New("not logged in")

func (c *Client) bearerToken(ctx context.Context, force bool) (string, error) {
	c.tokMu.Lock() // held across the fetch so concurrent callers share one refresh
	defer c.tokMu.Unlock()
	if !force && c.bearer != "" && time.Now().Before(c.bearerExp) {
		return c.bearer, nil
	}
	t, err := c.token(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnauthorized, err)
	}
	// login5 tokens last an hour; refresh well before that.
	c.bearer, c.bearerExp = t, time.Now().Add(45*time.Minute)
	return t, nil
}

// request describes one HTTP call. Body is re-sent on retries.
type request struct {
	op          string
	method      string
	url         string
	body        []byte
	contentType string
	accept      string
	idempotent  bool // safe to retry on 5xx / network errors
}

const maxAttempts = 4

// do sends req with identity headers, auth and retries. It returns the body of
// a 2xx response.
func (c *Client) do(ctx context.Context, req request) ([]byte, error) {
	forced := false
	for attempt := 1; ; attempt++ {
		if err := c.waitPause(ctx); err != nil {
			return nil, &Error{Op: req.op, Err: err}
		}
		status, body, hdr, err := c.send(ctx, req, forced)
		switch {
		case err != nil:
			if errors.Is(err, ErrUnauthorized) || ctx.Err() != nil || !req.idempotent || attempt >= maxAttempts {
				c.log.Warn("spotify request failed", "op", req.op, "err", err)
				return nil, &Error{Op: req.op, Err: err}
			}
		case status >= 200 && status < 300:
			return body, nil
		case status == http.StatusUnauthorized && !forced:
			forced = true
			continue
		case status == http.StatusTooManyRequests && attempt < maxAttempts:
			c.pause(retryAfter(hdr.Get("Retry-After"), attempt))
			continue
		case status >= 500 && req.idempotent && attempt < maxAttempts:
		default:
			e := &Error{Op: req.op, Status: status, Body: excerpt(body)}
			c.log.Warn("spotify request failed", "op", req.op, "status", status, "body", e.Body)
			return nil, e
		}
		if err := sleep(ctx, backoff(attempt)); err != nil {
			return nil, &Error{Op: req.op, Err: err}
		}
	}
}

func (c *Client) send(ctx context.Context, req request, forceToken bool) (int, []byte, http.Header, error) {
	bearer, err := c.bearerToken(ctx, forceToken)
	if err != nil {
		return 0, nil, nil, err
	}
	ct, err := c.clientToken(ctx)
	if err != nil {
		return 0, nil, nil, err
	}
	var rd io.Reader
	if req.body != nil {
		rd = bytes.NewReader(req.body)
	}
	hr, err := http.NewRequestWithContext(ctx, req.method, req.url, rd)
	if err != nil {
		return 0, nil, nil, err
	}
	c.id.setHeaders(hr.Header)
	hr.Header.Set("Authorization", "Bearer "+bearer)
	hr.Header.Set("client-token", ct)
	if req.contentType != "" {
		hr.Header.Set("Content-Type", req.contentType)
	}
	if req.accept != "" {
		hr.Header.Set("Accept", req.accept)
	}
	resp, err := c.http.Do(hr)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	return resp.StatusCode, body, resp.Header, err
}

func (c *Client) pause(d time.Duration) {
	c.mu.Lock()
	if until := time.Now().Add(d); until.After(c.pauseUntil) {
		c.pauseUntil = until
	}
	c.mu.Unlock()
	c.log.Warn("rate limited by spotify", "pause", d)
}

func (c *Client) waitPause(ctx context.Context) error {
	c.mu.Lock()
	d := time.Until(c.pauseUntil)
	c.mu.Unlock()
	if d <= 0 {
		return nil
	}
	return sleep(ctx, d)
}

// retryAfter parses a Retry-After header (seconds or HTTP date), falling back
// to exponential backoff. Capped so a bogus header can't hang the UI forever.
func retryAfter(h string, attempt int) time.Duration {
	d := backoff(attempt)
	if s, err := strconv.Atoi(strings.TrimSpace(h)); err == nil {
		d = time.Duration(s) * time.Second
	} else if t, err := http.ParseTime(h); err == nil {
		d = time.Until(t)
	}
	return max(time.Second, min(d, 2*time.Minute))
}

func backoff(attempt int) time.Duration {
	return time.Duration(1<<(attempt-1)) * 500 * time.Millisecond
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func excerpt(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

// getJSON is a GET against spclient decoding JSON into out.
func (c *Client) getJSON(ctx context.Context, op, path string, out any) error {
	body, err := c.do(ctx, request{op: op, method: "GET", url: c.spclientURL + path, accept: "application/json", idempotent: true})
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return &Error{Op: op, Err: fmt.Errorf("decoding: %w", err)}
	}
	return nil
}

// Reset forgets tokens and cached responses (on logout).
func (c *Client) Reset() {
	c.tokMu.Lock()
	c.bearer = ""
	c.tokMu.Unlock()
	c.mu.Lock()
	c.userURI = ""
	c.mu.Unlock()
	c.cache.invalidate("")
}
