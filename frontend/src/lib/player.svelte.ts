import * as App from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { main } from '../../wailsjs/go/models'

export type Status = main.PlayerStatus
export type RepeatMode = 'off' | 'context' | 'track'

export const player = $state({
  status: null as Status | null,
  fetchedAt: 0,
  now: Date.now(),
  version: 0, // bumped on every refresh (queue/devices views listen to it)
})

export function playing() {
  return !!player.status?.track && player.status.playing
}

export function position(): number {
  const s = player.status
  if (!s?.track) return 0
  const p = s.positionMs + (s.playing && !s.buffering ? player.now - player.fetchedAt : 0)
  return Math.min(p, s.track.durationMs)
}

export function repeatMode(): RepeatMode {
  const s = player.status
  return s?.repeatTrack ? 'track' : s?.repeatContext ? 'context' : 'off'
}

// Liked Songs plays as spotify:user:<name>:collection; the library lists it as
// spotify:collection:tracks.
export function isContext(uri: string): boolean {
  const ctx = player.status?.contextUri ?? ''
  if (!uri || !ctx) return false
  if (uri === 'spotify:collection:tracks') return ctx.endsWith(':collection')
  return ctx === uri
}

export const ctxPlaying = (uri: string) => isContext(uri) && playing()

// Play a context, or toggle pause if it's already the one playing.
export function playContext(uri: string) {
  if (isContext(uri)) return controls.playPause()
  return controls.play(uri)
}

let pending = 0
export function refresh() {
  clearTimeout(pending)
  pending = setTimeout(async () => {
    try {
      player.status = await App.Status()
      player.fetchedAt = player.now = Date.now()
      player.version++
    } catch {
      /* no session yet */
    }
  }, 30)
}

export function initPlayer() {
  EventsOn('player:event', refresh)
  setInterval(() => {
    if (playing()) player.now = Date.now()
  }, 250)
  // Re-sync occasionally so drift and missed events can't accumulate.
  setInterval(() => player.status && refresh(), 15000)
  refresh()
}

let volTimer = 0
let volPending = 0
let volSent = 0

// Optimistic helpers: update local state immediately, the event refresh confirms.
export const controls = {
  playPause() {
    if (player.status) {
      player.status.positionMs = position()
      player.fetchedAt = player.now = Date.now()
      player.status.playing = !player.status.playing
    }
    return App.PlayPause()
  },
  next: () => App.Next(),
  prev: () => App.Prev(),
  seek(ms: number) {
    if (player.status?.track) {
      player.status.positionMs = ms
      player.fetchedAt = player.now = Date.now()
    }
    return App.SeekTo(Math.round(ms))
  },
  // Dragging fires many inputs; each one becomes a connect-state push, so send
  // at most one every 150ms plus the final value.
  setVolume(v: number) {
    if (player.status) player.status.volume = v
    volPending = Math.round(v)
    if (volTimer) return
    App.SetVolume(volPending)
    volSent = volPending
    volTimer = setTimeout(() => {
      volTimer = 0
      if (volPending !== volSent) controls.setVolume(volPending)
    }, 150)
  },
  toggleShuffle: () => App.SetShuffle(!player.status?.shuffle),
  cycleRepeat() {
    const next: Record<RepeatMode, RepeatMode> = { off: 'context', context: 'track', track: 'off' }
    return App.SetRepeat(next[repeatMode()])
  },
  play: (uri: string, skipTo = '') => App.Play(uri, skipTo),
}
