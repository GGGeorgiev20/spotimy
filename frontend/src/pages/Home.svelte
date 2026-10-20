<script lang="ts">
  // Interim home: greeting + shortcuts to recently played library items.
  // The real personalized feed (pathfinder "home") lands in milestone 3.
  import Icon from '../components/Icon.svelte'
  import { App, pick, libraryVersion } from '../lib/api.svelte'
  import { controls, player, playing } from '../lib/player.svelte'
  import { openUri } from '../lib/router.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let items = $state<spotify.LibraryItem[]>([])
  $effect(() => {
    libraryVersion.n
    App.Library({ filters: [], sort: 'Recents', textFilter: '', offset: 0, limit: 8, folderUri: '' })
      .then((l) => (items = l.items.filter((i) => i.type !== 'folder').slice(0, 8)))
      .catch(() => (items = []))
  })
  const h = new Date().getHours()
  const greeting = h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening'
  const ctx = $derived(player.status?.context_uri ?? '')
  const isCtx = (uri: string) => ctx === uri || (uri === 'spotify:collection:tracks' && ctx.endsWith(':collection'))
</script>

<div class="home">
  <h1>{greeting}</h1>
  <div class="shortcuts">
    {#each items as it (it.uri)}
      <div class="sc" role="link" tabindex="0" onclick={() => openUri(it.uri)} onkeydown={(e) => e.key === 'Enter' && openUri(it.uri)}>
        <img src={pick(it.images, 64)} alt="" />
        <span class="name">{it.name}</span>
        <button class="play" aria-label="Play {it.name}" onclick={(e) => (e.stopPropagation(), isCtx(it.uri) ? controls.playPause() : controls.play(it.uri))}>
          <Icon name={isCtx(it.uri) && playing() ? 'pause' : 'play'} />
        </button>
      </div>
    {/each}
  </div>
</div>

<style>
  .home { padding: 64px 24px 24px; background: linear-gradient(rgba(83, 83, 83, .6), var(--surface) 300px); min-height: 100%; }
  h1 { font-size: 32px; margin: 0 0 24px; }
  .shortcuts { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 8px 12px; }
  .sc { display: flex; align-items: center; gap: 12px; height: 64px; background: hsla(0,0%,100%,.07); border-radius: 4px; overflow: hidden; cursor: pointer; position: relative; padding-right: 8px; }
  .sc:hover { background: hsla(0,0%,100%,.2); }
  .sc img { width: 64px; height: 64px; object-fit: cover; flex: none; box-shadow: 0 8px 24px rgba(0,0,0,.5); }
  .name { flex: 1; font-weight: 700; font-size: 16px; overflow: hidden; display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; }
  .play { width: 32px; height: 32px; border-radius: 50%; background: var(--accent); color: #000; display: grid; place-items: center; opacity: 0; flex: none; box-shadow: 0 8px 8px rgba(0,0,0,.3); }
  .sc:hover .play { opacity: 1; }
</style>
