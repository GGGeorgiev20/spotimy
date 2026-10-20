package spotify

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testClient(t *testing.T) *Client {
	c := New(DefaultIdentity(), func(context.Context) (string, error) { return "tok", nil }, slog.New(slog.NewTextHandler(io.Discard, nil)), "")
	c.clientTok, c.clientTokEx = "ct", time.Now().Add(time.Hour)
	return c
}

func TestRetryAfter(t *testing.T) {
	if d := retryAfter("3", 1); d != 3*time.Second {
		t.Fatalf("seconds: got %v", d)
	}
	if d := retryAfter("", 3); d != 2*time.Second {
		t.Fatalf("fallback: got %v", d)
	}
	if d := retryAfter("99999", 1); d != 2*time.Minute {
		t.Fatalf("cap: got %v", d)
	}
}

func Test429BackoffThenSuccess(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("client-token") != "ct" {
			t.Errorf("missing auth headers: %v", r.Header)
		}
		if calls == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			return
		}
		w.Write([]byte(`ok`))
	}))
	defer srv.Close()

	c := testClient(t)
	start := time.Now()
	body, err := c.do(context.Background(), request{op: "t", method: "GET", url: srv.URL, idempotent: true})
	if err != nil || string(body) != "ok" {
		t.Fatalf("got %q, %v", body, err)
	}
	if calls != 2 || time.Since(start) < time.Second {
		t.Fatalf("expected one retry after >=1s, calls=%d elapsed=%v", calls, time.Since(start))
	}
}

func TestTokenRefreshOn401(t *testing.T) {
	tokens := []string{"old", "new"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer new" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`ok`))
	}))
	defer srv.Close()

	c := testClient(t)
	n := 0
	c.token = func(context.Context) (string, error) { n++; return tokens[min(n-1, 1)], nil }
	if _, err := c.do(context.Background(), request{op: "t", method: "GET", url: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected a forced refresh, token fetched %d times", n)
	}
	// Cached afterwards.
	if _, err := c.do(context.Background(), request{op: "t", method: "GET", url: srv.URL}); err != nil || n != 2 {
		t.Fatalf("expected cached token, n=%d err=%v", n, err)
	}
}

func TestNonRetryableErrorIsTyped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"errors":[{"message":"PersistedQueryNotFound"}]}`))
	}))
	defer srv.Close()
	_, err := testClient(t).do(context.Background(), request{op: "home", method: "POST", url: srv.URL})
	e, ok := err.(*Error)
	if !ok || e.Op != "home" || e.Status != 400 || e.Body == "" {
		t.Fatalf("got %#v", err)
	}
}

func TestOperationsRegistry(t *testing.T) {
	c := testClient(t)
	for _, op := range []string{"home", "fetchPlaylist", "getAlbum", "queryArtistOverview", "searchDesktop", "libraryV3"} {
		if len(c.ops[op]) != 64 {
			t.Errorf("%s: bad hash %q", op, c.ops[op])
		}
	}
	override := filepath.Join(t.TempDir(), "ops.json")
	os.WriteFile(override, []byte(`{"home":"abc"}`), 0o600)
	ops := loadOperations(override, c.log)
	if ops["home"] != "abc" || len(ops["getAlbum"]) != 64 {
		t.Fatalf("override not applied per name: %v", ops["home"])
	}
	if err := c.query(context.Background(), "nope", nil, 0, nil); err == nil {
		t.Fatal("unknown op should fail")
	}
}

func TestCacheByteCap(t *testing.T) {
	c := newCache(1000)
	for _, k := range []string{"a", "b", "c", "d", "e"} { // 5 x 240 > 1000: evicts a
		c.put(k, make([]byte, 240), time.Hour)
		time.Sleep(time.Millisecond)
	}
	if _, ok := c.get("a"); ok {
		t.Error("oldest entry should be evicted")
	}
	if _, ok := c.get("e"); !ok || c.size != 960 {
		t.Errorf("newest should stay, size=%d", c.size)
	}
	c.put("huge", make([]byte, 300), time.Hour) // > max/4 is not cached
	if _, ok := c.get("huge"); ok || c.size != 960 {
		t.Errorf("oversized entry cached, size=%d", c.size)
	}
	c.invalidate("")
	if c.size != 0 {
		t.Errorf("size after invalidate = %d", c.size)
	}
}
