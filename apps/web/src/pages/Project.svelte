<script lang="ts">
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import { api, ApiError } from '../lib/api'
  import ApplicationForm from '../lib/ApplicationForm.svelte'
  import DatabaseForm from '../lib/DatabaseForm.svelte'
  import { databaseTypeLabel } from '../lib/databaseTypes'
  import { sourceLine } from '../lib/buildPacks'
  import EnvironmentVariables from '../lib/EnvironmentVariables.svelte'
  import ServiceForm from '../lib/ServiceForm.svelte'
  import { go, href } from '../lib/router.svelte'
  import { session } from '../lib/session.svelte'
  import StatusBadge from '../lib/StatusBadge.svelte'
  import type { Application, ApplicationInput, Database, DatabaseInput, Project, Service, ServiceInput } from '../lib/types'

  let { id }: { id: number } = $props()

  let project = $state.raw<Project | null>(null)
  let loadError = $state('')
  let databases = $state.raw<Database[]>([])
  let services = $state.raw<Service[]>([])
  // The Resource being added, and the Environment it goes in.
  let adding = $state<{ environment: number; kind: 'choose' | 'application' | 'database' | 'service' } | null>(null)
  let deleteError = $state('')

  $effect(() => {
    project = null
    loadError = ''
    databases = []
    services = []
    Promise.all([
      api<{ project: Project }>('GET', `/projects/${id}`),
      api<{ databases: Database[] }>('GET', `/projects/${id}/databases`),
      api<{ services: Service[] }>('GET', `/projects/${id}/services`),
    ])
      .then(([p, d, sv]) => {
        databases = d.databases
        services = sv.services
        project = p.project
      })
      .catch((e) => (loadError = e.message))
  })

  async function addApplication(environmentId: number, input: ApplicationInput) {
    const { application } = await api<{ application: Application }>(
      'POST',
      `/environments/${environmentId}/applications`,
      input,
    )
    go(`/applications/${application.id}`)
  }

  async function addDatabase(environmentId: number, input: DatabaseInput) {
    const { database } = await api<{ database: Database }>('POST', `/environments/${environmentId}/databases`, input)
    go(`/databases/${database.id}`)
  }

  async function addService(environmentId: number, input: ServiceInput) {
    const { service } = await api<{ service: Service }>('POST', `/environments/${environmentId}/services`, input)
    go(`/services/${service.id}`)
  }

  async function remove() {
    if (!project) return
    deleteError = ''
    try {
      await api('DELETE', `/projects/${project.id}`)
      go('/projects')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      deleteError = err.message
    }
  }

  const crumbName = $derived(project?.name)
  $effect(() => {
    if (crumbName !== undefined) breadcrumb.set({ label: crumbName })
  })
</script>


{#if loadError}
  <p class="error">{loadError}</p>
{:else if !project}
  <p class="muted">Loading…</p>
{:else}
  <div class="head">
    <div>
      <h1>{project.name}</h1>
      {#if project.description}<p class="muted">{project.description}</p>{/if}
    </div>
    {#if session.canWrite}<button class="danger" onclick={remove}>Delete project</button>{/if}
  </div>
  {#if deleteError}<p class="error">{deleteError}</p>{/if}

  <details>
    <summary>Shared variables of the project</summary>
    <EnvironmentVariables
      path={`/projects/${project.id}/variables`}
      description="Every application in this project gets these, unless its environment or the application sets the same name."
    />
  </details>

  {#each project.environments ?? [] as env (env.id)}
    {@const envDatabases = databases.filter((d) => d.environment_id === env.id)}
    {@const envServices = services.filter((sv) => sv.environment_id === env.id)}
    <section>
      <div class="head">
        <h2>{env.name}</h2>
        {#if session.canWrite && adding?.environment !== env.id}
          <button class="primary" onclick={() => (adding = { environment: env.id, kind: 'choose' })}>+ New Resource</button>
        {/if}
      </div>
      <details>
        <summary>Shared variables of {env.name}</summary>
        <EnvironmentVariables
          path={`/environments/${env.id}/variables`}
          description={`Every application in ${env.name} gets these, unless it sets the same name. They win over the project's.`}
        />
      </details>
      {#if adding?.environment === env.id && adding.kind === 'choose'}
        <div class="card">
          <h3>New Resource</h3>
          <div class="buttons">
            <button onclick={() => (adding = { environment: env.id, kind: 'application' })}>Application</button>
            <button onclick={() => (adding = { environment: env.id, kind: 'database' })}>Database</button>
            <button onclick={() => (adding = { environment: env.id, kind: 'service' })}>Service</button>
            <button onclick={() => (adding = null)}>Cancel</button>
          </div>
        </div>
      {:else if adding?.environment === env.id && adding.kind === 'application'}
        <div class="card">
          <ApplicationForm
            submitLabel="Create application"
            onsubmit={(input) => addApplication(env.id, input)}
            oncancel={() => (adding = null)}
          />
        </div>
      {:else if adding?.environment === env.id && adding.kind === 'database'}
        <div class="card">
          <DatabaseForm
            submitLabel="Create database"
            onsubmit={(input) => addDatabase(env.id, input)}
            oncancel={() => (adding = null)}
          />
        </div>
      {:else if adding?.environment === env.id && adding.kind === 'service'}
        <div class="card">
          <ServiceForm onsubmit={(input) => addService(env.id, input)} oncancel={() => (adding = null)} />
        </div>
      {/if}
      <h3>Resources</h3>
      {#if env.applications.length === 0 && envDatabases.length === 0 && envServices.length === 0}
        <p class="muted">No resources in {env.name} yet.</p>
      {/if}
      {#if env.applications.length > 0}
        <table>
          <thead><tr><th>Application</th><th>Source</th><th>Domain</th></tr></thead>
          <tbody>
            {#each env.applications as a (a.id)}
              <tr>
                <td><a href={href(`/applications/${a.id}`)}>{a.name}</a></td>
                <td class="mono muted">{sourceLine(a)}</td>
                <td class="mono">{a.domains[0]}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
      {#if envDatabases.length > 0}
        <table>
          <thead><tr><th>Database</th><th>Database type</th><th>Status</th></tr></thead>
          <tbody>
            {#each envDatabases as d (d.id)}
              <tr>
                <td><a href={href(`/databases/${d.id}`)}>{d.name}</a></td>
                <td class="muted">{databaseTypeLabel(d.type)} {d.version}</td>
                <td><StatusBadge status={d.status} /></td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
      {#if envServices.length > 0}
        <table>
          <thead><tr><th>Service</th><th>Domain</th><th>Components</th><th>Status</th></tr></thead>
          <tbody>
            {#each envServices as sv (sv.id)}
              {@const primary = sv.components.find((c) => c.public)}
              <tr>
                <td><a href={href(`/services/${sv.id}`)}>{sv.name}</a></td>
                <td class="mono">
                  {#if primary?.url}<a href={primary.url} target="_blank" rel="noreferrer">{primary.domains[0]}</a>{:else}<span class="muted">none</span>{/if}
                </td>
                <td class="muted">{sv.components.length}</td>
                <td><StatusBadge status={sv.status} /></td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </section>
  {/each}
{/if}

<style>
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    margin-bottom: 0.75rem;
  }
  .head h1,
  .head h2,
  .head p {
    margin: 0;
  }
  section {
    margin-top: 1.5rem;
    display: grid;
    gap: 0.75rem;
  }
  section .head {
    margin: 0;
  }
  .buttons {
    display: flex;
    gap: 0.5rem;
  }
  h2 {
    text-transform: capitalize;
  }
  h3 {
    margin: 0;
  }
  details {
    margin: 0.5rem 0;
  }
  summary {
    cursor: pointer;
    margin-bottom: 0.5rem;
  }
</style>
