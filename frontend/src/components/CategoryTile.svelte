<script lang="ts">
  // A "Browse all" tile: colored card, title top-left, tilted artwork.
  import { pick } from '../lib/api.svelte'
  import { navigate } from '../lib/router.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { cat }: { cat: spotify.Category } = $props()
  const go = () => navigate(`/genre/${cat.uri.split(':')[2]}`)
</script>

<div class="tile" role="link" tabindex="0" style:background={cat.color} onclick={go} onkeydown={(e) => e.key === 'Enter' && go()}>
  <span class="name">{cat.name}</span>
  {#if cat.images?.length}<img src={pick(cat.images, 300)} alt="" loading="lazy" />{/if}
</div>

<style>
  .tile { position: relative; aspect-ratio: 16 / 9; border-radius: 8px; overflow: hidden; cursor: pointer; container-type: inline-size; }
  .name { position: absolute; top: 0; left: 0; padding: 16px; max-width: 100%; font-size: clamp(16px, 9cqi, 24px); font-weight: 700; overflow-wrap: break-word; }
  img {
    position: absolute; right: 0; bottom: 0; width: 45%; aspect-ratio: 1; border-radius: 4px;
    transform: rotate(25deg) translate(18%, -2%); box-shadow: 0 2px 4px rgba(0,0,0,.2);
  }
</style>
