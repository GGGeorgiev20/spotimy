package spotify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLyricsParseAndMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/color-lyrics/v2/track/none" {
			w.WriteHeader(404)
			return
		}
		w.Write([]byte(`{"lyrics":{"syncType":"LINE_SYNCED","providerDisplayName":"Test","lines":[
			{"startTimeMs":"1000","words":"first line"},{"startTimeMs":"2500","words":"second line"}]}}`))
	}))
	defer srv.Close()
	c := testClient(t)
	c.spclientURL = srv.URL

	l, err := c.Lyrics(context.Background(), "spotify:track:abc")
	if err != nil || l == nil || !l.Synced || len(l.Lines) != 2 || l.Lines[1].StartMs != 2500 || l.Provider != "Test" {
		t.Fatalf("lyrics: %+v %v", l, err)
	}
	if l, err := c.Lyrics(context.Background(), "spotify:track:none"); l != nil || err != nil {
		t.Fatalf("missing lyrics should be nil, nil: %+v %v", l, err)
	}
}
