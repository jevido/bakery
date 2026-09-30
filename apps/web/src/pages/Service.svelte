<script lang="ts">
  import { api } from '../lib/api'
  import { href } from '../lib/router.svelte'
  import StatusBadge from '../lib/StatusBadge.svelte'
  import type { Service } from '../lib/types'

  let { id }: { id: number } = $props()

  let service = $state.raw<Service | null>(null)
  let loadError = $state('')

  $effect(() => {
    service = null
    api<{ service: Service }>('GET', `/services/${id}`)
      .then((r) => (service = r.service))
      .catch((e) => (loadError = e.message))
  })
</script>

{#if loadError}
  <p class="error">{loadError}</p>
{:else if !service}
  <p class="muted">Loading…</p>
{:else}
  <p><a href={href(`/projects/${service.project_id}`)}>Project</a> /</p>
  <h1>{service.name} <StatusBadge status={service.status} /></h1>
{/if}
