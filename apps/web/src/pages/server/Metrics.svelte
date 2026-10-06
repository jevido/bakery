<script lang="ts">
  // Coolify's Server Metrics page (resources/views/livewire/server/charts.blade.php,
  // app/Livewire/Server/Charts.php; Apache-2.0, see NOTICE). Coolify draws
  // Sentinel's stored history for a chosen time range; The Bakery keeps no
  // history yet, so the charts hold what was read every 5 s since the page
  // opened (one hour at most), and the containers table of the old page
  // follows them.
  import { api } from '../../lib/api'
  import { percent, size } from '../../lib/format'
  import { href } from '../../lib/router.svelte'
  import type { ContainerMetrics, Metrics, Server } from '../../lib/types'
  import Empty from '../../lib/ui/Empty.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import UsageChart, { type Sample } from './UsageChart.svelte'

  let { server }: { server: Server } = $props()

  // Coolify's cpuColor and ramColor (layouts/base.blade.php), in both themes.
  const cpuColor = '#1e90ff'
  const ramColor = '#00ced1'
  const maxSamples = 720

  let metrics = $state.raw<Metrics | null>(null)
  let metricsError = $state('')
  let cpu = $state.raw<Sample[]>([])
  let memory = $state.raw<Sample[]>([])

  const keep = (list: Sample[], s: Sample) => [...list, s].slice(-maxSamples)

  async function read(id: number) {
    try {
      const m = await api<Metrics>('GET', `/servers/${id}/metrics`)
      if (id !== server.id) return
      metrics = m
      metricsError = ''
      const at = new Date(m.server.read_at).getTime() || Date.now()
      cpu = keep(cpu, { at, value: m.server.cpu_percent })
      const total = m.server.memory_total_bytes
      memory = keep(memory, { at, value: total ? (m.server.memory_used_bytes / total) * 100 : 0 })
    } catch (e) {
      metricsError = e instanceof Error ? e.message : String(e)
    }
  }

  const reachable = $derived(server.status === 'reachable')
  const serverId = $derived(server.id)
  $effect(() => {
    const id = serverId
    metrics = null
    cpu = []
    memory = []
    if (!reachable) return
    read(id)
    const t = setInterval(() => read(id), 5000)
    return () => clearInterval(t)
  })

  function ownerHref(c: ContainerMetrics): string | null {
    switch (c.owner) {
      case 'application':
        return href(`/applications/${c.owner_id}`)
      case 'database':
        return href(`/databases/${c.owner_id}`)
      case 'service':
        return href(`/services/${c.owner_id}`)
      default:
        return null
    }
  }
</script>

<div class="chrome application-settings-form flex w-full min-w-0 flex-col gap-6">
  {#if !reachable}
    <SettingsSection id="server-metrics-overview-section" title="Metrics" helper="Inspect recent CPU and memory usage of this server.">
      <Empty
        size="sm"
        title="Metrics unavailable"
        description="The Bakery reads metrics once this server is validated and reachable."
        icon="dashboard"
      />
    </SettingsSection>
  {:else}
    <SettingsSection id="server-metrics-overview-section" title="Metrics" helper="Inspect recent CPU and memory usage of this server.">
      {#snippet actions()}
        <StatusBadge status="Live updates" type="success" />
      {/snippet}
      <p class="text-sm text-neutral-950 dark:text-fg" data-testid="metrics-range">Since you opened this page</p>
      <p class="mt-3 text-xs leading-5 text-neutral-500 dark:text-fg-dim">
        The charts refresh every five seconds and keep the last hour.
      </p>
      {#if metricsError}<p class="mt-3 text-sm text-error">{metricsError}</p>{/if}
    </SettingsSection>

    <SettingsSection id="server-cpu-metrics-section" title="CPU usage" helper="Percentage of available CPU capacity used by this server.">
      <UsageChart name="CPU" color={cpuColor} samples={cpu} empty={metricsError ? 'No CPU metrics available' : 'Loading CPU metrics…'} />
    </SettingsSection>

    <SettingsSection id="server-memory-metrics-section" title="Memory usage" helper="Percentage of physical memory currently used by this server.">
      <UsageChart name="Memory" color={ramColor} samples={memory} empty={metricsError ? 'No memory metrics available' : 'Loading memory metrics…'} />
    </SettingsSection>

    <SettingsSection id="server-containers-metrics-section" title="Containers" helper="What each container The Bakery runs on this server uses now." flush>
      {#if !metrics}
        <p class="px-4 py-4 text-sm text-neutral-500 dark:text-fg-dim">Reading metrics…</p>
      {:else}
        <p class="border-b border-neutral-200 px-4 py-3 text-xs text-neutral-500 dark:border-white/[0.08] dark:text-fg-dim" data-testid="podman-storage">
          Podman storage: images {size(metrics.server.images_bytes)}, containers {size(metrics.server.containers_bytes)}, volumes
          {size(metrics.server.volumes_bytes)}.
        </p>
        {#if metrics.containers.length === 0}
          <div class="p-6">
            <Empty size="sm" title="No containers" description="The Bakery runs no containers on this server." icon="servers" />
          </div>
        {:else}
          <div class="data-table" data-testid="containers">
            <div class="data-table-header server-containers-table-grid">
              <span>Container</span>
              <span>Belongs to</span>
              <span>CPU</span>
              <span>Memory</span>
            </div>
            {#each metrics.containers as c (c.name)}
              {@const link = ownerHref(c)}
              <div class="data-table-row server-containers-table-grid border-b border-neutral-200 last:border-b-0 dark:border-white/[0.08]">
                <div class="min-w-0 truncate font-mono text-[12px] text-neutral-950 dark:text-fg">{c.name}</div>
                <div class="min-w-0 truncate text-[11px] text-neutral-600 dark:text-fg-dim">
                  {#if link}<a class="hover:underline" href={link}>{c.owner} {c.owner_id}</a>{:else}{c.owner || '—'}{/if}
                </div>
                <div class="text-[11px] text-neutral-600 tabular-nums dark:text-fg-dim">{percent(c.cpu_percent)}</div>
                <div class="text-[11px] text-neutral-600 tabular-nums dark:text-fg-dim">
                  {size(c.memory_used_bytes)}
                  {#if c.memory_limit_bytes && metrics.server.memory_total_bytes && c.memory_limit_bytes < metrics.server.memory_total_bytes}
                    <span class="text-neutral-400 dark:text-fg-faint">/ {size(c.memory_limit_bytes)}</span>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      {/if}
    </SettingsSection>
  {/if}
</div>
