<script lang="ts">
  // A user's profile: header, the listener's own top artists/tracks, public playlists.
  import EntityHeader from '../components/EntityHeader.svelte'
  import Shelf from '../components/Shelf.svelte'
  import TrackTable from '../components/TrackTable.svelte'
  import Unavailable from '../components/Unavailable.svelte'
  import { App, pick } from '../lib/api.svelte'
  import { Paged } from '../lib/paged.svelte'
  import { setChrome } from '../lib/chrome.svelte'
  import { headerColor } from '../lib/format'
  import type { spotify } from '../../wailsjs/go/models'

  let { id, scroller }: { id: string; scroller?: HTMLElement } = $props()
  let p = $state<spotify.Profile | null>(null)
  let top = $state<Paged | null>(null)
  let error = $state('')
  const color = $derived(p ? headerColor(p.color) : 'rgb(83, 83, 83)')

  $effect(() => {
    App.Profile(decodeURIComponent(id))
      .then((r) => {
        p = r
        const items = r.topTracks
        top = new Paged(async () => ({ total: items.length, items }))
        top.seed({ total: items.length, items })
        setChrome({ title: r.name, color: headerColor(r.color) })
      })
      .catch((e) => (error = String(e)))
  })

  const plural = (n: number, w: string) => `${n} ${w}${n === 1 ? '' : 's'}`
</script>

{#if error}
  <Unavailable what="This profile" {error} />
{:else if p}
  <EntityHeader image={p.images.length ? pick(p.images, 300) : ''} round type="Profile" title={p.name} {color}>
    {#snippet meta()}
      <span class="sub">
        {[p!.publicPlaylists && plural(p!.publicPlaylists, 'Public Playlist'), plural(p!.followers, 'Follower'), `${p!.following} Following`].filter(Boolean).join(' • ')}
      </span>
    {/snippet}
  </EntityHeader>
  <div class="body">
    {#if p.topArtists.length}
      <Shelf title="Top artists this month" note="Only visible to you" items={p.topArtists} />
    {/if}
    {#if top && p.topTracks.length}
      <section class="tracks">
        <h2>Top tracks this month</h2>
        <p class="note">Only visible to you</p>
        <TrackTable tracks={top} {scroller} contextUri="" variant="search" />
      </section>
    {/if}
    {#if p.playlists.length}
      <Shelf title="Public Playlists" items={p.playlists} />
    {/if}
    {#if p.followerList.length}<Shelf title="Followers" items={p.followerList} />{/if}
    {#if p.followingList.length}<Shelf title="Following" items={p.followingList} />{/if}
  </div>
{/if}

<style>
  .sub { color: hsla(0, 0%, 100%, .85); }
  .body { padding-top: 24px; }
  .tracks { padding: 0 24px 32px; }
  .note { margin: 2px 0 16px; font-size: 14px; color: var(--text-2); }
  h2 { margin: 0; font-size: 24px; }
</style>
