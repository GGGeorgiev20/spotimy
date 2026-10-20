<script lang="ts">
  import Card from '../components/Card.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import { App } from '../lib/api.svelte'
  import { navigate } from '../lib/router.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { id, kind, scroller }: { id: string; kind: string; scroller: HTMLElement | undefined } = $props()
  const uri = $derived(`spotify:artist:${id}`)
  let name = $state('')
  let cards = $state<spotify.Card[]>([])
  let total = $state(0)
  let error = $state('')
  let busy = false

  $effect(() => {
    const u = uri, k = kind
    cards = []
    App.Artist(u).then((a) => (name = a.name)).catch(() => {})
    App.Discography(u, k, 0, 50)
      .then((p) => ((cards = p.cards ?? []), (total = p.total)))
      .catch((e) => (error = String(e)))
  })

  $effect(() => {
    if (!scroller) return
    const onScroll = async () => {
      if (busy || cards.length >= total || scroller!.scrollTop + scroller!.clientHeight < scroller!.scrollHeight - 600) return
      busy = true
      try {
        const p = await App.Discography(uri, kind, cards.length, 50)
        cards = [...cards, ...(p.cards ?? [])]
      } finally {
        busy = false
      }
    }
    scroller.addEventListener('scroll', onScroll, { passive: true })
    return () => scroller!.removeEventListener('scroll', onScroll)
  })

  const kinds = [['all', 'All'], ['albums', 'Albums'], ['singles', 'Singles and EPs'], ['compilations', 'Compilations']]
</script>

{#if error}
  <Unavailable what="Discography" {error} />
{:else}
  <div class="page">
    <h1><a href="/" onclick={(e) => (e.preventDefault(), navigate(`/artist/${id}`))}>{name}</a></h1>
    <div class="chips">
      {#each kinds as [k, label] (k)}
        <button class="chip" class:on={kind === k} onclick={() => navigate(`/artist/${id}/discography/${k}`)}>{label}</button>
      {/each}
    </div>
    <div class="grid">{#each cards as card (card.uri)}<Card {card} />{/each}</div>
  </div>
{/if}

<style>
  .page { padding: 64px 12px 24px; }
  h1 { margin: 0 12px 16px; font-size: 32px; }
  .chips { display: flex; gap: 8px; padding: 0 12px 16px; }
  .chip { background: hsla(0,0%,100%,.07); border-radius: 999px; padding: 4px 12px; font-size: 14px; }
  .chip.on { background: var(--text); color: #000; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(176px, 1fr)); }
</style>
