<script lang="ts">
  import Icon from './Icon.svelte'
  import Slider from './Slider.svelte'
  import { player, playing, position, repeatMode, controls } from '../lib/player.svelte'
  import { fmtDuration as fmtTime } from '../lib/format'
  import { img, saved, loadSaved, toggleSaved } from '../lib/api.svelte'
  import { openUri } from '../lib/router.svelte'

  const s = $derived(player.status)
  const track = $derived(s?.track)
  const max = $derived(s?.volume_steps || 1)
  let lastVolume = 0
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
      <button class="cover-btn" onclick={() => track.album_uri && openUri(track.album_uri)} aria-label="Open album">
        {#if track.album_cover_url}<img class="cover" src={img(track.album_cover_url)} alt="" />{:else}<div class="cover"></div>{/if}
      </button>
      <div class="meta">
        <a class="title ellipsis" href="/" onclick={(e) => (e.preventDefault(), track.album_uri && openUri(track.album_uri))}>{track.name}</a>
        <div class="artists ellipsis">
          {#each track.artist_names as name, i}{#if i > 0}, {/if}<a href="/" onclick={(e) => (e.preventDefault(), track.artist_uris?.[i] && openUri(track.artist_uris[i]))}>{name}</a>{/each}
        </div>
      </div>
      <button class="icon-btn save" class:on={saved[track.uri]} onclick={() => toggleSaved(track.uri)} title={saved[track.uri] ? 'Remove from Liked Songs' : 'Save to Liked Songs'}>
        <Icon name={saved[track.uri] ? 'check-circle' : 'plus-circle'} />
      </button>
    {/if}
  </div>

  <div class="center">
    <div class="buttons">
      <button class="icon-btn" class:active={s?.shuffle_context} disabled={!track} onclick={controls.toggleShuffle} title="Enable shuffle">
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
        max={track?.duration ?? 0}
        oninput={(v) => (seekPreview = v)}
        onchange={(v) => {
          seekPreview = null
          controls.seek(v)
        }}
      />
      <span class="time">{fmtTime(track?.duration ?? 0)}</span>
    </div>
  </div>

  <div class="right">
    <button class="icon-btn" disabled title="Now playing view"><Icon name="now-playing" /></button>
    <button class="icon-btn" disabled title="Lyrics"><Icon name="lyrics" /></button>
    <button class="icon-btn" disabled title="Queue"><Icon name="queue" /></button>
    <button class="icon-btn" disabled title="Connect to a device"><Icon name="devices" /></button>
    <button class="icon-btn" onclick={toggleMute} title={s?.volume === 0 ? 'Unmute' : 'Mute'}><Icon name={volIcon} /></button>
    <div class="volume">
      <Slider label="Volume" value={s?.volume ?? 0} {max} oninput={controls.setVolume} onchange={controls.setVolume} />
    </div>
  </div>
</footer>

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
  .volume { width: 93px; display: flex; margin-left: 4px; }
</style>
