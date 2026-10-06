<script lang="ts">
  // Coolify's notification event grid (resources/views/components/notification/event-grid.blade.php,
  // Apache-2.0, see NOTICE): one multiselect per group of Event kinds. Only
  // the groups The Bakery has events for are drawn: Deployments, Backups and
  // Servers. With `threaded` (Telegram) the Forum topics section follows.
  // The Email page draws its own grid in Coolify (email.blade.php): no
  // helper, "Server" and "Server disk usage", and ids such as
  // deployment-email-events.
  import type { EventKind } from '../../lib/types'
  import Input from '../../lib/ui/Input.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import EventMultiselect from './EventMultiselect.svelte'

  let {
    channel,
    eventKinds,
    selected,
    disabled = false,
    ontoggle,
    threaded = false,
    threadIds = $bindable({}),
    threadErrors,
    formId,
  }: {
    /** The Channel kind, for the ids. */
    channel: string
    eventKinds: EventKind[]
    selected: string[]
    disabled?: boolean
    ontoggle: (kind: string) => unknown
    threaded?: boolean
    /** Telegram: the forum topic (message thread id) per Event kind. */
    threadIds?: Record<string, string>
    threadErrors?: string
    /** The form the topic inputs belong to, so they save with it. */
    formId?: string
  } = $props()

  const email = $derived(channel === 'email')
  const groupOf = (kind: string) => (kind.startsWith('deployment_') ? 'Deployments' : kind.startsWith('backup_') ? 'Backups' : 'Servers')

  const groups = $derived(
    ['Deployments', 'Backups', 'Servers']
      .map((label) => ({
        key: label,
        label: email && label === 'Servers' ? 'Server' : label,
        id: email ? `${label.toLowerCase().slice(0, -1)}-email-events` : `${channel}-${label.toLowerCase()}-events`,
        events: eventKinds
          .filter((e) => groupOf(e.kind) === label)
          .map((e) => ({
            kind: e.kind,
            label: email && e.kind === 'server_disk_usage' ? 'Server disk usage' : e.label,
            enabled: selected.includes(e.kind),
          })),
      }))
      .filter((g) => g.events.length > 0),
  )

  const threadGroups = $derived(
    groups.map((g) => ({ label: g.label, events: g.events.filter((e) => e.enabled) })).filter((g) => g.events.length > 0),
  )
</script>

<div class={email ? 'application-settings-form' : 'flex flex-col gap-6'}>
  <SettingsSection
    id="{channel}-notification-events"
    title="Notification events"
    helper={email ? undefined : 'Choose which events send a notification on this channel.'}
  >
    <div class="grid gap-4 lg:grid-cols-2">
      {#each groups as group (group.key)}
        <div class="min-w-0">
          <EventMultiselect id={group.id} label={group.label} events={group.events} {disabled} {ontoggle} />
        </div>
      {/each}
    </div>
  </SettingsSection>

  {#if threaded}
    <SettingsSection
      id="{channel}-forum-topics"
      title="Forum topics"
      helper="Optional. Route enabled events to a Telegram forum topic using its message thread ID. Leave blank to post in the main chat."
    >
      {#if threadGroups.length === 0}
        <p class="text-[13px] leading-relaxed text-neutral-500 dark:text-fg-dim">Enable one or more events above to assign forum topic IDs.</p>
      {:else}
        <div class="flex flex-col gap-5" data-testid="forum-topics">
          {#each threadGroups as group (group.label)}
            <div class="min-w-0">
              <div class="mb-2 text-[11px] font-medium tracking-wide text-neutral-500 uppercase dark:text-fg-dim">{group.label}</div>
              <div class="divide-y divide-neutral-200 overflow-hidden rounded-xl border border-neutral-200 dark:divide-white/[0.06] dark:border-white/[0.08]">
                {#each group.events as event (event.kind)}
                  <div class="grid gap-2 px-3.5 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,14rem)] sm:items-center sm:gap-4">
                    <div class="min-w-0">
                      <div class="truncate text-[13px] font-medium text-black dark:text-fg">{event.label}</div>
                      <div class="text-[11px] text-neutral-500 dark:text-fg-dim">Topic ID</div>
                    </div>
                    <Input
                      type="password"
                      bind:value={() => threadIds[event.kind] ?? '', (v) => (threadIds = { ...threadIds, [event.kind]: v })}
                      placeholder="Optional"
                      inputmode="numeric"
                      form={formId}
                      aria-label="{event.label} topic ID"
                      autocomplete="new-password"
                      data-testid="topic-{event.kind}"
                    />
                  </div>
                {/each}
              </div>
            </div>
          {/each}
          {#if threadErrors}<p class="text-xs text-error">{threadErrors}</p>{/if}
        </div>
      {/if}
    </SettingsSection>
  {/if}
</div>
