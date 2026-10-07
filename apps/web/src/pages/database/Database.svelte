<script lang="ts">
  // Coolify's Database page (resources/views/livewire/project/database/configuration.blade.php,
  // Apache-2.0, see NOTICE): the heading, the configuration sidebar and the
  // sub-page the URL names.
  import { api, ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import { databaseTypeLabel } from '../../lib/databaseTypes'
  import { databasePath, href, type DatabasePage, type ScheduledBackupSection } from '../../lib/router.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Database, DatabaseInput, Environment, Server } from '../../lib/types'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import Danger from '../application/Danger.svelte'
  import PersistentStorage from '../application/PersistentStorage.svelte'
  import ResourceLimits from '../application/ResourceLimits.svelte'
  import RuntimeLogs from '../application/RuntimeLogs.svelte'
  import Servers from '../application/Servers.svelte'
  import Backups from './Backups.svelte'
  import ConfigurationSidebar from './ConfigurationSidebar.svelte'
  import General from './General.svelte'
  import Heading from './Heading.svelte'
  import ScheduledBackupPage from './ScheduledBackupPage.svelte'

  let {
    projectId,
    environmentId,
    id,
    page,
    scheduledBackupId,
    backupSection,
  }: {
    projectId: number
    environmentId: number
    id: number
    page: DatabasePage
    scheduledBackupId: number | null
    backupSection: ScheduledBackupSection
  } = $props()

  let database = $state.raw<Database | null>(null)
  let environment = $state.raw<Environment | null>(null)
  // Databases run on the server The Bakery runs on.
  let server = $state.raw<Server | null>(null)
  let loadError = $state('')
  let busy = $state(false)

  async function load() {
    const r = await api<{ database: Database }>('GET', `/databases/${id}`)
    database = r.database
  }

  $effect(() => {
    database = null
    loadError = ''
    // The person may have moved on while it was on its way; a late answer
    // must not send them back.
    let gone = false
    api<{ database: Database }>('GET', `/databases/${id}`)
      .then(({ database: d }) => {
        if (gone) return
        // A link with another Project or Environment in it still opens the
        // Database, at its own path.
        if (d.project_id !== projectId || d.environment_id !== environmentId) {
          const backup = scheduledBackupId ? `/${scheduledBackupId}${backupSection ? `/${backupSection}` : ''}` : ''
          location.replace('#' + databasePath(d, page) + backup)
          return
        }
        database = d
        api<{ environment: Environment }>('GET', `/environments/${d.environment_id}`)
          .then((e) => (environment = e.environment))
          .catch(() => {})
        api<{ servers: Server[] }>('GET', '/servers')
          .then((s) => (server = s.servers.find((x) => x.kind === 'local') ?? null))
          .catch(() => {})
      })
      .catch((e) => {
        if (!gone) loadError = e.message
      })
    return () => (gone = true)
  })

  // Quickly while it is starting, slower otherwise so a crash shows up.
  let starting = $derived(database?.status === 'starting')
  $effect(() => {
    const t = setInterval(() => load().catch(() => {}), starting ? 2000 : 5000)
    return () => clearInterval(t)
  })

  async function act(action: 'start' | 'stop' | 'restart') {
    busy = true
    if (action === 'restart') toast.info('Restarting database.')
    try {
      const r = await api<{ database: Database }>('POST', `/databases/${id}/${action}`)
      database = r.database
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error(`Database not ${action === 'stop' ? 'stopped' : action === 'start' ? 'started' : 'restarted'}`, err.message)
    } finally {
      busy = false
    }
  }

  // Saves part of the Database, the rest as it is now; the sub-pages throw
  // the API's ApiError back to show it.
  async function patch(change: Partial<DatabaseInput>) {
    if (!database) return
    const d = database
    const r = await api<{ database: Database }>('PATCH', `/databases/${id}`, {
      name: d.name,
      description: d.description,
      version: d.version,
      public_port: d.public_port,
      resource_limits: d.resource_limits,
      ...change,
    } satisfies DatabaseInput)
    database = r.database
  }

  // Projects › Project › Environment › Database, with its status.
  const crumbs = $derived(
    database && environment ? { project: environment.project_name ?? 'Project', environment: environment.name, name: database.name } : null,
  )
  // A running Database has passed its readiness probe, The Bakery's
  // healthcheck, so it reads as Coolify's `running:healthy`.
  const status = $derived(database ? (database.status === 'running' ? 'running:healthy' : database.status) : null)
  $effect(() => {
    if (crumbs)
      breadcrumb.set(
        { label: 'Projects', href: href('/projects') },
        { label: crumbs.project, href: href(`/project/${projectId}`) },
        { label: crumbs.environment, href: href(`/project/${projectId}/environment/${environmentId}`) },
        { label: crumbs.name, summary: status ?? undefined, summaryTitle: 'Database status' },
      )
  })
</script>

{#if loadError}
  <p class="chrome text-sm text-error">{loadError}</p>
{:else if !database}
  <div class="chrome"><Spinner text="Loading…" /></div>
{:else}
  <Heading
    name={database.name}
    status={status ?? database.status}
    detail={`${databaseTypeLabel(database.type)} ${database.version}`}
    error={database.error}
    stopped={database.desired_state === 'stopped'}
    canDeploy={projectAccess.can('deploy')}
    {busy}
    onstart={() => act('start')}
  />
  {#if projectAccess.can('deploy')}
    <div class="hidden" aria-hidden="true">
      <ConfirmationModal
        title="Confirm Database Restart?"
        buttonTitle="Restart"
        actions={['This database will be unavailable during the restart.', 'If the database is currently in use, data could be lost.']}
        confirmWithText={false}
        step2ButtonText="Restart Database"
        onconfirm={() => act('restart')}
      >
        {#snippet trigger(show)}
          <button id="database-restart-trigger" type="button" onclick={show}>Restart</button>
        {/snippet}
      </ConfirmationModal>
      <ConfirmationModal
        title="Confirm Database Stopping?"
        buttonTitle="Stop"
        actions={['This database will be stopped.', 'If the database is currently in use, data could be lost.']}
        confirmWithText={false}
        onconfirm={() => act('stop')}
      >
        {#snippet trigger(show)}
          <button id="database-stop-trigger" type="button" onclick={show}>Stop</button>
        {/snippet}
      </ConfirmationModal>
    </div>
  {/if}

  {#if page === 'backups' && scheduledBackupId && database.backups_supported}
    <ScheduledBackupPage {database} id={scheduledBackupId} section={backupSection} ondatabase={(d) => (database = d)} />
  {:else}
    <section class="mt-4 w-full max-w-none lg:mt-0">
      <div class="grid min-w-0 gap-8 xl:grid-cols-[210px_minmax(0,1fr)] xl:gap-8">
        <ConfigurationSidebar {database} {page} />

        <div class="min-w-0">
          {#if page === ''}
            <General {database} onchange={(d) => (database = d)} />
          {:else if page === 'resource-limits'}
            <ResourceLimits
              limits={database.resource_limits}
              onsave={(resource_limits) => patch({ resource_limits })}
              applied="The database restarts with them."
            />
          {:else if page === 'persistent-storage'}
            <PersistentStorage
              storages={[database.volume]}
              helper="The database keeps its data in this volume, which outlives restarts, settings changes and new images."
            />
          {:else if page === 'servers'}
            <Servers {server} {status} />
          {:else if page === 'logs'}
            <RuntimeLogs url={`/api/databases/${database.id}/logs`} container={database.status === 'stopped' ? '' : database.container} />
          {:else if page === 'backups' && database.backups_supported}
            <Backups {database} />
          {:else if page === 'danger' && projectAccess.can('manage_applications')}
            <!-- Coolify calls a Database a "resource" here. Its network,
                 configuration and Docker cleanup checkboxes have nothing
                 behind them for a Database. -->
            <Danger
              label="resource"
              name={database.name}
              url={`/databases/${database.id}`}
              checkboxes={[{ id: 'delete_volumes', label: 'Permanently delete all volumes associated with this resource.', checked: true }]}
              notes={['Its Backup executions on this server are removed; copies in S3 storage stay.']}
              back={`/project/${projectId}/environment/${environmentId}`}
            />
          {:else}
            <div class="chrome">
              <h2 class="text-[15px]! font-semibold! text-black dark:text-fg">Not available</h2>
              <p class="mt-2 text-[13px] text-neutral-600 dark:text-fg-dim">This Database has no such page.</p>
            </div>
          {/if}
        </div>
      </div>
    </section>
  {/if}
{/if}
