package spotify

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var errNotFound = errors.New("not found")

type Me struct {
	User
	Username string `json:"username"`
	Color    string `json:"color"` // avatar placeholder background
}

// Me fetches the logged-in user's profile and remembers the URI for
// ownership checks.
func (c *Client) Me(ctx context.Context) (*Me, error) {
	var resp struct {
		Me struct {
			Profile struct {
				gqlUser
				AvatarBackgroundColor int `json:"avatarBackgroundColor"`
			} `json:"profile"`
		} `json:"me"`
	}
	if err := c.query(ctx, "profileAttributes", map[string]any{}, 10*time.Minute, &resp); err != nil {
		return nil, err
	}
	p := resp.Me.Profile
	if p.Name == "" {
		// Spotify always sends a name (the username when there's no display
		// name); none means its profile service failed this time.
		return nil, &Error{Op: "profileAttributes", Err: errors.New("the profile came without a name")}
	}
	me := &Me{User: p.user(), Username: p.Username, Color: fmt.Sprintf("#%06x", p.AvatarBackgroundColor&0xffffff)}
	c.mu.Lock()
	c.userURI = me.URI
	c.mu.Unlock()
	return me, nil
}

// UserURI is the logged-in user's URI once Me has been called.
func (c *Client) UserURI() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.userURI
}
