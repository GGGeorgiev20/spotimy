<script lang="ts">
  import Icon from './Icon.svelte'
  import Virtual from './Virtual.svelte'
  import { App, pick, saved, toast, toggleSaved, type Track } from '../lib/api.svelte'
  import { openMenu } from '../lib/menu.svelte'
  import type { Paged } from '../lib/paged.svelte'
  import { fmtAdded, fmtDuration, fmtNumber } from '../lib/format'
  import { player, playing, controls, isContext } from '../lib/player.svelte'
  import { navigate, uriToPath } from '../lib/router.svelte'

  let {
    tracks,
    scroller,
    contextUri,
    variant = 'playlist',
    onplayed,
    playlist,
  }: {
    tracks: Paged
    scroller: HTMLElement | undefined
    contextUri: string
    variant?: 'playlist' | 'album' | 'artist' | 'search'
    onplayed?: (t: Track) => void
    playlist?: { uri: string; canEdit: boolean } // enables remove and drag-reorder
  } = $props()

  // Drag-reorder (editable playlists only): drop above or below a row.
  let dragFrom = $state(-1)
  let dropAt = $state<{ i: number; after: boolean } | null>(null)
  const canDrag = $derived(!!playlist?.canEdit)

  function dragStart(e: DragEvent, i: number) {
    dragFrom = i
    e.dataTransfer!.effectAllowed = 'move'
    e.dataTransfer!.setData('text/plain', tracks.items[i]?.uri ?? '')
  }
  function dragOver(e: DragEvent, i: number) {
    if (dragFrom < 0) return
    e.preventDefault()
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    dropAt = { i, after: e.clientY > r.top + r.height / 2 }
  }
  async function drop(e: DragEvent) {
    e.preventDefault()
    const from = tracks.items[dragFrom]
    const to = dropAt && tracks.items[dropAt.i]
    const after = dropAt?.after
    dragFrom = -1
    dropAt = null
    if (!from?.uid || !to?.uid || from.uid === to.uid || !playlist) return
    try {
      await App.MovePlaylistItems(playlist.uri, [from.uid], after ? { after: to.uid } : { before: to.uid })
      window.dispatchEvent(new CustomEvent('playlist-changed', { detail: playlist.uri }))
    } catch (err) {
      toast(`Couldn't move: ${err}`)
    }
  }

  // Columns per variant, as in the official client.
  const showImage = $derived(variant !== 'album')
  const showAlbum = $derived(variant === 'playlist' || variant === 'search')
  const showAdded = $derived(variant === 'playlist')
  const showPlays = $derived(variant === 'artist')
  let selected = $state(-1)

  const current = $derived(player.status?.track?.uri)
  // An empty contextUri plays the track on its own (search results).
  const inContext = $derived(!contextUri || isContext(contextUri))

  function play(t: Track) {
    if (!t.playable) return
    onplayed?.(t)
    if (inContext && current === t.uri) controls.playPause()
    else if (contextUri) controls.play(contextUri, t.uri)
    else controls.play(t.uri)
  }

  function link(e: MouseEvent, uri: string) {
    e.preventDefault()
    e.stopPropagation()
    navigate(uriToPath(uri))
  }

  function key(e: KeyboardEvent, t: Track | undefined) {
    if (e.key === 'Enter' && t) play(t)
  }
</script>

<div class="table v-{variant}" role="grid" aria-rowcount={tracks.total}>
  {#if variant !== 'artist'}
    <div class="head" role="row">
      <div class="idx">#</div>
      <div>Title</div>
      {#if showAlbum}<div class="hide-narrow">Album</div>{/if}
      {#if showAdded}<div class="hide-narrower">Date added</div>{/if}
      <div class="dur"><Icon name="clock" /></div>
    </div>
  {/if}

  <Virtual count={tracks.total} rowHeight={56} {scroller} onrange={(s, e) => tracks.ensure(s, e + 20)}>
    {#snippet row(i)}
      {@const t = tracks.items[i]}
      {@const isCurrent = !!t && inContext && current === t.uri}
      <div
        class="row"
        class:selected={selected === i}
        class:current={isCurrent}
        class:disabled={t && !t.playable}
        class:drop-before={dropAt?.i === i && !dropAt.after}
        class:drop-after={dropAt?.i === i && dropAt.after}
        role="row"
        tabindex="-1"
        draggable={canDrag && !!t}
        ondragstart={(e) => dragStart(e, i)}
        ondragover={(e) => dragOver(e, i)}
        ondrop={drop}
        ondragend={() => ((dragFrom = -1), (dropAt = null))}
        oncontextmenu={(e) => t && ((selected = i), openMenu(e, { kind: 'track', track: t, playlist }))}
        onclick={() => (selected = i)}
        ondblclick={() => t && play(t)}
        onkeydown={(e) => key(e, t)}
      >
        {#if t}
          <div class="idx">
            {#if isCurrent && playing()}
              <span class="eq" aria-label="Playing"><i></i><i></i><i></i><i></i></span>
            {:else}
              <span class="num">{variant === 'album' ? t.trackNumber : i + 1}</span>
            {/if}
            <button class="play" onclick={(e) => (e.stopPropagation(), play(t))} aria-label="Play {t.name}">
              <Icon name={isCurrent && playing() ? 'pause' : 'play'} />
            </button>
          </div>
          <div class="title">
            {#if showImage}
              <img src={pick(t.album?.images, 40)} alt="" loading="lazy" />
            {/if}
            <div class="names">
              <div class="name ellipsis">{t.name}</div>
              <div class="artists ellipsis">
                {#if t.explicit}<span class="e" title="Explicit">E</span>{/if}
                {#each t.artists as a, k}{#if k > 0}, {/if}<a href="/" onclick={(e) => link(e, a.uri)}>{a.name}</a>{/each}
              </div>
            </div>
          </div>
          {#if showAlbum}
            <div class="ellipsis sub hide-narrow">
              {#if t.album}<a href="/" onclick={(e) => link(e, t.album!.uri)}>{t.album.name}</a>{/if}
            </div>
          {/if}
          {#if showAdded}<div class="sub hide-narrower">{fmtAdded(t.addedAt ?? '')}</div>{/if}
          {#if showPlays}<div class="sub plays">{t.playcount ? fmtNumber(t.playcount) : ''}</div>{/if}
          <div class="dur sub">
            <button
              class="save"
              class:on={saved[t.uri]}
              onclick={(e) => (e.stopPropagation(), toggleSaved(t.uri))}
              aria-label={saved[t.uri] ? 'Remove from Liked Songs' : 'Save to Liked Songs'}
            >
              <Icon name={saved[t.uri] ? 'check-circle' : 'plus-circle'} />
            </button>
            <span>{fmtDuration(t.durationMs)}</span>
            <button class="more" aria-label="More options for {t.name}" onclick={(e) => ((selected = i), openMenu(e, { kind: 'track', track: t, playlist }))}><Icon name="more" /></button>
          </div>
        {:else}
          <div class="idx"><span class="num">{i + 1}</span></div>
          <div class="title"><div class="skeleton"></div></div>
        {/if}
      </div>
    {/snippet}
  </Virtual>
</div>

<style>
  .table { padding: 0 24px; --cols: [index] 16px [first] 6fr [var1] 4fr [var2] 3fr [last] minmax(120px, 1fr); }
  .table.v-album { --cols: [index] 16px [first] 4fr [last] minmax(120px, 1fr); }
  .table.v-search { --cols: [index] 16px [first] 4fr [var1] 2fr [last] minmax(120px, 1fr); }
  .table.v-artist { --cols: [index] 16px [first] 4fr [var1] 2fr [last] minmax(120px, 1fr); }
  .plays { font-variant-numeric: tabular-nums; }
  .head, .row {
    display: grid;
    grid-template-columns: var(--cols);
    grid-gap: 16px;
    align-items: center;
    padding: 0 16px;
  }
  .head {
    height: 36px;
    color: var(--text-2);
    font-size: 14px;
    border-bottom: 1px solid hsla(0, 0%, 100%, .1);
    margin-bottom: 16px;
    position: sticky;
    top: 64px;
    z-index: 2;
    margin-left: -24px;
    margin-right: -24px;
    padding: 0 40px;
  }
  :global(.scroll[data-stuck]) .head { background: var(--surface-2); border-bottom-color: transparent; }
  .row.drop-before { box-shadow: inset 0 2px 0 var(--accent); }
  .row.drop-after { box-shadow: inset 0 -2px 0 var(--accent); }
  .row { height: 56px; border-radius: 4px; color: var(--text-2); }
  .row:hover { background: hsla(0, 0%, 100%, .1); }
  .row.selected { background: hsla(0, 0%, 100%, .3); }
  .row.disabled { opacity: .4; }
  .idx { text-align: right; position: relative; font-variant-numeric: tabular-nums; }
  .idx .play { display: none; position: absolute; right: -4px; top: 50%; transform: translateY(-50%); color: var(--text); }
  .row:hover .idx .num, .row:hover .idx .eq { visibility: hidden; }
  .row:hover .idx .play { display: block; }
  .dur { display: flex; justify-content: flex-end; align-items: center; gap: 16px; font-variant-numeric: tabular-nums; }
  .head .dur { padding-right: 32px; }
  .title { display: flex; align-items: center; gap: 12px; min-width: 0; }
  .title img { width: 40px; height: 40px; border-radius: 4px; flex: none; background: var(--surface-3); }
  .names { min-width: 0; }
  .name { color: var(--text); font-size: 16px; }
  .current .name, .current .num { color: var(--accent); }
  .artists { font-size: 14px; }
  .sub { font-size: 14px; }
  a:hover { color: var(--text); }
  .e {
    display: inline-grid; place-items: center;
    width: 16px; height: 16px; margin-right: 6px; border-radius: 2px;
    background: hsla(0, 0%, 100%, .6); color: #000; font-size: 9px; font-weight: 700; vertical-align: 1px;
  }
  .save { visibility: hidden; color: var(--text-2); display: grid; }
  .save:hover { color: var(--text); }
  .save.on { visibility: visible; color: var(--accent); }
  .row:hover .save { visibility: visible; }
  .more { visibility: hidden; color: var(--text); }
  .row:hover .more { visibility: visible; }
  .skeleton { width: 40%; height: 14px; border-radius: 7px; background: var(--surface-2); }
  .eq { display: inline-flex; gap: 2px; align-items: flex-end; height: 14px; }
  .eq i { width: 2px; background: var(--accent); animation: eq 0.9s ease-in-out infinite; }
  .eq i:nth-child(2) { animation-delay: -.3s; }
  .eq i:nth-child(3) { animation-delay: -.6s; }
  .eq i:nth-child(4) { animation-delay: -.45s; }
  @keyframes eq { 0%, 100% { height: 3px; } 50% { height: 14px; } }
  @container main (max-width: 900px) { .hide-narrower { display: none; } .table.v-playlist { --cols: [index] 16px [first] 6fr [var1] 4fr [last] minmax(120px, 1fr); } }
  @container main (max-width: 640px) { .hide-narrow { display: none; } .table.v-playlist, .table.v-search { --cols: [index] 16px [first] 6fr [last] minmax(120px, 1fr); } }
</style>
