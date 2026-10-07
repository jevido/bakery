<script lang="ts">
  // Coolify's Scheduled backup settings
  // (resources/views/livewire/project/database/backup-edit.blade.php and
  // backup-edit/{general,s3,retention,danger}.blade.php;
  // app/Livewire/Project/Database/{BackupEdit,BackupNow}.php; Apache-2.0, see
  // NOTICE), one section per URL. Enable/Disable, Enable/Disable S3 and the
  // S3 storage save at once, as Coolify's do; Frequency and Retention save
  // through the unsaved bar. Of Coolify's fields only those The Bakery has
  // are here: no database selection, timeout or missing-backup alert, one
  // Retention count for local and S3 copies together, and the local copy is
  // always kept.
  import { untrack } from 'svelte'
  import { api, ApiError } from '../../lib/api'
  import { databasePath, go, href, type ScheduledBackupSection } from '../../lib/router.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { BackupExecution, Database, S3Storage, ScheduledBackup, ScheduledBackupInput } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'

  let {
    database,
    scheduledBackup: sb,
    storages,
    section,
    onchange,
  }: {
    database: Database
    scheduledBackup: ScheduledBackup
    storages: S3Storage[] | null
    section: ScheduledBackupSection
    onchange: (s: ScheduledBackup) => void
  } = $props()

  const canUpdate = $derived(projectAccess.can('manage_applications'))
  const running = $derived(database.status === 'running')

  let frequency = $state(untrack(() => sb.cron))
  let retention = $state<number | null | undefined>(untrack(() => sb.retention))
  let pick = $state(untrack(() => String(sb.s3_storage_id ?? storages?.[0]?.id ?? '')))
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)
  let busy = $state(false)

  // The storages arrive after the page; preselect the first for Enable S3.
  $effect(() => {
    if (pick === '' && storages?.length) pick = String(storages[0].id)
  })

  function reset() {
    frequency = sb.cron
    retention = sb.retention
    errors = {}
  }

  const dirty = $derived(
    canUpdate && (section === 'retention' ? retention !== sb.retention : section === '' ? frequency !== sb.cron : false),
  )

  async function patch(change: Partial<ScheduledBackupInput>): Promise<boolean> {
    errors = {}
    try {
      const r = await api<{ scheduled_backup: ScheduledBackup }>('PATCH', `/scheduled-backups/${sb.id}`, {
        enabled: sb.enabled,
        cron: sb.cron,
        retention: sb.retention,
        s3_storage_id: sb.s3_storage_id,
        ...change,
      } satisfies ScheduledBackupInput)
      onchange(r.scheduled_backup)
      return true
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.fromEntries(Object.entries(err.errors).map(([k, v]) => [k.replace('scheduled_backup.', ''), v]))
      if (Object.keys(errors).length === 0) toast.error('Backup not updated', err.message)
      return false
    }
  }

  async function save() {
    if (saving || !dirty) return
    saving = true
    const change = section === 'retention' ? { retention: retention ?? 0 } : { cron: frequency.trim() }
    if (await patch(change)) toast.success('Backup updated successfully.')
    saving = false
  }

  async function instant(change: Partial<ScheduledBackupInput>, message: string) {
    busy = true
    if (await patch(change)) toast.success(message)
    busy = false
  }

  async function backUpNow() {
    busy = true
    try {
      await api<{ backup_execution: BackupExecution }>('POST', `/scheduled-backups/${sb.id}/backup-executions`)
      toast.success('Backup queued. It will be available in a few minutes.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Backup not started', err.message)
    } finally {
      busy = false
    }
  }

  async function remove(selected: string[]) {
    const q = new URLSearchParams({
      delete_local: String(selected.includes('delete_associated_backups_locally')),
      delete_s3: String(selected.includes('delete_associated_backups_s3')),
    })
    try {
      await api('DELETE', `/scheduled-backups/${sb.id}?${q}`)
    } catch (err) {
      toast.error('Backup schedule not deleted', err instanceof Error ? err.message : String(err))
      throw err
    }
    toast.success('Backup schedule deleted.')
    go(databasePath(database, 'backups'))
  }

  const utc = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC' })
  const storageName = $derived(storages?.find((s) => s.id === sb.s3_storage_id)?.name)
</script>

{#if section === 'danger'}
  <div class="application-settings-form">
    <SettingsSection
      id="delete-backup-schedule-section"
      title="Delete backup schedule"
      helper="Permanently remove this schedule and optionally its backup archives."
    >
      <div class="chrome flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="min-w-0">
          <h4 class="text-sm font-semibold text-red-700 dark:text-red-300">This action cannot be undone.</h4>
          <div class="mt-2 max-w-2xl space-y-2 text-[13px] leading-5 text-red-700/80 dark:text-red-300/80">
            <p>You can select which backup archives to remove.</p>
          </div>
        </div>
        <div class="shrink-0">
          <ConfirmationModal
            title="Confirm Backup Schedule Deletion?"
            buttonTitle="Delete schedule"
            variant="error"
            checkboxes={[
              { id: 'delete_associated_backups_locally', label: 'All backups will be permanently deleted from local storage.' },
              {
                id: 'delete_associated_backups_s3',
                label: 'All backups will be permanently deleted (associated with this backup job) from the selected S3 Storage.',
              },
            ]}
            actions={[
              'The selected backup schedule will be deleted.',
              'Scheduled backups for this database will stop if this is its only schedule.',
            ]}
            confirmationText={database.name}
            confirmationLabel="Enter the database name to confirm deletion."
            shortConfirmationLabel="Database Name"
            onconfirm={remove}
          />
        </div>
      </div>
    </SettingsSection>
  </div>
{:else if section === 's3'}
  {#if storages && storages.length === 0}
    <SettingsSection id="s3-storage-section" title="S3 storage" helper="Send backup archives to a validated object storage destination." flush>
      <Empty title="No validated S3 storage" description="Add and validate an S3 storage destination before enabling remote backups." icon="storages">
        {#if projectAccess.can('manage_servers')}<a class="button" href={href('/storages')}>Open S3 storage</a>{/if}
      </Empty>
    </SettingsSection>
  {:else}
    <SettingsSection
      id="s3-storage-section"
      title="S3 storage"
      helper="Choose where remote backups are stored. The Bakery always keeps the local copy too."
    >
      {#snippet actions()}
        {#if canUpdate && storages}
          {#if sb.s3_storage_id == null}
            <Button
              variant="highlighted"
              disabled={busy || pick === ''}
              onclick={() => instant({ s3_storage_id: Number(pick) }, 'S3 backups enabled.')}>Enable S3</Button
            >
          {:else}
            <Button disabled={busy} onclick={() => instant({ s3_storage_id: null }, 'S3 backups disabled.')}>Disable S3</Button>
          {/if}
        {/if}
      {/snippet}
      <div class="grid gap-4 sm:grid-cols-2">
        {#if storages}
          <Select
            label="S3 storage"
            required={sb.s3_storage_id != null}
            bind:value={pick}
            error={errors.s3_storage_id}
            disabled={!canUpdate || busy}
            onchange={() => sb.s3_storage_id != null && instant({ s3_storage_id: Number(pick) }, 'Backup updated successfully.')}
            data-testid="backup-s3-storage"
          >
            {#each storages as s (s.id)}<option value={String(s.id)}>{s.name}</option>{/each}
          </Select>
        {:else}
          <Input label="S3 storage" disabled value={sb.s3_storage_id == null ? 'Local only' : (storageName ?? 'Hidden (viewers cannot see it)')} />
        {/if}
        <Input label="Status" disabled value={sb.s3_storage_id == null ? 'Disabled' : 'Enabled'} data-testid="backup-s3-status" />
      </div>
    </SettingsSection>
  {/if}
{:else}
  <form
    onsubmit={(e) => {
      e.preventDefault()
      save()
    }}
  >
    {#if canUpdate}
      <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
    {/if}
    {#if section === 'retention'}
      <SettingsSection
        id="retention-section"
        title="Retention"
        helper="Once more backups exist than you keep, the oldest is removed, from local storage and S3 storage together."
      >
        <div>
          <h3 class="mb-3 text-sm font-semibold text-black dark:text-fg">Local backups</h3>
          <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <Input
              label="Backups to keep"
              type="number"
              min="1"
              max="100"
              required
              bind:value={retention}
              error={errors.retention}
              disabled={!canUpdate}
              helper="Maximum number of recent backups, between 1 and 100. Their S3 copies follow."
            />
          </div>
        </div>
      </SettingsSection>
    {:else}
      <SettingsSection id="backup-schedule-section" title="Backup schedule" helper="Choose when the backup runs.">
        {#snippet actions()}
          {#if canUpdate}
            <div class="flex items-center gap-2">
              {#if !sb.enabled}
                <Button variant="highlighted" disabled={busy} onclick={() => instant({ enabled: true }, 'Backup enabled.')}>Enable backup</Button>
              {:else}
                <Button disabled={busy} onclick={() => instant({ enabled: false }, 'Backup disabled.')}>Disable backup</Button>
              {/if}
              <Button
                disabled={busy || !running}
                title={running ? undefined : 'The database must be running to start a backup.'}
                onclick={backUpNow}>Back up now</Button
              >
            </div>
          {/if}
        {/snippet}
        <div class="space-y-5">
          <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <Input
              label="Frequency"
              required
              bind:value={frequency}
              error={errors.cron}
              disabled={!canUpdate}
              helper="You can use every_minute, hourly, daily, weekly, monthly, yearly or a cron expression."
            />
            <Input label="Timezone" disabled value="UTC" required helper="Scheduled backups run in UTC." />
          </div>
          <p class="text-xs text-neutral-500 dark:text-fg-dim" data-testid="next-backup">
            {#if sb.enabled && sb.next_backup_at}
              Next backup: {utc.format(new Date(sb.next_backup_at))} UTC
            {:else}
              This schedule is disabled.
            {/if}
          </p>
        </div>
      </SettingsSection>
    {/if}
  </form>
{/if}
