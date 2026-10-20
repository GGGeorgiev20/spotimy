package spotify

import (
	"context"
	"sort"
)

// Friend is one entry of Friend Activity: what a followed user last played.
type Friend struct {
	User      Card  `json:"user"`
	Track     Card  `json:"track"`  // subtitle is the artist
	Artist    Card  `json:"artist"` // name and uri only
	Context   Card  `json:"context"`
	Timestamp int64 `json:"timestamp"` // unix ms
	Playing   bool  `json:"playing"`   // within the last few minutes
}

type buddyEntity struct {
	URI      string `json:"uri"`
	Name     string `json:"name"`
	ImageURL string `json:"imageUrl"`
}

func (b buddyEntity) card(typ string) Card {
	return Card{URI: b.URI, Type: typ, Name: b.Name, Images: imageFromURL(b.ImageURL)}
}

// Friends returns Friend Activity, most recent first.
func (c *Client) Friends(ctx context.Context, nowMs int64) ([]Friend, error) {
	var resp struct {
		Friends []struct {
			Timestamp int64       `json:"timestamp"`
			User      buddyEntity `json:"user"`
			Track     struct {
				buddyEntity
				Artist  buddyEntity `json:"artist"`
				Context buddyEntity `json:"context"`
			} `json:"track"`
		} `json:"friends"`
	}
	if err := c.getJSON(ctx, "buddylist", "/presence-view/v1/buddylist", &resp); err != nil {
		return nil, err
	}
	out := make([]Friend, 0, len(resp.Friends))
	for _, f := range resp.Friends {
		tr := f.Track.card("track")
		tr.Subtitle = f.Track.Artist.Name
		out = append(out, Friend{
			User: f.User.card("user"), Track: tr,
			Artist: f.Track.Artist.card("artist"), Context: f.Track.Context.card(URIType(f.Track.Context.URI)),
			Timestamp: f.Timestamp, Playing: nowMs-f.Timestamp < 5*60*1000,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	return out, nil
}
