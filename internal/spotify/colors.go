package spotify

import (
	"context"
	"errors"
	"strings"
	"time"
)

// imageURI is the spotify:image: or spotify:mosaic: URI of a CDN image URL.
func imageURI(u string) string {
	if _, id, ok := strings.Cut(u, "i.scdn.co/image/"); ok {
		return "spotify:image:" + id
	}
	if _, ids, ok := strings.Cut(u, "mosaic.scdn.co/"); ok {
		ids = ids[strings.Index(ids, "/")+1:]
		var parts []string
		for ; len(ids) >= 40; ids = ids[40:] { // image ids are 40 hex digits
			parts = append(parts, ids[:40])
		}
		return "spotify:mosaic:" + strings.Join(parts, ":")
	}
	return ""
}

// ImageColor asks Spotify for the dark color it extracts from a cover
// ("#rrggbb"), the one the web player tints headers with. Only i.scdn.co
// images have an id to ask about.
func (c *Client) ImageColor(ctx context.Context, imageURL string) (string, error) {
	uri := imageURI(imageURL)
	if uri == "" {
		return "", errors.New("not a spotify image")
	}
	var resp struct {
		ExtractedColors []struct {
			ColorDark struct {
				Hex string `json:"hex"`
			} `json:"colorDark"`
		} `json:"extractedColors"`
	}
	vars := map[string]any{"imageUris": []string{uri}}
	if err := c.query(ctx, "fetchExtractedColors", vars, 24*time.Hour, &resp); err != nil {
		return "", err
	}
	if len(resp.ExtractedColors) == 0 || resp.ExtractedColors[0].ColorDark.Hex == "" {
		return "", errors.New("no color")
	}
	return resp.ExtractedColors[0].ColorDark.Hex, nil
}
