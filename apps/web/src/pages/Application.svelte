<script lang="ts">
  import { api, ApiError } from '../lib/api'
  import ApplicationForm from '../lib/ApplicationForm.svelte'
  import ContainerLogs from '../lib/ContainerLogs.svelte'
  import Deployments from '../lib/Deployments.svelte'
  import EnvEditor from '../lib/EnvEditor.svelte'
  import { go, href } from '../lib/router.svelte'
  import StatusBadge from '../lib/StatusBadge.svelte'
  import type { Application, ApplicationInput, Deployment } from '../lib/types'

  let { id }: { id: number } = $props()

  type Tab = 'deployments' | 'logs' | 'general' | 'env'

  let application = $state.raw<Application | null>(null)
  let deployments = $state.raw<Deployment[]>([])
  let loadError = $state('')
  let tab = $state<Tab>('deployments')
  let selected = $state<number | null>(null)
  let saved = $state(false)
  let deployError = $state('')
  let deploying = $state(false)

  let latest = $derived(deployments[0] ?? null)
  let active = $derived(deployments.some((d) => d.active))

  async function loadDeployments() {
    const r = await api<{ deployments: Deployment[] }>('GET', `/applications/${id}/deployments`)
    deployments = r.deployments
  }

  $effect(() => {
    application = null
    deployments = []
    loadError = ''
    selected = null
    api<{ application: Application }>('GET', `/applications/${id}`)
      .then((r) => (application = r.application))
      .then(loadDeployments)
      .catch((e) => (loadError = e.message))
  })

  // Keeps the list and badge current while a deployment is under way.
  $effect(() => {
    if (!active) return
    const t = setInterval(() => loadDeployments().catch(() => {}), 3000)
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
        <a class="mono" href={application.public_url} target="_blank" rel="noreferrer">{application.public_url}</a>
      </p>
    </div>
    <button class="primary" onclick={deploy} disabled={deploying || active}>
      {active ? 'Deploying…' : 'Deploy'}
    </button>
  </div>
  {#if deployError}<p class="error">{deployError}</p>{/if}

  <div class="tabs" role="tablist">
    <button role="tab" aria-selected={tab === 'deployments'} onclick={() => (tab = 'deployments')}>Deployments</button>
    <button role="tab" aria-selected={tab === 'logs'} onclick={() => (tab = 'logs')}>Logs</button>
    <button role="tab" aria-selected={tab === 'general'} onclick={() => (tab = 'general')}>General</button>
    <button role="tab" aria-selected={tab === 'env'} onclick={() => (tab = 'env')}>Environment variables</button>
  </div>

  {#if tab === 'deployments'}
    <Deployments {deployments} bind:selected onchange={() => loadDeployments().catch(() => {})} />
  {:else if tab === 'logs'}
    <ContainerLogs applicationId={application.id} />
  {:else if tab === 'general'}
    {#key application.id}
      <ApplicationForm
        initial={application}
        submitLabel="Save"
        domainPlaceholder={`${application.slug}.localhost`}
        onsubmit={update}
      />
    {/key}
    {#if saved}<p class="ok">Saved. Changes apply on the next deploy.</p>{/if}
    <h2>Danger zone</h2>
    <button class="danger" onclick={remove}>Delete application</button>
  {:else}
    <EnvEditor applicationId={application.id} />
  {/if}
{/if}

<style>
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
