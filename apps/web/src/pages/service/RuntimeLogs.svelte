<script lang="ts">
  // Coolify's Runtime Logs for a Service (resources/views/livewire/project/shared/logs.blade.php,
  // Apache-2.0, see NOTICE): one logs card per Component that has a
  // Container, open by default only when there is one.
  import type { Service } from '../../lib/types'
  import RuntimeLogs from '../application/RuntimeLogs.svelte'

  let { service }: { service: Service } = $props()

  let running = $derived(service.components.filter((c) => c.status !== 'missing'))
</script>

{#if running.length === 0}
  <RuntimeLogs url="" container="" />
{:else}
  <div class="flex flex-col">
    {#each running as c (c.name)}
      <RuntimeLogs
        url={`/api/services/${service.id}/components/${encodeURIComponent(c.name)}/logs`}
        container={c.container}
        expandByDefault={running.length === 1}
      />
    {/each}
  </div>
{/if}
