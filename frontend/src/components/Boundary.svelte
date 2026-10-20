<script lang="ts">
  // Keeps a rendering error in one area from blanking the whole window.
  import type { Snippet } from 'svelte'
  let { children, label = 'This section' }: { children: Snippet; label?: string } = $props()
</script>

<svelte:boundary onerror={(e) => console.error(label, e)}>
  {@render children()}
  {#snippet failed(error, reset)}
    <div class="failed">
      <p>{label} ran into a problem.</p>
      <button onclick={reset}>Try again</button>
      <small>{String(error)}</small>
    </div>
  {/snippet}
</svelte:boundary>

<style>
  .failed { padding: 48px 24px; text-align: center; color: var(--text-2); display: flex; flex-direction: column; align-items: center; gap: 12px; }
  button { background: var(--text); color: #000; font-weight: 700; border-radius: 999px; padding: 8px 24px; }
  small { font-size: 11px; opacity: .6; max-width: 480px; word-break: break-word; }
</style>
