//go:build test_unit

package spclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	librespot "github.com/devgianlu/go-librespot"
	connectpb "github.com/devgianlu/go-librespot/proto/spotify/connectstate"
)

func testSpclient(t *testing.T) *Spclient {
	t.Helper()

	base, err := url.Parse("https://gew4-spclient.spotify.com/")
	if err != nil {
		t.Fatalf("failed parsing base url: %v", err)
	}

	return &Spclient{baseUrl: base}
}

func TestHmRequestUrl(t *testing.T) {
	c := testSpclient(t)

	tests := []struct {
		name string
		hm   string
		want string
	}{
		{
			name: "no query",
			hm:   "hm://playlist/v2/playlist/37i9dQZF1E36KLdUfLiuUo",
			want: "https://gew4-spclient.spotify.com/playlist/v2/playlist/37i9dQZF1E36KLdUfLiuUo",
		},
		{
			// The "?" must survive: JoinPath would escape it to %3F and the
			// parameters would become part of the path.
			name: "lexicon session, query preserved",
			hm:   "hm://lexicon-session-provider/context-resolve/v2/session?contextUri=spotify:playlist:37i9dQZF1EYkqdzj48dyYq&reason=state_restore",
			want: "https://gew4-spclient.spotify.com/lexicon-session-provider/context-resolve/v2/session?contextUri=spotify:playlist:37i9dQZF1EYkqdzj48dyYq&reason=state_restore",
		},
		{
			// Paging through a DJ session; the colons of the contextUri stay
			// unescaped because the query is passed through verbatim.
			name: "lexicon next page",
			hm:   "hm://lexicon-session-provider/context-resolve/v2/session/0000?contextUri=spotify:playlist:37i9dQZF1EYkqdzj48dyYq&previousSegmentId=XX-Segment-30000",
			want: "https://gew4-spclient.spotify.com/lexicon-session-provider/context-resolve/v2/session/0000?contextUri=spotify:playlist:37i9dQZF1EYkqdzj48dyYq&previousSegmentId=XX-Segment-30000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.hmRequestUrl(tt.hm)
			if err != nil {
				t.Fatalf("hmRequestUrl(%q) failed: %v", tt.hm, err)
			}
			if got.String() != tt.want {
				t.Errorf("hmRequestUrl(%q)\n got %s\nwant %s", tt.hm, got, tt.want)
			}
		})
	}
}

func TestHmRequestUrlRejectsNonHm(t *testing.T) {
	c := testSpclient(t)

	for _, u := range []string{"", "https://example.com/x", "/context-resolve/v1/spotify:track:x"} {
		if _, err := c.hmRequestUrl(u); err == nil {
			t.Errorf("hmRequestUrl(%q) succeeded, want an error", u)
		}
	}
}

// A station's tracks are marked as autoplay even when they came with no
// metadata, rather than panicking on the nil map.
func TestStationTracksWithoutMetadata(t *testing.T) {
	r, err := NewContextResolver(context.Background(), &librespot.NullLogger{}, nil, &connectpb.Context{
		Uri:   "spotify:station:track:3j7BhP71ROCpc9R3w9P9UE",
		Pages: []*connectpb.ContextPage{{Tracks: []*connectpb.ContextTrack{{Uri: "spotify:track:3j7BhP71ROCpc9R3w9P9UE"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.ctx.Pages[0].Tracks[0].Metadata["autoplay.is_autoplay"] != "true" {
		t.Error("not marked autoplay")
	}
}

// An artist's album pages, all listed up front, are fetched a few at a time
// rather than one after another.
func TestPrefetchPagesInParallel(t *testing.T) {
	var inFlight, most atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for m := most.Load(); n > m && !most.CompareAndSwap(m, n); m = most.Load() {
		}
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte(`{"tracks":[{"uri":"spotify:track:3j7BhP71ROCpc9R3w9P9UE"}]}`))
	}))
	defer srv.Close()
	base, _ := url.Parse(srv.URL + "/")
	sp := &Spclient{log: &librespot.NullLogger{}, client: srv.Client(), baseUrl: base,
		accessToken: func(context.Context, bool) (string, error) { return "t", nil }}
	spotCtx := &connectpb.Context{Uri: "spotify:artist:4Z8W4fKeB5YxbusRsdQVPb", Pages: []*connectpb.ContextPage{{Tracks: []*connectpb.ContextTrack{{Uri: "spotify:track:3CRDbSIZ4r5MsZ0YwxuEkn"}}}}}
	for i := range 20 {
		spotCtx.Pages = append(spotCtx.Pages, &connectpb.ContextPage{PageUrl: fmt.Sprintf("hm://artistplaycontext/v1/page/%d", i)})
	}
	r, err := NewContextResolver(context.Background(), &librespot.NullLogger{}, sp, spotCtx)
	if err != nil {
		t.Fatal(err)
	}

	r.PrefetchPages(context.Background(), 256)

	for i, p := range r.ctx.Pages {
		if len(p.Tracks) == 0 {
			t.Fatalf("page %d not loaded", i)
		}
	}
	if most.Load() < 2 {
		t.Errorf("pages were fetched one at a time")
	}
}
