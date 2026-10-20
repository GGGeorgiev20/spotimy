package spotify

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	librespot "github.com/devgianlu/go-librespot"
	pbdata "github.com/devgianlu/go-librespot/proto/spotify/clienttoken/data/v0"
	pbhttp "github.com/devgianlu/go-librespot/proto/spotify/clienttoken/http/v0"
	"google.golang.org/protobuf/proto"
)

// ClientToken returns the cached client-token, fetching a new one when it is
// due for refresh. The same token is handed to go-librespot so both halves of
// the app present one identity.
func (c *Client) ClientToken(ctx context.Context) (string, error) { return c.clientToken(ctx) }

func (c *Client) clientToken(ctx context.Context) (string, error) {
	c.ctMu.Lock() // held across the fetch so concurrent callers share one request
	defer c.ctMu.Unlock()
	if c.clientTok != "" && time.Now().Before(c.clientTokEx) {
		return c.clientTok, nil
	}
	tok, refresh, err := fetchClientToken(ctx, c.http, c.id)
	if err != nil {
		if c.clientTok != "" {
			return c.clientTok, nil // keep using the old one; it usually still works
		}
		return "", &Error{Op: "clienttoken", Err: err}
	}
	c.clientTok, c.clientTokEx = tok, time.Now().Add(refresh)
	return tok, nil
}

func fetchClientToken(ctx context.Context, hc *http.Client, id Identity) (string, time.Duration, error) {
	body, err := proto.Marshal(&pbhttp.ClientTokenRequest{
		RequestType: pbhttp.ClientTokenRequestType_REQUEST_CLIENT_DATA_REQUEST,
		Request: &pbhttp.ClientTokenRequest_ClientData{
			ClientData: &pbhttp.ClientDataRequest{
				ClientId:      id.ClientID,
				ClientVersion: id.AppVersion,
				Data: &pbhttp.ClientDataRequest_ConnectivitySdkData{
					ConnectivitySdkData: &pbdata.ConnectivitySdkData{
						DeviceId:             id.DeviceID,
						PlatformSpecificData: librespot.GetPlatformSpecificData(),
					},
				},
			},
		},
	})
	if err != nil {
		return "", 0, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", clientTokenURL, bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Accept", "application/x-protobuf")
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("User-Agent", id.userAgent())
	resp, err := hc.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, excerpt(data))
	}
	var out pbhttp.ClientTokenResponse
	if err := proto.Unmarshal(data, &out); err != nil {
		return "", 0, err
	}
	g := out.GetGrantedToken()
	if g == nil {
		return "", 0, fmt.Errorf("no token granted (response type %v)", out.ResponseType)
	}
	refresh := time.Duration(g.RefreshAfterSeconds) * time.Second
	if refresh <= 0 {
		refresh = time.Hour
	}
	return g.Token, refresh, nil
}
