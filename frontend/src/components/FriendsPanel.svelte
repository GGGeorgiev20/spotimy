<script lang="ts">
  // Friend Activity: what followed users last played. Refreshes every two
  // minutes while open.
  import Icon from './Icon.svelte'
  import { App, pick } from '../lib/api.svelte'
  import { openUri } from '../lib/router.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { onclose }: { onclose: () => void } = $props()
  let friends = $state<spotify.Friend[] | null>(null)
  let error = $state('')
  let now = $state(Date.now())

  $effect(() => {
    const load = () =>
      App.Friends()
        .then((f) => ((friends = f), (error = ''), (now = Date.now())))
        .catch((e) => (error = String(e)))
    load()
    const t = setInterval(load, 120_000)
    return () => clearInterval(t)
  })

  function ago(ts: number) {
    const m = Math.floor((now - ts) / 60000)
    if (m < 60) return `${Math.max(m, 1)} min`
    const h = Math.floor(m / 60)
    if (h < 24) return `${h} hr`
    const d = Math.floor(h / 24)
    return d < 7 ? `${d} d` : `${Math.floor(d / 7)} w`
  }
  const go = (e: MouseEvent, uri: string) => (e.preventDefault(), uri && openUri(uri))
</script>

<aside class="panel">
  <header>
    <h2>Friend Activity</h2>
    <button class="icon-btn" onclick={onclose} aria-label="Close Friend Activity"><Icon name="close" /></button>
  </header>
  <div class="body">
    {#if error}
      <p class="muted">Couldn't load Friend Activity.</p>
    {:else if friends && !friends.length}
      <p class="muted">Follow friends to see what they're listening to.</p>
    {/if}
    {#each friends ?? [] as f (f.user.uri)}
      <div class="row">
        <button class="avatar" onclick={() => openUri(f.user.uri)} aria-label={f.user.name}>
          {#if f.user.images?.length}<img src={pick(f.user.images, 64)} alt="" />{:else}<Icon name="user" />{/if}
          {#if f.playing}<span class="live"></span>{/if}
        </button>
        <div class="meta">
          <div class="top">
            <a href="/" class="name ellipsis" onclick={(e) => go(e, f.user.uri)}>{f.user.name}</a>
            <span class="when">{#if f.playing}<Icon name="volume" size={14} />{:else}{ago(f.timestamp)}{/if}</span>
          </div>
          <div class="line ellipsis">
            <a href="/" onclick={(e) => go(e, f.context.uri)}>{f.track.name}</a>
            <span> • </span>
            <a href="/" onclick={(e) => go(e, f.artist.uri)}>{f.artist.name}</a>
          </div>
          {#if f.context.name}
            <a href="/" class="ctx ellipsis" onclick={(e) => go(e, f.context.uri)}><Icon name={f.context.type === 'album' ? 'album' : f.context.type === 'artist' ? 'artist' : 'playlist'} size={12} />{f.context.name}</a>
          {/if}
        </div>
      </div>
    {/each}
  </div>
</aside>

<style>
  .panel { height: 100%; display: flex; flex-direction: column; background: var(--surface); border-radius: var(--radius); overflow: hidden; }
  header { display: flex; justify-content: space-between; align-items: center; padding: 16px 16px 8px; }
  h2 { margin: 0; font-size: 16px; }
  .body { flex: 1; overflow-y: auto; padding: 8px 16px 16px; }
  .muted { color: var(--text-2); font-size: 14px; }
  .row { display: flex; gap: 12px; margin-bottom: 20px; }
  .avatar { position: relative; flex: none; width: 40px; height: 40px; border-radius: 50%; background: var(--surface-3); display: grid; place-items: center; color: var(--text-2); }
  .avatar img { width: 100%; height: 100%; border-radius: 50%; object-fit: cover; }
  .live { position: absolute; right: 0; top: 0; width: 10px; height: 10px; border-radius: 50%; background: #4687d6; border: 2px solid var(--surface); }
  .meta { flex: 1; min-width: 0; font-size: 13px; color: var(--text-2); display: flex; flex-direction: column; gap: 2px; }
  .top { display: flex; justify-content: space-between; gap: 8px; }
  .name { color: var(--text); font-weight: 700; font-size: 14px; }
  .when { flex: none; }
  .line a:hover, .name:hover, .ctx:hover { text-decoration: underline; }
  .line a:first-child { color: hsla(0,0%,100%,.85); }
  .ctx { display: flex; align-items: center; gap: 6px; }
</style>
