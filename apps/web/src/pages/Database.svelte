<script lang="ts">
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import { api, ApiError } from '../lib/api'
  import ContainerLogs from '../lib/ContainerLogs.svelte'
  import CopyButton from '../lib/CopyButton.svelte'
  import DatabaseBackups from '../lib/DatabaseBackups.svelte'
  import DatabaseForm from '../lib/DatabaseForm.svelte'
  import { databaseTypeLabel } from '../lib/databaseTypes'
  import { go } from '../lib/router.svelte'
  import { session } from '../lib/session.svelte'
  import StatusBadge from '../lib/StatusBadge.svelte'
  import type { Database, DatabaseInput } from '../lib/types'

  let { id }: { id: number } = $props()

  type Tab = 'connect' | 'logs' | 'backups' | 'settings'

  let database = $state.raw<Database | null>(null)
  let loadError = $state('')
  let tab = $state<Tab>('connect')
  let busy = $state(false)
  let actionError = $state('')
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
    load().catch((e) => (loadError = e.message))
  })

  // Quickly while it is starting, slower otherwise so a crash shows up.
  let starting = $derived(database?.status === 'starting')
  $effect(() => {
    const t = setInterval(() => load().catch(() => {}), starting ? 2000 : 5000)
    return () => clearInterval(t)
  })

  async function act(action: 'start' | 'stop' | 'restart') {
    busy = true
    actionError = ''
    try {
      const r = await api<{ database: Database }>('POST', `/databases/${id}/${action}`)
      database = r.database
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      actionError = err.message
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
    const projectId = database.project_id
    await api('DELETE', `/databases/${id}`)
    go(`/projects/${projectId}`)
  }

  function hidden(secret: string, url?: string | null): string {
    if (!url) return ''
    return showPasswords ? url : url.replace(`:${encodeURIComponent(secret)}@`, ':••••••@')
  }

  // The top bar's breadcrumb: Project › Database.
  const crumbProject = $derived(database?.project_id)
  const crumbName = $derived(database?.name)
  $effect(() => {
    if (crumbProject !== undefined && crumbName !== undefined) breadcrumb.resource(crumbProject, crumbName)
  })
</script>

{#if loadError}
  <p class="error">{loadError}</p>
{:else if !database}
  <p class="muted">Loading…</p>
{:else}
  {@const creds = database.credentials}
  <div class="head">
    <div class="heading">
      <h1>{database.name}</h1>
      <StatusBadge status={database.status} />
      <p class="muted">{databaseTypeLabel(database.type)} {database.version}</p>
    </div>
    {#if session.canWrite}<div class="buttons">
      {#if database.desired_state === 'stopped'}
        <button class="primary" disabled={busy} onclick={() => act('start')}>Start</button>
      {:else}
        <button disabled={busy} onclick={() => act('restart')}>Restart</button>
        <button disabled={busy} onclick={() => act('stop')}>Stop</button>
      {/if}
    </div>{/if}
  </div>
  {#if actionError}<p class="error">{actionError}</p>{/if}
  {#if database.error}<p class="error">{database.error}</p>{/if}

  <div class="tabs" role="tablist">
    <button role="tab" aria-selected={tab === 'connect'} onclick={() => (tab = 'connect')}>Connect</button>
    <button role="tab" aria-selected={tab === 'logs'} onclick={() => (tab = 'logs')}>Logs</button>
    <button role="tab" aria-selected={tab === 'backups'} onclick={() => (tab = 'backups')}>Backups</button>
    <button role="tab" aria-selected={tab === 'settings'} onclick={() => (tab = 'settings')}>Settings</button>
  </div>

  {#if tab === 'connect' && database.secrets_hidden}
    <p class="muted">The credentials and connection URLs are hidden for viewers.</p>
  {:else if tab === 'connect' && creds}
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
          <span class="muted">Not published. Set a public port under Settings to reach it from outside the server.</span>
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
  {:else if tab === 'logs'}
    {#key database.status === 'stopped'}
      <ContainerLogs
        url={`/api/databases/${database.id}/logs`}
        empty="No container: the database is stopped."
        stopped="The container stopped (a restart or a settings change replaces it)."
      />
    {/key}
  {:else if tab === 'backups'}
    <DatabaseBackups {database} onchange={(d) => (database = d)} />
  {:else if tab === 'settings'}
    {#key formKey}
      <DatabaseForm database={database} submitLabel="Save" onsubmit={update} />
    {/key}
    {#if saved}<p class="ok">Saved.</p>{/if}
    {#if session.canWrite}
      <h2>Danger zone</h2>
      <p class="muted">Deleting removes the container, the data volume and the backups on this server. Copies in S3 storage stay.</p>
      <button class="danger" onclick={remove}>Delete database</button>
    {/if}
  {/if}
{/if}

<style>
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    margin-bottom: 1rem;
  }
  .heading {
    display: grid;
    grid-template-columns: auto auto;
    justify-content: start;
    justify-items: start;
    align-items: center;
    column-gap: 0.75rem;
  }
  .heading h1 {
    margin: 0;
  }
  .heading p {
    grid-column: 1 / -1;
    margin: 0.2rem 0 0;
  }
  .buttons {
    display: flex;
    gap: 0.5rem;
  }
  .tabs {
    display: flex;
    gap: 0.25rem;
    border-bottom: 1px solid var(--border);
    margin-bottom: 1.25rem;
    overflow-x: auto;
  }
  .tabs button {
    background: none;
    border: 0;
    border-bottom: 2px solid transparent;
    border-radius: 0;
    color: var(--muted);
    padding: 0.5rem 0.75rem;
    white-space: nowrap;
  }
  .tabs button[aria-selected='true'] {
    color: var(--text);
    border-bottom-color: var(--accent);
  }
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
