<script lang="ts">
  // Coolify's Backup executions
  // (resources/views/livewire/project/database/backup-executions.blade.php and
  // app/Livewire/Project/Database/BackupExecutions.php, Apache-2.0, see
  // NOTICE): ten to a page, refreshed every five seconds (two while one
  // runs), with download and delete. Restore sits here too, behind a
  // confirmation that names the Database, where Coolify has an Import page.
  // The Bakery's rows show the trigger where Coolify shows the dumped
  // database's name, and never outlive their local file, so Coolify's
  // "Clean deleted entries" has nothing to clean. A Backup execution's
  // contents are Secrets: viewers see the list without any action.
  import { api, ApiError } from '../../lib/api'
  import { ago, duration, size } from '../../lib/format'
  import Icon from '../../lib/Icon.svelte'
  import { session } from '../../lib/session.svelte'
  import type { BackupExecution, Database, ScheduledBackup } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import { executionStatus } from './backupStatus'

  let {
    database,
    scheduledBackup: sb,
    ondatabase,
  }: { database: Database; scheduledBackup: ScheduledBackup; ondatabase: (d: Database) => void } = $props()

  const take = 10
  let executions = $state.raw<BackupExecution[] | null>(null)
  let skip = $state(0)
  let loadError = $state('')
  let now = $state(Date.now())

  async function load() {
    const r = await api<{ backup_executions: BackupExecution[] }>('GET', `/scheduled-backups/${sb.id}/backup-executions`)
    executions = r.backup_executions
    now = Date.now()
    if (skip >= executions.length) skip = Math.max(0, Math.floor((executions.length - 1) / take) * take)
  }

  $effect(() => {
    void sb.id
    load().catch((e) => (loadError = e.message))
  })

  const active = $derived(database.restoring || (executions ?? []).some((b) => b.status === 'running'))
  $effect(() => {
    // Coolify polls the first page only.
    if (skip > 0 && !active) return
    const t = setInterval(() => load().catch(() => {}), active ? 2000 : 5000)
    return () => clearInterval(t)
  })

  const page = $derived((executions ?? []).slice(skip, skip + take))
  const total = $derived(executions?.length ?? 0)

  async function refreshDatabase() {
    const r = await api<{ database: Database }>('GET', `/databases/${database.id}`)
    ondatabase(r.database)
  }

  async function remove(b: BackupExecution, selected: string[]) {
    try {
      await api('DELETE', `/backup-executions/${b.id}?delete_s3=${selected.includes('delete_backup_s3')}`)
    } catch (err) {
      toast.error('Backup not deleted', err instanceof Error ? err.message : String(err))
      throw err
    }
    toast.success('Backup deleted.')
    await load()
  }

  let cleaning = $state(false)
  async function cleanupFailed() {
    cleaning = true
    try {
      const failed = (executions ?? []).filter((b) => b.status === 'failed')
      for (const b of failed) await api('DELETE', `/backup-executions/${b.id}?delete_s3=true`)
      toast.success(failed.length ? 'Failed backups cleaned up.' : 'No failed backups to clean up.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Failed backups not cleaned up', err.message)
    } finally {
      cleaning = false
      await load().catch(() => {})
    }
  }

  async function restore(b: BackupExecution) {
    try {
      await api('POST', `/backup-executions/${b.id}/restore`)
    } catch (err) {
      toast.error('Backup not restored', err instanceof Error ? err.message : String(err))
      throw err
    }
    toast.info('Restoring the backup.')
    await refreshDatabase().catch(() => {})
  }

  let copied = $state<number | null>(null)
  async function copy(b: BackupExecution) {
    await navigator.clipboard?.writeText(b.file_name)
    copied = b.id
    setTimeout(() => (copied = null), 1000)
  }

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })
</script>

<section class="application-settings-section overflow-hidden">
  <div class="application-settings-section-header">
    <div>
      <h2>Executions</h2>
      <p>Review generated archives, storage availability, and backup output.</p>
    </div>
    {#if session.can('manage_applications')}
      <div class="flex flex-wrap items-center gap-2">
        <Button loading={cleaning} onclick={cleanupFailed}>Clean failed backups</Button>
      </div>
    {/if}
  </div>

  <div class="application-settings-section-body p-0!">
    {#if database.status !== 'running' || database.restoring || database.last_restore}
      <div class="chrome border-b border-neutral-200 px-4 py-3 text-[13px] dark:border-white/[0.06]" role="status">
        {#if database.restoring}
          <span class="inline-flex items-center gap-2"><Spinner /> Restoring…</span>
        {:else if database.status !== 'running'}
          <span class="text-neutral-600 dark:text-fg-dim">Start the database to back it up or restore it.</span>
        {:else if database.last_restore?.error}
          <span class="text-error">The last restore failed: {database.last_restore.error}</span>
        {:else if database.last_restore}
          <span class="text-success">Restored at {when.format(new Date(database.last_restore.finished_at))}.</span>
        {/if}
      </div>
    {/if}

    {#if loadError}
      <p class="chrome p-4 text-sm text-error">{loadError}</p>
    {:else if executions === null}
      <div class="chrome p-4"><Spinner text="Loading…" /></div>
    {:else if total === 0}
      <div class="p-4">
        <Empty size="sm" title="No backup executions" description="Execution history appears here after the schedule runs." icon="browser-terminal" />
      </div>
    {:else}
      <div class="chrome data-table deployment-table-scroll backup-executions-table-scroll" data-testid="backup-executions">
        <div class="data-table-header backup-executions-table-grid h-auto rounded-none px-4 py-2.5 text-[11px]">
          <span>Status</span>
          <span>Trigger</span>
          <span>Backup path</span>
          <span>Finished</span>
          <span>Duration</span>
          <span>Size</span>
          <span>Availability</span>
          <span class="text-right">Actions</span>
        </div>
        {#each page as b (b.id)}
          {@const status = executionStatus(b.status)}
          <div class="backup-execution-row" data-testid="backup-execution">
            <div
              class="data-table-row backup-executions-table-grid min-h-14 border-b border-neutral-200 px-4 py-2.5 dark:border-white/[0.06]"
            >
              <div class="flex items-center gap-2">
                <StatusBadge status={status.label} type={status.type} />
                {#if b.status === 'running'}<Spinner />{/if}
              </div>
              <div class="truncate text-[12px] font-medium text-black dark:text-fg">{b.trigger === 'manual' ? 'Manual' : 'Scheduled'}</div>
              <div class="flex min-w-0 items-center gap-1">
                <code class="truncate font-mono text-[11px] text-neutral-600 select-all dark:text-fg-dim" title={`Backup path: ${b.file_name}`}
                  >{b.file_name}</code
                >
                <button type="button" class="icon-button shrink-0" title="Copy backup path" aria-label="Copy backup path" onclick={() => copy(b)}>
                  <Icon name={copied === b.id ? 'check' : 'file-content'} class="size-3.5" />
                </button>
              </div>
              <div class="text-[11px] text-neutral-600 dark:text-fg-dim" title={b.finished_at ? when.format(new Date(b.finished_at)) : undefined}>
                {b.status === 'running' ? 'Running now' : b.finished_at ? ago(b.finished_at, now) : '-'}
              </div>
              <div class="text-[11px] text-neutral-600 dark:text-fg-dim">
                {duration(b.started_at, b.status === 'running' || !b.finished_at ? new Date(now) : b.finished_at)}
              </div>
              <div class="text-[11px] text-neutral-600 dark:text-fg-dim">{b.size_bytes ? size(b.size_bytes) : '-'}</div>
              <div class="flex flex-wrap items-center gap-1.5">
                <StatusBadge label="Local" status={b.local ? 'Available' : 'Deleted'} type={b.local ? 'success' : 'neutral'} />
                {#if b.s3}<StatusBadge label="S3" status="Available" type="success" />{/if}
              </div>
              <div class="flex items-center justify-end gap-1">
                {#if session.can('manage_applications')}
                  {#if b.status === 'succeeded'}
                    <a
                      class="icon-button shrink-0"
                      href={`/api/backup-executions/${b.id}/download`}
                      download={b.file_name}
                      title="Download backup"
                      aria-label="Download backup"
                    >
                      <Icon name="upload" class="size-3.5 rotate-180" />
                    </a>
                    <ConfirmationModal
                      title="Confirm Backup Restore?"
                      actions={[
                        `The data in ${database.name} will be replaced with this backup.`,
                        'Anything written to the database since this backup will be lost.',
                      ]}
                      confirmationText={database.name}
                      confirmationLabel="Enter the database name to confirm the restore."
                      shortConfirmationLabel="Database Name"
                      warningMessage="This operation replaces the database's data and cannot be undone."
                      onconfirm={() => restore(b)}
                    >
                      {#snippet trigger(show)}
                        <button
                          type="button"
                          class="icon-button shrink-0"
                          title="Restore backup"
                          aria-label="Restore backup"
                          disabled={active || database.status !== 'running'}
                          onclick={show}
                        >
                          <Icon name="time-back" class="size-3.5" />
                        </button>
                      {/snippet}
                    </ConfirmationModal>
                  {/if}
                  {#if b.status !== 'running'}
                    <ConfirmationModal
                      title="Confirm Backup Deletion?"
                      variant="error"
                      checkboxes={b.s3 ? [{ id: 'delete_backup_s3', label: 'Delete the selected backup permanently from S3 Storage' }] : []}
                      actions={[b.local ? 'This backup will be permanently deleted from local storage.' : 'This backup execution record will be deleted.']}
                      confirmationText={b.file_name}
                      confirmationLabel="Enter the backup filename to confirm."
                      shortConfirmationLabel="Backup Filename"
                      onconfirm={(selected) => remove(b, selected)}
                    >
                      {#snippet trigger(show)}
                        <button
                          type="button"
                          class="icon-button shrink-0 text-red-500 hover:text-red-600 dark:text-red-400 dark:hover:text-red-300"
                          title="Delete backup"
                          aria-label="Delete backup"
                          onclick={show}
                        >
                          <Icon name="trash" class="size-3.5" />
                        </button>
                      {/snippet}
                    </ConfirmationModal>
                  {/if}
                {/if}
              </div>
            </div>
            {#if b.error}
              <div class="border-t border-neutral-200 bg-neutral-50 px-4 py-3 dark:border-white/[0.06] dark:bg-white/[0.02]">
                <pre
                  class="max-h-48 overflow-auto rounded-lg bg-neutral-100 p-3 font-mono text-xs leading-5 whitespace-pre-wrap text-neutral-700 dark:bg-black/20 dark:text-fg-dim">{b.error}</pre>
              </div>
            {/if}
          </div>
        {/each}
      </div>

      <div
        class="chrome flex min-h-11 items-center justify-between border-t border-neutral-200 px-4 text-[11px] text-neutral-500 dark:border-white/[0.08] dark:text-fg-faint"
      >
        <span>{skip + 1}-{Math.min(skip + take, total)} of {total}</span>
        <div class="flex items-center gap-1">
          <button type="button" class="icon-button" disabled={skip === 0} onclick={() => (skip = Math.max(0, skip - take))} aria-label="Previous page">
            <Icon name="arrow-right" class="size-3.5 rotate-180" />
          </button>
          <button type="button" class="icon-button" disabled={skip + take >= total} onclick={() => (skip += take)} aria-label="Next page">
            <Icon name="arrow-right" class="size-3.5" />
          </button>
        </div>
      </div>
    {/if}
  </div>
</section>
