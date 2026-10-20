<script lang="ts">
  // Shown when one section's internal API call fails (e.g. a stale hash): the
  // rest of the app keeps working. Details are in the log.
  let { what, error }: { what: string; error: string } = $props()
</script>

<div class="unavail" title={error}>
  <h2>{what} isn't available right now</h2>
  <p>Spotify's API returned something we couldn't read. Check the log for details.</p>
</div>

<style>
  .unavail { padding: 96px 24px; text-align: center; color: var(--text-2); }
  h2 { color: var(--text); font-size: 24px; margin: 0 0 8px; }
  p { margin: 0; font-size: 14px; }
</style>
