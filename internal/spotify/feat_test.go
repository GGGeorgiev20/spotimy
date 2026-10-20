package spotify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// serve answers every request (pathfinder or spclient) with body.
func serve(t *testing.T, c *Client, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	c.pathfinderURL, c.spclientURL = srv.URL, srv.URL
}

func TestPlaylistRecommendations(t *testing.T) {
	c := testClient(t)
	serve(t, c, `{"recommendedTracks":[{"id":"t1","originalId":"spotify:track:t1","name":"My Hero","explicit":true,
		"artists":[{"id":"a1","name":"Foo Fighters"}],"duration":260026,
		"album":{"id":"b1","name":"The Colour","imageUrl":"u300","largeImageUrl":"u640"}}]}`)
	got, err := c.PlaylistRecommendations(context.Background(), "spotify:playlist:p", []string{"spotify:track:x"}, 10)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v, %v", got, err)
	}
	r := got[0]
	if r.URI != "spotify:track:t1" || r.Artists[0].URI != "spotify:artist:a1" || r.Album.URI != "spotify:album:b1" ||
		len(r.Album.Images) != 2 || !r.Explicit || !r.Playable || r.DurationMs != 260026 {
		t.Errorf("track: %+v", r)
	}
}

func TestWhatsNew(t *testing.T) {
	c := testClient(t)
	recent := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	serve(t, c, `{"data":{"whatsNewFeedItems":{"items":[
	 {"id":"i1","state":{"state":"NEW","timestamp":null},"content":{"__typename":"AlbumResponseWrapper","data":{"__typename":"Album","uri":"spotify:album:a","name":"Alb","albumType":"SINGLE",
	  "date":{"isoString":"2026-03-05T00:00:00Z"},"artists":{"items":[{"uri":"spotify:artist:x","profile":{"name":"Band"}}]},"coverArt":{"sources":[{"url":"c","width":300,"height":300}]}}}},
	 {"id":"i2","state":{"state":"SEEN","timestamp":{"isoString":"`+recent+`"}},"content":{"__typename":"EpisodeOrChapterResponseWrapper","data":{"__typename":"Episode","uri":"spotify:episode:e","name":"Ep",
	  "releaseDate":{"isoString":"2026-03-04T00:00:00Z"},"podcastV2":{"data":{"__typename":"Podcast","uri":"spotify:show:s","name":"Show"}}}}},
	 {"id":"i3","state":{"state":"SEEN","timestamp":{"isoString":"2020-01-01T00:00:00Z"}},"content":{"__typename":"EpisodeOrChapterResponseWrapper","data":{"__typename":"Chapter","uri":"spotify:episode:c"}}},
	 {"id":"i4","state":{"state":"SEEN","timestamp":{"isoString":"2020-01-01T00:00:00Z"}},"content":{"__typename":"WhatsNewFeedNotification","data":{}}}
	]}}}`)
	got, err := c.WhatsNew(context.Background(), nil)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	a, e := got[0], got[1]
	if !a.New || a.AlbumType != "Single" || a.Artists[0].Name != "Band" || a.Date != "2026-03-05T00:00:00Z" {
		t.Errorf("album: %+v", a)
	}
	if !e.New || e.Episode == nil || e.Episode.ShowName != "Show" {
		t.Errorf("episode: %+v", e)
	}
}

func TestSearchGenres(t *testing.T) {
	c := testClient(t)
	serve(t, c, `{"data":{"searchV2":{"genres":{"totalCount":1,"items":[{"__typename":"GenreResponseWrapper","data":{"__typename":"Genre","name":"Rock","uri":"spotify:genre:r","image":{"sources":[{"url":"g","width":274,"height":274}]}}}]}}}}`)
	p, err := c.SearchType(context.Background(), "rock", "genres", 0, 30)
	if err != nil || len(p.Cards) != 1 || p.Cards[0].Type != "genre" || p.Cards[0].Name != "Rock" || len(p.Cards[0].Images) != 1 {
		t.Fatalf("got %+v, %v", p, err)
	}
}

func TestTasteExcluded(t *testing.T) {
	c := testClient(t)
	c.userURI = "spotify:user:me"
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = r.URL.Path + " " + string(b)
		io.WriteString(w, `{"found":[true,false]}`)
	}))
	t.Cleanup(srv.Close)
	c.spclientURL = srv.URL
	got, err := c.TasteExcluded(context.Background(), []string{"spotify:playlist:a", "spotify:playlist:b"})
	if err != nil || len(got) != 2 || !got[0] || got[1] {
		t.Fatalf("got %v, %v", got, err)
	}
	if !strings.Contains(body, "/collection/v2/contains") || !strings.Contains(body, `"set":"ignoreinrecs"`) || !strings.Contains(body, `"username":"me"`) {
		t.Errorf("request: %s", body)
	}
	if err := c.SetTasteExcluded(context.Background(), "spotify:playlist:a", false); err != nil || !strings.Contains(body, `/collection/v2/write {"items":[{"is_removed":true,"uri":"spotify:playlist:a"}]`) {
		t.Errorf("remove: %v %s", err, body)
	}
}
