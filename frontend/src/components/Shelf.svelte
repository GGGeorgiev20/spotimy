<script lang="ts">
  // One row of cards with a title and "Show all", like the official shelves.
  // Cards that don't fit on the row are hidden rather than wrapped.
  import Card from './Card.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let {
    title,
    items,
    showAll,
    oncard,
  }: { title: string; items: spotify.Card[]; showAll?: () => void; oncard?: (c: spotify.Card) => void } = $props()
</script>

<section class="shelf">
  <header>
    {#if showAll}
      <button class="title" onclick={showAll}>{title}</button>
      <button class="all" onclick={showAll}>Show all</button>
    {:else}
      <h2 class="title">{title}</h2>
    {/if}
  </header>
  <div class="row">
    {#each items as card (card.uri)}<Card {card} onopen={oncard} />{/each}
  </div>
</section>

<style>
  .shelf { padding: 0 12px; margin-bottom: 24px; }
  header { display: flex; align-items: baseline; justify-content: space-between; padding: 0 12px; margin-bottom: 4px; }
  .title { font-size: 24px; font-weight: 700; margin: 0; color: var(--text); text-align: left; }
  button.title:hover { text-decoration: underline; }
  .all { font-size: 14px; font-weight: 700; color: var(--text-2); white-space: nowrap; }
  .all:hover { text-decoration: underline; color: var(--text); }
  .row {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(176px, 1fr));
    grid-template-rows: auto;
    grid-auto-rows: 0;
    overflow: hidden;
  }
</style>
