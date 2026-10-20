<script lang="ts">
  import Icon from './Icon.svelte'
  import { pick } from '../lib/api.svelte'
  import { controls } from '../lib/player.svelte'
  import { openUri } from '../lib/router.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { card }: { card: spotify.Card } = $props()
  const round = $derived(card.type === 'artist' || card.type === 'user')
</script>

<div class="card" role="link" tabindex="0" onclick={() => openUri(card.uri)} onkeydown={(e) => e.key === 'Enter' && openUri(card.uri)}>
  <div class="art" class:round>
    {#if card.images?.length}<img src={pick(card.images, 200)} alt="" loading="lazy" />{/if}
    <button class="play" aria-label="Play {card.name}" onclick={(e) => (e.stopPropagation(), controls.play(card.uri))}>
      <Icon name="play" size={24} />
    </button>
  </div>
  <div class="name ellipsis">{card.name}</div>
  <div class="sub">{card.subtitle}</div>
</div>

<style>
  .card { padding: 12px; border-radius: 6px; cursor: pointer; min-width: 0; }
  .card:hover { background: var(--surface-2); }
  .art { position: relative; aspect-ratio: 1; border-radius: 6px; background: var(--surface-3); margin-bottom: 8px; box-shadow: 0 8px 24px rgba(0,0,0,.5); }
  .art.round { border-radius: 50%; }
  .art img { width: 100%; height: 100%; object-fit: cover; border-radius: inherit; display: block; }
  .play {
    position: absolute; right: 8px; bottom: 8px;
    width: 48px; height: 48px; border-radius: 50%;
    display: grid; place-items: center;
    background: var(--accent); color: #000;
    box-shadow: 0 8px 8px rgba(0,0,0,.3);
    opacity: 0; transform: translateY(8px);
    transition: all .3s ease;
  }
  .card:hover .play { opacity: 1; transform: none; }
  .play:hover { transform: scale(1.04) !important; background: var(--accent-hover); }
  .name { font-size: 16px; font-weight: 400; color: var(--text); }
  .sub { font-size: 14px; color: var(--text-2); margin-top: 4px; display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
</style>
