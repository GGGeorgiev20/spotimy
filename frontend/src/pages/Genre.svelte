<script lang="ts">
  // A genre/mood page from "Browse all".
  import Shelf from '../components/Shelf.svelte'
  import CategoryTile from '../components/CategoryTile.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import { App } from '../lib/api.svelte'
  import { setChrome } from '../lib/chrome.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { id }: { id: string } = $props()
  let page = $state<spotify.BrowsePage | null>(null)
  let error = $state('')
  $effect(() => {
    App.Browse(`spotify:page:${id}`)
      .then((p) => {
        page = p
        setChrome({ title: p.title, color: p.color })
      })
      .catch((e) => (error = String(e)))
  })
</script>

{#if error}
  <Unavailable what="This page" {error} />
{:else if page}
  <div class="page" style:--c={page.color}>
    <h1>{page.title}</h1>
    {#each page.sections as s (s.uri)}<Shelf title={s.title} items={s.items} />{/each}
    {#if page.categories.length}
      <h2>Browse all</h2>
      <div class="grid">{#each page.categories as cat (cat.uri)}<CategoryTile {cat} />{/each}</div>
    {/if}
  </div>
{/if}

<style>
  .page { padding-bottom: 24px; background: linear-gradient(color-mix(in srgb, var(--c) 60%, transparent), var(--surface) 360px); }
  h1 { margin: 0; padding: 96px 24px 32px; font-size: clamp(48px, 8cqi, 96px); font-weight: 900; letter-spacing: -.04em; }
  h2 { margin: 16px 24px; font-size: 24px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 24px; padding: 0 24px; }
</style>
