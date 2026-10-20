package spotify

import (
	"context"
	"encoding/json"
)

// "Exclude from your taste profile" is the user's "ignoreinrecs" collection
// set (spclient collection/v2, JSON as the web player calls it).

const tasteSet = "ignoreinrecs"

func (c *Client) collection(ctx context.Context, op, path string, items []map[string]any, idempotent bool) ([]byte, error) {
	user, err := c.username()
	if err != nil {
		return nil, &Error{Op: op, Err: err}
	}
	body, _ := json.Marshal(map[string]any{"username": user, "set": tasteSet, "items": items})
	return c.do(ctx, request{
		op: op, method: "POST", url: c.spclientURL + "/collection/v2/" + path, body: body,
		contentType: "application/json", accept: "application/json", idempotent: idempotent,
	})
}

// TasteExcluded reports for each uri whether it is left out of the taste profile.
func (c *Client) TasteExcluded(ctx context.Context, uris []string) ([]bool, error) {
	items := make([]map[string]any, len(uris))
	for i, u := range uris {
		items[i] = map[string]any{"uri": u}
	}
	const op = "collection ignoreinrecs contains"
	resp, err := c.collection(ctx, op, "contains", items, true)
	if err != nil {
		return nil, err
	}
	var r struct {
		Found []bool `json:"found"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return nil, &Error{Op: op, Err: err}
	}
	return r.Found, nil
}

// SetTasteExcluded leaves uri out of (or puts it back into) the taste profile.
func (c *Client) SetTasteExcluded(ctx context.Context, uri string, excluded bool) error {
	item := map[string]any{"uri": uri}
	if !excluded {
		item["is_removed"] = true
	}
	_, err := c.collection(ctx, "collection ignoreinrecs write", "write", []map[string]any{item}, false)
	return err
}
