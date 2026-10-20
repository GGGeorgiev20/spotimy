<script lang="ts">
  import { dialog } from '../lib/dialog.svelte'
  import { App } from '../lib/api.svelte'
  import { openUri } from '../lib/router.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let credits = $state<spotify.Credit[] | null>(null)
  $effect(() => {
    const d = dialog.current
    if (d?.kind !== 'credits') return
    credits = null
    App.NowPlaying(d.uri, '').then((i) => (credits = i.credits ?? [])).catch(() => (credits = []))
  })

  let name = $state('')
  let description = $state('')
  $effect(() => {
    const d = dialog.current
    if (d?.kind === 'edit') {
      name = d.name
      description = d.description
    }
  })

  function close(result: unknown) {
    const d = dialog.current
    dialog.current = null
    ;(d?.resolve as (v: unknown) => void)?.(result)
  }
  function key(e: KeyboardEvent) {
    if (e.key === 'Escape') close(dialog.current?.kind === 'confirm' ? false : null)
  }
</script>

<svelte:window onkeydown={dialog.current ? key : undefined} />

{#if dialog.current}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="overlay" onclick={() => close(dialog.current?.kind === 'confirm' ? false : null)}>
    <div class="box" role="dialog" tabindex="-1" aria-modal="true" onclick={(e) => e.stopPropagation()}>
      {#if dialog.current.kind === 'confirm'}
        <h2>{dialog.current.title}</h2>
        <p>{dialog.current.body}</p>
        <div class="actions">
          <button class="ghost" onclick={() => close(false)}>Cancel</button>
          <button class="primary" onclick={() => close(true)}>{dialog.current.ok}</button>
        </div>
      {:else if dialog.current.kind === 'credits'}
        <h2>Credits</h2>
        <p class="track">{dialog.current.track}</p>
        {#if !credits}
          <p>Loading…</p>
        {:else if !credits.length}
          <p>No credits available.</p>
        {:else}
          <div class="credits">
            {#each credits as c (c.name)}
              <div class="credit">
                {#if c.uri}<button class="cname" onclick={() => (close(null), openUri(c.uri))}>{c.name}</button>{:else}<span class="cname">{c.name}</span>{/if}
                <span class="roles">{c.roles}</span>
              </div>
            {/each}
          </div>
        {/if}
      {:else}
        <h2>Edit details</h2>
        <form onsubmit={(e) => (e.preventDefault(), name.trim() && close({ name: name.trim(), description }))}>
          <!-- svelte-ignore a11y_autofocus -->
          <input bind:value={name} placeholder="Add a name" autofocus maxlength="100" />
          <textarea bind:value={description} placeholder="Add an optional description" maxlength="300"></textarea>
          <div class="actions">
            <button class="primary" type="submit" disabled={!name.trim()}>Save</button>
          </div>
        </form>
      {/if}
    </div>
  </div>
{/if}

<style>
  .overlay { position: fixed; inset: 0; background: rgba(0,0,0,.7); display: grid; place-items: center; z-index: 200; }
  .box { width: min(524px, 90vw); background: #282828; border-radius: 8px; padding: 24px; box-shadow: 0 4px 60px rgba(0,0,0,.5); }
  h2 { margin: 0 0 16px; font-size: 24px; }
  p { color: var(--text-2); margin: 0 0 24px; }
  .track { margin: -8px 0 16px; font-weight: 700; color: var(--text); }
  .credits { max-height: 60vh; overflow-y: auto; display: flex; flex-direction: column; gap: 12px; }
  .credit { display: flex; flex-direction: column; }
  .cname { font-size: 16px; text-align: left; }
  button.cname:hover { text-decoration: underline; }
  .roles { font-size: 14px; color: var(--text-2); }
  form { display: flex; flex-direction: column; gap: 12px; }
  input, textarea { background: hsla(0,0%,100%,.1); border: 1px solid transparent; border-radius: 4px; padding: 10px 12px; font-size: 14px; outline: none; resize: none; }
  textarea { height: 120px; font-family: inherit; }
  input:focus, textarea:focus { background: #333; border-color: #535353; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
  .primary { background: var(--text); color: #000; font-weight: 700; border-radius: 999px; padding: 12px 32px; }
  .primary:hover:not(:disabled) { transform: scale(1.04); }
  .primary:disabled { opacity: .5; }
  .ghost { color: var(--text-2); font-weight: 700; padding: 12px 24px; }
  .ghost:hover { color: var(--text); }
</style>
