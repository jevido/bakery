<script lang="ts">
  // Coolify's Database Backups page
  // (resources/views/livewire/project/database/backup/index.blade.php,
  // scheduled-backups.blade.php and create-scheduled-backup.blade.php;
  // app/Livewire/Project/Database/{ScheduledBackups,CreateScheduledBackup}.php;
  // Apache-2.0, see NOTICE): the summary, the Scheduled backups with their
  // latest Backup execution, search, and "+ Add" asking for a Frequency.
  // A new Scheduled backup starts on, local only, as Coolify's does, and
  // keeps The Bakery's default Retention.
  import { api, ApiError } from '../../lib/api'
  import Icon from '../../lib/Icon.svelte'
  import { databasePath, go, href } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { BackupExecution, Database, S3Storage, ScheduledBackup, ScheduledBackupInput } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import { executionStatus } from './backupStatus'

  let { database }: { database: Database } = $props()

  let schedules = $state.raw<ScheduledBackup[] | null>(null)
  let executions = $state.raw<BackupExecution[]>([])
  let storages = $state.raw<S3Storage[]>([])
  let loadError = $state('')
  let search = $state('')

  async function load() {
    const [s, e] = await Promise.all([
      api<{ scheduled_backups: ScheduledBackup[] }>('GET', `/databases/${database.id}/scheduled-backups`),
      api<{ backup_executions: BackupExecution[] }>('GET', `/databases/${database.id}/backup-executions`),
    ])
    schedules = s.scheduled_backups
    executions = e.backup_executions
  }

  $effect(() => {
    void database.id
    load().catch((e) => (loadError = e.message))
    if (session.can('see_secrets')) {
      api<{ s3_storages: S3Storage[] }>('GET', '/s3-storages')
        .then((r) => (storages = r.s3_storages))
        .catch(() => {})
    }
  })

  // Refreshed while a Backup execution runs, as Coolify's latest run spinner.
  const running = $derived(executions.some((b) => b.status === 'running'))
  $effect(() => {
    if (!running) return
    const t = setInterval(() => load().catch(() => {}), 2000)
    return () => clearInterval(t)
  })

  // Executions are listed newest first, so the first of each is its latest.
  const bySchedule = $derived.by(() => {
    const out = new Map<number, BackupExecution[]>()
    for (const b of executions) out.set(b.scheduled_backup_id, [...(out.get(b.scheduled_backup_id) ?? []), b])
    return out
  })

  function storageName(s: ScheduledBackup): string {
    if (s.s3_storage_id == null) return 'Local only'
    return storages.find((x) => x.id === s.s3_storage_id)?.name ?? 'S3 storage'
  }

  function matches(s: ScheduledBackup, q: string): boolean {
    q = q.toLowerCase()
    return (
      q === '' ||
      database.name.toLowerCase().includes(q) ||
      s.cron.toLowerCase().includes(q) ||
      (s.s3_storage_id != null && storageName(s).toLowerCase().includes(q))
    )
  }
  const shown = $derived((schedules ?? []).filter((s) => matches(s, search)))

  function backupPath(s: ScheduledBackup, section = ''): string {
    return `${databasePath(database, 'backups')}/${s.id}${section ? `/${section}` : ''}`
  }

  // + Add
  let adding = $state(false)
  let frequency = $state('')
  let addErrors = $state<Record<string, string>>({})
  let saving = $state(false)

  async function add(e: SubmitEvent) {
    e.preventDefault()
    saving = true
    addErrors = {}
    try {
      const r = await api<{ scheduled_backup: ScheduledBackup }>('POST', `/databases/${database.id}/scheduled-backups`, {
        enabled: true,
        cron: frequency,
        retention: 7,
        s3_storage_id: null,
      } satisfies ScheduledBackupInput)
      adding = false
      frequency = ''
      go(backupPath(r.scheduled_backup))
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      addErrors = { cron: err.errors['scheduled_backup.cron'] ?? err.message }
    } finally {
      saving = false
    }
  }
</script>

<div class="application-settings-form flex min-w-0 flex-col gap-6">
  <SettingsSection
    id="database-backups-section"
    title="Database backups"
    helper="Automate database backups and track the latest execution for each schedule."
  >
    {#snippet actions()}
      {#if session.can('manage_applications')}
        <Modal title="New Scheduled Backup" bind:open={adding} onclose={() => (addErrors = {})}>
          {#snippet trigger(show)}
            <Button variant="highlighted" onclick={show}>+ Add</Button>
          {/snippet}
          <form class="application-settings-form flex w-full flex-col gap-4" onsubmit={add}>
            <Input
              label="Frequency"
              placeholder="0 0 * * * or daily"
              helper="You can use every_minute, hourly, daily, weekly, monthly, yearly or a cron expression. Times are UTC."
              required
              bind:value={frequency}
              error={addErrors.cron}
            />
            <div class="flex justify-end border-t border-neutral-200 pt-4 dark:border-white/[0.08]">
              <Button type="submit" variant="highlighted" loading={saving}>Add schedule</Button>
            </div>
          </form>
        </Modal>
      {/if}
    {/snippet}
    <div class="grid gap-4 sm:grid-cols-3" data-testid="backup-summary">
      {#each [['Schedules', schedules?.length ?? 0], ['Enabled', (schedules ?? []).filter((s) => s.enabled).length], ['Total executions', executions.length]] as [label, n] (label)}
        <div>
          <p class="text-xs font-medium text-neutral-500 dark:text-fg-dim">{label}</p>
          <p class="mt-1 text-xl font-semibold text-neutral-950 tabular-nums dark:text-fg">{n}</p>
        </div>
      {/each}
    </div>
  </SettingsSection>

  {#if loadError}
    <p class="chrome text-sm text-error">{loadError}</p>
  {:else if schedules === null}
    <div class="chrome"><Spinner text="Loading…" /></div>
  {:else if schedules.length === 0}
    <Empty size="sm" title="No scheduled backups" description="Create a schedule to start protecting this database." icon="storages" />
  {:else}
    <div class="chrome flex min-w-0 flex-col gap-3">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="relative max-w-sm">
          <Icon
            name="search"
            class="pointer-events-none absolute top-1/2 left-2.5 z-10 size-3.5 -translate-y-1/2 text-neutral-400 dark:text-fg-faint"
          />
          <input type="search" bind:value={search} aria-label="Search backups" class="input h-8! w-full pl-8! text-[13px]!" placeholder="Search backups" />
        </div>
      </div>

      {#if shown.length > 0}
        <div
          class="data-table overflow-hidden rounded-xl border border-neutral-200 bg-white shadow-sm dark:border-white/[0.08] dark:bg-white/[0.05]"
          data-testid="scheduled-backups"
        >
          <div class="data-table-header scheduled-backups-table-grid">
            <span>Schedule</span>
            <span>Latest run</span>
            <span>S3 storage</span>
            <span>Executions</span>
            <span class="text-right">Action</span>
          </div>
          {#each shown as s (s.id)}
            {@const own = bySchedule.get(s.id) ?? []}
            {@const latest = own[0] ? executionStatus(own[0].status) : { label: 'Never run', type: 'neutral' as const }}
            <div class="data-table-row scheduled-backups-table-grid border-b border-neutral-200 last:border-b-0 dark:border-white/[0.06]">
              <div class="min-w-0">
                <span class="block truncate text-[12px] font-semibold text-black dark:text-fg">{s.cron}</span>
              </div>
              <div class="flex items-center gap-2">
                <StatusBadge status={latest.label} type={latest.type} />
                {#if own[0]?.status === 'running'}<Spinner />{/if}
              </div>
              <div class="truncate text-[11px] text-neutral-600 dark:text-fg-dim">{storageName(s)}</div>
              <div class="text-[11px] text-neutral-600 dark:text-fg-dim">
                <a href={href(backupPath(s, 'executions'))} class="font-medium hover:text-black hover:underline dark:hover:text-fg">{own.length}</a>
              </div>
              <div class="flex justify-end">
                <a class="button" href={href(backupPath(s))}>Manage</a>
              </div>
            </div>
          {/each}
          <div
            class="flex min-h-11 items-center border-t border-neutral-200 px-4 text-[11px] text-neutral-500 dark:border-white/[0.08] dark:text-fg-faint"
          >
            <span>{shown.length} {shown.length === 1 ? 'schedule' : 'schedules'}</span>
          </div>
        </div>
      {:else}
        <div class="border-t border-neutral-200 dark:border-white/[0.06]">
          <Empty size="sm" title="No matching backup schedules" description="Try another database name, frequency, or storage name." />
        </div>
      {/if}
    </div>
  {/if}
</div>
