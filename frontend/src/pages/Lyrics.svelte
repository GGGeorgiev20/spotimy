<script lang="ts">
  import { untrack } from 'svelte'
  import { App, coverColor } from '../lib/api.svelte'
  import { lyricsColor } from '../lib/format'
  import { player, playing, position, controls } from '../lib/player.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { scroller }: { scroller: HTMLElement | undefined } = $props()

  const uri = $derived(player.status?.track?.uri ?? '')
  let lyrics = $state<spotify.Lyrics | null>(null)
  let loading = $state(true)
  let color = $state('rgb(83, 83, 83)')
  let lineNow = $state(0) // position used for highlighting, updated at line boundaries
  let userScrolledAt = 0

  $effect(() => {
    const u = uri
    lyrics = null
    loading = true
    if (!u) return
    App.Lyrics(u)
      .then((l) => u === uri && (lyrics = l))
      .catch(() => {})
      .finally(() => (loading = false))
    untrack(() => coverColor(player.status?.track?.images)).then((c) => (color = lyricsColor(c)))
  })

  const active = $derived.by(() => {
    if (!lyrics?.synced) return -1
    let i = -1
    for (let k = 0; k < lyrics.lines.length && lyrics.lines[k].startMs <= lineNow; k++) i = k
    return i
  })

  // Wake up exactly when the next line starts (plus the 1s global tick).
  $effect(() => {
    player.now
    player.fetchedAt
    lineNow = position()
    if (!lyrics?.synced || !playing()) return
    const nextLine = lyrics.lines.find((l) => l.startMs > lineNow)
    if (!nextLine) return
    const t = setTimeout(() => (lineNow = position()), Math.max(20, nextLine.startMs - lineNow))
    return () => clearTimeout(t)
  })

  // Keep the active line about a third of the way down, unless the user
  // scrolled in the last few seconds.
  $effect(() => {
    const i = active
    if (i < 0 || !scroller || Date.now() - userScrolledAt < 4000) return
    const el = scroller.querySelector<HTMLElement>(`[data-line="${i}"]`)
    if (!el) return
    const target = el.offsetTop - scroller.clientHeight / 3
    scroller.scrollTo({ top: target, behavior: 'smooth' })
  })
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="lyrics" style:--c={color} onwheel={() => (userScrolledAt = Date.now())}>
  {#if !uri}
    <p class="msg">Play something to see its lyrics.</p>
  {:else if loading}
    <p class="msg">Loading lyrics…</p>
  {:else if !lyrics?.lines?.length}
    <p class="msg">Lyrics aren't available for this song.</p>
  {:else}
    {#each lyrics.lines as line, i (i)}
      <button
        class="line"
        class:active={i === active}
        class:past={lyrics.synced && i < active}
        class:static={!lyrics.synced}
        data-line={i}
        disabled={!lyrics.synced}
        onclick={() => controls.seek(line.startMs)}
      >{line.words || '♪'}</button>
    {/each}
    {#if lyrics.provider}<p class="provider">Lyrics provided by {lyrics.provider}</p>{/if}
  {/if}
</div>

<style>
  .lyrics {
    min-height: 100%;
    padding: 96px max(32px, 8cqw) 50vh;
    background: var(--c);
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
  .msg { font-size: 24px; font-weight: 700; color: rgba(255,255,255,.8); }
  .line {
    font-size: clamp(24px, 3.2cqw, 32px);
    font-weight: 700;
    letter-spacing: -0.01em;
    text-align: left;
    color: rgba(0, 0, 0, .55);
    transition: color .2s;
    line-height: 1.3;
  }
  .line:hover:not(:disabled) { color: rgba(255,255,255,.9); text-decoration: underline; }
  .line.past { color: rgba(255,255,255,.7); }
  .line.active { color: #fff; }
  .line.static { color: #fff; cursor: default; }
  .provider { margin-top: 32px; font-size: 12px; color: rgba(255,255,255,.7); }
</style>
