<script lang="ts">
  import LogView from './LogView.svelte'

  let { applicationId }: { applicationId: number } = $props()

  // Bumped to reconnect after the container stopped or was replaced.
  let attempt = $state(0)
  let reason = $state('')
</script>

<div class="head">
  <span class="muted">Live output of the running container (last 200 lines, then new ones).</span>
  {#if reason}
    <button onclick={() => ((reason = ''), attempt++)}>Reconnect</button>
  {/if}
</div>
{#key attempt}
  <LogView
    url={`/api/applications/${applicationId}/logs`}
    empty="No running container. Deploy the application first."
    onend={(e) => (reason = e.reason ?? 'ended')}
  />
{/key}
{#if reason === 'stopped'}<p class="muted">The container stopped (a new deployment may have replaced it).</p>{/if}

<style>
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    margin-bottom: 0.75rem;
  }
</style>
