# Development

Notes for building and working on Spotimy. The [README](../README.md) is the user-facing overview.

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

- `SPOTIMY_LOG=debug` turns on verbose logging, including every player request and go-librespot's debug output.
- `SPOTIMY_HARNESS=127.0.0.1:<port>` runs the interface headless for automated UI testing, controlled over HTTP (navigate, click, type, screenshot, read the accessibility tree). It runs in sandbox mode: no player session, and anything that would change the account or playback is recorded instead of sent. See `internal/ui/harness.go`.

Tests:

```bash
make test
```

go-librespot is vendored in `third_party/go-librespot` (go.mod replaces the module with it), patched to start tracks faster:

- the track's metadata and its audio files come in one request;
- the audio key is fetched while storage resolves and the first chunk downloads;
- a play request's track starts loading while its context resolves;
- the next track is readied ten seconds into the current one rather than 30 seconds before its end, so skipping plays at once;
- when Spotify throttles audio keys (code 2) playback stops and keys aren't asked for again for 15 seconds, instead of skipping through the whole context asking for a key per track;
- status reports the track that is loading when its metadata is known;
- set_queue reorders the upcoming context tracks too (moving songs around "Next from", and between it and the queue), not just the queue, and removes a context track left out from among the ones it lists.
- A playing track's position keeps counting past ten minutes from its last load, seek or resume (it was taken for a stale timestamp and reported as the start), capped at the track's length.
- The status reports a track relinked to its regional alternative under the id that was asked for, while it loads and once it plays.
- a track's end is reported once the audio output has played what it buffered of it (PulseAudio keeps two seconds), so the next track doesn't show, and count its position, that much early;
- set_audio_options changes the listening settings while it runs: the streaming bitrate and autoplay (next track), volume normalisation and its level (next track), crossfade (at once, or from the next track change while audio is buffered ahead) and a mono downmix (at once).

Its own tests run with `cd third_party/go-librespot && go test -tags test_unit ./...`.

`cmd/spdev` is a development tool. It logs in with the credentials the app stored and calls the internal API directly. Use it to probe variables, capture fixtures, or check a broken operation:

```bash
go run ./cmd/spdev query getAlbum '{"uri":"spotify:album:4LH4d3cOWNNsVw41Gqt2kv","locale":"","offset":0,"limit":50}'
go run ./cmd/spdev get '/color-lyrics/v2/track/4oXg7xT4ksBxHTx8PcmSXw?format=json'
go run ./cmd/spdev cluster
```

A missing GraphQL variable comes back as an error that names it, for example ``missing variable `$enableWatchFeedEntrypoint` ``. Strip tokens, user IDs, names and lyrics from any response before saving it under `internal/spotify/testdata`.

## Packages

The packages install the binary, the desktop entry (which also gives the window its icon) and the icon. Building them:

```bash
git tag v0.1.0      # the version comes from the latest tag
make rpm            # openSUSE package, with rpmbuild, into build/dist
make arch           # Arch package, with makepkg in an archlinux Docker container, into build/dist
make appimage APPIMAGETOOL=/path/to/appimagetool   # AppImage, built in a debian:bookworm container
```

`packaging/opensuse/spotimy.spec` and `packaging/arch/PKGBUILD` build from the source tarball `make dist` makes. Without packages:

```bash
make build
make install PREFIX=~/.local
```

## Where things live

| What | Where |
|---|---|
| Login credentials | OS keyring, service `spotimy` (or `state.json` with mode 0600 if no keyring is available) |
| Device id, volume, player event state | `~/.config/spotimy/state.json` (mode 0600) |
| User config and pathfinder hash overrides | `~/.config/spotimy/` |
| Audio and image cache | `~/.cache/spotimy/` |

Log out from the profile menu to delete the credentials, the state and the cache.

## Updating a broken pathfinder hash

Pathfinder queries are persisted queries identified by a sha256 hash. When Spotify ships a new web player, a hash can go stale. The affected section then shows as unavailable, and the log names the operation:

```
level=WARN msg="spotify request failed" op=fetchPlaylist status=400 body="...PersistedQueryNotFound..."
```

All hashes live in one registry, `internal/spotify/operations.json`. To fix a broken one:

1. Open open.spotify.com in a browser, open DevTools, go to Network, filter by `pathfinder`, and trigger the broken view. Copy `extensions.persistedQuery.sha256Hash` from the request with the matching `operationName`.
2. Either put it in `~/.config/spotimy/operations.json` as `{"fetchPlaylist": "<hash>"}`, which takes effect on restart with no rebuild, or update `internal/spotify/operations.json` and rebuild.

To check a fix without the app, run `go run ./cmd/spdev query <operationName> '<variables>'`. If the variables or the response shape changed as well, the operation's typed method in `internal/spotify` has to be updated.

The registry was extracted from the web player's JavaScript, the main bundle plus every lazy chunk listed in its `.u=` chunk map, by matching `"<name>","query|mutation","<sha256>"`. Redo that if many hashes break at once.
