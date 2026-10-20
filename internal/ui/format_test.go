package ui

import (
	"testing"
	"time"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

func TestFormat(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	for got, want := range map[string]string{
		FmtDuration(187000):                               "3:07",
		FmtDuration(3723000):                              "1:02:03",
		FmtTotal(2832000):                                 "47 min 12 sec",
		FmtTotal(4980000):                                 "1 hr 23 min",
		FmtTotal(43200000):                                "about 12 hr",
		FmtNumber(1234567):                                "1,234,567",
		FmtNumber("999"):                                  "999",
		FmtNumber(-1000):                                  "-1,000",
		FmtAdded("2026-10-20T09:00:00Z", now):             "3 hours ago",
		FmtAdded("2026-10-17T12:00:00Z", now):             "3 days ago",
		FmtAdded("2026-10-06T12:00:00Z", now):             "2 weeks ago",
		FmtAgo(now.Add(-90*time.Minute).UnixMilli(), now): "1 hr",
		FmtListLength([]spotify.Track{{DurationMs: 60000}, {DurationMs: 120000}}, 4): "about 6 min 0 sec",
		URIToPath("spotify:user:me:collection"):                                      "/collection/tracks",
		URIToPath("spotify:playlist:abc"):                                            "/playlist/abc",
		URIToPath("spotify:user:bob"):                                                "/user/bob",
		URIToPath("spotify:page:0JQ5DAqbMKFQ00XGBls6ym"):                             "/genre/0JQ5DAqbMKFQ00XGBls6ym",
	} {
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}
