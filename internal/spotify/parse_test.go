package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// fixtureClient answers every pathfinder query with testdata/<name>.json.
func fixtureClient(t *testing.T, name string) *Client {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OperationName string `json:"operationName"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.OperationName == "" {
			t.Error("request without operationName")
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	c := testClient(t)
	c.pathfinderURL = srv.URL
	return c
}

func TestParseLibrary(t *testing.T) {
	lib, err := fixtureClient(t, "library").Library(context.Background(), LibraryQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if lib.Total == 0 || len(lib.Items) != 3 {
		t.Fatalf("total=%d items=%d", lib.Total, len(lib.Items))
	}
	liked := lib.Items[0]
	if liked.URI != "spotify:collection:tracks" || liked.Type != "collection" || !liked.Pinned || liked.Count == 0 || len(liked.Images) == 0 {
		t.Errorf("liked songs row: %+v", liked)
	}
	pl := lib.Items[1]
	if pl.Type != "playlist" || pl.Name == "" || pl.Owner != "Test User" || pl.Subtitle != "Playlist • Test User" {
		t.Errorf("playlist row: %+v", pl)
	}
}

func TestParsePlaylist(t *testing.T) {
	p, err := fixtureClient(t, "playlist").Playlist(context.Background(), "spotify:playlist:37i9dQZF1DXcBWIGoYBM5M", 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name == "" || p.Owner.Name == "" || len(p.Images) == 0 || p.Tracks.Total < 3 || len(p.Tracks.Items) != 3 {
		t.Fatalf("playlist: %+v", p)
	}
	tr := p.Tracks.Items[0]
	if tr.URI == "" || tr.UID == "" || tr.Name == "" || len(tr.Artists) == 0 || tr.Album == nil || tr.DurationMs == 0 || tr.AddedAt == "" {
		t.Errorf("track: %+v", tr)
	}
}

func TestParseAlbum(t *testing.T) {
	a, err := fixtureClient(t, "album").Album(context.Background(), "spotify:album:4LH4d3cOWNNsVw41Gqt2kv")
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "The Dark Side of the Moon" || a.Type != "ALBUM" || a.Artists[0].Name != "Pink Floyd" || a.Total != 10 || len(a.Copyright) == 0 {
		t.Fatalf("album: %+v", a)
	}
	tr := a.Tracks[0]
	if tr.Name != "Speak to Me" || tr.TrackNumber != 1 || tr.DurationMs != 64333 || tr.Album.URI != a.URI || tr.UID == "" {
		t.Errorf("track: %+v", tr)
	}
}

func TestParseLikedSongs(t *testing.T) {
	page, err := fixtureClient(t, "liked").LikedSongs(context.Background(), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total == 0 || len(page.Items) != 2 || page.Items[0].URI != "spotify:track:test0" || page.Items[0].AddedAt == "" {
		t.Fatalf("liked: %+v", page)
	}
}

func TestGraphQLErrorWithoutDataFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errors":[{"message":"boom"}],"data":null}`))
	}))
	defer srv.Close()
	c := testClient(t)
	c.pathfinderURL = srv.URL
	_, err := c.Album(context.Background(), "spotify:album:x")
	if e, ok := err.(*Error); !ok || e.Op != "getAlbum" {
		t.Fatalf("got %v", err)
	}
}

func TestURIType(t *testing.T) {
	for uri, want := range map[string]string{
		"spotify:playlist:abc":         "playlist",
		"spotify:collection:tracks":    "collection",
		"spotify:user:me:collection":   "collection",
		"spotify:album:x":              "album",
		"spotify:user:me:playlist:abc": "playlist",
		"spotify:user:me":              "user",
	} {
		if got := URIType(uri); got != want {
			t.Errorf("%s: got %s want %s", uri, got, want)
		}
	}
}
