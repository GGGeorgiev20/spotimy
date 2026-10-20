<script lang="ts">
  import EntityHeader from '../components/EntityHeader.svelte'
  import ActionBar from '../components/ActionBar.svelte'
  import EpisodeRow from '../components/EpisodeRow.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import { App, coverColor, pick, saved } from '../lib/api.svelte'
  import { setChrome } from '../lib/chrome.svelte'
  import { headerColor } from '../lib/format'
  import { ctxPlaying, playContext } from '../lib/player.svelte'
  import { toggleLibrary } from '../lib/actions'
  import type { spotify } from '../../wailsjs/go/models'

  let { id, scroller }: { id: string; scroller: HTMLElement | undefined } = $props()
  const uri = $derived(`spotify:show:${id}`)
  let show = $state<spotify.Show | null>(null)
  let episodes = $state<spotify.Episode[]>([])
  let error = $state('')
  let color = $state('rgb(83, 83, 83)')
  let busy = false

  $effect(() => {
    const u = uri
    show = null
    App.Show(u)
      .then(async (s) => {
        show = s
        episodes = s.episodes ?? []
        saved[u] = s.saved
        color = headerColor(await coverColor(s.images))
        setChrome({ title: s.name, color, contextUri: u, onplay: () => playContext(u) })
      })
      .catch((e) => (error = String(e)))
  })

  // Older episodes load as the bottom comes into view.
  $effect(() => {
    if (!scroller) return
    const onScroll = async () => {
      if (busy || !show || episodes.length >= show.total) return
      if (scroller!.scrollTop + scroller!.clientHeight < scroller!.scrollHeight - 800) return
      busy = true
      try {
        episodes = [...episodes, ...(await App.ShowEpisodes(uri, episodes.length, 50))]
      } finally {
        busy = false
      }
    }
    scroller.addEventListener('scroll', onScroll, { passive: true })
    return () => scroller!.removeEventListener('scroll', onScroll)
  })
</script>

{#if error}
  <Unavailable what="This show" {error} />
{:else if show}
  <EntityHeader image={pick(show.images, 300)} type="Podcast" title={show.name} {color}>
    {#snippet meta()}<span class="pub">{show!.publisher}</span>{/snippet}
  </EntityHeader>
  <ActionBar isPlaying={ctxPlaying(uri)} onplay={() => playContext(uri)} saved={saved[uri]} onsave={() => toggleLibrary(uri)} />
  <div class="layout">
    <section class="eps">
      <h2>All Episodes</h2>
      {#each episodes as ep (ep.uri)}<EpisodeRow {ep} contextUri={uri} />{/each}
    </section>
    <aside class="about">
      <h2>About</h2>
      <p>{show.description}</p>
    </aside>
  </div>
{/if}

<style>
  .pub { font-weight: 700; font-size: 16px; }
  .layout { display: grid; grid-template-columns: minmax(0, 2fr) minmax(220px, 1fr); gap: 32px; padding: 0 24px; }
  h2 { font-size: 24px; margin: 0 0 16px; }
  .about p { color: var(--text-2); font-size: 14px; white-space: pre-line; overflow-wrap: anywhere; }
  @container main (max-width: 900px) { .layout { grid-template-columns: 1fr; } .about { order: -1; } }
</style>
