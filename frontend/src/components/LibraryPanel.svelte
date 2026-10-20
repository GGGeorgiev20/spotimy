<script lang="ts">
  import Icon from './Icon.svelte'
  import Virtual from './Virtual.svelte'
  import { App, libraryVersion, pick } from '../lib/api.svelte'
  import { Paged } from '../lib/paged.svelte'
  import { player, playing } from '../lib/player.svelte'
  import { current, openUri, uriToPath } from '../lib/router.svelte'
  import { prefs } from '../lib/prefs.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { collapsed, ontoggle }: { collapsed: boolean; ontoggle: () => void } = $props()

  type Item = spotify.LibraryItem
  let filter = $state('')
  let text = $state('')
  let searching = $state(false)
  let folder = $state<Item | null>(null)
  let filters = $state<string[]>([])
  let sorts = $state<string[]>(['Recents', 'Recently Added', 'Alphabetical', 'Creator'])
  let list = $state<Paged<Item> | null>(null)
  let error = $state('')
  let menuOpen = $state(false)
  let scroller: HTMLDivElement | undefined = $state()
  let width = $state(0)

  const filterLabels: Record<string, string> = { 'Podcasts & Shows': 'Podcasts & Shows' }

  // Reload whenever a query input or the library itself changes.
  $effect(() => {
    const q = { filters: filter ? [filter] : [], sort: prefs.libSort, textFilter: text, offset: 0, limit: 50, folderUri: folder?.uri ?? '' }
    libraryVersion.n
    const p = new Paged<Item>((offset, limit) => App.Library({ ...q, offset, limit }), 50, () => {})
    const t = setTimeout(
      () =>
        App.Library(q)
          .then((lib) => {
            p.seed({ total: lib.total, items: lib.items })
            if (!filter && !text && !folder) filters = lib.filters
            if (lib.sorts?.length) sorts = lib.sorts
            list = p
            error = ''
          })
          .catch((e) => (error = String(e))),
      text ? 250 : 0,
    )
    return () => clearTimeout(t)
  })

  const ctx = $derived(player.status?.context_uri ?? '')
  const here = $derived(current().path)
  const views = [
    { id: 'compact', label: 'Compact', icon: 'compact' },
    { id: 'list', label: 'List', icon: 'list' },
    { id: 'grid', label: 'Grid', icon: 'grid' },
  ] as const
  const rowH = $derived(collapsed ? 64 : prefs.libView === 'compact' ? 32 : 64)
  const cols = $derived(!collapsed && prefs.libView === 'grid' ? Math.max(2, Math.floor((width - 16) / 160)) : 1)
  const rows = $derived(list ? Math.ceil(list.total / cols) : 0)

  function open(it: Item) {
    if (it.type === 'folder') {
      folder = it
      return
    }
    openUri(it.uri)
  }

  function isCtx(it: Item) {
    if (it.type === 'collection') return ctx.endsWith(':collection') && it.uri === 'spotify:collection:tracks'
    return ctx === it.uri
  }
  const sub = (it: Item) => it.subtitle
</script>

<nav class="lib" class:collapsed bind:clientWidth={width}>
  <header>
    <button class="title" onclick={ontoggle} title={collapsed ? 'Expand Your Library' : 'Collapse Your Library'}>
      <Icon name={collapsed ? 'library' : 'collapse'} size={collapsed ? 24 : 16} />
      {#if !collapsed}<span>Your Library</span>{/if}
    </button>
    {#if !collapsed}
      <button class="create" title="Create" disabled><Icon name="plus" /><span>Create</span></button>
    {/if}
  </header>

  {#if !collapsed}
    {#if folder}
      <div class="folder-bar">
        <button class="icon-btn" onclick={() => (folder = null)} aria-label="Go back"><Icon name="chevron-left" /></button>
        <span class="ellipsis">{folder.name}</span>
      </div>
    {:else if filters.length}
      <div class="chips">
        {#if filter}
          <button class="chip-x" onclick={() => (filter = '')} aria-label="Clear filters"><Icon name="close" /></button>
        {/if}
        {#each filters as f}
          {#if !filter || filter === f}
            <button class="chip" class:on={filter === f} onclick={() => (filter = filter === f ? '' : f)}>{filterLabels[f] ?? f}</button>
          {/if}
        {/each}
      </div>
    {/if}

    <div class="tools">
      <div class="search" class:open={searching || text}>
        <button class="icon-btn" onclick={() => (searching = true)} aria-label="Search in Your Library"><Icon name="search" /></button>
        {#if searching || text}
          <!-- svelte-ignore a11y_autofocus -->
          <input bind:value={text} placeholder="Search in Your Library" autofocus onblur={() => (searching = false)} />
        {/if}
      </div>
      <div class="sort">
        <button class="sort-btn" onclick={() => (menuOpen = !menuOpen)}>
          {prefs.libSort}
          <Icon name={views.find((v) => v.id === prefs.libView)?.icon ?? 'list'} />
        </button>
        {#if menuOpen}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="backdrop" onclick={() => (menuOpen = false)}></div>
          <div class="menu" role="menu">
            <div class="menu-label">Sort by</div>
            {#each sorts as s}
              <button class="menu-item" class:on={prefs.libSort === s} onclick={() => ((prefs.libSort = s), (menuOpen = false))}>
                {s}{#if prefs.libSort === s}<Icon name="check" />{/if}
              </button>
            {/each}
            <hr />
            <div class="menu-label">View as</div>
            {#each views as v}
              <button class="menu-item" class:on={prefs.libView === v.id} onclick={() => ((prefs.libView = v.id), (menuOpen = false))}>
                <span class="mi"><Icon name={v.icon} />{v.label}</span>{#if prefs.libView === v.id}<Icon name="check" />{/if}
              </button>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  {/if}

  <div class="items" bind:this={scroller}>
    {#if error}
      <p class="err" title={error}>Couldn't load Your Library.</p>
    {:else if list}
      <Virtual count={rows} rowHeight={cols > 1 ? Math.floor((width - 16) / cols) + 64 : rowH} {scroller} onrange={(s, e) => list!.ensure(s * cols, e * cols + 50)}>
        {#snippet row(r)}
          {#if cols > 1}
            <div class="grid-row" style:grid-template-columns="repeat({cols}, 1fr)">
              {#each { length: cols } as _, c}
                {@const it = list!.items[r * cols + c]}
                {#if it}
                  <button class="gcard" class:here={here === uriToPath(it.uri)} onclick={() => open(it)}>
                    <img class:round={it.type === 'artist'} src={pick(it.images, 160)} alt="" loading="lazy" />
                    <span class="name ellipsis" class:playing={isCtx(it)}>{it.name}</span>
                    <span class="sub ellipsis">{sub(it)}</span>
                  </button>
                {/if}
              {/each}
            </div>
          {:else}
            {@const it = list!.items[r]}
            {#if it}
              <button
                class="item"
                class:compact={prefs.libView === 'compact' && !collapsed}
                class:here={here === uriToPath(it.uri)}
                onclick={() => open(it)}
                title={collapsed ? `${it.name}\n${sub(it)}` : undefined}
              >
                {#if collapsed || prefs.libView !== 'compact'}
                  {#if it.type === 'folder'}
                    <span class="thumb folder"><Icon name="library" size={20} /></span>
                  {:else}
                    <img class="thumb" class:round={it.type === 'artist'} src={pick(it.images, 64)} alt="" loading="lazy" />
                  {/if}
                {/if}
                {#if !collapsed}
                  <span class="text">
                    <span class="name ellipsis" class:playing={isCtx(it)}>{it.name}</span>
                    {#if prefs.libView !== 'compact'}
                      <span class="sub ellipsis">
                        {#if it.pinned}<span class="pin"><Icon name="pin" size={12} /></span>{/if}{sub(it)}
                      </span>
                    {:else}
                      <span class="sub ellipsis"> • {it.subtitle.split(' • ')[0]}</span>
                    {/if}
                  </span>
                  {#if isCtx(it) && playing()}<span class="spk"><Icon name="volume" /></span>{/if}
                {/if}
              </button>
            {/if}
          {/if}
        {/snippet}
      </Virtual>
    {/if}
  </div>
</nav>

<style>
  .lib { height: 100%; display: flex; flex-direction: column; background: var(--surface); border-radius: var(--radius); overflow: hidden; }
  header { display: flex; align-items: center; justify-content: space-between; padding: 12px 16px 8px; min-height: 56px; }
  .collapsed header { justify-content: center; padding: 16px 8px 8px; }
  .title { display: flex; align-items: center; gap: 12px; color: var(--text-2); font-weight: 700; font-size: 16px; }
  .title:hover { color: var(--text); }
  .title span { color: var(--text); }
  .create { display: flex; align-items: center; gap: 6px; background: var(--surface-2); border-radius: 999px; padding: 8px 16px 8px 12px; font-weight: 700; }
  .create:hover:not(:disabled) { background: var(--surface-3); }
  .create:disabled { opacity: .6; cursor: default; }

  .chips { display: flex; gap: 8px; padding: 0 16px; margin: 4px 0 8px; overflow: hidden; }
  .chip { background: var(--surface-2); border-radius: 999px; padding: 4px 12px; font-size: 14px; white-space: nowrap; }
  .chip:hover { background: var(--surface-3); }
  .chip.on { background: var(--text); color: #000; }
  .chip-x { width: 32px; height: 32px; flex: none; border-radius: 50%; background: var(--surface-2); display: grid; place-items: center; }
  .folder-bar { display: flex; align-items: center; gap: 8px; padding: 0 12px 8px; font-weight: 700; }

  .tools { display: flex; align-items: center; justify-content: space-between; padding: 0 8px 0 12px; }
  .search { display: flex; align-items: center; border-radius: 4px; }
  .search.open { background: var(--surface-2); }
  .search input { background: none; border: 0; outline: 0; width: 160px; font-size: 13px; padding-right: 8px; }
  .sort { position: relative; }
  .sort-btn { display: flex; align-items: center; gap: 8px; color: var(--text-2); font-size: 14px; padding: 4px 8px; }
  .sort-btn:hover { color: var(--text); transform: scale(1.02); }
  .backdrop { position: fixed; inset: 0; z-index: 20; }
  .menu { position: absolute; right: 0; top: 32px; z-index: 21; min-width: 180px; background: var(--surface-3); border-radius: 4px; padding: 4px; box-shadow: 0 16px 24px rgba(0,0,0,.3), 0 6px 8px rgba(0,0,0,.2); }
  .menu-label { font-size: 11px; font-weight: 700; color: var(--text-2); padding: 12px 12px 4px; }
  .menu-item { width: 100%; display: flex; justify-content: space-between; align-items: center; padding: 12px; font-size: 14px; border-radius: 2px; color: hsla(0,0%,100%,.9); text-align: left; }
  .menu-item:hover { background: hsla(0,0%,100%,.1); }
  .menu-item.on { color: var(--accent); }
  .mi { display: flex; align-items: center; gap: 12px; }
  hr { border: 0; border-top: 1px solid hsla(0,0%,100%,.1); margin: 4px 0; }

  .items { flex: 1; overflow-y: auto; padding: 8px 8px 8px; }
  .collapsed .items { padding: 8px 4px; scrollbar-width: none; }
  .item { width: 100%; height: 100%; display: flex; align-items: center; gap: 12px; padding: 8px; border-radius: 6px; text-align: left; }
  .item.compact { padding: 0 8px; }
  .collapsed .item { justify-content: center; }
  .item:hover { background: var(--hover); }
  .item.here { background: var(--surface-2); }
  .item.here:hover { background: var(--surface-3); }
  .thumb { width: 48px; height: 48px; border-radius: 4px; flex: none; object-fit: cover; background: var(--surface-3); }
  .thumb.round { border-radius: 50%; }
  .thumb.folder { display: grid; place-items: center; color: var(--text-2); }
  .text { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .compact .text { flex-direction: row; align-items: baseline; gap: 4px; }
  .name { font-size: 16px; color: var(--text); }
  .compact .name { font-size: 14px; flex: none; max-width: 70%; }
  .name.playing { color: var(--accent); }
  .sub { font-size: 14px; color: var(--text-2); }
  .pin { color: var(--accent); display: inline-block; margin-right: 6px; vertical-align: -1px; }
  .spk { color: var(--accent); flex: none; }
  .err { color: var(--text-2); padding: 16px; font-size: 14px; }

  .grid-row { display: grid; height: 100%; }
  .gcard { display: flex; flex-direction: column; gap: 4px; padding: 8px; border-radius: 6px; text-align: left; min-width: 0; }
  .gcard:hover, .gcard.here { background: var(--hover); }
  .gcard img { width: 100%; aspect-ratio: 1; border-radius: 6px; object-fit: cover; background: var(--surface-3); margin-bottom: 4px; }
  .gcard img.round { border-radius: 50%; }
</style>
