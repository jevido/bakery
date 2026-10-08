<script lang="ts" module>
  import type { StatusType } from '@bakery/ui/statusColors'

  export type Summary = {
    label: string
    type: StatusType
    container: { label: string; type: StatusType }
    health: { label: string; type: StatusType }
  }

  function headline(s: string): string {
    return s
      .replace(/[_-]+/g, ' ')
      .trim()
      .split(/\s+/)
      .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
      .join(' ')
  }

  /**
   * Coolify's status-summary for a status such as `running:healthy`: the
   * container's state, its health, and the one label the pill shows.
   */
  export function summarize(status: string): Summary {
    const raw = status.toLowerCase().trim()
    const [state = 'unknown', health] = raw.split(':')
    const containerLabel = headline(state || 'unknown')
    const healthLabel =
      health === 'healthy' ? 'Healthy' : health === 'unhealthy' ? 'Unhealthy' : health === 'starting' ? 'Starting' : 'Not configured'
    const containerType: StatusType = state.startsWith('running')
      ? 'success'
      : ['starting', 'restarting', 'degraded'].some((s) => state.startsWith(s))
        ? 'warning'
        : 'error'
    const healthType: StatusType = health === 'healthy' ? 'success' : health === 'unhealthy' ? 'error' : 'warning'
    let label: string, type: StatusType
    if (containerType === 'error') [label, type] = [containerLabel, 'error']
    else if (state.startsWith('degraded')) [label, type] = ['Degraded', 'warning']
    else if (healthType === 'error') [label, type] = ['Degraded', 'error']
    else if (containerType === 'warning') [label, type] = [containerLabel, 'warning']
    else if (health === 'starting') [label, type] = [`${containerLabel} (healthcheck starting)`, 'warning']
    else if (healthType === 'warning') [label, type] = [`${containerLabel} (no healthcheck)`, 'warning']
    else [label, type] = [containerLabel, 'success']
    return {
      label,
      type,
      container: { label: containerLabel, type: containerType },
      health: { label: healthLabel, type: healthType },
    }
  }
</script>

<script lang="ts">
  // Coolify's status-summary (resources/views/components/status-summary.blade.php,
  // Apache-2.0, see NOTICE): a status pill that opens the container's state
  // and its healthcheck, in Paperclip's popover.
  import * as Popover from '@bakery/ui/components/ui/popover'
  import Icon from '../Icon.svelte'
  import { statusBadgeClasses, statusDotClasses } from '@bakery/ui/statusColors'
  import Helper from './Helper.svelte'

  let {
    status,
    title = 'Application status',
    containerName = 'Container',
    align = 'left',
  }: { status: string; title?: string; containerName?: string; align?: 'left' | 'right' } = $props()

  const summary = $derived(summarize(status))
  let open = $state(false)
</script>

<Popover.Root bind:open>
  <Popover.Trigger
    class={[
      'chrome inline-flex max-w-full shrink-0 cursor-pointer items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap transition-opacity outline-none hover:opacity-80 focus-visible:ring-2 focus-visible:ring-ring/50',
      statusBadgeClasses[summary.type],
    ]}
  >
    <span class={['size-1.5 shrink-0 rounded-full', statusDotClasses[summary.type]]}></span>
    <span>{summary.label}</span>
    <span class={['inline-flex transition-transform', open && 'rotate-180']}>
      <Icon name="chevron-down" class="size-3 opacity-55" />
    </span>
  </Popover.Trigger>
  <Popover.Content align={align === 'right' ? 'end' : 'start'} class="w-[min(16rem,calc(100vw-1.5rem))] p-1 sm:w-64" role="menu">
    <div class="px-2 py-1.5 text-xs font-medium text-muted-foreground">{title}</div>
    <div class="flex items-center gap-2.5 rounded-sm px-2 py-1.5 text-sm">
      <span class={['size-1.5 shrink-0 rounded-full', statusDotClasses[summary.container.type]]}></span>
      <span class="flex-1">{containerName}</span>
      <span class="text-muted-foreground">{summary.container.label}</span>
    </div>
    <div class="flex items-center gap-2.5 rounded-sm px-2 py-1.5 text-sm">
      <span class={['size-1.5 shrink-0 rounded-full', statusDotClasses[summary.health.type]]}></span>
      <span class="flex-1">Healthcheck</span>
      <span class="inline-flex items-center gap-1.5 text-muted-foreground">
        {summary.health.label}
        {#if summary.health.label === 'Not configured'}
          <Helper
            label="About unconfigured healthchecks"
            helper="No healthcheck is configured, so The Bakery can only report the container state. Traffic can still be routed to the container, but The Bakery cannot verify that the application inside it is ready to receive requests."
          />
        {/if}
      </span>
    </div>
  </Popover.Content>
</Popover.Root>
