<script lang="ts">
  // Coolify's Scheduled backup page
  // (resources/views/livewire/project/database/backup/execution.blade.php and
  // components/backup-sidebar.blade.php, Apache-2.0, see NOTICE): the backup
  // sidebar in place of the Database's, and the section the URL names,
  // drawn with ResourceNav as the Database's own ConfigurationSidebar is
  // (the choice over Paperclip's PageTabBar is recorded in NOTES).
  import ArrowLeft from '@lucide/svelte/icons/arrow-left'
  import { api } from '../../lib/api'
  import type { IconName } from '../../lib/Icon.svelte'
  import { databasePath, href, type ScheduledBackupSection } from '../../lib/router.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import ResourceNav from '../../lib/ResourceNav.svelte'
  import SidebarNavItem from '../../lib/SidebarNavItem.svelte'
  import type { Database, S3Storage, ScheduledBackup } from '../../lib/types'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import BackupEdit from './BackupEdit.svelte'
  import BackupExecutions from './BackupExecutions.svelte'

  let {
    database,
    id,
    section,
    ondatabase,
  }: { database: Database; id: number; section: ScheduledBackupSection; ondatabase: (d: Database) => void } = $props()

  let scheduledBackup = $state.raw<ScheduledBackup | null>(null)
  // null until loaded; viewers never load them (their keys are Secrets).
  let storages = $state.raw<S3Storage[] | null>(null)
  let loadError = $state('')

  $effect(() => {
    scheduledBackup = null
    loadError = ''
    api<{ scheduled_backup: ScheduledBackup }>('GET', `/scheduled-backups/${id}`)
      .then((r) => {
        if (r.scheduled_backup.database_id !== database.id) loadError = 'This Database has no such Scheduled backup.'
        else scheduledBackup = r.scheduled_backup
      })
      .catch((e) => (loadError = e.message))
  })
  $effect(() => {
    if (!projectAccess.can('see_secrets')) return
    api<{ s3_storages: S3Storage[] }>('GET', '/s3-storages')
      .then((r) => (storages = r.s3_storages))
      .catch(() => (storages = []))
  })

  const items: { key: ScheduledBackupSection; label: string; icon: IconName }[] = [
    { key: '', label: 'General', icon: 'settings' },
    { key: 's3', label: 'S3 storage', icon: 'storages' },
    { key: 'retention', label: 'Retention', icon: 'unordered-list' },
    { key: 'executions', label: 'Executions', icon: 'browser-terminal' },
    { key: 'danger', label: 'Danger Zone', icon: 'shield-alert' },
  ]
  const shownItems = $derived(items.filter((i) => i.key !== 'danger' || projectAccess.can('manage_applications')))
  const base = $derived(`${databasePath(database, 'backups')}/${id}`)

  const groups = $derived([
    {
      label: 'Backup',
      items: shownItems.map((item) => ({
        label: item.label,
        path: item.key ? `${base}/${item.key}` : base,
        icon: item.icon,
        active: section === item.key,
      })),
    },
  ])
</script>

<section class="mt-4 w-full max-w-none lg:mt-0">
  <div class="grid min-w-0 gap-8 xl:grid-cols-[210px_minmax(0,1fr)] xl:gap-8">
    <div class="flex min-w-0 flex-col gap-3 xl:self-start">
      <SidebarNavItem href={href(databasePath(database, 'backups'))} label="Back to database" icon={ArrowLeft} inline />
      <ResourceNav {groups} label="Backup settings" />
    </div>

    <div class="min-w-0">
      {#if loadError}
        <p class="text-sm text-destructive">{loadError}</p>
      {:else if !scheduledBackup}
        <Spinner text="Loading…" />
      {:else if section === 'executions'}
        <BackupExecutions {database} {scheduledBackup} {ondatabase} />
      {:else if section === 'danger' && !projectAccess.can('manage_applications')}
        <div>
          <h2 class="text-sm font-semibold text-foreground">Not available</h2>
          <p class="mt-2 text-sm text-muted-foreground">Viewers cannot delete a Scheduled backup.</p>
        </div>
      {:else}
        {#key `${scheduledBackup.id}-${section}`}
          <BackupEdit {database} {scheduledBackup} {storages} {section} onchange={(s) => (scheduledBackup = s)} />
        {/key}
      {/if}
    </div>
  </div>
</section>
