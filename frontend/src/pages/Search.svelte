<script lang="ts">
  import { untrack } from 'svelte'
  import Card from '../components/Card.svelte'
  import Shelf from '../components/Shelf.svelte'
  import TrackTable from '../components/TrackTable.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import Icon from '../components/Icon.svelte'
  import { App, pick, toast } from '../lib/api.svelte'
  import { Paged } from '../lib/paged.svelte'
  import { navigate, openUri } from '../lib/router.svelte'
  import { playContext } from '../lib/player.svelte'
  import { search } from '../lib/search.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { query, tab = 'all', scroller }: { query: string; tab?: string; scroller: HTMLElement | undefined } = $props()

  const tabs = [
    ['all', 'All'], ['tracks', 'Songs'], ['artists', 'Artists'], ['albums', 'Albums'], ['playlists', 'Playlists'],
    ['podcasts', 'Podcasts & Shows'], ['episodes', 'Episodes'], ['users', 'Profiles'],
  ]

  let all = $state<spotify.SearchResults | null>(null)
  let cards = $state<spotify.Card[]>([])
  let cardTotal = $state(0)
  let tracks = $state<Paged | null>(null)
  let topTracks = $state<Paged | null>(null)
  let recents = $state<spotify.Card[] | null>(null)
  let error = $state('')
  let loadingMore = false

  // Keep the top bar in sync when arriving via back/forward (only when the
  // route changes, never while the user is typing).
  $effect(() => {
    const q = query
    untrack(() => {
      if (search.q.trim() !== q) search.q = q
    })
  })

  $effect(() => {
    const q = query, t = tab
    error = ''
    if (!q) {
      App.RecentSearches().then((r) => (recents = r)).catch(() => (recents = []))
      return
    }
    if (t === 'all') {
      all = null
      App.Search(q)
        .then((r) => {
          const top4 = (r.tracks ?? []).slice(0, 4)
          topTracks = null
          if (top4.length) {
            const p = new Paged(async () => ({ total: top4.length, items: top4 }))
            p.seed({ total: top4.length, items: top4 })
            topTracks = p
          }
          all = r
        })
        .catch((e) => (error = String(e)))
    } else if (t === 'tracks') {
      const p = new Paged((off, lim) => App.SearchType(q, 'tracks', off, lim).then((r) => ({ total: r.total, items: r.tracks ?? [] })), 50)
      App.SearchType(q, 'tracks', 0, 50)
        .then((r) => (p.seed({ total: r.total, items: r.tracks ?? [] }), (tracks = p)))
        .catch((e) => (error = String(e)))
    } else {
      cards = []
      App.SearchType(q, t, 0, 50)
        .then((r) => ((cards = r.cards ?? []), (cardTotal = r.total)))
        .catch((e) => (error = String(e)))
    }
  })

  // Grid tabs load more as the bottom comes into view.
  $effect(() => {
    if (!scroller || tab === 'all' || tab === 'tracks') return
    const onScroll = async () => {
      if (loadingMore || cards.length >= cardTotal) return
      if (scroller!.scrollTop + scroller!.clientHeight < scroller!.scrollHeight - 600) return
      loadingMore = true
      try {
        const r = await App.SearchType(query, tab, cards.length, 50)
        cards = [...cards, ...(r.cards ?? [])]
      } finally {
        loadingMore = false
      }
    }
    scroller.addEventListener('scroll', onScroll, { passive: true })
    return () => scroller!.removeEventListener('scroll', onScroll)
  })

  const remember = (c: { uri: string }) => App.SaveRecentSearch(c.uri).catch(() => {})
  async function forget(uri: string) {
    try {
      await App.RemoveRecentSearches([uri])
      recents = recents?.filter((r) => r.uri !== uri) ?? null
    } catch (e) {
      toast(String(e))
    }
  }
  const tabPath = (t: string) => `/search/${encodeURIComponent(query)}${t === 'all' ? '' : '/' + t}`
</script>

{#if !query}
  <div class="page">
    {#if recents?.length}
      <h2>Recent searches</h2>
      <div class="grid">
        {#each recents as card (card.uri)}
          <div class="recent">
            <Card {card} />
            <button class="x" aria-label="Remove {card.name}" onclick={() => forget(card.uri)}><Icon name="close" size={12} /></button>
          </div>
        {/each}
      </div>
    {:else if recents}
      <p class="hint">Search for songs, artists, albums, playlists, podcasts and more.</p>
    {/if}
  </div>
{:else}
  <div class="tabs">
    {#each tabs as [id, label] (id)}
      <button class="chip" class:on={tab === id} onclick={() => navigate(tabPath(id))}>{label}</button>
    {/each}
  </div>
  {#if error}
    <Unavailable what="Search" {error} />
  {:else if tab === 'all' && all}
    {#if !all.top && !all.tracks?.length && !all.groups?.length}
      <p class="hint">No results found for "{query}"</p>
    {:else}
      <div class="top-row">
        {#if all.top}
          {@const top = all.top}
          <section class="top">
            <h2>Top result</h2>
            <div class="top-card" role="link" tabindex="0" onclick={() => (remember(top), openUri(top.uri))} onkeydown={(e) => e.key === 'Enter' && openUri(top.uri)}>
              <img class:round={top.type === 'artist'} src={pick(top.images, 120)} alt="" />
              <div class="top-name">{top.name}</div>
              <div class="top-sub"><span class="type">{top.type[0].toUpperCase() + top.type.slice(1)}</span>{top.subtitle && top.subtitle !== 'Artist' ? ' • ' + top.subtitle : ''}</div>
              <button class="play" aria-label="Play {top.name}" onclick={(e) => (e.stopPropagation(), remember(top), playContext(top.uri))}><Icon name="play" size={24} /></button>
            </div>
          </section>
        {/if}
        {#if topTracks}
          <section class="songs">
            <h2><button onclick={() => navigate(tabPath('tracks'))}>Songs</button></h2>
            <div class="compact"><TrackTable tracks={topTracks} {scroller} contextUri="" variant="artist" onplayed={remember} /></div>
          </section>
        {/if}
      </div>
      {#each all.groups as g (g.title)}
        <Shelf title={g.title} items={g.items} oncard={remember}
          showAll={() => navigate(tabPath({ Artists: 'artists', Albums: 'albums', Playlists: 'playlists', Podcasts: 'podcasts', Episodes: 'episodes', Profiles: 'users' }[g.title] ?? 'all'))} />
      {/each}
    {/if}
  {:else if tab === 'tracks' && tracks}
    <TrackTable {tracks} {scroller} contextUri="" variant="search" onplayed={remember} />
  {:else if tab !== 'all' && tab !== 'tracks'}
    <div class="grid pad">{#each cards as card (card.uri)}<Card {card} onopen={remember} />{/each}</div>
  {/if}
{/if}

<style>
  .page { padding: 72px 12px 24px; }
  h2 { font-size: 24px; margin: 0 12px 16px; }
  h2 button { font: inherit; }
  h2 button:hover { text-decoration: underline; }
  .hint { color: var(--text-2); text-align: center; padding: 64px 24px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(176px, 1fr)); }
  .grid.pad { padding: 0 12px; }
  .recent { position: relative; }
  .x { position: absolute; top: 18px; right: 18px; width: 24px; height: 24px; border-radius: 50%; background: rgba(0,0,0,.7); display: grid; place-items: center; color: var(--text-2); opacity: 0; }
  .recent:hover .x { opacity: 1; }
  .x:hover { color: var(--text); }
  .tabs { display: flex; gap: 8px; padding: 72px 24px 16px; flex-wrap: wrap; position: sticky; top: 0; z-index: 4; background: var(--surface); }
  .chip { background: hsla(0,0%,100%,.07); border-radius: 999px; padding: 4px 12px; font-size: 14px; white-space: nowrap; }
  .chip:hover { background: hsla(0,0%,100%,.1); }
  .chip.on { background: var(--text); color: #000; }
  .top-row { display: grid; grid-template-columns: minmax(280px, 2fr) 3fr; gap: 24px; padding: 0 24px 32px; }
  .top h2, .songs h2 { margin: 0 0 8px; }
  .top-card { position: relative; background: #181818; border-radius: 8px; padding: 20px; cursor: pointer; height: calc(100% - 44px); min-height: 220px; }
  .top-card:hover { background: #282828; }
  .top-card img { width: 92px; height: 92px; border-radius: 6px; object-fit: cover; box-shadow: 0 8px 24px rgba(0,0,0,.5); }
  .top-card img.round { border-radius: 50%; }
  .top-name { font-size: 32px; font-weight: 700; margin: 20px 0 6px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .top-sub { color: var(--text-2); font-size: 14px; }
  .type { color: var(--text); }
  .play { position: absolute; right: 20px; bottom: 20px; width: 48px; height: 48px; border-radius: 50%; background: var(--accent); color: #000; display: grid; place-items: center; opacity: 0; transform: translateY(8px); transition: all .3s; box-shadow: 0 8px 8px rgba(0,0,0,.3); }
  .top-card:hover .play { opacity: 1; transform: none; }
  .compact :global(.table) { padding: 0; }
  .compact :global(.head) { display: none; }
  @container main (max-width: 760px) { .top-row { grid-template-columns: 1fr; } }
</style>
