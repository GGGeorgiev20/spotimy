<script lang="ts">
  import Icon from './Icon.svelte'
  import { App, pick, toast } from '../lib/api.svelte'
  import { player, playing } from '../lib/player.svelte'
  import { openUri } from '../lib/router.svelte'
  import { openMenu } from '../lib/menu.svelte'
  import type { main } from '../../wailsjs/go/models'

  let { onclose }: { onclose: () => void } = $props()

  let queue = $state<main.Queue | null>(null)
  let error = $state('')
  let timer = 0

  // Reload whenever the player or cluster changes (debounced).
  $effect(() => {
    player.version
    clearTimeout(timer)
    timer = setTimeout(load, 200)
    return () => clearTimeout(timer)
  })
  async function load() {
    try {
      queue = await App.Queue()
      error = ''
    } catch (e) {
      error = String(e)
    }
  }

  const track = $derived(player.status?.track)
  const queued = $derived((queue?.next ?? []).filter((t) => t.provider === 'queue'))
  const upcoming = $derived((queue?.next ?? []).filter((t) => t.provider !== 'queue'))

  async function setQueue(next: main.QueueTrack[]) {
    try {
      await App.SetQueue(next.map((t) => ({ uri: t.uri, uid: t.uid ?? '', provider: t.provider })))
      if (queue) queue.next = next
    } catch (e) {
      toast(`Couldn't update queue: ${e}`)
    }
  }
  const remove = (t: main.QueueTrack) => setQueue((queue?.next ?? []).filter((x) => x !== t))
  $effect(() => {
    const on = (e: Event) => {
      const t = queue?.next[(e as CustomEvent).detail]
      if (t) remove(t)
    }
    window.addEventListener('queue-remove', on)
    return () => window.removeEventListener('queue-remove', on)
  })
  const clearQueued = () => setQueue((queue?.next ?? []).filter((x) => x.provider !== 'queue'))

  // Drag to reorder within the user queue.
  let dragIdx = $state(-1)
  function drop(target: number) {
    if (dragIdx < 0 || !queue || dragIdx === target) return (dragIdx = -1)
    const list = [...queue.next]
    const [moved] = list.splice(dragIdx, 1)
    list.splice(target, 0, moved)
    dragIdx = -1
    setQueue(list)
  }
</script>

<aside class="panel">
  <header>
    <h2>Queue</h2>
    <button class="icon-btn" onclick={onclose} aria-label="Close"><Icon name="close" /></button>
  </header>
  <div class="body">
    {#if track}
      <h3>Now playing</h3>
      {@render row(track.name, track.artists, track.images, track.albumUri, true)}
    {/if}
    {#if error}
      <p class="muted">Queue unavailable</p>
    {/if}
    {#if queued.length}
      <div class="sec-head">
        <h3>Next in queue</h3>
        <button class="clear" onclick={clearQueued}>Clear queue</button>
      </div>
      {#each queued as t, k (k)}
        {@const i = queue!.next.indexOf(t)}
        <div
          class="drag" role="listitem" draggable="true"
          ondragstart={() => (dragIdx = i)}
          ondragover={(e) => e.preventDefault()}
          ondrop={() => drop(i)}
          oncontextmenu={(e) => openMenu(e, { kind: 'queue', track: t, index: i })}
        >
          {@render row(t.name, t.artists, t.album?.images, t.album?.uri, false, () => remove(t))}
        </div>
      {/each}
    {/if}
    {#if upcoming.length}
      <h3>Next up</h3>
      {#each upcoming.slice(0, 50) as t, k (k)}
        {@const i = queue!.next.indexOf(t)}
        <div role="listitem" oncontextmenu={(e) => openMenu(e, { kind: 'queue', track: t, index: i })}>
          {@render row(t.name, t.artists, t.album?.images, t.album?.uri, false, () => remove(t))}
        </div>
      {/each}
    {/if}
  </div>
</aside>

{#snippet row(name: string, artists: { name: string; uri: string }[] | undefined, images: any, albumUri: string | undefined, current: boolean, onremove?: () => void)}
  <div class="row" class:current>
    <img src={pick(images, 48)} alt="" />
    <div class="meta">
      <button class="name ellipsis" onclick={() => albumUri && openUri(albumUri)}>
        {#if current && playing()}<span class="eq"><Icon name="volume" size={12} /></span>{/if}{name}
      </button>
      <div class="artists ellipsis">
        {#each artists ?? [] as a, k}{#if k > 0}, {/if}<a href="/" onclick={(e) => (e.preventDefault(), openUri(a.uri))}>{a.name}</a>{/each}
      </div>
    </div>
    {#if onremove}<button class="rm" onclick={onremove} aria-label="Remove from queue"><Icon name="close" /></button>{/if}
  </div>
{/snippet}

<style>
  .panel { height: 100%; display: flex; flex-direction: column; background: var(--surface); border-radius: var(--radius); overflow: hidden; }
  header { display: flex; justify-content: space-between; align-items: center; padding: 16px 16px 8px; }
  h2 { font-size: 16px; margin: 0; }
  .body { flex: 1; overflow-y: auto; padding: 0 8px 16px; }
  h3 { font-size: 16px; margin: 16px 8px 8px; }
  .sec-head { display: flex; justify-content: space-between; align-items: baseline; padding-right: 8px; }
  .clear { font-size: 14px; color: var(--text-2); font-weight: 700; }
  .clear:hover { color: var(--text); }
  .muted { color: var(--text-2); padding: 0 8px; font-size: 14px; }
  .row { display: flex; align-items: center; gap: 12px; padding: 8px; border-radius: 6px; }
  .row:hover { background: hsla(0,0%,100%,.1); }
  .row img { width: 48px; height: 48px; border-radius: 4px; flex: none; background: var(--surface-3); }
  .meta { flex: 1; min-width: 0; }
  .name { display: block; max-width: 100%; text-align: left; font-size: 16px; color: var(--text); }
  .current .name { color: var(--accent); }
  .eq { color: var(--accent); margin-right: 6px; display: inline-block; vertical-align: -1px; }
  .artists { font-size: 14px; color: var(--text-2); }
  .artists a:hover { color: var(--text); text-decoration: underline; }
  .rm { color: var(--text-2); visibility: hidden; display: grid; }
  .row:hover .rm { visibility: visible; }
  .rm:hover { color: var(--text); }
  .drag { cursor: grab; }
</style>
