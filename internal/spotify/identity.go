package spotify

import (
	"net/http"

	librespot "github.com/devgianlu/go-librespot"
)

// Identity is the official client we present as. It must match the client ID
// go-librespot logged in with (the desktop client), so tokens and headers agree.
type Identity struct {
	ClientID   string
	AppVersion string // e.g. 1.2.77.358; goes stale, overridable in config.json
	Platform   string // app-platform header
	Language   string // Accept-Language
	DeviceID   string
}

// DefaultAppVersion is the desktop version matching go-librespot's version code.
const DefaultAppVersion = "1.2.77.358"

func DefaultIdentity() Identity {
	return Identity{
		ClientID:   librespot.ClientIdHex,
		AppVersion: DefaultAppVersion,
		Platform:   "Linux",
		Language:   "en",
	}
}

func (id Identity) userAgent() string {
	return "Spotify/" + id.AppVersion + " Linux/0 (PC desktop)"
}

func (id Identity) setHeaders(h http.Header) {
	h.Set("User-Agent", id.userAgent())
	h.Set("app-platform", id.Platform)
	h.Set("spotify-app-version", id.AppVersion)
	h.Set("Accept-Language", id.Language)
}
