<script lang="ts">
  // Spotify-style thin progress bar. While dragging the value is local; `onchange`
  // fires once on release, `oninput` on every move.
  let {
    value,
    max,
    onchange,
    oninput,
    label,
  }: {
    value: number
    max: number
    onchange: (v: number) => void
    oninput?: (v: number) => void
    label: string
  } = $props()

  let bar: HTMLDivElement
  let dragValue: number | null = $state(null)
  const shown = $derived(dragValue ?? value)
  const pct = $derived(max > 0 ? Math.max(0, Math.min(100, (shown / max) * 100)) : 0)

  function at(e: PointerEvent) {
    const r = bar.getBoundingClientRect()
    return Math.max(0, Math.min(1, (e.clientX - r.left) / r.width)) * max
  }
  function down(e: PointerEvent) {
    if (max <= 0) return
    bar.setPointerCapture(e.pointerId)
    dragValue = at(e)
    oninput?.(dragValue)
  }
  function move(e: PointerEvent) {
    if (dragValue === null) return
    dragValue = at(e)
    oninput?.(dragValue)
  }
  function up() {
    if (dragValue === null) return
    onchange(dragValue)
    dragValue = null
  }
  function key(e: KeyboardEvent) {
    const step = max / 20
    if (e.key === 'ArrowRight') onchange(Math.min(max, value + step))
    else if (e.key === 'ArrowLeft') onchange(Math.max(0, value - step))
    else return
    e.preventDefault()
  }
</script>

<div
  class="slider"
  class:dragging={dragValue !== null}
  bind:this={bar}
  role="slider"
  tabindex="0"
  aria-label={label}
  aria-valuemin={0}
  aria-valuemax={max}
  aria-valuenow={Math.round(shown)}
  onpointerdown={down}
  onpointermove={move}
  onpointerup={up}
  onpointercancel={up}
  onkeydown={key}
>
  <div class="track">
    <div class="fill" style:width="{pct}%"></div>
  </div>
  <div class="thumb" style:left="{pct}%"></div>
</div>

<style>
  .slider {
    position: relative;
    flex: 1;
    height: 12px;
    display: flex;
    align-items: center;
    cursor: pointer;
    outline: none;
  }
  .track {
    width: 100%;
    height: 4px;
    border-radius: 2px;
    background: #4d4d4d;
    overflow: hidden;
  }
  .fill { height: 100%; background: var(--text); border-radius: 2px; }
  .thumb {
    position: absolute;
    width: 12px;
    height: 12px;
    margin-left: -6px;
    border-radius: 50%;
    background: var(--text);
    box-shadow: 0 2px 4px rgba(0,0,0,.5);
    display: none;
  }
  .slider:hover .fill, .slider.dragging .fill, .slider:focus-visible .fill { background: var(--accent); }
  .slider:hover .thumb, .slider.dragging .thumb, .slider:focus-visible .thumb { display: block; }
</style>
