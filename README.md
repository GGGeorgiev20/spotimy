# personal-spotify

A lightweight Spotify desktop client for Linux, for personal use. It looks like the official client but is a native Go app: the interface is drawn with [Gio](https://gioui.org) on the GPU (no web view), and [go-librespot](https://github.com/devgianlu/go-librespot) is embedded for login, playback and Spotify Connect. Spotify Premium is required.

It talks to Spotify's internal API (pathfinder GraphQL, spclient and the dealer), the same API the official client uses. It does not use the public Web API.

## System dependencies (openSUSE Tumbleweed)

```bash
sudo zypper in go wayland-devel libxkbcommon-devel Mesa-libEGL-devel alsa-devel libogg-devel libvorbis-devel flac-devel mpg123-devel ffmpeg
```

- Gio draws through EGL/OpenGL ES on Wayland. The build uses the tags `nox11,novulkan` (no X11 backend, no Vulkan headers needed); the Makefile sets them.
- go-librespot needs `libogg`, `libvorbis`, `flac`, `mpg123` and `alsa`. Audio goes out through PulseAudio, and PipeWire's pulse server handles it on PipeWire systems.
- Canvas videos (the looping clips in the Now Playing view) are decoded by the `ffmpeg` binary. Without it the Now Playing view shows the cover art instead.
- Color emoji come from Noto Color Emoji if it's installed.
- Login credentials are stored in the Secret Service keyring (KWallet/ksecretd or GNOME Keyring), so one has to be running.

## Development

```bash
make run
```

Environment variables:

- `PERSONAL_SPOTIFY_LOG=debug` turns on verbose logging, including every player request and go-librespot's debug output.
- `PERSONAL_SPOTIFY_HARNESS=127.0.0.1:<port>` runs the interface headless for automated UI testing, controlled over HTTP (navigate, click, type, screenshot, read the accessibility tree). It runs in sandbox mode: no player session, and anything that would change the account or playback is recorded instead of sent. See `internal/ui/harness.go`.

Tests:

```bash
make test
```

`cmd/spdev` is a development tool. It logs in with the credentials the app stored and calls the internal API directly. Use it to probe variables, capture fixtures, or check a broken operation:

```bash
go run ./cmd/spdev query getAlbum '{"uri":"spotify:album:4LH4d3cOWNNsVw41Gqt2kv","locale":"","offset":0,"limit":50}'
go run ./cmd/spdev get '/color-lyrics/v2/track/4oXg7xT4ksBxHTx8PcmSXw?format=json'
go run ./cmd/spdev cluster
```

A missing GraphQL variable comes back as an error that names it, for example ``missing variable `$enableWatchFeedEntrypoint` ``. Strip tokens, user IDs, names and lyrics from any response before saving it under `internal/spotify/testdata`.

## Features

- Home with your personalized feed and filter chips, Search with every result tab (Songs, Playlists, Artists, Genres & Moods, Albums, Profiles, Podcasts & Shows, Episodes), recent searches and Browse all, genre and mood pages, and What's New.
- Album, playlist, artist (discography in list or grid view, every "Show all" page, On Tour, merch, About with links), profile (followers, following, follow and unfollow) and podcast pages. Playlists include Spotify-made ones such as Discover Weekly and the Daily Mixes.
- Your Library: filters, sort, search, list/compact/grid views, pins and folders. It can be resized and collapsed to a rail.
- Playlist editing: create, rename, delete, add, remove, drag-reorder, search and sort within a list, recommended songs, and "Exclude from your taste profile".
- Queue (view, add, remove, reorder, clear) and Recently played, radio for songs, artists, albums and playlists, and autoplay.
- Lyrics: time-synced, the current line highlighted, auto-scrolling, click a line to seek.
- Now Playing view with the Canvas video, About the artist, credits and next in queue; full-screen mode.
- Friend Activity.
- Spotify Connect: list devices, transfer playback, and control whichever device is playing.
- MPRIS (media keys and desktop widgets), served by go-librespot under the name `go-librespot`.
- The official keyboard shortcuts. Press Ctrl+/ for the list.
- Single instance: launching it again, or opening a `spotify:` or open.spotify.com link with it, goes to the running app.

## Release build and desktop entry

```bash
make build
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

To check a fix without the app, run `go run ./cmd/spdev query <operationName> '<variables>'`. If the variables or the response shape changed as well, the operation's typed method in `internal/spotify` has to be updated.

The registry was extracted from the web player's JavaScript, the main bundle plus every lazy chunk listed in its `.u=` chunk map, by matching `"<name>","query|mutation","<sha256>"`. Redo that if many hashes break at once.

## License

GPL-3.0, the same license as go-librespot.
