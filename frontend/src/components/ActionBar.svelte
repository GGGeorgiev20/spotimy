<script lang="ts">
  import Icon from './Icon.svelte'
  import type { Snippet } from 'svelte'
  let {
    isPlaying,
    onplay,
    saved,
    onsave,
    saveKind = 'check',
    extra,
    onmore,
  }: {
    isPlaying: boolean
    onplay: () => void
    saved?: boolean
    onsave?: () => void
    saveKind?: 'check' | 'heart'
    extra?: Snippet
    onmore?: (e: MouseEvent) => void
  } = $props()
</script>

<div class="bar">
  <button class="big-play" onclick={onplay} aria-label={isPlaying ? 'Pause' : 'Play'}>
    <Icon name={isPlaying ? 'pause' : 'play'} size={24} />
  </button>
  {#if onsave}
    <button class="act" class:on={saved} onclick={onsave} aria-label={saved ? 'Remove from Your Library' : 'Save to Your Library'}>
      <Icon name={saved ? (saveKind === 'heart' ? 'heart-filled' : 'check-circle') : saveKind === 'heart' ? 'heart' : 'plus-circle'} size={32} />
    </button>
  {/if}
  {@render extra?.()}
  {#if onmore}<button class="act" aria-label="More options" onclick={onmore}><Icon name="more" size={32} /></button>{/if}
</div>

<style>
  .bar { position: relative; display: flex; align-items: center; gap: 24px; padding: 24px; }
  .big-play {
    width: 56px; height: 56px; border-radius: 50%;
    display: grid; place-items: center;
    background: var(--accent); color: #000;
    transition: transform .05s, background .1s;
  }
  .big-play:hover { transform: scale(1.04); background: var(--accent-hover); }
  .act { color: var(--text-2); display: grid; }
  .act:hover { color: var(--text); transform: scale(1.04); }
  .act.on { color: var(--accent); }
</style>
