<script lang="ts">
  import EntityHeader from '../components/EntityHeader.svelte'
  import ActionBar from '../components/ActionBar.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import { App, coverColor, pick, saved, loadSaved } from '../lib/api.svelte'
  import { setChrome } from '../lib/chrome.svelte'
  import { headerColor, fmtAdded } from '../lib/format'
  import { controls, player, playing } from '../lib/player.svelte'
  import { navigate } from '../lib/router.svelte'
  import { toggleLibrary } from '../lib/actions'
  import type { spotify } from '../../wailsjs/go/models'

  let { id }: { id: string } = $props()
  const uri = $derived(`spotify:episode:${id}`)
  let ep = $state<spotify.Episode | null>(null)
  let error = $state('')
  let color = $state('rgb(83, 83, 83)')

  $effect(() => {
    const u = uri
    ep = null
    loadSaved([u])
    App.Episode(u)
      .then(async (e) => {
        ep = e
        color = headerColor(await coverColor(e.images))
        setChrome({ title: e.name, color, contextUri: u, onplay: play })
      })
      .catch((e) => (error = String(e)))
  })

  const current = $derived(player.status?.track?.uri === uri)
  function play() {
    if (current) return controls.playPause()
    return controls.play(uri)
  }
</script>

{#if error}
  <Unavailable what="This episode" {error} />
{:else if ep}
  <EntityHeader image={pick(ep.images, 300)} type="Podcast Episode" title={ep.name} {color}>
    {#snippet meta()}
      <a href="/" class="show" onclick={(e) => (e.preventDefault(), navigate(`/show/${ep!.showUri.split(':')[2]}`))}>{ep!.showName}</a>
    {/snippet}
  </EntityHeader>
  <ActionBar isPlaying={current && playing()} onplay={play} saved={saved[uri]} onsave={() => toggleLibrary(uri)} />
  <div class="body">
    <div class="meta">{fmtAdded(ep.releaseDate)} • {Math.round(ep.durationMs / 60000)} min{ep.state === 'IN_PROGRESS' ? ` • ${Math.round((ep.durationMs - ep.playedMs) / 60000)} min left` : ''}</div>
    <h2>Episode Description</h2>
    <p>{ep.description}</p>
  </div>
{/if}

<style>
  .show { font-weight: 700; font-size: 16px; }
  .body { padding: 0 24px; max-width: 800px; }
  .meta { color: var(--text-2); font-size: 14px; margin-bottom: 24px; }
  h2 { font-size: 24px; margin: 0 0 12px; }
  p { color: var(--text-2); white-space: pre-line; overflow-wrap: anywhere; }
</style>
