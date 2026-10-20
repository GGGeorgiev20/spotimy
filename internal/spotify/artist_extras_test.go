package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestArtistGoods(t *testing.T) {
	var g gqlGoods
	err := json.Unmarshal([]byte(`{
		"concerts": {"totalCount": 5, "items": [
			{"data": {"uri": "spotify:concert:a", "title": "Radiohead", "location": {"city": "Saitama", "name": "Arena"}, "startDateIsoString": "2027-06-08T19:00+09:00"}},
			{"data": {"uri": "spotify:concert:b", "title": "Bad date", "startDateIsoString": "soon"}}]},
		"merch": {"items": [{"nameV2": "T-shirt", "price": "£45.00", "url": "https://shop/x", "image": {"sources": [{"url": "https://i/1"}]}}]}}`), &g)
	if err != nil {
		t.Fatal(err)
	}
	cs := g.concerts()
	if len(cs) != 1 || cs[0].City != "Saitama" || cs[0].Start.Format("Mon Jan 2 3:04 PM") != "Tue Jun 8 7:00 PM" {
		t.Errorf("concerts: %+v", cs)
	}
	if m := g.merch(); len(m) != 1 || m[0].Name != "T-shirt" || len(m[0].Images) != 1 {
		t.Errorf("merch: %+v", m)
	}
}

func TestSetFollowing(t *testing.T) {
	result := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OperationName string
			Variables     struct{ Usernames []string }
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.OperationName != "followUsers" || len(req.Variables.Usernames) != 1 || req.Variables.Usernames[0] != "someone" {
			t.Errorf("request: %+v", req)
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"followUsers": map[string]any{
			"responses": []map[string]any{{"__typename": "FollowUserResult", "username": "someone", "result": result}}}}})
	}))
	defer srv.Close()
	c := testClient(t)
	c.pathfinderURL = srv.URL
	if err := c.SetFollowing(context.Background(), "spotify:user:someone", true); err != nil {
		t.Fatal(err)
	}
	result = false
	if err := c.SetFollowing(context.Background(), "spotify:user:someone", true); err == nil {
		t.Error("a follow the server didn't apply should fail")
	}
}
