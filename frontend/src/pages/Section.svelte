<script lang="ts">
  // "Show all" for a home shelf.
  import Card from '../components/Card.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import { App } from '../lib/api.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { id }: { id: string } = $props()
  let shelf = $state<spotify.Shelf | null>(null)
  let error = $state('')
  $effect(() => {
    App.HomeSection(`spotify:section:${id}`).then((s) => (shelf = s)).catch((e) => (error = String(e)))
  })
</script>

{#if error}
  <Unavailable what="This section" {error} />
{:else if shelf}
  <div class="page">
    <h1>{shelf.title}</h1>
    <div class="grid">{#each shelf.items as card (card.uri)}<Card {card} />{/each}</div>
  </div>
{/if}

<style>
  .page { padding: 64px 12px 24px; }
  h1 { margin: 0 12px 16px; font-size: 32px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(176px, 1fr)); }
</style>
