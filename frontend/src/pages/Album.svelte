<script lang="ts">
  import EntityHeader from '../components/EntityHeader.svelte'
  import ActionBar from '../components/ActionBar.svelte'
  import TrackTable from '../components/TrackTable.svelte'
  import Card from '../components/Card.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import { App, coverColor, pick, saved } from '../lib/api.svelte'
  import { openMenu } from '../lib/menu.svelte'
  import { toggleLibrary } from '../lib/actions'
  import { Paged } from '../lib/paged.svelte'
  import { setChrome } from '../lib/chrome.svelte'
  import { fmtTotal, headerColor } from '../lib/format'
  import { ctxPlaying, playContext } from '../lib/player.svelte'
  import { openUri } from '../lib/router.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { id, scroller }: { id: string; scroller: HTMLElement | undefined } = $props()
  const uri = $derived(`spotify:album:${id}`)

  let al = $state<spotify.Album | null>(null)
  let error = $state('')
  let color = $state('rgb(83, 83, 83)')
  let tracks = $state<Paged | null>(null)

  const isPlaying = $derived(ctxPlaying(uri))
  const play = () => playContext(uri)

  $effect(() => {
    const u = uri
    al = null
    error = ''
    App.Album(u)
      .then(async (a) => {
        al = a
        saved[u] = a.saved
        const t = new Paged(async () => ({ total: a.total, items: a.tracks }))
        t.seed({ total: a.tracks.length, items: a.tracks })
        tracks = t
        color = headerColor(await coverColor(a.images))
        setChrome({ title: a.name, color, contextUri: u, onplay: play })
      })
      .catch((e) => (error = String(e)))
  })

  const typeLabel = $derived(al ? { ALBUM: 'Album', SINGLE: 'Single', EP: 'EP', COMPILATION: 'Compilation' }[al.type] ?? 'Album' : '')
  const total = $derived(al ? al.tracks.reduce((s, t) => s + t.durationMs, 0) : 0)
  const released = $derived(al?.date ? new Date(al.date).toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric' }) : '')
</script>

{#if error}
  <Unavailable what="This album" {error} />
{:else if al && tracks}
  <EntityHeader image={pick(al.images, 300)} type={typeLabel} title={al.name} {color}>
    {#snippet meta()}
      {#if al!.artists[0]?.images?.length}<img class="avatar" src={pick(al!.artists[0].images, 24)} alt="" />{/if}
      {#each al!.artists as a, i}
        {#if i > 0}<span class="dot">•</span>{/if}
        <a href="/" class="artist" onclick={(e) => (e.preventDefault(), openUri(a.uri))}>{a.name}</a>
      {/each}
      <span class="dot">•</span><span class="sub">{al!.date.slice(0, 4)}</span>
      <span class="dot">•</span><span class="sub">{al!.total} songs, {fmtTotal(total)}</span>
    {/snippet}
  </EntityHeader>
  <ActionBar {isPlaying} onplay={play} contextUri={uri} saved={saved[uri]} onsave={() => toggleLibrary(uri)}
    onmore={(e) => openMenu(e, { kind: 'card', card: { uri, type: 'album', name: al!.name, subtitle: '', images: al!.images } as spotify.Card })} />
  <TrackTable {tracks} {scroller} contextUri={uri} variant="album" />
  <div class="foot">
    <div class="date">{released}</div>
    {#each al.copyright as c}<div class="copy">{c}</div>{/each}
  </div>
  {#if al.more?.length}
    <section class="more">
      <h2>More by {al.artists[0]?.name}</h2>
      <div class="grid">
        {#each al.more.slice(0, 8) as card (card.uri)}<Card {card} />{/each}
      </div>
    </section>
  {/if}
{/if}

<style>
  .avatar { width: 24px; height: 24px; border-radius: 50%; margin-right: 4px; }
  .artist { font-weight: 700; }
  .dot, .sub { color: hsla(0, 0%, 100%, .7); }
  .foot { padding: 32px 24px 0; color: var(--text-2); font-size: 11px; }
  .date { font-size: 14px; margin-bottom: 4px; }
  .more { padding: 32px 12px 24px; }
  .more h2 { font-size: 24px; margin: 0 12px 8px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); grid-auto-rows: auto; }
</style>
