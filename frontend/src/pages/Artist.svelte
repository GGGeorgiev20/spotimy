<script lang="ts">
  import EntityHeader from '../components/EntityHeader.svelte'
  import ActionBar from '../components/ActionBar.svelte'
  import TrackTable from '../components/TrackTable.svelte'
  import Shelf from '../components/Shelf.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import Icon from '../components/Icon.svelte'
  import { App, coverColor, pick, saved } from '../lib/api.svelte'
  import { Paged } from '../lib/paged.svelte'
  import { setChrome } from '../lib/chrome.svelte'
  import { fmtNumber, headerColor } from '../lib/format'
  import { ctxPlaying, playContext } from '../lib/player.svelte'
  import { navigate } from '../lib/router.svelte'
  import { openMenu } from '../lib/menu.svelte'
  import { toggleLibrary } from '../lib/actions'
  import type { spotify } from '../../wailsjs/go/models'

  let { id, scroller }: { id: string; scroller: HTMLElement | undefined } = $props()
  const uri = $derived(`spotify:artist:${id}`)

  let ar = $state<spotify.ArtistPage | null>(null)
  let error = $state('')
  let color = $state('rgb(83, 83, 83)')
  let more = $state(false)
  let disc = $state<'popular' | 'albums' | 'singles' | 'compilations'>('popular')
  let top = $state<Paged | null>(null)

  const isPlaying = $derived(ctxPlaying(uri))
  const play = () => playContext(uri)

  $effect(() => {
    const u = uri
    ar = null
    error = ''
    more = false
    App.Artist(u)
      .then(async (a) => {
        ar = a
        saved[u] = a.following
        color = headerColor(await coverColor(a.header?.length ? a.header : a.avatar))
        setChrome({ title: a.name, color, contextUri: u, onplay: play, threshold: a.header?.length ? 360 : 280 })
      })
      .catch((e) => (error = String(e)))
  })

  $effect(() => {
    if (!ar) return
    const items = ar.topTracks.slice(0, more ? 10 : 5)
    const p = new Paged(async () => ({ total: items.length, items }))
    p.seed({ total: items.length, items })
    top = p
  })

  const discItems = $derived(
    !ar ? [] : disc === 'popular' ? ar.popular : disc === 'albums' ? ar.albums : disc === 'singles' ? ar.singles : ar.compilations,
  )
  const chips = $derived(
    !ar
      ? []
      : ([
          ['popular', 'Popular releases', ar.popular.length],
          ['albums', 'Albums', ar.totals[0]],
          ['singles', 'Singles and EPs', ar.totals[1]],
          ['compilations', 'Compilations', ar.totals[2]],
        ] as const).filter((c) => c[2] > 0),
  )
  const discKind = $derived(disc === 'popular' ? 'all' : disc)
</script>

{#if error}
  <Unavailable what="This artist" {error} />
{:else if ar}
  {#if ar.header?.length}
    <header class="banner" style:background-image="url({pick(ar.header, 1280)})" style:--c={color}>
      <div class="banner-info">
        {#if ar.verified}<div class="verified"><span class="badge"><Icon name="check" size={12} /></span>Verified Artist</div>{/if}
        <h1>{ar.name}</h1>
        <div class="listeners">{fmtNumber(ar.monthlyListeners)} monthly listeners</div>
      </div>
    </header>
    <div class="fade" style:--c={color}></div>
  {:else}
    <EntityHeader image={pick(ar.avatar, 300)} round type={ar.verified ? 'Verified Artist' : 'Artist'} title={ar.name} {color}>
      {#snippet meta()}<span class="sub">{fmtNumber(ar!.monthlyListeners)} monthly listeners</span>{/snippet}
    </EntityHeader>
  {/if}

  <ActionBar {isPlaying} onplay={play} onmore={(e) => openMenu(e, { kind: 'card', card: { uri, type: 'artist', name: ar!.name, subtitle: '', images: ar!.avatar } as spotify.Card })}>
    {#snippet extra()}
      <button class="follow" onclick={() => toggleLibrary(uri)}>{saved[uri] ? 'Following' : 'Follow'}</button>
    {/snippet}
  </ActionBar>

  {#if top}
    <section class="popular">
      <h2>Popular</h2>
      <TrackTable tracks={top} {scroller} contextUri={uri} variant="artist" />
      {#if ar.topTracks.length > 5}
        <button class="see" onclick={() => (more = !more)}>{more ? 'Show less' : 'See more'}</button>
      {/if}
    </section>
  {/if}

  {#if chips.length}
    <section class="disc">
      <header>
        <button class="h2" onclick={() => navigate(`/artist/${id}/discography/${discKind}`)}>Discography</button>
        <button class="all" onclick={() => navigate(`/artist/${id}/discography/${discKind}`)}>Show all</button>
      </header>
      <div class="chips">
        {#each chips as [k, label] (k)}
          <button class="chip" class:on={disc === k} onclick={() => (disc = k)}>{label}</button>
        {/each}
      </div>
      <Shelf title="" items={discItems} />
    </section>
  {/if}

  {#each ar.shelves as s (s.title)}
    <Shelf title={s.title} items={s.items} />
  {/each}

  {#if ar.bio}
    <section class="about">
      <h2>About</h2>
      <div class="about-card" style:background-image={ar.gallery?.length ? `url(${pick(ar.gallery, 640)})` : undefined}>
        <div class="about-text">
          <div class="big">{fmtNumber(ar.monthlyListeners)} monthly listeners</div>
          <p>{ar.bio}</p>
        </div>
      </div>
    </section>
  {/if}
{/if}

<style>
  .banner {
    height: 40cqw; max-height: 400px; min-height: 280px;
    background-size: cover; background-position: center 20%;
    position: relative; display: flex; align-items: flex-end;
  }
  .banner::before { content: ''; position: absolute; inset: 0; background: linear-gradient(transparent 30%, rgba(0,0,0,.5)); }
  .banner-info { position: relative; padding: 0 24px 24px; }
  .verified { display: flex; align-items: center; gap: 8px; font-size: 14px; }
  .badge { width: 20px; height: 20px; border-radius: 50%; background: #4cb3ff; display: grid; place-items: center; color: #fff; }
  .banner h1 { font-size: clamp(48px, 9cqw, 96px); font-weight: 900; letter-spacing: -0.04em; margin: 4px 0 8px; line-height: 1; }
  .listeners { font-size: 16px; }
  .fade { height: 232px; margin-bottom: -232px; background: linear-gradient(rgba(0,0,0,.6) 0, var(--surface) 100%), var(--c); }
  .sub { color: hsla(0,0%,100%,.7); }
  .follow { border: 1px solid #727272; border-radius: 999px; padding: 6px 16px; font-size: 14px; font-weight: 700; }
  .follow:hover { border-color: var(--text); transform: scale(1.04); }
  section { position: relative; }
  h2, .h2 { font-size: 24px; font-weight: 700; margin: 0 24px 16px; }
  .popular { margin-bottom: 40px; }
  .see { margin: 16px 0 0 40px; color: var(--text-2); font-weight: 700; font-size: 14px; }
  .see:hover { color: var(--text); }
  .disc header { display: flex; justify-content: space-between; align-items: baseline; padding-right: 24px; }
  .h2 { margin: 0 0 8px 24px; }
  .h2:hover { text-decoration: underline; }
  .all { font-size: 14px; font-weight: 700; color: var(--text-2); }
  .all:hover { text-decoration: underline; color: var(--text); }
  .chips { display: flex; gap: 8px; padding: 0 24px 8px; }
  .chip { background: hsla(0,0%,100%,.07); border-radius: 999px; padding: 4px 12px; font-size: 14px; }
  .chip.on { background: var(--text); color: #000; }
  .disc :global(.shelf header) { display: none; }
  .about { padding: 0 24px 24px; }
  .about h2 { margin: 0 0 16px; }
  .about-card { border-radius: 8px; background-color: #242424; background-size: cover; background-position: center; min-height: 360px; max-width: 760px; display: flex; align-items: flex-end; overflow: hidden; }
  .about-text { padding: 40px; background: linear-gradient(transparent, rgba(0,0,0,.8)); width: 100%; }
  .big { font-weight: 700; font-size: 16px; margin-bottom: 8px; }
  .about-text p { margin: 0; color: hsla(0,0%,100%,.85); display: -webkit-box; -webkit-line-clamp: 3; line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; }
</style>
