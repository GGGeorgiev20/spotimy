<script lang="ts">
  import Icon from './Icon.svelte'
  import { App, toast } from '../lib/api.svelte'
  import { player } from '../lib/player.svelte'
  import type { spotify } from '../../wailsjs/go/models'

  let { onclose }: { onclose: () => void } = $props()
  let devices = $state<spotify.Device[] | null>(null)

  $effect(() => {
    player.version
    App.Devices().then((d) => (devices = d ?? [])).catch(() => (devices = []))
  })

  const icon = (t: string) => (t === 'SMARTPHONE' ? 'devices' : t === 'SPEAKER' ? 'volume' : 'now-playing')
  const current = $derived(devices?.find((d) => d.active) ?? devices?.find((d) => d.isPlayer))
  const others = $derived(devices?.filter((d) => d !== current) ?? [])

  async function transfer(d: spotify.Device) {
    try {
      await App.TransferPlayback(d.id)
      onclose()
    } catch (e) {
      toast(`Couldn't switch to ${d.name}: ${e}`)
    }
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="backdrop" onclick={onclose}></div>
<div class="pop" role="dialog" aria-label="Connect to a device">
  {#if current}
    <div class="cur">
      <Icon name="volume" size={24} />
      <div>
        <div class="label">Current device</div>
        <div class="name">{current.isPlayer ? 'This computer' : current.name}</div>
      </div>
    </div>
  {/if}
  <h3>Select another device</h3>
  {#if devices && !others.length}
    <p class="muted">No other devices found. Open Spotify on your phone or speaker on the same account.</p>
  {/if}
  {#each others as d (d.id)}
    <button class="dev" onclick={() => transfer(d)}>
      <Icon name={icon(d.type)} size={20} />
      <span class="ellipsis">{d.isPlayer ? 'This computer' : d.name}</span>
    </button>
  {/each}
</div>

<style>
  .backdrop { position: fixed; inset: 0; z-index: 60; }
  .pop { position: absolute; bottom: 56px; right: -40px; z-index: 61; width: 320px; background: #282828; border-radius: 8px; padding: 16px 8px 8px; box-shadow: 0 16px 24px rgba(0,0,0,.3), 0 6px 8px rgba(0,0,0,.2); }
  .cur { display: flex; gap: 16px; align-items: center; padding: 0 8px 16px; color: var(--accent); }
  .label { font-size: 16px; font-weight: 700; color: var(--text); }
  .name { font-size: 14px; color: var(--accent); }
  h3 { font-size: 14px; margin: 8px 8px; }
  .muted { color: var(--text-2); font-size: 13px; padding: 0 8px; }
  .dev { width: 100%; display: flex; align-items: center; gap: 16px; padding: 12px 8px; border-radius: 4px; font-size: 14px; text-align: left; }
  .dev:hover { background: hsla(0,0%,100%,.1); }
</style>
