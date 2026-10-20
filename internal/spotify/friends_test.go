package spotify

import "testing"

func TestParseFeed(t *testing.T) {
	body := []byte(`{"entities":[
	 {"userEntity":{"uri":"spotify:user:aaa","activity":{"entityUri":"spotify:track:t1","contextUri":"spotify:playlist:p1","isPlaying":true,"timestamp":"2026-10-20T14:46:26.484Z"}}},
	 {"userEntity":{"uri":"spotify:user:bbb","activity":{"entityUri":"spotify:track:t2","timestamp":"2026-10-20T08:07:07.628Z"}}},
	 {"userEntity":{"uri":"spotify:user:ccc"}},
	 {"followEntity":{"uri":"spotify:artist:zzz","activity":{"entityUri":"spotify:track:t3"}}}]}`)
	got, err := parseFeed(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries (no-activity and non-user dropped), got %+v", got)
	}
	if g := got[0]; g.Username != "aaa" || g.Track != "spotify:track:t1" || g.Context != "spotify:playlist:p1" || !g.Playing || g.TimeMs != 1792507586484 {
		t.Errorf("entry 0: %+v", g)
	}
	if g := got[1]; g.Username != "bbb" || g.Context != "" || g.Playing || g.TimeMs != 1792483627628 {
		t.Errorf("entry 1: %+v", g)
	}
}
