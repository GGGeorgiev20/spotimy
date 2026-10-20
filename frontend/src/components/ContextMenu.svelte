<script lang="ts">
  import Icon from './Icon.svelte'
  import { menu, closeMenu } from '../lib/menu.svelte'
  import { App, saved, toggleSaved, loadSaved, toast } from '../lib/api.svelte'
  import { openUri } from '../lib/router.svelte'
  import {
    addToQueue, queueContext, addToPlaylist, createPlaylist, editPlaylist, deletePlaylist,
    setPinned, copyLink, toggleLibrary,
  } from '../lib/actions'
  import type { spotify } from '../../wailsjs/go/models'

  type Item = { label: string; icon?: string; run?: () => unknown; sub?: 'playlists' | 'artists'; danger?: boolean }

  let el: HTMLDivElement | undefined = $state()
  let pos = $state({ x: 0, y: 0 })
  let sub = $state<'' | 'playlists' | 'artists'>('')
  let playlists = $state<spotify.EditablePlaylist[] | null>(null)
  let filter = $state('')

  const t = $derived(menu.target)
  const uris = $derived(t?.kind === 'track' || t?.kind === 'queue' ? [t.track.uri] : [])

  // Keep the menu inside the window.
  $effect(() => {
    if (!menu.open || !el) return
    const r = el.getBoundingClientRect()
    pos = {
      x: Math.min(menu.x, window.innerWidth - r.width - 8),
      y: Math.min(menu.y, window.innerHeight - r.height - 8),
    }
  })

  $effect(() => {
    if (menu.open) {
      sub = ''
      playlists = null
      filter = ''
      if (t?.kind === 'card') loadSaved([t.card.uri])
    }
  })

  async function openPlaylists() {
    sub = 'playlists'
    if (!playlists) {
      try {
        playlists = await App.EditablePlaylists(uris)
      } catch (e) {
        playlists = []
        toast(`Couldn't load playlists: ${e}`)
      }
    }
  }

  const items = $derived.by((): Item[] => {
    if (!t) return []
    if (t.kind === 'track' || t.kind === 'queue') {
      const tr = t.track
      const list: Item[] = [{ label: 'Add to playlist', icon: 'plus', sub: 'playlists' }]
      if (t.kind === 'track' && t.playlist?.canEdit && tr.uid)
        list.push({
          label: 'Remove from this playlist', icon: 'trash',
          run: async () => {
            try {
              await App.RemoveFromPlaylist(t.playlist!.uri, [tr.uid!])
              toast('Removed from playlist')
              window.dispatchEvent(new CustomEvent('playlist-changed', { detail: t.playlist!.uri }))
            } catch (e) {
              toast(`Couldn't remove: ${e}`)
            }
          },
        })
      list.push({
        label: saved[tr.uri] ? 'Remove from your Liked Songs' : 'Save to your Liked Songs',
        icon: saved[tr.uri] ? 'check-circle' : 'plus-circle',
        run: () => toggleSaved(tr.uri),
      })
      if (t.kind === 'track') list.push({ label: 'Add to queue', icon: 'queue', run: () => addToQueue([tr.uri]) })
      if (t.kind === 'queue')
        list.push({ label: 'Remove from queue', icon: 'close', run: () => window.dispatchEvent(new CustomEvent('queue-remove', { detail: t.index })) })
      if (tr.artists?.length === 1) list.push({ label: 'Go to artist', icon: 'artist', run: () => openUri(tr.artists[0].uri) })
      else if (tr.artists?.length) list.push({ label: 'Go to artist', icon: 'artist', sub: 'artists' })
      if (tr.album?.uri) list.push({ label: 'Go to album', icon: 'album', run: () => openUri(tr.album!.uri) })
      list.push({ label: 'Copy song link', icon: 'share', run: () => copyLink(tr.uri) })
      return list
    }
    const c = t.card
    const own = c.type === 'playlist' && !!t.own
    const list: Item[] = []
    if (c.type === 'album' || c.type === 'playlist') list.push({ label: 'Add to queue', icon: 'queue', run: () => queueContext(c.uri) })
    if (c.type === 'artist')
      list.push({ label: saved[c.uri] ? 'Unfollow' : 'Follow', icon: saved[c.uri] ? 'close' : 'user', run: () => toggleLibrary(c.uri) })
    else if (c.type !== 'collection' && c.type !== 'folder' && c.type !== 'user' && !(t.library && own))
      list.push({
        label: saved[c.uri] ? 'Remove from Your Library' : 'Add to Your Library',
        icon: saved[c.uri] ? 'check-circle' : 'plus-circle',
        run: () => toggleLibrary(c.uri),
      })
    if (t.library && c.type !== 'folder')
      list.push({ label: t.pinned ? 'Unpin' : 'Pin', icon: 'pin', run: () => setPinned(c.uri, !t.pinned) })
    if (c.type === 'playlist' && own) {
      list.push({ label: 'Edit details', icon: 'edit', run: () => editPlaylist(c.uri, c.name, t.description ?? '', t.onedit) })
      list.push({ label: 'Delete', icon: 'trash', danger: true, run: () => deletePlaylist(c.uri, c.name) })
    }
    if (c.type === 'collection') list.push({ label: 'Create playlist', icon: 'plus', run: () => createPlaylist() })
    if (c.type !== 'collection' && c.type !== 'folder') list.push({ label: 'Copy link', icon: 'share', run: () => copyLink(c.uri) })
    return list
  })

  function run(i: Item) {
    if (i.sub === 'playlists') return openPlaylists()
    if (i.sub) return (sub = i.sub)
    closeMenu()
    i.run?.()
  }

  const shown = $derived((playlists ?? []).filter((p) => p.name.toLowerCase().includes(filter.toLowerCase())))
</script>

<svelte:window onkeydown={(e) => menu.open && e.key === 'Escape' && closeMenu()} onblur={closeMenu} />

{#if menu.open && t}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="backdrop" onclick={closeMenu} oncontextmenu={(e) => (e.preventDefault(), closeMenu())}></div>
  <div class="menu" role="menu" bind:this={el} style:left="{pos.x}px" style:top="{pos.y}px">
    {#if sub === 'playlists'}
      <button class="mi back" onclick={() => (sub = '')}><Icon name="chevron-left" /> Add to playlist</button>
      <input class="find" placeholder="Find a playlist" bind:value={filter} />
      <button class="mi" onclick={() => (closeMenu(), createPlaylist(uris))}><Icon name="plus" /> New playlist</button>
      <hr />
      <div class="scroll">
        {#if !playlists}<div class="mi muted">Loading…</div>{/if}
        {#each shown as p (p.uri)}
          <button class="mi" onclick={() => (closeMenu(), addToPlaylist(p, uris))}>
            <span class="ellipsis">{p.name}</span>{#if p.contains}<span class="has"><Icon name="check" /></span>{/if}
          </button>
        {/each}
      </div>
    {:else if sub === 'artists' && (t.kind === 'track' || t.kind === 'queue')}
      <button class="mi back" onclick={() => (sub = '')}><Icon name="chevron-left" /> Go to artist</button>
      {#each t.track.artists as a (a.uri)}
        <button class="mi" onclick={() => (closeMenu(), openUri(a.uri))}>{a.name}</button>
      {/each}
    {:else}
      {#each items as i (i.label)}
        <button class="mi" class:danger={i.danger} onclick={() => run(i)}>
          {#if i.icon}<Icon name={i.icon} />{/if}<span class="label">{i.label}</span>
          {#if i.sub}<Icon name="chevron-right" />{/if}
        </button>
      {/each}
    {/if}
  </div>
{/if}

<style>
  .backdrop { position: fixed; inset: 0; z-index: 150; }
  .menu {
    position: fixed; z-index: 151; min-width: 220px; max-width: 360px;
    background: #282828; border-radius: 4px; padding: 4px;
    box-shadow: 0 16px 24px rgba(0,0,0,.3), 0 6px 8px rgba(0,0,0,.2);
  }
  .mi { width: 100%; display: flex; align-items: center; gap: 12px; padding: 12px 8px 12px 12px; border-radius: 2px; font-size: 14px; color: hsla(0,0%,100%,.9); text-align: left; }
  .mi:hover { background: hsla(0,0%,100%,.1); }
  .mi .label { flex: 1; }
  .mi.danger { color: #f3727f; }
  .mi.back { color: var(--text-2); font-size: 13px; }
  .mi.muted { color: var(--text-2); }
  .has { margin-left: auto; color: var(--accent); }
  .find { margin: 4px 8px 4px; width: calc(100% - 16px); background: #3e3e3e; border: 0; border-radius: 4px; padding: 8px 10px; font-size: 14px; outline: none; }
  .scroll { max-height: 320px; overflow-y: auto; }
  hr { border: 0; border-top: 1px solid hsla(0,0%,100%,.1); margin: 4px 0; }
</style>
