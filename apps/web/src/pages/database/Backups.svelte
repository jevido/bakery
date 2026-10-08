<script lang="ts">
  // Coolify's Database Backups page
  // (resources/views/livewire/project/database/backup/index.blade.php,
  // scheduled-backups.blade.php and create-scheduled-backup.blade.php;
  // app/Livewire/Project/Database/{ScheduledBackups,CreateScheduledBackup}.php;
  // Apache-2.0, see NOTICE): the summary, the Scheduled backups with their
  // latest Backup execution, search, and "+ Add" asking for a Frequency.
  // A new Scheduled backup starts on, local only, as Coolify's does, and
  // keeps The Bakery's default Retention.
  //
  // Laid out as Paperclip's list pattern (ui/src/pages/ProjectDetail.tsx,
  // MIT), as Deployment history is: a CollectionToolbar, the Scheduled
  // backups as EntityRows in one card.
  import { Card } from '@bakery/ui/components/ui/card'
  import { api, ApiError } from '../../lib/api'
  import CollectionToolbar from '../../lib/CollectionToolbar.svelte'
  import EntityRow from '../../lib/EntityRow.svelte'
  import { databasePath, go, href } from '../../lib/router.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import SearchField from '../../lib/SearchField.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import type { BackupExecution, Database, S3Storage, ScheduledBackup, ScheduledBackupInput } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import { executionStatus } from './backupStatus'

  let { database }: { database: Database } = $props()

  let schedules = $state.raw<ScheduledBackup[] | null>(null)
  let executions = $state.raw<BackupExecution[]>([])
  let storages = $state.raw<S3Storage[]>([])
  let loadError = $state('')
  let searchText = $state('')

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
    if (projectAccess.can('see_secrets')) {
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
  const shown = $derived((schedules ?? []).filter((s) => matches(s, searchText)))

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

<div class="flex min-w-0 flex-col gap-6">
  <SettingsGroup
    id="database-backups-section"
    label="Database backups"
    hint="Automate database backups and track the latest execution for each schedule."
  >
    {#snippet actions()}
      {#if projectAccess.can('manage_applications')}
        <Modal title="New Scheduled Backup" bind:open={adding} onclose={() => (addErrors = {})}>
          {#snippet trigger(show)}
            <Button variant="highlighted" onclick={show}>+ Add</Button>
          {/snippet}
          <form class="flex w-full flex-col gap-4" onsubmit={add}>
            <Input
              label="Frequency"
              placeholder="0 0 * * * or daily"
              helper="You can use every_minute, hourly, daily, weekly, monthly, yearly or a cron expression. Times are UTC."
              required
              bind:value={frequency}
              error={addErrors.cron}
            />
            <div class="flex justify-end border-t border-border pt-4">
              <Button type="submit" variant="highlighted" loading={saving}>Add schedule</Button>
            </div>
          </form>
        </Modal>
      {/if}
    {/snippet}
    <div class="grid gap-4 sm:grid-cols-3" data-testid="backup-summary">
      {#each [['Schedules', schedules?.length ?? 0], ['Enabled', (schedules ?? []).filter((s) => s.enabled).length], ['Total executions', executions.length]] as [label, n] (label)}
        <div>
          <p class="text-xs font-medium text-muted-foreground">{label}</p>
          <p class="mt-1 text-xl font-semibold text-foreground tabular-nums">{n}</p>
        </div>
      {/each}
    </div>
  </SettingsGroup>

  {#if loadError}
    <p class="text-sm text-destructive">{loadError}</p>
  {:else if schedules === null}
    <Spinner text="Loading…" />
  {:else if schedules.length === 0}
    <Empty size="sm" title="No scheduled backups" description="Create a schedule to start protecting this database." icon="storages" />
  {:else}
    <div class="flex min-w-0 flex-col gap-3">
      <CollectionToolbar ariaLabel="Scheduled backups controls">
        {#snippet search()}
          <SearchField bind:value={searchText} label="Search backups" />
        {/snippet}
      </CollectionToolbar>

      {#if shown.length > 0}
        <Card class="block gap-0 overflow-hidden py-0" data-testid="scheduled-backups">
          {#each shown as s (s.id)}
            {@const own = bySchedule.get(s.id) ?? []}
            {@const latest = own[0] ? executionStatus(own[0].status) : { label: 'Never run', type: 'neutral' as const }}
            <EntityRow href={href(backupPath(s))} title={s.cron} subtitle={storageName(s)} reserveSubtitleSpace data-testid="scheduled-backup-row">
              {#snippet trailing()}
                <a
                  href={href(backupPath(s, 'executions'))}
                  class="hidden text-xs text-muted-foreground hover:text-foreground hover:underline sm:inline"
                >
                  {own.length} {own.length === 1 ? 'execution' : 'executions'}
                </a>
                <StatusBadge status={latest.label} type={latest.type} />
                {#if own[0]?.status === 'running'}<Spinner />{/if}
              {/snippet}
            </EntityRow>
          {/each}
          <footer class="flex min-h-11 items-center border-t border-border px-4 text-xs text-muted-foreground">
            <span>{shown.length} {shown.length === 1 ? 'schedule' : 'schedules'}</span>
          </footer>
        </Card>
      {:else}
        <Card class="block py-0">
          <Empty size="sm" title="No matching backup schedules" description="Try another database name, frequency, or storage name." />
        </Card>
      {/if}
    </div>
  {/if}
</div>
