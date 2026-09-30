<script lang="ts">
  import { packLabel } from '../lib/buildPacks'
  import { api, ApiError } from '../lib/api'
  import ApplicationForm from '../lib/ApplicationForm.svelte'
  import ContainerLogs from '../lib/ContainerLogs.svelte'
  import DeployKey from '../lib/DeployKey.svelte'
  import Deployments from '../lib/Deployments.svelte'
  import EnvEditor from '../lib/EnvEditor.svelte'
  import Routing from '../lib/Routing.svelte'
  import { go, href } from '../lib/router.svelte'
  import StatusBadge from '../lib/StatusBadge.svelte'
  import Webhook from '../lib/Webhook.svelte'
  import type { Application, ApplicationInput, Deployment, Server } from '../lib/types'

  let { id }: { id: number } = $props()

  type Tab = 'deployments' | 'logs' | 'source' | 'general' | 'routing' | 'env'

  let application = $state.raw<Application | null>(null)
  let deployments = $state.raw<Deployment[]>([])
  let loadError = $state('')
  let tab = $state<Tab>('deployments')
  let selected = $state<number | null>(null)
  let saved = $state(false)
  let deployError = $state('')
  let deploying = $state(false)

  // The Target server, for its name and, on a Remote server, where DNS
  // must point.
  let server = $state.raw<Server | null>(null)

  let latest = $derived(deployments[0] ?? null)
  let active = $derived(deployments.some((d) => d.active))
  // One Deployment can wait behind the running one; a second cannot.
  let queued = $derived(deployments.some((d) => d.status === 'queued'))

  async function loadDeployments() {
    const r = await api<{ deployments: Deployment[] }>('GET', `/applications/${id}/deployments`)
    deployments = r.deployments
  }

  $effect(() => {
    application = null
    server = null
    deployments = []
    loadError = ''
    selected = null
    api<{ application: Application }>('GET', `/applications/${id}`)
      .then((r) => {
        application = r.application
        api<{ server: Server }>('GET', `/servers/${r.application.server_id}`)
          .then((s) => (server = s.server))
          .catch(() => {})
      })
      .then(loadDeployments)
      .catch((e) => (loadError = e.message))
  })

  // Keeps the list and badge current: quickly while a deployment is under
  // way, slower otherwise so one started by a push shows up by itself.
  $effect(() => {
    const t = setInterval(() => loadDeployments().catch(() => {}), active ? 3000 : 5000)
    return () => clearInterval(t)
  })

  async function deploy() {
    deploying = true
    deployError = ''
    try {
      const r = await api<{ deployment: Deployment }>('POST', `/applications/${id}/deploy`)
      await loadDeployments()
      selected = r.deployment.id
      tab = 'deployments'
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      deployError = err.message
    } finally {
      deploying = false
    }
  }

  async function update(input: ApplicationInput) {
    saved = false
    const r = await api<{ application: Application }>('PATCH', `/applications/${id}`, input)
    application = r.application
    saved = true
  }

  async function remove() {
    if (!application) return
    const projectId = application.project_id
    await api('DELETE', `/applications/${id}`)
    go(`/projects/${projectId}`)
  }
</script>

{#if loadError}
  <p class="error">{loadError}</p>
{:else if !application}
  <p class="muted">Loading…</p>
{:else}
  <p class="crumbs">
    <a href={href('/projects')}>Projects</a> /
    <a href={href(`/projects/${application.project_id}`)}>Project</a> /
  </p>
  <div class="head">
    <div class="title">
      <h1>{application.name}</h1>
      {#if latest}<StatusBadge status={latest.status} />{/if}
      <p>
        <span class="muted">{packLabel[application.build_pack]} ·</span>
        {#each application.public_urls as url, i (url)}
          {#if i > 0}<span class="muted">{' · '}</span>{/if}
          <a class="mono" href={url} target="_blank" rel="noreferrer">{url}</a>
        {/each}
        {#if server}
          <span class="muted">{' · on '}</span><a href={href(`/servers/${server.id}`)}>{server.name}</a>
        {/if}
      </p>
      {#if server?.kind === 'remote'}
        <p class="muted">Its domains must point at {server.host}, where this server's proxy serves them.</p>
      {/if}
      {#if application.storages.length > 0 || application.resource_limits.memory_mb || application.resource_limits.cpus}
        <p class="muted settings">
          {#each application.storages as s (s.name)}<span>storage <span class="mono">{s.name}</span> at <span class="mono">{s.mount_path}</span></span>{/each}
          {#if application.resource_limits.memory_mb}<span>{application.resource_limits.memory_mb} MB memory</span>{/if}
          {#if application.resource_limits.cpus}<span>{application.resource_limits.cpus} CPU</span>{/if}
        </p>
      {/if}
    </div>
    <button class="primary" onclick={deploy} disabled={deploying || queued}>
      {queued ? 'Queued…' : active ? 'Deploy again' : 'Deploy'}
    </button>
  </div>
  {#if deployError}<p class="error">{deployError}</p>{/if}
  {#if application.deploy_key_public && deployments.length === 0}
    <p class="muted">
      Private repository: add the deploy key from the <button class="link" onclick={() => (tab = 'source')}>Source</button> tab to
      the repository before the first deploy.
    </p>
  {/if}

  <div class="tabs" role="tablist">
    <button role="tab" aria-selected={tab === 'deployments'} onclick={() => (tab = 'deployments')}>Deployments</button>
    <button role="tab" aria-selected={tab === 'logs'} onclick={() => (tab = 'logs')}>Logs</button>
    <button role="tab" aria-selected={tab === 'source'} onclick={() => (tab = 'source')}>Source</button>
    <button role="tab" aria-selected={tab === 'general'} onclick={() => (tab = 'general')}>General</button>
    <button role="tab" aria-selected={tab === 'routing'} onclick={() => (tab = 'routing')}>Routing</button>
    <button role="tab" aria-selected={tab === 'env'} onclick={() => (tab = 'env')}>Environment variables</button>
  </div>

  {#if tab === 'deployments'}
    <Deployments {deployments} bind:selected serverNames={server ? { [server.id]: server.name } : {}} onchange={() => loadDeployments().catch(() => {})} />
  {:else if tab === 'logs'}
    <ContainerLogs
      url={`/api/applications/${application.id}/logs`}
      empty="No running container. Deploy the application first."
      stopped="The container stopped (a new deployment may have replaced it)."
    />
  {:else if tab === 'source' && application.build_pack === 'image'}
    <dl class="source">
      <dt>Image</dt>
      <dd class="mono">{application.image_reference}</dd>
      <dt>Registry credentials</dt>
      <dd>{application.registry_username ? `as ${application.registry_username}` : 'none (public image)'}</dd>
    </dl>
    <p class="muted">Every deploy pulls the image again, so a moved tag is picked up. Rollbacks start the exact image pulled then.</p>
  {:else if tab === 'source'}
    <dl class="source">
      <dt>Repository</dt>
      <dd class="mono">{application.git_url}</dd>
      <dt>Branch</dt>
      <dd class="mono">{application.git_branch}</dd>
      {#if application.build_pack === 'static'}
        <dt>Publish directory</dt>
        <dd class="mono">{application.publish_directory}</dd>
      {:else if application.build_pack === 'dockerfile'}
        <dt>Dockerfile</dt>
        <dd class="mono">{application.dockerfile_path}</dd>
      {/if}
    </dl>
    {#if application.deploy_key_public}
      <DeployKey {application} onchange={(a) => (application = a)} />
    {:else}
      <p class="muted">A public https repository needs no key. For a private one, use its SSH URL (git@host:owner/repo.git) under General.</p>
    {/if}
    <Webhook applicationId={application.id} />
  {:else if tab === 'general'}
    {#key application.id}
      <ApplicationForm
        initial={application}
        submitLabel="Save"
        domainPlaceholder={`${application.slug}.localhost`}
        onsubmit={update}
      />
    {/key}
    {#if saved}<p class="ok">Saved. Domains apply at once; everything else on the next deploy.</p>{/if}
    <h2>Danger zone</h2>
    <button class="danger" onclick={remove}>Delete application</button>
  {:else if tab === 'routing'}
    <Routing applicationId={application.id} domains={application.domains} />
  {:else}
    <EnvEditor path={`/applications/${application.id}/env`} />
  {/if}
{/if}

<style>
  .link {
    background: none;
    border: 0;
    padding: 0;
    color: var(--accent);
    text-decoration: underline;
  }
  .source {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 0.3rem 1rem;
    margin: 0 0 1rem;
  }
  .source dt {
    color: var(--muted);
  }
  .source dd {
    margin: 0;
  }
  .crumbs {
    margin: 0 0 0.5rem;
    font-size: 0.85rem;
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    margin-bottom: 1rem;
  }
  .title {
    display: grid;
    grid-template-columns: auto auto;
    justify-content: start;
    justify-items: start;
    align-items: center;
    column-gap: 0.75rem;
  }
  .title h1 {
    margin: 0;
  }
  .title p {
    grid-column: 1 / -1;
    margin: 0.2rem 0 0;
  }
  .settings {
    display: flex;
    flex-wrap: wrap;
    gap: 0 1rem;
    font-size: 0.85rem;
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
  .ok {
    color: var(--ok);
  }
</style>
