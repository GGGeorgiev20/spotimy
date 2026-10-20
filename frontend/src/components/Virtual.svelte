<script lang="ts">
  // Windowed list: renders only rows near the viewport of `scroller` (which may
  // be an ancestor, so a page header can scroll away above the list).
  import { onMount, type Snippet } from 'svelte'

  let {
    count,
    rowHeight,
    scroller,
    row,
    overscan = 8,
    onrange,
  }: {
    count: number
    rowHeight: number
    scroller: HTMLElement | undefined
    row: Snippet<[number]>
    overscan?: number
    onrange?: (start: number, end: number) => void
  } = $props()

  let el: HTMLDivElement
  let start = $state(0)
  let end = $state(0)

  function update() {
    if (!scroller || !el) return
    const top = el.getBoundingClientRect().top - scroller.getBoundingClientRect().top
    const first = Math.floor(-top / rowHeight)
    const visible = Math.ceil(scroller.clientHeight / rowHeight)
    const s = Math.max(0, first - overscan)
    const e = Math.min(count, first + visible + overscan)
    if (s !== start || e !== end) {
      start = s
      end = Math.max(s, e)
      onrange?.(start, end)
    }
  }

  let frame = 0
  function schedule() {
    if (!frame) frame = requestAnimationFrame(() => ((frame = 0), update()))
  }

  $effect(() => {
    count
    rowHeight
    update()
  })

  $effect(() => {
    if (!scroller) return
    scroller.addEventListener('scroll', schedule, { passive: true })
    const ro = new ResizeObserver(schedule)
    ro.observe(scroller)
    return () => {
      scroller.removeEventListener('scroll', schedule)
      ro.disconnect()
    }
  })

  onMount(update)
</script>

<div bind:this={el} style:height="{count * rowHeight}px" style:position="relative">
  {#each { length: end - start } as _, k (start + k)}
    <div class="vrow" style:top="{(start + k) * rowHeight}px" style:height="{rowHeight}px">
      {@render row(start + k)}
    </div>
  {/each}
</div>

<style>
  .vrow { position: absolute; left: 0; right: 0; }
</style>
