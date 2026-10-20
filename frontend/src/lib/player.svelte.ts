import * as App from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { daemon } from '../../wailsjs/go/models'

export type Status = daemon.ApiStatus
export type RepeatMode = 'off' | 'context' | 'track'

export const player = $state({
  status: null as Status | null,
  fetchedAt: 0,
  now: Date.now(),
})

export function playing() {
  const s = player.status
  return !!s?.track && !s.stopped && !s.paused
}

export function position(): number {
  const s = player.status
  if (!s?.track) return 0
  const p = s.track.position + (playing() && !s.buffering ? player.now - player.fetchedAt : 0)
  return Math.min(p, s.track.duration)
}

export function repeatMode(): RepeatMode {
  const s = player.status
  return s?.repeat_track ? 'track' : s?.repeat_context ? 'context' : 'off'
}

let pending = 0
export function refresh() {
  clearTimeout(pending)
  pending = setTimeout(async () => {
    try {
      player.status = await App.Status()
      player.fetchedAt = player.now = Date.now()
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
  playPause: () => App.PlayPause(),
  next: () => App.Next(),
  prev: () => App.Prev(),
  seek(ms: number) {
    if (player.status?.track) {
      player.status.track.position = ms
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
    volTimer = setTimeout(function flush() {
      volTimer = 0
      if (volPending !== volSent) controls.setVolume(volPending)
    }, 150)
  },
  toggleShuffle: () => App.SetShuffle(!player.status?.shuffle_context),
  cycleRepeat() {
    const next: Record<RepeatMode, RepeatMode> = { off: 'context', context: 'track', track: 'off' }
    return App.SetRepeat(next[repeatMode()])
  },
  play: (uri: string, skipTo = '') => App.Play(uri, skipTo),
}

export function fmtTime(ms: number) {
  const s = Math.floor(ms / 1000)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const ss = String(s % 60).padStart(2, '0')
  return h ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`
}
