<script lang="ts">
  import EntityHeader from '../components/EntityHeader.svelte'
  import ActionBar from '../components/ActionBar.svelte'
  import TrackTable from '../components/TrackTable.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import { App, coverColor, pick, saved, toggleSaved } from '../lib/api.svelte'
  import { Paged } from '../lib/paged.svelte'
  import { setChrome } from '../lib/chrome.svelte'
  import { fmtNumber, fmtTotal, headerColor } from '../lib/format'
  import { player, playing, controls } from '../lib/player.svelte'
  import { navigate } from '../lib/router.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { id, scroller }: { id: string; scroller: HTMLElement | undefined } = $props()
  const uri = $derived(`spotify:playlist:${id}`)

  let pl = $state<spotify.Playlist | null>(null)
  let error = $state('')
  let color = $state('rgb(83, 83, 83)')
  let tracks = $state<Paged | null>(null)

  const isPlaying = $derived(player.status?.context_uri === uri && playing())

  function play() {
    if (player.status?.context_uri === uri) controls.playPause()
    else controls.play(uri)
  }

  $effect(() => {
    const u = uri
    pl = null
    error = ''
    tracks = null
    App.Playlist(u, 0, 100)
      .then(async (p) => {
        pl = p
        saved[u] = p.following
        const t = new Paged(async (off, lim) => (await App.Playlist(u, off, lim)).tracks)
        t.seed(p.tracks)
        tracks = t
        color = headerColor(await coverColor(p.images))
        setChrome({ title: p.name, color, contextUri: u, onplay: play })
      })
      .catch((e) => (error = String(e)))
  })

  // Strip the HTML links Spotify puts in descriptions; show text only.
  function text(html: string) {
    return new DOMParser().parseFromString(html, 'text/html').body.textContent ?? ''
  }
  const total = $derived(tracks && tracks.loaded.length === tracks.total ? tracks.loaded.reduce((s, t) => s + t.durationMs, 0) : 0)
</script>

{#if error}
  <Unavailable what="This playlist" {error} />
{:else if pl && tracks}
  <EntityHeader image={pick(pl.images, 300)} type={pl.collaborative ? 'Collaborative playlist' : 'Playlist'} title={pl.name} {color}>
    {#snippet meta()}
      {#if pl!.description}<div class="desc">{text(pl!.description)}</div>{/if}
      <div class="line">
        {#if pl!.owner.images?.length}<img class="avatar" src={pick(pl!.owner.images, 24)} alt="" />{/if}
        <a href="/" class="owner" onclick={(e) => (e.preventDefault(), navigate(`/user/${pl!.owner.uri.split(':')[2]}`))}>{pl!.owner.name}</a>
        {#if pl!.followers > 0}<span class="dot">•</span><span class="sub">{fmtNumber(pl!.followers)} saves</span>{/if}
        <span class="dot">•</span><span class="sub">{tracks!.total} songs{total ? `, ${fmtTotal(total)}` : ''}</span>
      </div>
    {/snippet}
  </EntityHeader>
  <ActionBar {isPlaying} onplay={play} saved={saved[uri]} onsave={pl.isOwner ? undefined : () => toggleSaved(uri)} />
  <TrackTable {tracks} {scroller} contextUri={uri} />
{/if}

<style>
  .desc { width: 100%; color: hsla(0, 0%, 100%, .7); font-size: 14px; margin-bottom: 4px; }
  .line { display: flex; align-items: center; gap: 4px; flex-wrap: wrap; }
  .avatar { width: 24px; height: 24px; border-radius: 50%; margin-right: 4px; }
  .owner { font-weight: 700; }
  .dot, .sub { color: hsla(0, 0%, 100%, .7); }
</style>
