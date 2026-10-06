<script lang="ts">
  // Coolify's Scheduled backup page
  // (resources/views/livewire/project/database/backup/execution.blade.php and
  // components/backup-sidebar.blade.php, Apache-2.0, see NOTICE): the backup
  // sidebar in place of the Database's, and the section the URL names.
  import { api } from '../../lib/api'
  import Icon, { type IconName } from '../../lib/Icon.svelte'
  import { databasePath, href, type ScheduledBackupSection } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
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
    if (!session.can('see_secrets')) return
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
  const shownItems = $derived(items.filter((i) => i.key !== 'danger' || session.can('manage_applications')))
  const base = $derived(`${databasePath(database, 'backups')}/${id}`)
</script>

<section class="mt-4 w-full max-w-none lg:mt-0">
  <div class="grid min-w-0 gap-8 xl:grid-cols-[210px_minmax(0,1fr)] xl:gap-8">
    <aside class="chrome min-w-0 xl:self-start">
      <nav
        aria-label="Backup settings"
        class="grid grid-cols-2 gap-0.5 border-y border-neutral-200 py-3 sm:grid-cols-3 xl:grid-cols-1 xl:border-y-0 xl:py-0 dark:border-white/[0.06]"
      >
        <div class="nav-section hidden xl:block">Backup</div>
        <a class="menu-item" href={href(databasePath(database, 'backups'))}>
          <Icon name="logout" class="menu-item-icon rotate-180" />
          <span class="menu-item-label">Back to database</span>
        </a>
        {#each shownItems as item (item.key)}
          <a
            class={['menu-item', section === item.key && 'menu-item-active']}
            href={href(item.key ? `${base}/${item.key}` : base)}
            aria-current={section === item.key ? 'page' : undefined}
          >
            <Icon name={item.icon} class="menu-item-icon" />
            <span class="menu-item-label">{item.label}</span>
          </a>
        {/each}
      </nav>
    </aside>

    <div class="min-w-0">
      {#if loadError}
        <p class="chrome text-sm text-error">{loadError}</p>
      {:else if !scheduledBackup}
        <div class="chrome"><Spinner text="Loading…" /></div>
      {:else if section === 'executions'}
        <BackupExecutions {database} {scheduledBackup} {ondatabase} />
      {:else if section === 'danger' && !session.can('manage_applications')}
        <div class="chrome">
          <h2 class="text-[15px]! font-semibold! text-black dark:text-fg">Not available</h2>
          <p class="mt-2 text-[13px] text-neutral-600 dark:text-fg-dim">Viewers cannot delete a Scheduled backup.</p>
        </div>
      {:else}
        {#key `${scheduledBackup.id}-${section}`}
          <BackupEdit {database} {scheduledBackup} {storages} {section} onchange={(s) => (scheduledBackup = s)} />
        {/key}
      {/if}
    </div>
  </div>
</section>
