<script lang="ts">
  import { OpenLoginURL } from '../../wailsjs/go/main/App'
  import type { main } from '../../wailsjs/go/models'
  let { state }: { state: main.AppState } = $props()
</script>

<div class="login">
  {#if state.state === 'login'}
    <h1>Log in to Spotify</h1>
    <p>Finish logging in on the Spotify page that opened in your browser.</p>
    <button class="primary" onclick={OpenLoginURL} disabled={!state.loginUrl}>Open login page again</button>
  {:else if state.state === 'error'}
    <h1>Can't connect</h1>
    <p class="err">{state.error}</p>
    <p>Retrying…</p>
  {:else}
    <div class="spinner"></div>
  {/if}
</div>

<style>
  .login { height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px; text-align: center; padding: 32px; background: linear-gradient(#2a2a2a, #000); }
  h1 { font-size: 32px; margin: 0 0 8px; }
  p { color: var(--text-2); margin: 0; max-width: 420px; }
  .err { color: #f3727f; font-size: 13px; word-break: break-word; }
  .primary { margin-top: 24px; background: var(--accent); color: #000; font-weight: 700; padding: 14px 32px; border-radius: 999px; }
  .primary:hover { background: var(--accent-hover); transform: scale(1.04); }
  .primary:disabled { opacity: .5; }
  .spinner { width: 40px; height: 40px; border: 4px solid var(--surface-3); border-top-color: var(--accent); border-radius: 50%; animation: spin 1s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>
