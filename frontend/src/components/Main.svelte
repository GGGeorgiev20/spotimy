<script lang="ts">
  import Icon from './Icon.svelte'
  import Playlist from '../pages/Playlist.svelte'
  import Album from '../pages/Album.svelte'
  import Collection from '../pages/Collection.svelte'
  import Home from '../pages/Home.svelte'
  import Placeholder from '../pages/Placeholder.svelte'
  import { current, saveScroll } from '../lib/router.svelte'
  import { chrome, setChrome } from '../lib/chrome.svelte'
  import { player, playing } from '../lib/player.svelte'

  let scroller: HTMLDivElement | undefined = $state()
  let scrollTop = $state(0)

  const entry = $derived(current())
  const parts = $derived(entry.path.split('/').filter(Boolean))
  const page = $derived(parts[0] ?? '')
  const id = $derived(parts[1] ?? '')

  // Reset chrome before the new page's own effects set it, and restore the
  // entry's scroll position on navigation.
  $effect.pre(() => {
    const e = entry
    setChrome({})
    requestAnimationFrame(() => scroller && (scroller.scrollTop = e.scroll))
  })

  const opacity = $derived(Math.max(0, Math.min(1, (scrollTop - chrome.threshold + 64) / 64)))
  const ctxPlaying = $derived(!!chrome.contextUri && player.status?.context_uri === chrome.contextUri && playing())

  function onscroll() {
    scrollTop = scroller!.scrollTop
    saveScroll(scrollTop)
  }
</script>

<main class="main">
  <div class="sticky" style:--c={chrome.color} style:--o={opacity}>
    {#if chrome.onplay}
      <div class="sticky-content" style:opacity={opacity}>
        <button class="play" onclick={() => chrome.onplay?.()} aria-label={ctxPlaying ? 'Pause' : 'Play'} tabindex={opacity < 1 ? -1 : 0}>
          <Icon name={ctxPlaying ? 'pause' : 'play'} size={20} />
        </button>
        <span class="t ellipsis">{chrome.title}</span>
      </div>
    {/if}
  </div>
  <div class="scroll" bind:this={scroller} {onscroll} data-stuck={opacity >= 1 || undefined}>
    {#key entry}
      {#if page === ''}
        <Home />
      {:else if page === 'playlist'}
        <Playlist {id} {scroller} />
      {:else if page === 'album'}
        <Album {id} {scroller} />
      {:else if page === 'collection' && id === 'tracks'}
        <Collection {scroller} />
      {:else}
        <Placeholder path={entry.path} />
      {/if}
    {/key}
  </div>
</main>

<style>
  .main { position: relative; height: 100%; background: var(--surface); border-radius: var(--radius); overflow: hidden; container: main / inline-size; }
  .scroll { height: 100%; overflow-y: auto; overflow-x: hidden; scroll-behavior: auto; }
  .scroll :global(> :last-child) { padding-bottom: 32px; }
  .sticky {
    position: absolute; top: 0; left: 0; right: 12px; height: 64px; z-index: 5;
    pointer-events: none;
  }
  .sticky::before {
    content: ''; position: absolute; inset: 0;
    background: linear-gradient(rgba(0,0,0,.5), rgba(0,0,0,.5)), var(--c);
    opacity: var(--o);
  }
  .sticky-content { position: relative; height: 100%; display: flex; align-items: center; gap: 8px; padding: 0 24px; pointer-events: auto; }
  .play { width: 48px; height: 48px; border-radius: 50%; background: var(--accent); color: #000; display: grid; place-items: center; }
  .play:hover { transform: scale(1.04); background: var(--accent-hover); }
  .t { font-size: 24px; font-weight: 700; }
</style>
