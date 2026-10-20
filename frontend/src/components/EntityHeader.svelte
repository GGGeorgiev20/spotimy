<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    image,
    type,
    title,
    color,
    round = false,
    meta,
    onedit,
  }: { image: string; type: string; title: string; color: string; round?: boolean; meta?: Snippet; onedit?: () => void } = $props()

  // Long titles get a smaller ceiling, as in the official client.
  const size = $derived(title.length > 40 ? 'small' : title.length > 18 ? 'medium' : 'large')
</script>

<header class="hdr" style:--c={color}>
  <div class="art" class:round>
    {#if image}<img src={image} alt="" />{/if}
  </div>
  <div class="info">
    <span class="type">{type}</span>
    {#if onedit}
      <button class="title-btn" onclick={onedit} title="Edit details"><h1 class={size}>{title}</h1></button>
    {:else}
      <h1 class={size}>{title}</h1>
    {/if}
    <div class="meta">{@render meta?.()}</div>
  </div>
</header>
<div class="fade" style:--c={color}></div>

<style>
  .hdr {
    display: flex;
    align-items: flex-end;
    gap: 24px;
    padding: 32px 24px 24px;
    background: linear-gradient(transparent 0, rgba(0, 0, 0, .5) 100%), var(--c);
  }
  .art {
    width: clamp(128px, 20cqw, 232px);
    aspect-ratio: 1;
    flex: none;
    border-radius: 4px;
    box-shadow: 0 4px 60px rgba(0, 0, 0, .5);
    background: var(--surface-3);
    overflow: hidden;
  }
  .art.round { border-radius: 50%; }
  .art img { width: 100%; height: 100%; object-fit: cover; display: block; }
  .info { min-width: 0; display: flex; flex-direction: column; gap: 8px; }
  .type { font-size: 14px; font-weight: 500; }
  h1 {
    margin: 0;
    font-weight: 900;
    letter-spacing: -0.04em;
    line-height: 1;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 3; line-clamp: 3;
    -webkit-box-orient: vertical;
    padding-bottom: 0.08em;
  }
  h1.large { font-size: clamp(32px, 8cqw, 96px); }
  h1.medium { font-size: clamp(32px, 6cqw, 72px); }
  h1.small { font-size: clamp(24px, 4cqw, 48px); }
  .title-btn { text-align: left; padding: 0; }
  .meta { font-size: 14px; display: flex; flex-wrap: wrap; align-items: center; gap: 4px; margin-top: 4px; }
  .fade {
    height: 232px;
    margin-bottom: -232px;
    background: linear-gradient(rgba(0, 0, 0, .6) 0, var(--surface) 100%), var(--c);
  }
</style>
