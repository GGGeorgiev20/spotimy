<script lang="ts">
  import { onMount } from 'svelte'
  import { GetState, Logout } from '../wailsjs/go/main/App'
  import { EventsOn } from '../wailsjs/runtime/runtime'
  import type { main } from '../wailsjs/go/models'
  import Login from './components/Login.svelte'
  import PlayerBar from './components/PlayerBar.svelte'
  import { controls, initPlayer, refresh } from './lib/player.svelte'

  let appState: main.AppState | null = $state(null)
  let uri = $state('spotify:track:4cOdK2wGLETKBW3PvgPWqT')

  onMount(() => {
    GetState().then((s) => (appState = s))
    EventsOn('app:state', (s: main.AppState) => {
      appState = s
      if (s.state === 'ready') refresh()
    })
    initPlayer()
  })
</script>

{#if appState?.state === 'ready'}
  <div class="shell">
    <main class="main">
      <h2>Milestone 1</h2>
      <form onsubmit={(e) => { e.preventDefault(); controls.play(uri) }}>
        <input bind:value={uri} placeholder="spotify:track:…" />
        <button type="submit">Play</button>
      </form>
      <button class="logout" onclick={Logout}>Log out</button>
    </main>
    <PlayerBar />
  </div>
{:else if appState}
  <Login state={appState} />
{/if}

<style>
  .shell { height: 100%; display: grid; grid-template-rows: 1fr auto; gap: 8px; padding-top: 8px; }
  .main { margin: 0 8px; background: var(--surface); border-radius: var(--radius); padding: 24px; }
  form { display: flex; gap: 8px; margin: 16px 0; }
  input { flex: 1; max-width: 480px; background: var(--surface-2); border: 1px solid var(--surface-3); border-radius: 4px; padding: 8px 12px; }
  form button, .logout { background: var(--text); color: #000; font-weight: 700; border-radius: 999px; padding: 8px 20px; }
</style>
