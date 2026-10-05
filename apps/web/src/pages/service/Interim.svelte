<script lang="ts">
  // The Service's Runtime Logs as they were on the old tabbed page, until
  // they are ported to Coolify's markup.
  import ContainerLogs from '../../lib/ContainerLogs.svelte'
  import type { Service } from '../../lib/types'

  let { service }: { service: Service } = $props()

  let logComponent = $state('')
  let logTarget = $derived(logComponent || service.components[0]?.name || '')
</script>

<div class="row">
  <label>
    Component
    <select value={logTarget} onchange={(e) => (logComponent = e.currentTarget.value)}>
      {#each service.components as c (c.name)}<option value={c.name}>{c.name}</option>{/each}
    </select>
  </label>
</div>
{#key `${logTarget}-${service.desired_state}`}
  <ContainerLogs
    url={`/api/services/${service.id}/components/${encodeURIComponent(logTarget)}/logs`}
    empty="No container: the service is stopped or this component has not started."
    stopped="The container stopped (a restart or a redeploy replaces it)."
  />
{/key}

<style>
  .row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    flex-wrap: wrap;
  }
</style>
