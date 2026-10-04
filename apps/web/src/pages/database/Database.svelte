<script lang="ts">
  // Coolify's Database page (resources/views/livewire/project/database/configuration.blade.php,
  // Apache-2.0, see NOTICE): the heading, the configuration sidebar and the
  // sub-page the URL names.
  import { api, ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import ContainerLogs from '../../lib/ContainerLogs.svelte'
  import CopyButton from '../../lib/CopyButton.svelte'
  import DatabaseBackups from '../../lib/DatabaseBackups.svelte'
  import DatabaseForm from '../../lib/DatabaseForm.svelte'
  import { databaseTypeLabel } from '../../lib/databaseTypes'
  import { databasePath, go, href, type DatabasePage, type ScheduledBackupSection } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Database, DatabaseInput, Environment, Server } from '../../lib/types'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import Servers from '../application/Servers.svelte'
  import ConfigurationSidebar from './ConfigurationSidebar.svelte'
  import Heading from './Heading.svelte'

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
  let saved = $state(false)
  let showPasswords = $state(false)
  // Bumped after a save, so the settings form starts from what was saved.
  let formKey = $state(0)

  async function load() {
    const r = await api<{ database: Database }>('GET', `/databases/${id}`)
    database = r.database
  }

  $effect(() => {
    database = null
    loadError = ''
    api<{ database: Database }>('GET', `/databases/${id}`)
      .then(({ database: d }) => {
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
      .catch((e) => (loadError = e.message))
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

  async function update(input: DatabaseInput) {
    saved = false
    const r = await api<{ database: Database }>('PATCH', `/databases/${id}`, input)
    database = r.database
    formKey++
    saved = true
  }

  async function remove() {
    if (!database) return
    const ok = confirm(
      `Delete ${database.name}? Its container, all its data and its backups on this server are removed; copies in S3 storage stay. This cannot be undone.`,
    )
    if (!ok) return
    await api('DELETE', `/databases/${id}`)
    go(`/project/${projectId}/environment/${environmentId}`)
  }

  function hidden(secret: string, url?: string | null): string {
    if (!url) return ''
    return showPasswords ? url : url.replace(`:${encodeURIComponent(secret)}@`, ':••••••@')
  }

  // Projects › Project › Environment › Database, with its status.
  const crumbs = $derived(
    database && environment ? { project: environment.project_name ?? 'Project', environment: environment.name, name: database.name } : null,
  )
  const status = $derived(database?.status ?? null)
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
  {@const creds = database.credentials}
  <Heading
    name={database.name}
    status={database.status}
    detail={`${databaseTypeLabel(database.type)} ${database.version}`}
    error={database.error}
    stopped={database.desired_state === 'stopped'}
    canWrite={session.canWrite}
    {busy}
    onstart={() => act('start')}
  />
  {#if session.canWrite}
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

  <section class="mt-4 w-full max-w-none lg:mt-0">
    <div class="grid min-w-0 gap-8 xl:grid-cols-[210px_minmax(0,1fr)] xl:gap-8">
      <ConfigurationSidebar {database} {page} />

      <!-- Until General, Persistent Storage, Resource Limits, Backups and
           Danger Zone are ported, the old tabs' contents render here. -->
      <div class="min-w-0">
        {#if page === ''}
          {#if database.secrets_hidden}
            <p class="muted">The credentials and connection URLs are hidden for viewers.</p>
          {:else if creds}
            <dl class="connect">
              <dt>Internal URL</dt>
              <dd>
                <span class="mono" data-testid="internal-url">{hidden(creds.password, database.internal_url)}</span>
                <CopyButton text={database.internal_url ?? ''} />
              </dd>
              <dt>Public URL</dt>
              <dd>
                {#if database.public_url}
                  <span class="mono" data-testid="public-url">{hidden(creds.password, database.public_url)}</span>
                  <CopyButton text={database.public_url} />
                {:else}
                  <span class="muted">Not published. Set a public port below to reach it from outside the server.</span>
                {/if}
              </dd>
              <dt>Username</dt>
              <dd class="mono">{creds.username}</dd>
              <dt>Password</dt>
              <dd>
                <span class="mono">{showPasswords ? creds.password : '••••••••'}</span>
                <CopyButton text={creds.password} />
              </dd>
              {#if creds.root_password}
                <dt>Root password</dt>
                <dd>
                  <span class="mono">{showPasswords ? creds.root_password : '••••••••'}</span>
                  <CopyButton text={creds.root_password} />
                </dd>
              {/if}
              <dt>Database</dt>
              <dd class="mono">{creds.database_name}</dd>
            </dl>
            <p><button onclick={() => (showPasswords = !showPasswords)}>{showPasswords ? 'Hide' : 'Show'} passwords</button></p>
            <p class="muted">
              Applications in any project reach it on the internal URL: paste it into an environment variable such as
              <span class="mono">DATABASE_URL</span>.
            </p>
          {/if}
        {/if}
        {#if page === '' || page === 'resource-limits'}
          {#key formKey}
            <DatabaseForm {database} submitLabel="Save" onsubmit={update} />
          {/key}
          {#if saved}<p class="ok">Saved.</p>{/if}
        {:else if page === 'persistent-storage'}
          <p class="muted">The Database keeps its data in its own volume, which outlives restarts and settings changes.</p>
        {:else if page === 'servers'}
          <Servers {server} {status} />
        {:else if page === 'logs'}
          {#key database.status === 'stopped'}
            <ContainerLogs
              url={`/api/databases/${database.id}/logs`}
              empty="No container: the database is stopped."
              stopped="The container stopped (a restart or a settings change replaces it)."
            />
          {/key}
        {:else if page === 'backups' && database.backups_supported}
          <DatabaseBackups {database} onchange={(d) => (database = d)} />
        {:else if page === 'danger' && session.canWrite}
          <h2>Danger zone</h2>
          <p class="muted">Deleting removes the container, the data volume and the backups on this server. Copies in S3 storage stay.</p>
          <button class="danger" onclick={remove}>Delete database</button>
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

<style>
  .connect {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: 0.5rem 1rem;
    align-items: center;
    margin: 0 0 1rem;
  }
  .connect dt {
    color: var(--muted);
  }
  .connect dd {
    margin: 0;
    display: flex;
    gap: 0.5rem;
    align-items: center;
    flex-wrap: wrap;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .ok {
    color: var(--ok);
  }
</style>
