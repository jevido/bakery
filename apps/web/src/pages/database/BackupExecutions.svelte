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
  //
  // Laid out as the Environment Variables list is (lib/EnvironmentVariables.svelte,
  // after phase 29 task 04): dense rows in a bordered card, ClientPagination
  // under them (Coolify's ten to a page, in the browser).
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import { api, ApiError } from '../../lib/api'
  import { ago, duration, size } from '../../lib/format'
  import Icon from '../../lib/Icon.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import type { BackupExecution, Database, ScheduledBackup } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ClientPagination from '../../lib/ui/ClientPagination.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import { executionStatus } from './backupStatus'

  let {
    database,
    scheduledBackup: sb,
    ondatabase,
  }: { database: Database; scheduledBackup: ScheduledBackup; ondatabase: (d: Database) => void } = $props()

  let executions = $state.raw<BackupExecution[] | null>(null)
  let page = $state(1)
  let pageSize = $state(10)
  let loadError = $state('')
  let now = $state(Date.now())

  async function load() {
    const r = await api<{ backup_executions: BackupExecution[] }>('GET', `/scheduled-backups/${sb.id}/backup-executions`)
    executions = r.backup_executions
    now = Date.now()
    const lastPage = Math.max(1, Math.ceil(executions.length / pageSize))
    if (page > lastPage) page = lastPage
  }

  $effect(() => {
    void sb.id
    load().catch((e) => (loadError = e.message))
  })

  const active = $derived(database.restoring || (executions ?? []).some((b) => b.status === 'running'))
  $effect(() => {
    // Coolify polls the first page only.
    if (page > 1 && !active) return
    const t = setInterval(() => load().catch(() => {}), active ? 2000 : 5000)
    return () => clearInterval(t)
  })

  const shown = $derived((executions ?? []).slice((page - 1) * pageSize, page * pageSize))
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
  const pageSizeKey = 'bakery.page-size.backup-executions'
</script>

<SettingsGroup id="backup-executions-section" label="Executions" hint="Review generated archives, storage availability, and backup output.">
  {#snippet actions()}
    {#if projectAccess.can('manage_applications')}
      <Button loading={cleaning} onclick={cleanupFailed}>Clean failed backups</Button>
    {/if}
  {/snippet}

  {#if database.status !== 'running' || database.restoring || database.last_restore}
    <div class="rounded-md border border-border px-4 py-3 text-sm" role="status">
      {#if database.restoring}
        <span class="inline-flex items-center gap-2"><Spinner /> Restoring…</span>
      {:else if database.status !== 'running'}
        <span class="text-muted-foreground">Start the database to back it up or restore it.</span>
      {:else if database.last_restore?.error}
        <span class="text-destructive">The last restore failed: {database.last_restore.error}</span>
      {:else if database.last_restore}
        <span class="text-success">Restored at {when.format(new Date(database.last_restore.finished_at))}.</span>
      {/if}
    </div>
  {/if}

  <div class="overflow-hidden rounded-md border border-border" data-testid="backup-executions">
    {#if loadError}
      <p class="p-4 text-sm text-destructive">{loadError}</p>
    {:else if executions === null}
      <div class="p-4"><Spinner text="Loading…" /></div>
    {:else if total === 0}
      <div class="p-4">
        <Empty size="sm" title="No backup executions" description="Execution history appears here after the schedule runs." icon="browser-terminal" />
      </div>
    {:else}
      {#each shown as b (b.id)}
        {@const status = executionStatus(b.status)}
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 border-b border-border px-4 py-2.5 last:border-b-0" data-testid="backup-execution">
          <div class="flex items-center gap-2">
            <StatusBadge status={status.label} type={status.type} />
            {#if b.status === 'running'}<Spinner />{/if}
          </div>
          <span class="text-xs font-medium text-foreground">{b.trigger === 'manual' ? 'Manual' : 'Scheduled'}</span>
          <div class="flex min-w-0 shrink-0 basis-56 items-center gap-1">
            <code
              class="truncate font-mono text-xs text-muted-foreground"
              style="user-select: all"
              title={`Backup path: ${b.file_name}`}>{b.file_name}</code
            >
            <button
              type="button"
              class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'size-6 shrink-0 text-muted-foreground' })}
              title="Copy backup path"
              aria-label="Copy backup path"
              onclick={() => copy(b)}
            >
              <Icon name={copied === b.id ? 'check' : 'file-content'} class="size-3.5" />
            </button>
          </div>
          <span class="text-xs text-muted-foreground" title={b.finished_at ? when.format(new Date(b.finished_at)) : undefined}>
            {b.status === 'running' ? 'Running now' : b.finished_at ? ago(b.finished_at, now) : '-'}
          </span>
          <span class="text-xs text-muted-foreground">
            {duration(b.started_at, b.status === 'running' || !b.finished_at ? new Date(now) : b.finished_at)}
          </span>
          <span class="text-xs text-muted-foreground">{b.size_bytes ? size(b.size_bytes) : '-'}</span>
          <div class="flex flex-wrap items-center gap-1.5">
            <StatusBadge label="Local" status={b.local ? 'Available' : 'Deleted'} type={b.local ? 'success' : 'neutral'} />
            {#if b.s3}<StatusBadge label="S3" status="Available" type="success" />{/if}
          </div>
          <div class="ml-auto flex items-center gap-1">
            {#if projectAccess.can('manage_applications')}
              {#if b.status === 'succeeded'}
                <a
                  class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'size-7 shrink-0 text-muted-foreground' })}
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
                      class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'size-7 shrink-0 text-muted-foreground' })}
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
                      class={buttonVariants({ variant: 'ghost', size: 'icon-sm', class: 'size-7 shrink-0 text-destructive hover:text-destructive' })}
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
          {#if b.error}
            <pre
              class="mt-1 w-full max-h-48 overflow-auto rounded-lg bg-muted p-3 font-mono text-xs leading-5 whitespace-pre-wrap text-muted-foreground">{b.error}</pre>
          {/if}
        </div>
      {/each}
      <ClientPagination bind:page bind:pageSize {total} storageKey={pageSizeKey} />
    {/if}
  </div>
</SettingsGroup>
