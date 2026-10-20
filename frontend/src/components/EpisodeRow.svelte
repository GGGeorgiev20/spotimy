<script lang="ts">
  import Icon from './Icon.svelte'
  import { pick } from '../lib/api.svelte'
  import { fmtAdded } from '../lib/format'
  import { controls, player, playing } from '../lib/player.svelte'
  import { navigate } from '../lib/router.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { ep, contextUri = '' }: { ep: spotify.Episode; contextUri?: string } = $props()
  const id = $derived(ep.uri.split(':')[2])
  const current = $derived(player.status?.track?.uri === ep.uri)

  function play() {
    if (current) return controls.playPause()
    // go-librespot resumes episodes from their saved position.
    return contextUri ? controls.play(contextUri, ep.uri) : controls.play(ep.uri)
  }
  const mins = (ms: number) => Math.max(1, Math.round(ms / 60000))
  const left = $derived(ep.state === 'IN_PROGRESS' ? ep.durationMs - ep.playedMs : 0)
</script>

<div class="ep" role="link" tabindex="0" onclick={() => navigate(`/episode/${id}`)} onkeydown={(e) => e.key === 'Enter' && navigate(`/episode/${id}`)}>
  <img src={pick(ep.images, 112)} alt="" loading="lazy" />
  <div class="body">
    <div class="name" class:current>{ep.name}</div>
    {#if ep.showName}<div class="show">{ep.showName}</div>{/if}
    <p class="desc">{ep.description}</p>
    <div class="foot">
      <button class="play" onclick={(e) => (e.stopPropagation(), play())} aria-label="Play {ep.name}">
        <Icon name={current && playing() ? 'pause' : 'play'} />
      </button>
      {#if ep.explicit}<span class="e">E</span>{/if}
      <span class="sub">{fmtAdded(ep.releaseDate)}</span>
      <span class="sub">•</span>
      {#if ep.state === 'COMPLETED'}
        <span class="sub played"><Icon name="check-circle" size={14} /> Played</span>
      {:else if left}
        <span class="sub">{mins(left)} min left</span>
        <span class="bar"><span style:width="{(ep.playedMs / ep.durationMs) * 100}%"></span></span>
      {:else}
        <span class="sub">{mins(ep.durationMs)} min</span>
      {/if}
    </div>
  </div>
</div>

<style>
  .ep { display: flex; gap: 20px; padding: 16px; border-radius: 8px; cursor: pointer; border-top: 1px solid hsla(0,0%,100%,.07); }
  .ep:hover { background: hsla(0,0%,100%,.07); }
  img { width: 112px; height: 112px; border-radius: 8px; object-fit: cover; flex: none; background: var(--surface-3); }
  .body { min-width: 0; flex: 1; display: flex; flex-direction: column; gap: 6px; }
  .name { font-size: 16px; font-weight: 700; }
  .name.current { color: var(--accent); }
  .show { font-size: 14px; color: var(--text); }
  .desc { overflow-wrap: anywhere; margin: 0; font-size: 14px; color: var(--text-2); display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .foot { display: flex; align-items: center; gap: 8px; margin-top: 4px; }
  .play { width: 32px; height: 32px; border-radius: 50%; background: var(--text); color: #000; display: grid; place-items: center; }
  .play:hover { transform: scale(1.06); }
  .sub { font-size: 14px; color: var(--text-2); display: inline-flex; align-items: center; gap: 4px; }
  .played { color: var(--accent); }
  .bar { width: 100px; height: 4px; border-radius: 2px; background: hsla(0,0%,100%,.3); overflow: hidden; }
  .bar span { display: block; height: 100%; background: var(--text); }
  .e { display: inline-grid; place-items: center; width: 16px; height: 16px; border-radius: 2px; background: hsla(0,0%,100%,.6); color: #000; font-size: 9px; font-weight: 700; }
</style>
