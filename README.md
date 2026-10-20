# personal-spotify

A lightweight Spotify desktop client for Linux, for personal use. It looks like the official client but runs as a Go + Wails app on the system WebKitGTK, with [go-librespot](https://github.com/devgianlu/go-librespot) embedded for login, playback and Spotify Connect. Spotify Premium is required.

It talks to Spotify's internal API (pathfinder GraphQL, spclient and the dealer), the same API the official client uses. It does not use the public Web API.

## System dependencies (openSUSE Tumbleweed)

```bash
sudo zypper in go nodejs npm webkitgtk3-devel gtk3-devel alsa-devel libogg-devel libvorbis-devel flac-devel mpg123-devel gstreamer-plugins-bad
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0
```

- `webkitgtk3-devel` provides `webkit2gtk-4.1`. The build uses the `webkit2_41` tag, which is set in `wails.json`.
- go-librespot needs `libogg`, `libvorbis`, `flac`, `mpg123` and `alsa`. Audio goes out through PulseAudio, and PipeWire's pulse server handles it on PipeWire systems.
- Canvas videos need GStreamer plugins that can play H.264 MP4. `gstreamer-plugins-libav` is the usual choice, but on a stock Tumbleweed install it conflicts with `ffmpeg-8-mini-libs` and zypper offers to remove the X11 KWin session. Don't accept that. Canvas falls back to cover art when the video can't play.
- Login credentials are stored in the Secret Service keyring (KWallet/ksecretd or GNOME Keyring), so one has to be running.

## Development

```bash
wails dev
```

Environment variables:

- `PERSONAL_SPOTIFY_LOG=debug` turns on verbose logging, including every player request and go-librespot's debug output.
- `PERSONAL_SPOTIFY_GPU=1` turns on WebKit GPU compositing. It's off by default because that saves about 60 MB of RAM.

Tests:

```bash
go test ./internal/...
```

## Release build and desktop entry

```bash
wails build
install -Dm755 build/bin/personal-spotify ~/.local/bin/personal-spotify
install -Dm644 packaging/personal-spotify.desktop ~/.local/share/applications/personal-spotify.desktop
install -Dm644 packaging/personal-spotify.svg ~/.local/share/icons/hicolor/scalable/apps/personal-spotify.svg
```

## Where things live

| What | Where |
|---|---|
| Login credentials | OS keyring, service `personal-spotify` (or `state.json` with mode 0600 if no keyring is available) |
| Device id, volume, player event state | `~/.config/personal-spotify/state.json` (mode 0600) |
| User config and pathfinder hash overrides | `~/.config/personal-spotify/` |
| Audio and image cache | `~/.cache/personal-spotify/` |

Log out from the profile menu to delete the credentials, the state and the cache.

## Updating a broken pathfinder hash

Pathfinder queries are persisted queries identified by a sha256 hash. When Spotify ships a new web player, a hash can go stale. The affected section then shows as unavailable, and the log names the operation:

```
level=WARN msg="spotify request failed" op=fetchPlaylist status=400 body="...PersistedQueryNotFound..."
```

All hashes live in one registry, `internal/spotify/operations.json`. To fix a broken one:

1. Open open.spotify.com in a browser, open DevTools, go to Network, filter by `pathfinder`, and trigger the broken view. Copy `extensions.persistedQuery.sha256Hash` from the request with the matching `operationName`.
2. Either put it in `~/.config/personal-spotify/operations.json` as `{"fetchPlaylist": "<hash>"}`, which takes effect on restart with no rebuild, or update `internal/spotify/operations.json` and rebuild.

If the variables or the response shape changed as well, the operation's typed method in `internal/spotify` has to be updated.

## License

GPL-3.0, the same license as go-librespot.
