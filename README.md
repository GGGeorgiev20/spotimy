# Spotimy

A lightweight Spotify app for Linux that looks and behaves like the official desktop client, but is a small native program instead of a bundled web browser. It uses less memory, starts and skips songs quickly, and runs natively on Wayland.

It's a personal project (it used to be called personal-spotify): unofficial, not affiliated with or endorsed by Spotify, and it needs a **Spotify Premium** account.

## Install

Download the file for your system from the [releases page](https://github.com/GGGeorgiev20/personal-spotify/releases).

**openSUSE**

```bash
sudo zypper in ./spotimy-0.1.1-1.x86_64.rpm
```

**Arch Linux**

```bash
sudo pacman -U spotimy-0.1.1-1-x86_64.pkg.tar.zst
```

**Any other distribution** (Ubuntu 22.04+, Debian 12+, Fedora 35+, or anything with glibc 2.34 or newer)

```bash
chmod +x spotimy-0.1.1-x86_64.AppImage
./spotimy-0.1.1-x86_64.AppImage
```

Then open **Spotimy** from your app launcher. The first time, it opens Spotify's login page in your browser; once you've logged in there, the app takes over and remembers you.

### What you need

- A **Wayland** desktop (KDE Plasma, GNOME, Sway, Hyprland…). X11-only sessions aren't supported.
- A **keyring** (KWallet or GNOME Keyring), where your login is stored. Most desktops already run one.
- **Optional:** `ffmpeg` for the looping Canvas videos behind songs (without it you see the cover instead), and a color emoji font for emoji in playlist names.

## What it can do

- **Everything you browse in Spotify:** Home with its filters, search with live suggestions and every result tab, artists (discography, tour dates, merch, the About dialog with photos), albums, playlists (including Discover Weekly and the Daily Mixes), profiles, podcasts, genres and moods, and What's New.
- **Your Library:** filters, sorting, search, list, compact and grid views, pinned items and folders. It can be resized or collapsed to an icon rail.
- **Playlists:** create, rename, delete, add and remove songs, drag songs onto playlists in the sidebar, reorder by dragging, sort by any column, recommended songs.
- **Playback:** the full player bar, shuffle, repeat, radio, autoplay, the queue (drag songs between "Next in queue" and "Next from", click one to skip to it), and Recently played.
- **Now Playing:** the side view with Canvas videos, lyrics that follow the song, About the artist and credits; full screen and cinema mode with large synced lyrics.
- **Spotify Connect:** see your other devices, move playback between them, and control whichever one is playing.
- **Friend Activity**, right-click menus everywhere, the official keyboard shortcuts (Ctrl+/ lists them), and media keys and desktop media widgets.
- **Settings:** audio quality, volume normalization and level, crossfade, mono audio, autoplay, zoom level, compact library, start on login, and clearing the cache.
- **Picks up where you left off:** after a restart, your last song is loaded, paused at the moment you stopped.
- Opening a `spotify:` or open.spotify.com link brings it up in the app.

## Compared to the official client

### What's better

- **Memory:** about **310 MB** while playing, and around 450 MB at most after browsing many pages full of pictures. The official client bundles a whole Chromium browser and usually uses considerably more.
- **CPU:** about **3% of one core** while a song plays with the window open (measured on an Intel i5-13500T).
- **Small:** one program of about 25 MB, with no web browser inside.
- **Native Wayland:** sharp on scaled displays, without the XWayland workarounds the official client needs.
- **Faster playing and skipping:** the next song is prepared shortly after the current one starts, so skipping plays at once, and starting a song fetches what it needs in parallel instead of one piece at a time.
- **Calmer when Spotify throttles:** if Spotify briefly refuses to play, the app stops and tells you instead of rapidly skipping through your playlist.
- **Settings apply while playing:** quality, normalization, crossfade and mono change without restarting playback.

### What's missing

- **Downloads and offline listening.**
- **Local files** from your computer.
- **Jams, Blends and Spotify DJ** can't be started from the app, though Blend playlists play normally.
- **Music videos and video podcasts** play as audio only.
- **Audiobooks** are untested and may not play.
- **Equalizer, Automix, Smart Shuffle, sleep timer and the mini player.**
- **Privacy and social settings:** private session, sharing your listening activity, and what your profile shows. Changes made in another Spotify app still apply.
- **Some settings:** the lowest and lossless audio qualities, the explicit content filter, proxy settings and the download location.
- **Translations:** the app's own text is English only. Choosing a language in the settings only changes the language Spotify's content comes in, such as section titles on Home.
- **X11, Windows and macOS.**

## How it works

- **The interface** is written in Go with [Gio](https://gioui.org), which draws everything directly on the GPU. Every page, menu and animation is drawn by the app itself, matched to the official client's spacing, colors and wording, instead of being a web page.
- **Playback, login and Spotify Connect** come from [go-librespot](https://github.com/devgianlu/go-librespot), an open-source Spotify player, built into the app. Its copy here carries several changes: faster starts and skips, reordering the queue, and settings that apply while playing.
- **Data** comes from the same internal Spotify services the official app uses, not Spotify's public developer API. That's how it gets things the public API doesn't offer, such as lyrics, Canvas videos, Friend Activity and the full Home feed.
- **Your data stays on your computer:** your login lives in the system keyring, and settings and the play history live in `~/.config/spotimy`. Pictures and songs are cached in `~/.cache/spotimy`. Logging out from the profile menu deletes your login, the play history and the caches.

Because it relies on Spotify's internal services, a change on Spotify's side can break a section of the app until it's updated. Using an unofficial client may also go against Spotify's terms of service, so use it at your own risk.

Building from source and other developer notes are in [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

## Updates

I made this for myself, so don't expect regular updates: I'll change things when I notice something I don't like while using it.

## Made with Claude

This app was vibe coded: Claude, Anthropic's AI, wrote the code in Claude Code, while I directed what it should do, tested it and reported what was wrong.

## License

GPL-3.0, the same license as go-librespot.
