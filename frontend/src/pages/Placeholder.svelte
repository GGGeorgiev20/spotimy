<script lang="ts">
  let { path }: { path: string } = $props()
</script>

<div class="ph">
  <h2>Coming soon</h2>
  <p>{path} arrives in a later milestone.</p>
</div>

<style>
  .ph { padding: 120px 24px; text-align: center; color: var(--text-2); }
  h2 { color: var(--text); margin: 0 0 8px; }
</style>
