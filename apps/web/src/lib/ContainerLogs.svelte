<script lang="ts">
  import LogView from './LogView.svelte'

  // url is the log stream of an Application's or a Database's container.
  let {
    url,
    empty,
    stopped,
  }: {
    url: string
    /** Shown when there is no running container. */
    empty: string
    /** Shown after the container stopped. */
    stopped: string
  } = $props()

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
  <LogView {url} {empty} onend={(e) => (reason = e.reason ?? 'ended')} />
{/key}
{#if reason === 'stopped'}<p class="muted">{stopped}</p>{/if}

<style>
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    margin-bottom: 0.75rem;
  }
</style>
