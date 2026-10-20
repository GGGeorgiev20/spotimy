<script lang="ts">
  import { untrack } from 'svelte'
  import EntityHeader from '../components/EntityHeader.svelte'
  import ActionBar from '../components/ActionBar.svelte'
  import TrackTable from '../components/TrackTable.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import { App, coverColor, pick, saved } from '../lib/api.svelte'
  import { openMenu } from '../lib/menu.svelte'
  import { editPlaylist, toggleLibrary } from '../lib/actions'
  import { Paged } from '../lib/paged.svelte'
  import { setChrome } from '../lib/chrome.svelte'
  import { fmtNumber, fmtTotal, headerColor } from '../lib/format'
  import { ctxPlaying, playContext } from '../lib/player.svelte'
  import { navigate } from '../lib/router.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { id, scroller }: { id: string; scroller: HTMLElement | undefined } = $props()
  const uri = $derived(`spotify:playlist:${id}`)

  let pl = $state<spotify.Playlist | null>(null)
  let error = $state('')
  let color = $state('rgb(83, 83, 83)')
  let tracks = $state<Paged | null>(null)

  const isPlaying = $derived(ctxPlaying(uri))
  const play = () => playContext(uri)

  // Bumped after edits (remove, reorder, rename) to refetch.
  let version = $state(0)
  $effect(() => {
    const on = (e: Event) => (e as CustomEvent).detail === uri && version++
    window.addEventListener('playlist-changed', on)
    return () => window.removeEventListener('playlist-changed', on)
  })

  $effect(() => {
    const u = uri
    version
    // Only blank the page when switching playlists; a reload after an edit
    // keeps the old content until the new one arrives.
    untrack(() => {
      if (pl?.uri !== u) (pl = null), (tracks = null)
      error = ''
    })
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

  function more(e: MouseEvent) {
    const p = pl!
    openMenu(e, {
      kind: 'card',
      card: { uri: p.uri, type: 'playlist', name: p.name, subtitle: '', images: p.images } as spotify.Card,
      own: p.isOwner, description: text(p.description), onedit: () => version++,
    })
  }
  function edit() {
    if (pl?.isOwner) editPlaylist(pl.uri, pl.name, text(pl.description), () => version++)
  }

  // Strip the HTML links Spotify puts in descriptions; show text only.
  function text(html: string) {
    return new DOMParser().parseFromString(html, 'text/html').body.textContent ?? ''
  }
  // Exact once every row is loaded; until then an estimate from the loaded
  // rows, which is what the official client's "about" means too.
  const total = $derived.by(() => {
    const loaded = tracks?.loaded.filter(Boolean) ?? []
    if (!loaded.length) return ''
    const ms = loaded.reduce((s, t) => s + t.durationMs, 0)
    if (loaded.length === tracks!.total) return fmtTotal(ms)
    const est = fmtTotal((ms / loaded.length) * tracks!.total)
    return est.startsWith('about') ? est : `about ${est}`
  })
</script>

{#if error}
  <Unavailable what="This playlist" {error} />
{:else if pl && tracks}
  <EntityHeader image={pick(pl.images, 640)} type="Playlist" title={pl.name} {color} onedit={pl.isOwner ? edit : undefined}>
    {#snippet meta()}
      {#if pl!.description}<div class="desc">{text(pl!.description)}</div>{/if}
      <div class="line">
        {#if pl!.owner.images?.length}<img class="avatar" src={pick(pl!.owner.images, 24)} alt="" />{/if}
        <a href="/" class="owner" onclick={(e) => (e.preventDefault(), navigate(`/user/${pl!.owner.uri.split(':')[2]}`))}>{pl!.owner.name}</a>
        {#if pl!.members?.length === 1}<span class="sub">and</span><a href="/" class="owner" onclick={(e) => (e.preventDefault(), navigate(`/user/${pl!.members[0].uri.split(':')[2]}`))}>{pl!.members[0].name}</a>
        {:else if pl!.members?.length}<span class="sub">and {pl!.members.length} others</span>{/if}
        {#if pl!.followers > 0}<span class="dot">•</span><span class="sub">{fmtNumber(pl!.followers)} saves</span>{/if}
        <span class="dot">•</span><span class="sub">{tracks!.total} songs{total ? `, ${total}` : ''}</span>
      </div>
    {/snippet}
  </EntityHeader>
  <ActionBar {isPlaying} onplay={play} contextUri={uri} saved={saved[uri]} onsave={pl.isOwner ? undefined : () => toggleLibrary(uri)} onmore={more} />
  <TrackTable {tracks} {scroller} contextUri={uri} playlist={{ uri, canEdit: pl.canEdit }} />
{/if}

<style>
  .desc { width: 100%; color: hsla(0, 0%, 100%, .7); font-size: 14px; margin-bottom: 4px; }
  .line { display: flex; align-items: center; gap: 4px; flex-wrap: wrap; }
  .avatar { width: 24px; height: 24px; border-radius: 50%; margin-right: 4px; }
  .owner { font-weight: 700; }
  .dot, .sub { color: hsla(0, 0%, 100%, .7); }
</style>
