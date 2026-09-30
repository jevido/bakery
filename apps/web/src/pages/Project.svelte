<script lang="ts">
  import { api, ApiError } from '../lib/api'
  import ApplicationForm from '../lib/ApplicationForm.svelte'
  import DatabaseForm from '../lib/DatabaseForm.svelte'
  import { engineLabel } from '../lib/engines'
  import { sourceLine } from '../lib/buildPacks'
  import EnvEditor from '../lib/EnvEditor.svelte'
  import { go, href } from '../lib/router.svelte'
  import StatusBadge from '../lib/StatusBadge.svelte'
  import type { Application, ApplicationInput, Database, DatabaseInput, Project } from '../lib/types'

  let { id }: { id: number } = $props()

  let project = $state.raw<Project | null>(null)
  let loadError = $state('')
  let databases = $state.raw<Database[]>([])
  let addingTo = $state<number | null>(null)
  let addingDatabaseTo = $state<number | null>(null)
  let deleteError = $state('')

  $effect(() => {
    project = null
    loadError = ''
    databases = []
    Promise.all([
      api<{ project: Project }>('GET', `/projects/${id}`),
      api<{ databases: Database[] }>('GET', `/projects/${id}/databases`),
    ])
      .then(([p, d]) => {
        databases = d.databases
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
</script>

<p class="crumbs"><a href={href('/projects')}>Projects</a> /</p>

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
    <button class="danger" onclick={remove}>Delete project</button>
  </div>
  {#if deleteError}<p class="error">{deleteError}</p>{/if}

  <details>
    <summary>Shared variables of the project</summary>
    <EnvEditor
      path={`/projects/${project.id}/variables`}
      description="Every application in this project gets these, unless its environment or the application sets the same name."
    />
  </details>

  {#each project.environments ?? [] as env (env.id)}
    {@const envDatabases = databases.filter((d) => d.environment_id === env.id)}
    <section>
      <div class="head">
        <h2>{env.name}</h2>
        <div class="buttons">
          {#if addingDatabaseTo !== env.id}
            <button onclick={() => (addingDatabaseTo = env.id)}>New database</button>
          {/if}
          {#if addingTo !== env.id}
            <button class="primary" onclick={() => (addingTo = env.id)}>New application</button>
          {/if}
        </div>
      </div>
      <details>
        <summary>Shared variables of {env.name}</summary>
        <EnvEditor
          path={`/environments/${env.id}/variables`}
          description={`Every application in ${env.name} gets these, unless it sets the same name. They win over the project's.`}
        />
      </details>
      {#if addingTo === env.id}
        <div class="card">
          <ApplicationForm
            submitLabel="Create application"
            onsubmit={(input) => addApplication(env.id, input)}
            oncancel={() => (addingTo = null)}
          />
        </div>
      {/if}
      {#if addingDatabaseTo === env.id}
        <div class="card">
          <DatabaseForm
            submitLabel="Create database"
            onsubmit={(input) => addDatabase(env.id, input)}
            oncancel={() => (addingDatabaseTo = null)}
          />
        </div>
      {/if}
      {#if env.applications.length === 0}
        <p class="muted">No applications in {env.name} yet.</p>
      {:else}
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
          <thead><tr><th>Database</th><th>Engine</th><th>Status</th></tr></thead>
          <tbody>
            {#each envDatabases as d (d.id)}
              <tr>
                <td><a href={href(`/databases/${d.id}`)}>{d.name}</a></td>
                <td class="muted">{engineLabel(d.engine)} {d.version}</td>
                <td><StatusBadge status={d.status} /></td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </section>
  {/each}
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
  details {
    margin: 0.5rem 0;
  }
  summary {
    cursor: pointer;
    margin-bottom: 0.5rem;
  }
</style>
