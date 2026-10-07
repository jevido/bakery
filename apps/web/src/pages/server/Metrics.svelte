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
  import MetricCard from '../../lib/MetricCard.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import type { ContainerMetrics, Metrics, Server } from '../../lib/types'
  import Empty from '../../lib/ui/Empty.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import UsageChart, { type Sample } from './UsageChart.svelte'

  let { server }: { server: Server } = $props()

  // Two of the theme's chart colors in place of Coolify's cpuColor and
  // ramColor (#1e90ff, #00ced1). Paperclip's chart palette is grey, and
  // --chart-1 is too light to read on white, so memory takes --chart-4 in
  // light and --chart-1 in dark.
  const cpuTone = 'text-chart-2'
  const ramTone = 'text-chart-4 dark:text-chart-1'
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

<div class="flex w-full min-w-0 flex-col gap-8">
  {#if !reachable}
    <SettingsGroup id="server-metrics-overview-section" label="Metrics" hint="Inspect recent CPU and memory usage of this server.">
      <Empty
        size="sm"
        title="Metrics unavailable"
        description="The Bakery reads metrics once this server is validated and reachable."
        icon="dashboard"
      />
    </SettingsGroup>
  {:else}
    <SettingsGroup id="server-metrics-overview-section" label="Metrics" hint="Inspect recent CPU and memory usage of this server." wide>
      {#snippet actions()}
        <StatusBadge status="Live updates" type="success" />
      {/snippet}
      <p class="text-sm text-foreground" data-testid="metrics-range">Since you opened this page</p>
      <p class="text-xs text-muted-foreground">The charts refresh every five seconds and keep the last hour.</p>
      {#if metricsError}<p class="text-sm text-destructive">{metricsError}</p>{/if}
      {#if metrics}
        {@const total = metrics.server.memory_total_bytes}
        <div class="grid grid-cols-2 gap-1 rounded-md border border-border sm:gap-2 xl:grid-cols-4" data-testid="metrics-overview">
          <MetricCard icon="cpu" value={percent(metrics.server.cpu_percent)} label="CPU" />
          <MetricCard icon="storages" value={total ? percent((metrics.server.memory_used_bytes / total) * 100) : '—'} label="Memory">
            {#snippet description()}{size(metrics!.server.memory_used_bytes)} of {size(total)}{/snippet}
          </MetricCard>
          <MetricCard icon="servers" value={metrics.containers.length} label="Containers" />
          <MetricCard icon="layers" value={size(metrics.server.images_bytes)} label="Images" />
        </div>
      {/if}
    </SettingsGroup>

    <SettingsGroup id="server-cpu-metrics-section" label="CPU usage" hint="Percentage of available CPU capacity used by this server." wide>
      <UsageChart name="CPU" tone={cpuTone} samples={cpu} empty={metricsError ? 'No CPU metrics available' : 'Loading CPU metrics…'} />
    </SettingsGroup>

    <SettingsGroup id="server-memory-metrics-section" label="Memory usage" hint="Percentage of physical memory currently used by this server." wide>
      <UsageChart name="Memory" tone={ramTone} samples={memory} empty={metricsError ? 'No memory metrics available' : 'Loading memory metrics…'} />
    </SettingsGroup>

    <SettingsGroup id="server-containers-metrics-section" label="Containers" hint="What each container The Bakery runs on this server uses now." wide>
      {#if !metrics}
        <p class="text-sm text-muted-foreground">Reading metrics…</p>
      {:else}
        <p class="text-xs text-muted-foreground" data-testid="podman-storage">
          Podman storage: images {size(metrics.server.images_bytes)}, containers {size(metrics.server.containers_bytes)}, volumes
          {size(metrics.server.volumes_bytes)}.
        </p>
        <div class="overflow-hidden rounded-md border border-border">
          {#if metrics.containers.length === 0}
            <div class="p-4">
              <Empty size="sm" title="No containers" description="The Bakery runs no containers on this server." icon="servers" />
            </div>
          {:else}
            <div data-testid="containers">
              {#each metrics.containers as c (c.name)}
                {@const link = ownerHref(c)}
                <div class="flex items-center gap-3 border-b border-border px-4 py-2.5 last:border-b-0" data-testid="container">
                  <span class="min-w-0 flex-1 truncate font-mono text-xs text-foreground">{c.name}</span>
                  <span class="hidden w-36 truncate text-xs text-muted-foreground sm:block">
                    {#if link}<a class="hover:underline" href={link}>{c.owner} {c.owner_id}</a>{:else}{c.owner || '—'}{/if}
                  </span>
                  <span class="w-14 text-right text-xs text-muted-foreground tabular-nums" title="CPU">{percent(c.cpu_percent)}</span>
                  <span class="w-28 text-right text-xs text-muted-foreground tabular-nums" title="Memory">
                    {size(c.memory_used_bytes)}
                    {#if c.memory_limit_bytes && metrics.server.memory_total_bytes && c.memory_limit_bytes < metrics.server.memory_total_bytes}
                      <span class="text-muted-foreground/70">/ {size(c.memory_limit_bytes)}</span>
                    {/if}
                  </span>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    </SettingsGroup>
  {/if}
</div>
