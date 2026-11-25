//go:build test_unit

package spclient

import (
	"context"
	"net/url"
	"testing"

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
