package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// ---- Player ----

// Player is the playback state shown everywhere. Read it on the UI goroutine.
var Player struct {
	Status    *backend.PlayerStatus
	FetchedAt time.Time
	// Version bumps on every refresh (queue and device views reload on it).
	Version int
}

var (
	refreshPending bool
	volSent        int
	volPending     int
	volTimer       *time.Timer
)

// RefreshPlayer re-reads the status soon (coalescing bursts of events).
// Safe from any goroutine.
func RefreshPlayer() {
	Post(func() {
		if refreshPending {
			return
		}
		refreshPending = true
		time.AfterFunc(30*time.Millisecond, func() {
			Async(B.Status, func(s *backend.PlayerStatus, err error) {
				refreshPending = false
				if err != nil {
					return
				}
				loadingTrack(s, Player.Status, time.Now())
				Player.Status, Player.FetchedAt = s, time.Now()
				Player.Version++
				noteAlias(s.ContextURI)
			})
		})
	})
}

// settle tracks how long the playing track has been the same one.
var settle struct {
	uri   string
	since time.Time
}

// settleDelay is how long a track must stay before its lookups run.
const settleDelay = 600 * time.Millisecond

// Settled reports whether the playing track has stayed for a moment. Lookups
// made per track (lyrics, Now Playing details, saved state) wait for it, so a
// run of quick skips doesn't send a burst of requests for tracks only passed
// through; Spotify rate limits those.
func Settled(gtx layout.Context) bool {
	uri := ""
	if t := NowTrack(); t != nil {
		uri = t.URI
	}
	if uri != settle.uri {
		settle.uri, settle.since = uri, gtx.Now
	}
	if at := settle.since.Add(settleDelay); gtx.Now.Before(at) {
		gtx.Execute(op.InvalidateCmd{At: at})
		return false
	}
	return true
}

// Track is the playing track, or nil.
func NowTrack() *backend.NowTrack {
	if Player.Status == nil {
		return nil
	}
	return Player.Status.Track
}

// Playing reports audio running (not paused) with a track loaded.
func Playing() bool {
	return Player.Status != nil && Player.Status.Track != nil && Player.Status.Playing
}

// Position extrapolates the playback position to now.
func Position(now time.Time) time.Duration {
	s := Player.Status
	if s == nil || s.Track == nil {
		return 0
	}
	p := time.Duration(s.PositionMs) * time.Millisecond
	if s.Playing && !s.Buffering {
		p += now.Sub(Player.FetchedAt)
	}
	return min(p, time.Duration(s.Track.DurationMs)*time.Millisecond)
}

func RepeatMode() string {
	switch s := Player.Status; {
	case s == nil:
		return "off"
	case s.RepeatTrack:
		return "track"
	case s.RepeatContext:
		return "context"
	}
	return "off"
}

// IsContext reports whether uri is the playing context. Liked Songs plays as
// spotify:user:<name>:collection but is listed as spotify:collection:tracks.
func IsContext(uri string) bool {
	if Player.Status == nil || uri == "" || Player.Status.ContextURI == "" {
		return false
	}
	ctx := Player.Status.ContextURI
	if uri == "spotify:collection:tracks" {
		return len(ctx) > 11 && ctx[len(ctx)-11:] == ":collection"
	}
	return sameEntity(ctx, uri) || contextAliases[ctx] == uri
}

// sameEntity compares URIs by type and id, so the legacy
// spotify:user:<owner>:playlist:<id> form some players report matches
// spotify:playlist:<id>.
func sameEntity(a, b string) bool {
	return a == b || URIType(a) == URIType(b) && URIID(a) == URIID(b)
}

// CtxPlaying reports whether uri is the context playing right now.
func CtxPlaying(uri string) bool { return IsContext(uri) && Playing() }

// IsCurrent reports whether track uri is the one loaded (in contextURI, if set).
func IsCurrent(trackURI, contextURI string) bool {
	t := NowTrack()
	return t != nil && t.URI == trackURI && (contextURI == "" || IsContext(contextURI))
}

// IsCurrentTrack is IsCurrent for a row's track, which matches under either
// its own or its relinked URI: a playlist row may hold the original track
// while the player reports the relinked one, or the other way round.
func IsCurrentTrack(t spotify.Track, contextURI string) bool {
	return IsCurrent(t.URI, contextURI) || t.LinkedURI != "" && IsCurrent(t.LinkedURI, contextURI)
}

func playerCall(name string, f func() error) {
	Go("Couldn't "+name, f, RefreshPlayer)
}

// PlayContext plays a context, or toggles pause if it's already the one playing.
func PlayContext(uri string) {
	if IsContext(uri) {
		PlayPause()
		return
	}
	Play(uri, "")
}

func Play(uri, skipTo string) {
	playRequest, playRequestAt = uri, time.Now()
	playerCall("play", func() error { return B.Play(uri, skipTo) })
}

// PlayTrack plays t in context (or on its own when context is ""), showing
// it in the player bar at once, as the official client does, while the
// audio loads.
func PlayTrack(context string, t spotify.Track) {
	nt := &backend.NowTrack{URI: t.URI, Name: t.Name, Artists: t.Artists, DurationMs: int64(t.DurationMs)}
	if t.Album != nil {
		nt.AlbumURI, nt.AlbumName, nt.Images = t.Album.URI, t.Album.Name, t.Album.Images
	}
	ctx := context
	if ctx == "" {
		ctx = t.URI
	}
	starting.track, starting.context, starting.at = nt, ctx, time.Now()
	if s := Player.Status; s != nil {
		s.Track, s.PositionMs, s.Playing, s.Buffering, s.ContextURI = nt, 0, true, true, ctx
		Player.FetchedAt = time.Now()
		Player.Version++
	}
	if context == "" {
		Play(t.URI, "")
	} else {
		Play(context, t.URI)
	}
}

// starting is the track PlayTrack started, shown until the player reports
// it: while a track loads the player reports none.
var starting struct {
	track   *backend.NowTrack
	context string
	at      time.Time
}

// loadingTrack fills in the track of a status taken mid-load: the one just
// started, else the one on its way out, rather than an empty player bar.
func loadingTrack(s, prev *backend.PlayerStatus, now time.Time) {
	if st := starting.track; st != nil {
		age := now.Sub(starting.at)
		switch {
		case s.Track != nil && s.Track.URI == st.URI && !s.Buffering:
			starting.track = nil // it's playing
			return
		case age < 15*time.Second && (s.Buffering || age < startingGrace):
			// On its way: until then the player still reports the outgoing
			// track, and showing that would flash it between the click and
			// the new one.
			s.Track, s.PositionMs, s.ContextURI = st, 0, starting.context
			s.Playing, s.Buffering = true, true
			return
		default:
			starting.track = nil // something else plays: it failed or was replaced
		}
	}
	if s.Track == nil && s.Buffering && prev != nil {
		s.Track, s.PositionMs = prev.Track, prev.PositionMs
	}
}

// startingGrace is how long a started track is shown before the player has
// begun loading it (until then it reports the outgoing track as playing).
const startingGrace = 2 * time.Second

// The player may report a context under another URI than the one we asked
// it to play: Spotify resolves some playlists to other contexts (the "Liked
// Songs" playlist to spotify:user:<name>:collection). Remember what each
// reported context was started as, so the page that started it still shows
// as playing.
var (
	playRequest    string
	playRequestAt  time.Time
	contextAliases = map[string]string{}
)

func noteAlias(ctx string) {
	if playRequest == "" || ctx == "" || time.Since(playRequestAt) > 15*time.Second {
		return
	}
	if !sameEntity(ctx, playRequest) {
		contextAliases[ctx] = playRequest
	}
	playRequest = ""
}

// PlayPause toggles optimistically; the event refresh confirms.
func PlayPause() {
	if s := Player.Status; s != nil {
		now := time.Now()
		s.PositionMs = Position(now).Milliseconds()
		Player.FetchedAt = now
		s.Playing = !s.Playing
	}
	playerCall("play", B.PlayPause)
}

func Next() { playerCall("skip", B.Next) }
func Prev() { playerCall("go back", B.Prev) }

func Seek(ms int64) {
	if s := Player.Status; s != nil && s.Track != nil {
		s.PositionMs, Player.FetchedAt = ms, time.Now()
	}
	playerCall("seek", func() error { return B.SeekTo(ms) })
}

// SetVolume sends at most one change per 150ms while dragging, plus the last.
func SetVolume(v int) {
	if s := Player.Status; s != nil {
		s.Volume = v
	}
	volPending = v
	if volTimer != nil {
		return
	}
	send := func(v int) {
		volSent = v
		go B.SetVolume(int32(v))
	}
	send(v)
	volTimer = time.AfterFunc(150*time.Millisecond, func() {
		Post(func() {
			volTimer = nil
			if volPending != volSent {
				SetVolume(volPending)
			}
		})
	})
}

var lastVolume int

// ToggleMute mutes, or restores the volume from before the mute.
func ToggleMute() {
	s := Player.Status
	switch {
	case s == nil:
	case s.Volume > 0:
		lastVolume = s.Volume
		SetVolume(0)
	case lastVolume > 0:
		SetVolume(lastVolume)
	default:
		SetVolume(s.VolumeMax / 2)
	}
}

func ToggleShuffle() {
	on := Player.Status == nil || !Player.Status.Shuffle
	if Player.Status != nil {
		Player.Status.Shuffle = on
	}
	playerCall("shuffle", func() error { return B.SetShuffle(on) })
}

func CycleRepeat() {
	next := map[string]string{"off": "context", "context": "track", "track": "off"}[RepeatMode()]
	playerCall("repeat", func() error { return B.SetRepeat(next) })
}

// ---- Saved state (hearts and checks) ----

// Saved holds what's in the library, by URI; missing = unknown.
var Saved = map[string]bool{}

// LibraryVersion bumps whenever the library changes so the left panel reloads.
var LibraryVersion int

// LikeKey is the URI whose saved state a track row shows: a relinked track is
// liked under its original id.
func LikeKey(t spotify.Track) string {
	if t.LinkedURI != "" {
		return t.LinkedURI
	}
	return t.URI
}

// LoadSaved looks up unknown URIs in batches.
func LoadSaved(uris []string) {
	var missing []string
	seen := map[string]bool{}
	for _, u := range uris {
		if _, ok := Saved[u]; !ok && u != "" && !seen[u] {
			missing = append(missing, u)
			seen[u] = true
		}
	}
	for i := 0; i < len(missing); i += 50 {
		chunk := missing[i:min(i+50, len(missing))]
		Async(func() ([]bool, error) { return B.AreSaved(chunk) }, func(res []bool, err error) {
			if err != nil || len(res) != len(chunk) {
				return
			}
			for j, u := range chunk {
				Saved[u] = res[j]
			}
		})
	}
}

// InPlaylists holds whether a track is in one of the user's playlists
// (loaded with LoadInPlaylists; missing = no).
var InPlaylists = map[string]bool{}

// playlistIndex is when InPlaylists was last filled.
var playlistIndex time.Time

// LoadInPlaylists indexes the user's playlists in the background, at most
// every 10 minutes (adding to a playlist from here updates it at once).
func LoadInPlaylists() {
	if time.Since(playlistIndex) < 10*time.Minute {
		return
	}
	playlistIndex = time.Now()
	Async(B.PlaylistTracks, func(set map[string]bool, err error) {
		if err != nil {
			Log.Warn("indexing playlists", "err", err)
			playlistIndex = time.Time{}
			return
		}
		InPlaylists = set
	})
}

// Added reports a track saved anywhere, Liked Songs or a playlist: the green
// check the official player bar and Now Playing view show.
func Added(uri string) bool { return Saved[uri] || InPlaylists[uri] }

// AddedClicked handles that button: a song saved nowhere is liked, otherwise
// a menu shows where it can be saved or removed.
func AddedClicked(uri string) {
	if !Added(uri) {
		ToggleSaved(uri)
		return
	}
	OpenMenu([]MenuItem{
		{Label: "Liked Songs", Icon: "heart", Check: Saved[uri], Action: func() { ToggleSaved(uri) }},
		{Label: "Add to playlist", Icon: "plus", Sub: playlistPicker([]string{uri})},
	})
}

// ToggleSaved likes/unlikes (tracks) or adds/removes (albums, playlists...).
func ToggleSaved(uri string) {
	next := !Saved[uri]
	Saved[uri] = next
	kind := URIType(uri)
	Async(func() (struct{}, error) { return struct{}{}, B.SetSaved([]string{uri}, next) }, func(_ struct{}, err error) {
		if err != nil {
			Saved[uri] = !next
			Toast("Couldn't update Your Library: " + err.Error())
			return
		}
		LibraryVersion++
		switch {
		case kind == "track" && next:
			Toast("Added to Liked Songs")
		case kind == "track":
			Toast("Removed from Liked Songs")
		case next:
			Toast("Added to Your Library")
		default:
			Toast("Removed from Your Library")
		}
	})
}

// URIType is spotify.URIType.
func URIType(uri string) string { return spotify.URIType(uri) }

// ---- Me ----

// Me is the logged-in user (nil until loaded).
var Me *spotify.Me

func loadMe() {
	Async(B.Me, func(m *spotify.Me, err error) {
		if err == nil {
			Me = m
		}
	})
}

// ---- Toasts ----

type toast struct {
	text string
	at   time.Time
}

var toasts []toast

// Toast shows a short message above the player bar for 3 seconds.
func Toast(text string) {
	toasts = append(toasts, toast{text, time.Now()})
	Log.Info("toast", "text", text)
	time.AfterFunc(3*time.Second+50*time.Millisecond, Invalidate)
	Invalidate()
}

// ---- Prefs ----

// Prefs are per-device UI preferences, saved to the config dir.
var Prefs = struct {
	LibSort      string `json:"libSort"`
	LibView      string `json:"libView"` // list, compact, grid
	LibWidth     int    `json:"libWidth"`
	LibCollapsed bool   `json:"libCollapsed"`
	// RightPanel: queue, devices or friends over the Now Playing view; ""
	// (or "npv") is the Now Playing view itself, the default.
	RightPanel string `json:"rightPanel"`
	// NPVHidden collapses the Now Playing view to its rail.
	NPVHidden bool `json:"npvHidden"`
}{LibSort: "Recents", LibView: "list", LibWidth: 340}

var (
	prefsPath  string
	prefsSaved string
)

func loadPrefs(dir string) {
	prefsPath = filepath.Join(dir, "ui.json")
	if data, err := os.ReadFile(prefsPath); err == nil {
		_ = json.Unmarshal(data, &Prefs)
		prefsSaved = string(data)
	}
}

// savePrefs writes the prefs if they changed; called after each frame.
func savePrefs() {
	if prefsPath == "" {
		return
	}
	data, _ := json.Marshal(Prefs)
	if string(data) == prefsSaved {
		return
	}
	prefsSaved = string(data)
	go func() {
		_ = os.MkdirAll(filepath.Dir(prefsPath), 0o700)
		_ = os.WriteFile(prefsPath, data, 0o600)
	}()
}

// TogglePanel opens the right panel name, or closes it (back to the Now
// Playing view) if it's open. "npv" shows or collapses the Now Playing view.
func TogglePanel(name string) {
	switch {
	case name == "npv" && RightPanel() == "npv":
		Prefs.NPVHidden = true
	case name == "npv":
		Prefs.RightPanel, Prefs.NPVHidden = "", false
	case Prefs.RightPanel == name:
		Prefs.RightPanel = ""
	default:
		Prefs.RightPanel = name
	}
}

// RightPanel is the panel showing on the right: queue, devices, friends,
// npv, or "" for the collapsed Now Playing rail.
func RightPanel() string {
	switch Prefs.RightPanel {
	case "", "npv":
		if Prefs.NPVHidden {
			return ""
		}
		return "npv"
	}
	return Prefs.RightPanel
}

var lyricsKnown = map[string]*Res[bool]{}

// HasLyrics reports whether a track has lyrics (true until it's known).
func HasLyrics(uri string) bool {
	r := lyricsKnown[uri]
	if r == nil {
		r = &Res[bool]{}
		lyricsKnown[uri] = r
		r.Load(func() (bool, error) {
			l, err := B.Lyrics(uri)
			return l != nil && len(l.Lines) > 0, err
		})
	}
	return !r.Done || r.Err != nil || r.V
}
