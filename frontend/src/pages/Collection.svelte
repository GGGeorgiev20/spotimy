<script lang="ts">
  import EntityHeader from '../components/EntityHeader.svelte'
  import ActionBar from '../components/ActionBar.svelte'
  import TrackTable from '../components/TrackTable.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import { App, saved } from '../lib/api.svelte'
  import { Paged } from '../lib/paged.svelte'
  import { setChrome } from '../lib/chrome.svelte'
  import { me } from '../lib/me.svelte'
  import { ctxPlaying, playContext } from '../lib/player.svelte'
  import { fmtListLength } from '../lib/format'

  let { scroller }: { scroller: HTMLElement | undefined } = $props()

  const color = 'rgb(80, 56, 160)'
  const uri = $derived(me.user ? `spotify:user:${me.user.username}:collection` : '')
  let error = $state('')
  let tracks = $state<Paged | null>(null)

  const isPlaying = $derived(ctxPlaying(uri))
  const play = () => playContext(uri)

  $effect(() => {
    const t = new Paged((off, lim) => App.LikedSongs(off, lim))
    App.LikedSongs(0, 100)
      .then((p) => {
        t.seed(p)
        p.items.forEach((x) => (saved[x.uri] = true))
        tracks = t
      })
      .catch((e) => (error = String(e)))
  })
  $effect(() => setChrome({ title: 'Liked Songs', color, contextUri: uri, onplay: play }))
  const total = $derived(tracks ? fmtListLength(tracks.loaded, tracks.total) : '')
</script>

{#if error}
  <Unavailable what="Liked Songs" {error} />
{:else if tracks}
  <EntityHeader image="/img?u={encodeURIComponent('https://misc.scdn.co/liked-songs/liked-songs-300.png')}" type="Playlist" title="Liked Songs" {color}>
    {#snippet meta()}
      <span class="owner">{me.user?.name}</span><span class="dot">•</span><span class="sub">{tracks!.total} songs{total ? `, ${total}` : ''}</span>
    {/snippet}
  </EntityHeader>
  <ActionBar {isPlaying} onplay={play} contextUri={uri} />
  <TrackTable {tracks} {scroller} contextUri={uri} />
{/if}

<style>
  .owner { font-weight: 700; }
  .dot, .sub { color: hsla(0, 0%, 100%, .7); }
</style>
