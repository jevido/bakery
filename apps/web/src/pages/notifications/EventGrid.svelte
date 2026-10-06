<script lang="ts">
  // Coolify's notification event grid (resources/views/components/notification/event-grid.blade.php,
  // Apache-2.0, see NOTICE): one multiselect per group of Event kinds. Only
  // the groups The Bakery has events for are drawn: Deployments, Backups and
  // Servers. With `threaded` (Telegram) the Forum topics section follows.
  import type { EventKind } from '../../lib/types'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import EventMultiselect from './EventMultiselect.svelte'

  let {
    channel,
    eventKinds,
    selected,
    disabled = false,
    ontoggle,
  }: {
    /** The Channel kind, for the ids. */
    channel: string
    eventKinds: EventKind[]
    selected: string[]
    disabled?: boolean
    ontoggle: (kind: string) => unknown
  } = $props()

  const groupOf = (kind: string) => (kind.startsWith('deployment_') ? 'Deployments' : kind.startsWith('backup_') ? 'Backups' : 'Servers')

  const groups = $derived(
    ['Deployments', 'Backups', 'Servers']
      .map((label) => ({
        label,
        events: eventKinds
          .filter((e) => groupOf(e.kind) === label)
          .map((e) => ({ kind: e.kind, label: e.label, enabled: selected.includes(e.kind) })),
      }))
      .filter((g) => g.events.length > 0),
  )
</script>

<div class="flex flex-col gap-6">
  <SettingsSection id="{channel}-notification-events" title="Notification events" helper="Choose which events send a notification on this channel.">
    <div class="grid gap-4 lg:grid-cols-2">
      {#each groups as group (group.label)}
        <div class="min-w-0">
          <EventMultiselect id="{channel}-{group.label.toLowerCase()}-events" label={group.label} events={group.events} {disabled} {ontoggle} />
        </div>
      {/each}
    </div>
  </SettingsSection>
</div>
