<script lang="ts">
  import Icon from '../components/Icon.svelte'
  import Shelf from '../components/Shelf.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import { App, coverColor, pick } from '../lib/api.svelte'
  import { headerColor } from '../lib/format'
  import { ctxPlaying, playContext } from '../lib/player.svelte'
  import { navigate, openUri } from '../lib/router.svelte'
  import { openMenu } from '../lib/menu.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let facet = $state('')
  let home = $state<spotify.Home | null>(null)
  let error = $state('')
  let tint = $state('rgb(83, 83, 83)')

  $effect(() => {
    const f = facet
    error = ''
    App.Home(f)
      .then(async (h) => {
        home = h
        const first = h.sections.find((s) => s.kind === 'shortcuts')?.items[0]
        if (first) tint = headerColor(await coverColor(first.images))
      })
      .catch((e) => (error = String(e)))
  })

  const shortcuts = $derived(home?.sections.find((s) => s.kind === 'shortcuts')?.items.slice(0, 8) ?? [])
  const shelves = $derived(home?.sections.filter((s) => s.kind === 'shelf') ?? [])
  const chips = $derived([{ id: '', label: 'All' }, ...(home?.chips ?? [])])
</script>

<div class="home" style:--tint={tint}>
  <div class="chips">
    {#each chips as c (c.id)}
      <button class="chip" class:on={facet === c.id} onclick={() => (facet = c.id)}>{c.label}</button>
    {/each}
  </div>

  {#if error}
    <Unavailable what="Home" {error} />
  {:else if home}
    {#if shortcuts.length}
      <div class="shortcuts">
        {#each shortcuts as it (it.uri)}
          <div
            class="sc" role="link" tabindex="0"
            onclick={() => openUri(it.uri)}
            onkeydown={(e) => e.key === 'Enter' && openUri(it.uri)}
            onmouseenter={async () => (tint = headerColor(await coverColor(it.images)))}
            oncontextmenu={(e) => openMenu(e, { kind: 'card', card: it })}
          >
            <img src={pick(it.images, 64)} alt="" />
            <span class="name">{it.name}</span>
            <button class="play" aria-label="Play {it.name}" onclick={(e) => (e.stopPropagation(), playContext(it.uri))}>
              <Icon name={ctxPlaying(it.uri) ? 'pause' : 'play'} />
            </button>
          </div>
        {/each}
      </div>
    {/if}
    {#each shelves as s (s.uri)}
      <Shelf title={s.title} items={s.items} showAll={s.total > s.items.length ? () => navigate(`/section/${s.uri.split(':')[2]}`) : undefined} />
    {/each}
  {/if}
</div>

<style>
  .home { padding: 64px 0 24px; background: linear-gradient(color-mix(in srgb, var(--tint) 70%, transparent) 0, var(--surface) 332px); transition: background .6s; min-height: 100%; }
  .chips { display: flex; gap: 8px; padding: 0 24px 24px; }
  .chip { background: hsla(0,0%,100%,.07); border-radius: 999px; padding: 4px 12px; font-size: 14px; }
  .chip:hover { background: hsla(0,0%,100%,.1); }
  .chip.on { background: var(--text); color: #000; }
  .shortcuts { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 8px; padding: 0 24px 32px; }
  .sc { display: flex; align-items: center; gap: 12px; height: 48px; background: hsla(0,0%,100%,.1); border-radius: 4px; overflow: hidden; cursor: pointer; padding-right: 8px; }
  .sc:hover { background: hsla(0,0%,100%,.2); }
  .sc img { width: 48px; height: 48px; object-fit: cover; flex: none; box-shadow: 0 8px 24px rgba(0,0,0,.5); }
  .name { flex: 1; font-weight: 700; font-size: 14px; overflow: hidden; display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; }
  .play { width: 32px; height: 32px; border-radius: 50%; background: var(--accent); color: #000; display: grid; place-items: center; opacity: 0; flex: none; box-shadow: 0 8px 8px rgba(0,0,0,.3); }
  .sc:hover .play { opacity: 1; }
</style>
