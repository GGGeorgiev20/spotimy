<script lang="ts">
  import { onMount } from 'svelte'
  import { GetState } from '../wailsjs/go/main/App'
  import { EventsOn } from '../wailsjs/runtime/runtime'
  import type { main } from '../wailsjs/go/models'
  import Login from './components/Login.svelte'
  import TopBar from './components/TopBar.svelte'
  import LibraryPanel from './components/LibraryPanel.svelte'
  import Main from './components/Main.svelte'
  import PlayerBar from './components/PlayerBar.svelte'
  import ContextMenu from './components/ContextMenu.svelte'
  import Dialog from './components/Dialog.svelte'
  import QueuePanel from './components/QueuePanel.svelte'
  import FriendsPanel from './components/FriendsPanel.svelte'
  import NowPlayingPanel from './components/NowPlayingPanel.svelte'
  import Shortcuts from './components/Shortcuts.svelte'
  import Boundary from './components/Boundary.svelte'
  import { initPlayer, refresh } from './lib/player.svelte'
  import { loadMe } from './lib/me.svelte'
  import { prefs } from './lib/prefs.svelte'
  import { toasts } from './lib/api.svelte'

  let appState = $state<main.AppState | null>(null)

  onMount(() => {
    const ready = (s: main.AppState) => {
      appState = s
      if (s.state === 'ready') {
        refresh()
        loadMe()
      }
    }
    GetState().then(ready)
    EventsOn('app:state', ready)
    initPlayer()
  })

  // Library panel resize: drag the gap; dragging far left collapses it.
  const MIN = 280,
    MAX = 600,
    COLLAPSED = 72
  let dragging = $state(false)
  function startDrag(e: PointerEvent) {
    dragging = true
    const startX = e.clientX
    const startW = prefs.libCollapsed ? COLLAPSED : prefs.libWidth
    const move = (ev: PointerEvent) => {
      const w = startW + ev.clientX - startX
      if (w < MIN - 80) prefs.libCollapsed = true
      else {
        prefs.libCollapsed = false
        prefs.libWidth = Math.max(MIN, Math.min(MAX, w))
      }
    }
    const up = () => {
      dragging = false
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
  }
  const libWidth = $derived(prefs.libCollapsed ? COLLAPSED : prefs.libWidth)
</script>

{#if appState?.state === 'ready'}
  <div class="app" class:dragging>
    <TopBar />
    <div class="middle" style:--lib="{libWidth}px">
      <div class="left">
        <Boundary label="Your Library"><LibraryPanel collapsed={prefs.libCollapsed} ontoggle={() => (prefs.libCollapsed = !prefs.libCollapsed)} /></Boundary>
      </div>
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="resizer" onpointerdown={startDrag}></div>
      <div class="center"><Main /></div>
      {#if prefs.rightPanel === 'queue'}
        <div class="right"><Boundary label="The queue"><QueuePanel onclose={() => (prefs.rightPanel = '')} /></Boundary></div>
      {:else if prefs.rightPanel === 'friends'}
        <div class="right"><Boundary label="Friend Activity"><FriendsPanel onclose={() => (prefs.rightPanel = '')} /></Boundary></div>
      {:else if prefs.rightPanel === 'npv'}
        <div class="right"><Boundary label="Now Playing"><NowPlayingPanel onclose={() => (prefs.rightPanel = '')} /></Boundary></div>
      {/if}
    </div>
    <Boundary label="The player"><PlayerBar /></Boundary>
    <ContextMenu />
    <Shortcuts />
    <Dialog />
    <div class="toasts">
      {#each toasts as t (t.id)}<div class="toast">{t.text}</div>{/each}
    </div>
  </div>
{:else if appState}
  <Login state={appState} />
{/if}

<style>
  .app { height: 100%; display: grid; grid-template-rows: 64px minmax(0, 1fr) auto; }
  .app > :global(.remote) { grid-row: 4; }
  .app.dragging { cursor: col-resize; }
  .app.dragging * { user-select: none; }
  .middle { display: grid; grid-template-columns: var(--lib) 8px minmax(0, 1fr); padding: 0 8px; min-height: 0; }
  .middle:has(.right) { grid-template-columns: var(--lib) 8px minmax(0, 1fr) 8px clamp(280px, 25vw, 420px); }
  .right { grid-column: 5; min-height: 0; }
  .left, .center { min-height: 0; min-width: 0; }
  .resizer { cursor: col-resize; position: relative; }
  .resizer::after { content: ''; position: absolute; top: 0; bottom: 0; left: 3px; width: 2px; border-radius: 1px; }
  .resizer:hover::after { background: hsla(0, 0%, 100%, .3); }
  .toasts { position: fixed; bottom: 96px; left: 0; right: 0; display: flex; flex-direction: column; align-items: center; gap: 8px; pointer-events: none; z-index: 100; }
  .toast { background: #3d91f4; color: #fff; padding: 12px 16px; border-radius: 8px; font-size: 14px; box-shadow: 0 8px 24px rgba(0,0,0,.5); }
</style>
