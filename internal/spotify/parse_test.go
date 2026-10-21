package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	if tr.URI == "" || tr.UID == "" || tr.Name == "" || len(tr.Artists) == 0 || tr.Album == nil || tr.DurationMs == 0 || tr.AddedAt == "" || !tr.Video {
		t.Errorf("track: %+v", tr)
	}
	if len(p.Banner) != 1 || p.MadeFor != "Test User" || !p.PromoNote {
		t.Errorf("generated playlist header: banner=%v madeFor=%q promo=%v", p.Banner, p.MadeFor, p.PromoNote)
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
	if len(a.More) == 0 || a.More[0].Subtitle != "1979" { // "More by" only has years
		t.Errorf("more by: %+v", a.More)
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

func TestParseHome(t *testing.T) {
	h, err := fixtureClient(t, "home").Home(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if h.Greeting == "" || len(h.Chips) == 0 || len(h.Sections) != 3 {
		t.Fatalf("home: greeting=%q chips=%d sections=%d", h.Greeting, len(h.Chips), len(h.Sections))
	}
	if h.Sections[0].Kind != "shortcuts" || h.Sections[1].Label != "Made For" || h.Sections[1].Title != "Test User" || h.Sections[2].Kind != "recents" {
		t.Errorf("sections: %+v", h.Sections)
	}
	for _, s := range h.Sections {
		for _, c := range s.Items {
			if c.URI == "" || c.Name == "" || c.Type == "" {
				t.Errorf("bad card %+v", c)
			}
		}
	}
}

func TestParseSearch(t *testing.T) {
	r, err := fixtureClient(t, "search").Search(context.Background(), "radiohead")
	if err != nil {
		t.Fatal(err)
	}
	if r.Top == nil || len(r.Tracks) == 0 || r.Totals.Artists == 0 || len(r.Groups) < 4 {
		t.Fatalf("search: top=%v tracks=%d groups=%d", r.Top != nil, len(r.Tracks), len(r.Groups))
	}
	if r.Groups[0].Title != "Artists" || r.Groups[0].Items[0].Name != "Radiohead" || len(r.Groups[0].Items[0].Images) == 0 {
		t.Errorf("artists group: %+v", r.Groups[0].Items[0])
	}
}

func TestParseArtist(t *testing.T) {
	a, err := fixtureClient(t, "artist").Artist(context.Background(), "spotify:artist:4Z8W4fKeB5YxbusRsdQVPb")
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "Radiohead" || a.MonthlyListeners == 0 || len(a.TopTracks) == 0 {
		t.Fatalf("artist: name=%q listeners=%d top=%d", a.Name, a.MonthlyListeners, len(a.TopTracks))
	}
	if sub := a.Albums[0].Subtitle; len(sub) < 6 || sub[:2] != "20" {
		t.Errorf("album subtitle should start with the year: %q", sub)
	}
	if p := a.Pick; p == nil || p.Title != "Test Single" || p.Subtitle != "Single" || p.Comment == "" || len(p.Images) == 0 {
		t.Errorf("artist pick: %+v", a.Pick)
	}
	if len(a.Albums) == 0 || len(a.Popular) == 0 || len(a.Header) == 0 {
		t.Errorf("discography/header missing")
	}
	var fans bool
	for _, s := range a.Shelves {
		if s.Title == "Fans also like" && len(s.Items) > 0 && s.Items[0].Type == "artist" {
			fans = true
		}
	}
	if !fans {
		t.Errorf("no related artists shelf: %+v", a.Shelves)
	}
}

func TestLabelSplit(t *testing.T) {
	cases := []struct{ label, base, wantLabel, wantTitle string }{
		{"Made For Galin", "Made For {0}", "Made For", "Galin"},
		{"For fans of Radiohead", "For fans of {0}", "For fans of", "Radiohead"},
		{"Jump back in", "Jump back in", "", "Jump back in"},
		{"Galin's picks", "{0}'s picks", "", "Galin's picks"},
	}
	for _, c := range cases {
		l, title := gqlLabel{c.label, c.base}.split()
		if l != c.wantLabel || title != c.wantTitle {
			t.Errorf("%q: got (%q, %q)", c.base, l, title)
		}
	}
}

func TestImageFromURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://i.scdn.co/image/ab": "https://i.scdn.co/image/ab",
		"spotify:image:ab":           "https://i.scdn.co/image/ab",
		"spotify:mosaic:aa:bb:cc:dd": "https://mosaic.scdn.co/300/aabbccdd",
	} {
		if got := imageFromURL(in); len(got) != 1 || got[0].URL != want {
			t.Errorf("%s: got %v", in, got)
		}
	}
	if len(imageFromURL("")) != 0 {
		t.Error("empty url should give no image")
	}
}

func TestTrackRelinking(t *testing.T) {
	var g gqlTrack
	raw := `{"uri":"spotify:track:a","name":"x","relinkingInformation":{"linkedTrack":{"uri":"spotify:track:b"}}}`
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		t.Fatal(err)
	}
	if tr := g.track(); tr.URI != "spotify:track:a" || tr.LinkedURI != "spotify:track:b" {
		t.Errorf("got %+v", tr)
	}
}

func TestGroupDigits(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 33696821: "33,696,821", -1234: "-1,234"} {
		if got := groupDigits(n); got != want {
			t.Errorf("%d: got %q", n, got)
		}
	}
}

func TestParseSearchSuggestions(t *testing.T) {
	s, err := fixtureClient(t, "suggestions").SearchSuggestions(context.Background(), "radiohead")
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 4 || s[0].Query != "radiohead" || s[1].Query == "" || s[2].Card == nil || s[3].Card == nil {
		t.Fatalf("suggestions: %+v", s)
	}
	if a := s[2].Card; a.Type != "artist" || a.Name != "Radiohead" || !a.Verified || len(a.Images) == 0 || a.Detail != "Artist" {
		t.Errorf("artist: %+v", a)
	}
	if tr := s[3].Card; tr.Type != "track" || tr.Detail != "Song • Radiohead" || len(tr.Images) == 0 || tr.AlbumURI == "" || len(tr.Artists) != 1 || tr.Artists[0].URI == "" {
		t.Errorf("track: %+v", tr)
	}
}

func TestParseArtistAbout(t *testing.T) {
	a, err := fixtureClient(t, "about").ArtistAbout(context.Background(), "spotify:artist:06HL4z0CvFAxyc27GXpf02")
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "Taylor Swift" || !a.Verified || !a.Registered || !a.Autobiography || a.Followers == 0 || a.WorldRank != 5 {
		t.Errorf("about: %+v", a)
	}
	if len(a.Photos) != 2 || len(a.Cities) != 2 || a.Cities[0].Name != "London, GB" || len(a.Links) != 2 || len(a.Avatar) == 0 {
		t.Errorf("photos %d cities %v links %d", len(a.Photos), a.Cities, len(a.Links))
	}
	if strings.Contains(a.Bio, "&#x") {
		t.Errorf("bio entities not decoded: %q", a.Bio)
	}
}
