<script lang="ts">
  // The channel's recent Deliveries, The Bakery's own (Coolify keeps no
  // record of what it sent), drawn like the executions list of Coolify's
  // Docker Cleanup page: one row per Delivery with its status.
  import { api } from '../../lib/api'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import type { Delivery, EventKind } from '../../lib/types'
  import Empty from '../../lib/ui/Empty.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
  import { statusType } from '@bakery/ui/statusColors'

  let { channelId, eventKinds, version }: { channelId: number; eventKinds: EventKind[]; version: number } = $props()

  let deliveries = $state.raw<Delivery[] | null>(null)
  let error = $state('')

  $effect(() => {
    const id = channelId
    void version
    error = ''
    api<{ deliveries: Delivery[] }>('GET', `/notification-channels/${id}/deliveries`)
      .then((r) => {
        if (id === channelId) deliveries = r.deliveries
      })
      .catch((e) => (error = e.message))
  })

  const labels: Record<Delivery['status'], string> = { pending: 'Pending', sent: 'Sent', failed: 'Failed' }
  const label = (kind: string) => eventKinds.find((e) => e.kind === kind)?.label ?? kind
  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' })
</script>

<SettingsGroup id="notification-deliveries" label="Recent deliveries" hint="What The Bakery sent to this channel lately, and how it went.">
  <div class="overflow-hidden rounded-md border border-border">
    {#if error}
      <p class="p-4 text-sm text-destructive">{error}</p>
    {:else if deliveries === null}
      <div class="p-4"><Spinner text="Loading…" /></div>
    {:else if deliveries.length === 0}
      <div class="p-4" data-testid="deliveries-empty">
        <Empty size="sm" title="No deliveries" description="Send a test, or wait for the next event." icon="notifications" />
      </div>
    {:else}
      {#each deliveries as d (d.id)}
        <div class="border-b border-border px-4 py-3 last:border-b-0" data-testid="delivery">
          <div class="flex w-full items-start gap-4 text-left">
            <StatusBadge status={labels[d.status]} type={statusType(d.status)} class="mt-0.5 shrink-0" />
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium text-foreground">{d.title}</p>
              <p class="mt-0.5 text-xs text-muted-foreground">
                {label(d.event_kind)} · {when.format(new Date(d.created_at))} · {d.attempts}
                {d.attempts === 1 ? 'attempt' : 'attempts'}
              </p>
              {#if d.last_error}<p class="mt-1 text-xs break-words text-destructive">{d.last_error}</p>{/if}
            </div>
          </div>
        </div>
      {/each}
    {/if}
  </div>
</SettingsGroup>
