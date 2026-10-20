// spdev is a development tool for the internal API: it logs in with the
// credentials the app stored and runs raw calls, printing the response.
//
//	spdev query <operation> '<variables json>'
//	spdev get <spclient path>
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
	"time"

	"github.com/GGGeorgiev20/personal-spotify/internal/player"
	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	librespot "github.com/devgianlu/go-librespot"
	"github.com/devgianlu/go-librespot/login5"
	credentialspb "github.com/devgianlu/go-librespot/proto/spotify/login5/v3/credentials"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: spdev query <op> [vars-json] | spdev get <path>")
		os.Exit(2)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cfg, _ := os.UserConfigDir()
	cfg = filepath.Join(cfg, "personal-spotify")
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
		err = cn.CommandTo(ctx, deviceID, "play", map[string]any{"context": map[string]any{"uri": os.Args[2], "url": "context://" + os.Args[2]}})
		cn.Close()
	case "episodes": // saved episodes, to check the collection call
		_, err = client.Me(ctx)
		check(err)
		var eps []spotify.Episode
		eps, err = client.YourEpisodes(ctx)
		out, _ = json.MarshalIndent(map[string]any{"count": len(eps)}, "", "  ")
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
