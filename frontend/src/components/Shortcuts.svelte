<script lang="ts">
  // Official desktop shortcuts (support.spotify.com "Keyboard shortcuts", Linux).
  import { controls, player, position } from '../lib/player.svelte'
  import { back, forward, navigate } from '../lib/router.svelte'
  import { prefs } from '../lib/prefs.svelte'
  import { toggleSaved } from '../lib/api.svelte'
  import { createPlaylist } from '../lib/actions'
  import { menu } from '../lib/menu.svelte'
  import { dialog } from '../lib/dialog.svelte'

  let help = $state(false)

  const list: [string, string][] = [
    ['Space', 'Play / pause'],
    ['Ctrl + →', 'Next track'],
    ['Ctrl + ←', 'Previous track'],
    ['Shift + →', 'Seek forward 5 s'],
    ['Shift + ←', 'Seek backward 5 s'],
    ['Ctrl + ↑ / ↓', 'Volume up / down'],
    ['M', 'Mute / unmute'],
    ['Ctrl + S', 'Shuffle'],
    ['Ctrl + R', 'Repeat'],
    ['Alt + Shift + B', 'Like / unlike song'],
    ['Ctrl + K, Ctrl + L', 'Search'],
    ['Alt + ← / →', 'Go back / forward'],
    ['Ctrl + N', 'Create playlist'],
    ['Alt + Shift + H', 'Home'],
    ['Alt + Shift + S', 'Liked Songs'],
    ['Alt + Shift + Q', 'Queue'],
    ['Alt + Shift + J', 'Now Playing view'],
    ['Alt + Shift + R', 'Toggle Now Playing sidebar'],
    ['Alt + Shift + L', 'Toggle Your Library sidebar'],
    ['Ctrl + /', 'Keyboard shortcuts'],
  ]

  let lastVolume = 0
  function volume(delta: number) {
    const s = player.status
    if (!s) return
    controls.setVolume(Math.max(0, Math.min(s.volumeMax, s.volume + delta * s.volumeMax)))
  }

  function onkey(e: KeyboardEvent) {
    const t = e.target as HTMLElement
    const typing = t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable
    if (menu.open || dialog.current) return
    const k = e.key.toLowerCase()
    const ctrl = e.ctrlKey || e.metaKey
    const run = (f: () => unknown) => (e.preventDefault(), f())

    if (ctrl && (k === '/' || k === '?')) return run(() => (help = !help))
    if (help && k === 'escape') return run(() => (help = false))
    if (ctrl && (k === 'k' || k === 'l'))
      return run(() => document.querySelector<HTMLInputElement>('.search input')?.focus())
    if (typing) return

    const s = player.status
    if (!ctrl && !e.altKey && !e.shiftKey && k === ' ') return run(() => controls.playPause())
    if (ctrl && k === 'arrowright') return run(() => controls.next())
    if (ctrl && k === 'arrowleft') return run(() => controls.prev())
    if (ctrl && k === 'arrowup') return run(() => volume(0.1))
    if (ctrl && k === 'arrowdown') return run(() => volume(-0.1))
    if (e.shiftKey && !ctrl && !e.altKey && s?.track && (k === 'arrowright' || k === 'arrowleft'))
      return run(() => controls.seek(Math.max(0, Math.min(s.track!.durationMs, position() + (k === "arrowright" ? 5000 : -5000)))))
    if (!ctrl && !e.altKey && k === 'm' && s)
      return run(() => {
        if (s.volume > 0) (lastVolume = s.volume), controls.setVolume(0)
        else controls.setVolume(lastVolume || s.volumeMax / 2)
      })
    if (ctrl && k === 's') return run(() => controls.toggleShuffle())
    if (ctrl && k === 'r') return run(() => controls.cycleRepeat())
    if (ctrl && k === 'n') return run(() => createPlaylist())
    if (e.altKey && !e.shiftKey && k === 'arrowleft') return run(back)
    if (e.altKey && !e.shiftKey && k === 'arrowright') return run(forward)
    if (e.altKey && e.shiftKey) {
      if (k === 'b' && s?.track) return run(() => toggleSaved(s.track!.uri))
      if (k === 'h') return run(() => navigate('/'))
      if (k === 's') return run(() => navigate('/collection/tracks'))
      if (k === 'q') return run(() => (prefs.rightPanel = 'queue'))
      if (k === 'j') return run(() => (prefs.rightPanel = 'npv'))
      if (k === 'r') return run(() => (prefs.rightPanel = prefs.rightPanel === 'npv' ? '' : 'npv'))
      if (k === 'l') return run(() => (prefs.libCollapsed = !prefs.libCollapsed))
    }
  }
</script>

<svelte:window onkeydown={onkey} />

{#if help}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="overlay" onclick={() => (help = false)}>
    <div class="box" role="dialog" tabindex="-1" aria-label="Keyboard shortcuts" onclick={(e) => e.stopPropagation()}>
      <h2>Keyboard shortcuts</h2>
      <div class="grid">
        {#each list as [keys, what] (what)}
          <span class="what">{what}</span><span class="keys">{keys}</span>
        {/each}
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay { position: fixed; inset: 0; background: rgba(0,0,0,.7); display: grid; place-items: center; z-index: 200; }
  .box { width: min(560px, 90vw); max-height: 80vh; overflow-y: auto; background: #282828; border-radius: 8px; padding: 24px 32px; }
  h2 { margin: 0 0 16px; font-size: 24px; }
  .grid { display: grid; grid-template-columns: 1fr auto; gap: 10px 24px; font-size: 14px; }
  .what { color: var(--text); }
  .keys { color: var(--text-2); text-align: right; font-variant-numeric: tabular-nums; }
</style>
