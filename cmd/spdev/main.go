// spdev is a development tool for the internal API: it logs in with the
// credentials the app stored and runs raw calls, printing the response.
//
//	spdev query <operation> '<variables json>'
//	spdev get <spclient path>
//	spdev friends | post <path> <json> | mercury <hm:// uri> | dealer <secs> [path...]
//	spdev create <name> | rename <uri> <name> | delete <uri>
//	spdev cluster | play <context uri>
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/GGGeorgiev20/personal-spotify/internal/player"
	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	librespot "github.com/devgianlu/go-librespot"
	"github.com/devgianlu/go-librespot/apresolve"
	"github.com/devgianlu/go-librespot/dealer"
	"github.com/devgianlu/go-librespot/login5"
	devicespb "github.com/devgianlu/go-librespot/proto/spotify/connectstate/devices"
	credentialspb "github.com/devgianlu/go-librespot/proto/spotify/login5/v3/credentials"
	"github.com/devgianlu/go-librespot/session"
)

var l5tok librespot.GetLogin5TokenFunc

func main() {
	if len(os.Args) < 2 || (len(os.Args) < 3 && os.Args[1] != "friends") {
		fmt.Fprintln(os.Stderr, "usage: spdev query <op> [vars-json] | spdev get <path>")
		os.Exit(2)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cfg, _ := os.UserConfigDir()
	cfg = filepath.Join(cfg, "spotimy")
	user, cred, deviceID, err := player.StoredCredentials(cfg)
	check(err)

	id := spotify.DefaultIdentity()
	id.DeviceID = deviceID
	var l5 *login5.Login5
	client := spotify.New(id, func(ctx context.Context) (string, error) {
		return l5.AccessToken()(ctx, false)
	}, log, filepath.Join(cfg, "operations.json"))
	ct, err := client.ClientToken(ctx)
	check(err)
	l5tok = func(ctx context.Context, force bool) (string, error) { return l5.AccessToken()(ctx, force) }
	l5 = login5.NewLogin5(&librespot.NullLogger{}, http.DefaultClient, deviceID, ct)
	check(l5.Login(ctx, &credentialspb.StoredCredential{Username: user, Data: cred}))

	var out []byte
	switch os.Args[1] {
	case "query":
		vars := map[string]any{}
		if len(os.Args) > 3 {
			check(json.Unmarshal([]byte(os.Args[3]), &vars))
		}
		out, err = client.RawQuery(ctx, os.Args[2], vars)
	case "get":
		out, err = client.RawGet(ctx, os.Args[2], "application/json")
	case "friends": // Friend Activity as the app builds it (names redacted to A, B, ...)
		var fr []spotify.Friend
		fr, err = client.Friends(ctx, time.Now().UnixMilli())
		for i, f := range fr {
			fmt.Printf("friend %c: playing=%v ts=%d track=%q artist=%q album=%q context=%s %q\n", 'A'+i, f.Playing, f.Timestamp, f.Track.Name, f.Track.Subtitle, f.Album.Name, f.Context.Type, f.Context.Name)
		}
		fmt.Println("count:", len(fr))
	case "post": // read-style POST: spdev post <path> '<json>'
		out, err = client.RawPostJSON(ctx, os.Args[2], []byte(os.Args[3]))
	case "create", "rename", "delete":
		_, err = client.Me(ctx)
		check(err)
		switch os.Args[1] {
		case "create":
			var uri string
			uri, err = client.CreatePlaylist(ctx, os.Args[2])
			out = []byte(uri + "\n")
		case "rename":
			err = client.RenamePlaylist(ctx, os.Args[2], os.Args[3], nil)
		case "delete":
			err = client.DeletePlaylist(ctx, os.Args[2])
		}
	case "play": // play a context on this app's device (for testing without clicking)
		var cn *spotify.Connect
		cn, err = client.StartConnect(ctx, deviceID, nil)
		check(err)
		time.Sleep(2 * time.Second)
		target := deviceID
		if len(os.Args) > 3 { // spdev play <uri> [device-id]
			target = os.Args[3]
		}
		err = cn.CommandTo(ctx, target, "play", map[string]any{"context": map[string]any{"uri": os.Args[2], "url": "context://" + os.Args[2]}})
		cn.Close()
	case "episodes": // saved episodes, to check the collection call
		_, err = client.Me(ctx)
		check(err)
		var eps []spotify.Episode
		eps, err = client.YourEpisodes(ctx)
		out, _ = json.MarshalIndent(map[string]any{"count": len(eps)}, "", "  ")
	case "mercury": // read-only Hermes GET over the access point: spdev mercury <hm:// uri>
		var sess *session.Session
		sess, err = session.NewSessionFromOptions(ctx, &session.Options{
			Log: &librespot.NullLogger{}, DeviceType: devicespb.DeviceType_COMPUTER, DeviceId: deviceID,
			Credentials: session.StoredCredentials{Username: user, Data: cred}, ClientToken: ct,
		})
		check(err)
		out, err = sess.Mercury().Request(ctx, "GET", os.Args[2], nil, nil)
		sess.Close()
	case "dealer": // dealer only (no device registered): spdev dealer <secs> <path>... GETs each path with the connection id, prints pushes
		out, err = dealerProbe(ctx, client, os.Args[2:])
	case "cluster":
		var cn *spotify.Connect
		cn, err = client.StartConnect(ctx, deviceID, nil)
		check(err)
		time.Sleep(4 * time.Second)
		out, err = json.MarshalIndent(cn.State(), "", "  ")
		cn.Close()
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	check(err)
	os.Stdout.Write(out)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func dealerProbe(ctx context.Context, client *spotify.Client, args []string) ([]byte, error) {
	secs, _ := strconv.Atoi(args[0])
	addr, err := apresolve.NewApResolver(&librespot.NullLogger{}, http.DefaultClient, false).GetDealer(ctx)
	if err != nil {
		return nil, err
	}
	d := dealer.NewDealer(&librespot.NullLogger{}, http.DefaultClient, addr, func(ctx context.Context, force bool) (string, error) {
		return l5tok(ctx, force)
	})
	if err := d.Connect(ctx); err != nil {
		return nil, err
	}
	defer d.Close()
	msgs := d.ReceiveMessage("")
	deadline := time.After(time.Duration(secs) * time.Second)
	for {
		select {
		case <-deadline:
			return nil, nil
		case m := <-msgs:
			fmt.Printf("MSG %s hdr=%v len=%d %.300s\n", m.Uri, m.Headers, len(m.Payload), m.Payload)
			if cid := m.Headers["Spotify-Connection-Id"]; cid != "" {
				for _, p := range args[1:] {
					b, e := client.RawGetWith(ctx, p, "application/json", http.Header{"X-Spotify-Connection-Id": {cid}})
					fmt.Printf("GET %s -> err=%v %.600s\n", p, e, b)
				}
			}
		}
	}
}
