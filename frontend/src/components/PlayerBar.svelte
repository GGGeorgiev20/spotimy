<script lang="ts">
  import Icon from './Icon.svelte'
  import Slider from './Slider.svelte'
  import { player, playing, position, repeatMode, controls } from '../lib/player.svelte'
  import { fmtDuration as fmtTime } from '../lib/format'
  import { pick, saved, loadSaved, toggleSaved } from '../lib/api.svelte'
  import { openUri, navigate, back, current } from '../lib/router.svelte'
  import { prefs } from '../lib/prefs.svelte'
  import DevicePicker from './DevicePicker.svelte'

  const s = $derived(player.status)
  const track = $derived(s?.track)
  const max = $derived(s?.volumeMax || 1)
  let lastVolume = 0
  let devicesOpen = $state(false)
  const onLyrics = $derived(current().path === '/lyrics')
  let seekPreview: number | null = $state(null)

  function toggleMute() {
    if (!s) return
    if (s.volume > 0) {
      lastVolume = s.volume
      controls.setVolume(0)
    } else controls.setVolume(lastVolume || max / 2)
  }
  $effect(() => {
    if (track?.uri) loadSaved([track.uri])
  })

  const volIcon = $derived(!s || s.volume === 0 ? 'volume-off' : s.volume < max / 2 ? 'volume-low' : 'volume')
</script>

<footer class="bar">
  <div class="left">
    {#if track}
      <button class="cover-btn" onclick={() => track.albumUri && openUri(track.albumUri)} aria-label="Open album">
        {#if track.images?.length}<img class="cover" src={pick(track.images, 64)} alt="" />{:else}<div class="cover"></div>{/if}
      </button>
      <div class="meta">
        <a class="title ellipsis" href="/" onclick={(e) => (e.preventDefault(), track.albumUri && openUri(track.albumUri))}>{track.name}</a>
        <div class="artists ellipsis">
          {#each track.artists ?? [] as a, i}{#if i > 0}, {/if}<a href="/" onclick={(e) => (e.preventDefault(), a.uri && openUri(a.uri))}>{a.name}</a>{/each}
        </div>
      </div>
      <button class="icon-btn save" class:on={saved[track.uri]} onclick={() => toggleSaved(track.uri)} title={saved[track.uri] ? 'Remove from Liked Songs' : 'Save to Liked Songs'}>
        <Icon name={saved[track.uri] ? 'check-circle' : 'plus-circle'} />
      </button>
    {/if}
  </div>

  <div class="center">
    <div class="buttons">
      <button class="icon-btn" class:active={s?.shuffle} disabled={!track} onclick={controls.toggleShuffle} title="Enable shuffle">
        <Icon name="shuffle" />
      </button>
      <button class="icon-btn" disabled={!track} onclick={controls.prev} title="Previous"><Icon name="prev" /></button>
      <button class="play" disabled={!track} onclick={controls.playPause} title={playing() ? 'Pause' : 'Play'}>
        <Icon name={playing() ? 'pause' : 'play'} />
      </button>
      <button class="icon-btn" disabled={!track} onclick={controls.next} title="Next"><Icon name="next" /></button>
      <button class="icon-btn" class:active={repeatMode() !== 'off'} disabled={!track} onclick={controls.cycleRepeat} title="Repeat">
        <Icon name={repeatMode() === 'track' ? 'repeat-one' : 'repeat'} />
      </button>
    </div>
    <div class="progress">
      <span class="time right">{fmtTime(seekPreview ?? position())}</span>
      <Slider
        label="Change progress"
        value={position()}
        max={track?.durationMs ?? 0}
        oninput={(v) => (seekPreview = v)}
        onchange={(v) => {
          seekPreview = null
          controls.seek(v)
        }}
      />
      <span class="time">{fmtTime(track?.durationMs ?? 0)}</span>
    </div>
  </div>

  <div class="right">
    <button class="icon-btn" class:active={prefs.rightPanel === 'npv'} onclick={() => (prefs.rightPanel = prefs.rightPanel === 'npv' ? '' : 'npv')} title="Now playing view"><Icon name="now-playing" /></button>
    <button class="icon-btn" class:active={onLyrics} disabled={!track} onclick={() => (onLyrics ? back() : navigate('/lyrics'))} title="Lyrics"><Icon name="lyrics" /></button>
    <button class="icon-btn" class:active={prefs.rightPanel === 'queue'} onclick={() => (prefs.rightPanel = prefs.rightPanel === 'queue' ? '' : 'queue')} title="Queue"><Icon name="queue" /></button>
    <div class="dev-wrap">
      <button class="icon-btn" class:active={s?.remote} onclick={() => (devicesOpen = !devicesOpen)} title="Connect to a device"><Icon name="devices" /></button>
      {#if devicesOpen}<DevicePicker onclose={() => (devicesOpen = false)} />{/if}
    </div>
    <button class="icon-btn" onclick={toggleMute} title={s?.volume === 0 ? 'Unmute' : 'Mute'}><Icon name={volIcon} /></button>
    <div class="volume">
      <Slider label="Volume" value={s?.volume ?? 0} {max} oninput={controls.setVolume} onchange={controls.setVolume} />
    </div>
  </div>
</footer>
{#if s?.remote}
  <div class="remote"><Icon name="volume" size={14} /> Playing on {s.deviceName}</div>
{/if}

<style>
  .bar {
    height: 72px;
    display: grid;
    grid-template-columns: minmax(180px, 30%) minmax(0, 40%) minmax(180px, 30%);
    align-items: center;
    padding: 0 8px;
    background: var(--bg);
  }
  .left { display: flex; align-items: center; gap: 12px; min-width: 0; padding-left: 8px; }
  .cover { width: 56px; height: 56px; border-radius: 4px; background: var(--surface-3); flex: none; object-fit: cover; }
  .meta { min-width: 0; }
  .title { display: block; font-size: 14px; color: var(--text); }
  .artists { font-size: 12px; color: var(--text-2); margin-top: 2px; }
  .artists a:hover { color: var(--text); }
  .cover-btn { flex: none; display: block; }
  .save { flex: none; }
  .save.on { color: var(--accent); }

  .center { display: flex; flex-direction: column; align-items: center; gap: 4px; max-width: 722px; width: 100%; justify-self: center; }
  .buttons { display: flex; align-items: center; gap: 8px; }
  .play {
    width: 32px; height: 32px; margin: 0 8px;
    display: grid; place-items: center;
    background: var(--text); color: #000; border-radius: 50%;
    transition: transform .05s;
  }
  .play:hover:not(:disabled) { transform: scale(1.06); }
  .play:disabled { opacity: .4; cursor: default; }
  .progress { display: flex; align-items: center; gap: 8px; width: 100%; }
  .time { font-size: 12px; color: var(--text-2); min-width: 40px; font-variant-numeric: tabular-nums; }
  .time.right { text-align: right; }

  .right { display: flex; align-items: center; justify-content: flex-end; gap: 0; padding-right: 8px; }
  .dev-wrap { position: relative; }
  .remote { background: var(--accent); color: #000; font-size: 14px; font-weight: 600; height: 24px; display: flex; align-items: center; justify-content: flex-end; gap: 8px; padding: 0 32px; border-radius: 0 0 4px 4px; margin: 0 8px 4px; }
  .volume { width: 93px; display: flex; margin-left: 4px; }
</style>
