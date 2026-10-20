<script lang="ts">
  // One row of cards with a title and "Show all", like the official shelves.
  // Cards that don't fit on the row are hidden rather than wrapped.
  import Card from './Card.svelte'
  import { pick } from '../lib/api.svelte'
  import { openUri } from '../lib/router.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let {
    title,
    items,
    showAll,
    oncard,
    label = '',
    header,
    bare = false,
  }: {
    title: string
    items: spotify.Card[]
    showAll?: () => void
    oncard?: (c: spotify.Card) => void
    label?: string
    header?: spotify.Card
    bare?: boolean
  } = $props()
</script>

<section class="shelf">
  <header>
    {#if header?.images?.length}
      <button class="hdr-img" onclick={() => openUri(header.uri)} aria-label={header.name}>
        <img class:round={header.type === 'artist'} src={pick(header.images, 64)} alt="" />
      </button>
    {/if}
    <div class="titles">
      {#if label}<p class="label">{label}</p>{/if}
      {#if header}
        <button class="title" onclick={() => openUri(header.uri)}>{title}</button>
      {:else if showAll}
        <button class="title" onclick={showAll}>{title}</button>
      {:else}
        <h2 class="title">{title}</h2>
      {/if}
    </div>
    {#if showAll}<button class="all" onclick={showAll}>Show all</button>{/if}
  </header>
  <div class="row" class:bare>
    {#each items as card (card.uri)}<Card {card} onopen={oncard} {bare} />{/each}
  </div>
</section>

<style>
  .shelf { padding: 0 12px; margin-bottom: 24px; }
  header { display: flex; align-items: flex-end; gap: 12px; padding: 0 12px; margin-bottom: 4px; }
  .titles { flex: 1; min-width: 0; }
  .label { margin: 0 0 2px; font-size: 12px; color: var(--text-2); }
  .hdr-img img { width: 48px; height: 48px; border-radius: 4px; object-fit: cover; display: block; }
  .hdr-img img.round { border-radius: 50%; }
  .title { font-size: 24px; font-weight: 700; margin: 0; color: var(--text); text-align: left; }
  button.title:hover { text-decoration: underline; }
  .all { font-size: 14px; font-weight: 700; color: var(--text-2); white-space: nowrap; padding-bottom: 6px; }
  .all:hover { text-decoration: underline; color: var(--text); }
  .row {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(176px, 1fr));
    grid-template-rows: auto;
    grid-auto-rows: 0;
    overflow: hidden;
  }
  .row.bare { grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); }
</style>
