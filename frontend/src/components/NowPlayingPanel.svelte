<script lang="ts">
  import Icon from './Icon.svelte'
  import { App, pick, saved, loadSaved, toggleSaved } from '../lib/api.svelte'
  import { player } from '../lib/player.svelte'
  import { openUri } from '../lib/router.svelte'
  import { prefs } from '../lib/prefs.svelte'
  import { fmtNumber } from '../lib/format'
  import { toggleLibrary } from '../lib/actions'
  import type { main, spotify } from '../../wailsjs/go/models'

  let { onclose }: { onclose: () => void } = $props()

  const track = $derived(player.status?.track)
  // Effects key on these strings, not on `track`, which is a new object after
  // every status refresh.
  const uri = $derived(track?.uri ?? '')
  const artistUri = $derived(track?.artists?.[0]?.uri ?? '')
  const ctxUri = $derived(player.status?.contextUri ?? '')
  let info = $state<spotify.NowPlayingInfo | null>(null)
  let canvas = $state('')
  let next = $state<main.QueueTrack | null>(null)
  let contextName = $state('')
  let bioOpen = $state(false)

  // Per-track extras: canvas, artist card, credits.
  $effect(() => {
    const u = uri
    const artist = artistUri
    info = null
    canvas = ''
    bioOpen = false
    if (!u.startsWith('spotify:track:')) return
    loadSaved([u])
    App.Canvas(u).then((c) => u === uri && (canvas = c)).catch(() => {})
    App.NowPlaying(u, artist).then((i) => u === uri && (info = i)).catch(() => {})
    if (artist) loadSaved([artist])
  })

  // Next in queue (cheap: cluster state + one cached metadata lookup).
  $effect(() => {
    player.version
    App.Queue().then((q) => (next = q.next?.[0] ?? null)).catch(() => (next = null))
  })

  $effect(() => {
    const ctx = ctxUri
    contextName = ''
    const [, type] = ctx.split(':')
    if (type === 'playlist') App.Playlist(ctx, 0, 1).then((p) => (contextName = p.name)).catch(() => {})
    else if (type === 'album') App.Album(ctx).then((a) => (contextName = a.name)).catch(() => {})
    else if (type === 'artist') App.Artist(ctx).then((a) => (contextName = a.name)).catch(() => {})
    else if (ctx.endsWith(':collection')) contextName = 'Liked Songs'
  })
</script>

<aside class="npv" class:has-canvas={!!canvas}>
  <header>
    <span class="ctx ellipsis">{contextName || (track ? 'Now playing' : '')}</span>
    <button class="icon-btn" onclick={onclose} aria-label="Hide Now Playing view"><Icon name="close" /></button>
  </header>

  {#if !track}
    <p class="empty">Nothing is playing.</p>
  {:else}
    <div class="body">
      <div class="media">
        {#if canvas}
          <!-- Canvas: muted looping video in place of the cover. -->
          <video src={canvas} autoplay loop muted playsinline poster={pick(track.images, 640)}></video>
        {:else}
          <img src={pick(track.images, 640)} alt="" />
        {/if}
      </div>

      <div class="title-row">
        <div class="meta">
          <button class="title ellipsis" onclick={() => track.albumUri && openUri(track.albumUri)}>{track.name}</button>
          <div class="artists ellipsis">
            {#each track.artists ?? [] as a, i}{#if i > 0}{', '}{/if}<a href="/" onclick={(e) => (e.preventDefault(), openUri(a.uri))}>{a.name}</a>{/each}
          </div>
        </div>
        <button class="icon-btn save" class:on={saved[track.uri]} onclick={() => toggleSaved(track.uri)} aria-label="Save to Liked Songs">
          <Icon name={saved[track.uri] ? 'check-circle' : 'plus-circle'} size={20} />
        </button>
      </div>

      {#if info?.artist}
        {@const ar = info.artist}
        <section class="card artist">
          <div class="art-img" style:background-image={ar.gallery?.length || ar.images?.length ? `url(${pick(ar.gallery?.length ? ar.gallery : ar.images, 400)})` : undefined}>
            <span class="label">About the artist</span>
          </div>
          <div class="pad">
            <button class="name" onclick={() => openUri(ar.uri)}>{ar.name}</button>
            <div class="row">
              <span class="sub">{fmtNumber(ar.monthlyListeners)} monthly listeners</span>
              <button class="follow" onclick={() => toggleLibrary(ar.uri)}>{saved[ar.uri] ? 'Unfollow' : 'Follow'}</button>
            </div>
            {#if ar.bio}
              <button class="bio" class:open={bioOpen} onclick={() => (bioOpen = !bioOpen)}><span>{ar.bio}</span></button>
            {/if}
          </div>
        </section>
      {/if}

      {#if info?.credits?.length}
        <section class="card pad">
          <h3>Credits</h3>
          {#each info.credits.slice(0, 4) as c (c.name)}
            <div class="credit">
              {#if c.uri}<a href="/" onclick={(e) => (e.preventDefault(), openUri(c.uri))}>{c.name}</a>{:else}<span>{c.name}</span>{/if}
              <div class="sub">{c.roles}</div>
            </div>
          {/each}
        </section>
      {/if}

      {#if next}
        <section class="card pad">
          <div class="h-row">
            <h3>Next in queue</h3>
            <button class="link" onclick={() => (prefs.rightPanel = 'queue')}>Open queue</button>
          </div>
          <div class="next">
            <img src={pick(next.album?.images, 48)} alt="" />
            <div class="meta">
              <div class="ellipsis">{next.name}</div>
              <div class="sub ellipsis">{next.artists?.map((a) => a.name).join(', ')}</div>
            </div>
          </div>
        </section>
      {/if}
    </div>
  {/if}
</aside>

<style>
  .npv { height: 100%; display: flex; flex-direction: column; background: var(--surface); border-radius: var(--radius); overflow: hidden; }
  header { display: flex; justify-content: space-between; align-items: center; gap: 8px; padding: 16px 16px 8px; }
  .ctx { font-weight: 700; font-size: 16px; }
  .empty { color: var(--text-2); padding: 16px; }
  .body { flex: 1; overflow-y: auto; padding: 0 16px 16px; }
  .media { border-radius: 8px; overflow: hidden; background: var(--surface-3); aspect-ratio: 1; box-shadow: 0 8px 24px rgba(0,0,0,.5); }
  .has-canvas .media { aspect-ratio: 9 / 16; max-height: 60vh; margin: 0 auto; }
  .media img, .media video { width: 100%; height: 100%; object-fit: cover; display: block; }
  .title-row { display: flex; align-items: center; gap: 8px; margin: 16px 0 24px; }
  .meta { flex: 1; min-width: 0; }
  .title { display: block; max-width: 100%; font-size: 24px; font-weight: 700; text-align: left; }
  .title:hover { text-decoration: underline; }
  .artists { color: var(--text-2); font-size: 16px; margin-top: 4px; }
  .artists a:hover { color: var(--text); text-decoration: underline; }
  .save.on { color: var(--accent); }
  .card { background: var(--surface-2); border-radius: 8px; margin-bottom: 16px; overflow: hidden; }
  .pad { padding: 16px; }
  .art-img { height: 220px; background: #333 center / cover; position: relative; }
  .label { position: absolute; top: 16px; left: 16px; font-weight: 700; font-size: 16px; text-shadow: 0 1px 4px rgba(0,0,0,.6); }
  .name { font-size: 16px; font-weight: 700; }
  .name:hover { text-decoration: underline; }
  .row { display: flex; justify-content: space-between; align-items: center; gap: 8px; margin: 8px 0 12px; }
  .sub { color: var(--text-2); font-size: 14px; }
  .follow { border: 1px solid #727272; border-radius: 999px; padding: 4px 14px; font-size: 14px; font-weight: 700; flex: none; }
  .follow:hover { border-color: var(--text); transform: scale(1.04); }
  /* Clamp on an inner span: line-clamp doesn't apply to a <button> box. */
  .bio { display: block; width: 100%; text-align: left; color: var(--text-2); font-size: 14px; }
  .bio span { display: -webkit-box; -webkit-line-clamp: 3; line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; }
  .bio.open span { display: block; }
  h3 { font-size: 16px; margin: 0 0 12px; }
  .credit { margin-bottom: 12px; }
  .credit a:hover { text-decoration: underline; }
  .h-row { display: flex; justify-content: space-between; align-items: baseline; }
  .link { color: var(--text-2); font-size: 14px; font-weight: 700; }
  .link:hover { color: var(--text); text-decoration: underline; }
  .next { display: flex; gap: 12px; align-items: center; }
  .next img { width: 48px; height: 48px; border-radius: 4px; background: var(--surface-3); }
</style>
