<script lang="ts">
  import EntityHeader from '../components/EntityHeader.svelte'
  import EpisodeRow from '../components/EpisodeRow.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import { App } from '../lib/api.svelte'
  import { setChrome } from '../lib/chrome.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  const color = 'rgb(0, 100, 80)'
  let eps = $state<spotify.Episode[] | null>(null)
  let error = $state('')
  $effect(() => {
    App.YourEpisodes().then((e) => (eps = e)).catch((e) => (error = String(e)))
  })
  $effect(() => setChrome({ title: 'Your Episodes', color }))
</script>

{#if error}
  <Unavailable what="Your Episodes" {error} />
{:else if eps}
  <EntityHeader image="" type="Playlist" title="Your Episodes" {color}>
    {#snippet meta()}<span>{eps!.length} episodes</span>{/snippet}
  </EntityHeader>
  <div class="list">
    {#if !eps.length}<p class="empty">Episodes you save will appear here.</p>{/if}
    {#each eps as ep (ep.uri)}<EpisodeRow {ep} />{/each}
  </div>
{/if}

<style>
  .list { padding: 24px 24px 0; position: relative; }
  .empty { color: var(--text-2); }
</style>
