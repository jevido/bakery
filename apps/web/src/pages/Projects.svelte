<script lang="ts">
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import { api, ApiError } from '../lib/api'
  import Field from '../lib/Field.svelte'
  import { go, href, router } from '../lib/router.svelte'
  import { session } from '../lib/session.svelte'
  import type { Project } from '../lib/types'

  let projects = $state.raw<Project[] | null>(null)
  let loadError = $state('')
  let creating = $state(false)
  let name = $state('')
  let description = $state('')
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)

  api<{ projects: Project[] }>('GET', '/projects')
    .then((r) => (projects = r.projects))
    .catch((e) => (loadError = e.message))

  async function create(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    try {
      const { project } = await api<{ project: Project }>('POST', '/projects', { name, description })
      go(`/projects/${project.id}`)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      busy = false
    }
  }

  $effect(() => breadcrumb.set({ label: router.route.name === 'dashboard' ? 'Dashboard' : 'Projects' }))
</script>

<div class="head">
  <h1>Projects</h1>
  {#if !creating && session.canWrite}
    <button class="primary" onclick={() => (creating = true)}>New project</button>
  {/if}
</div>

{#if creating}
  <form class="card form" onsubmit={create}>
    <Field label="Name" bind:value={name} error={errors.name} required />
    <Field label="Description (optional)" bind:value={description} error={errors.description} />
    <div class="actions">
      <button type="button" onclick={() => (creating = false)}>Cancel</button>
      <button class="primary" disabled={busy}>Create project</button>
    </div>
  </form>
{/if}

{#if loadError}
  <p class="error">{loadError}</p>
{:else if projects === null}
  <p class="muted">Loading…</p>
{:else if projects.length === 0}
  <p class="muted">No projects yet. A project groups the applications of one product.</p>
{:else}
  <table>
    <thead><tr><th>Name</th><th>Description</th></tr></thead>
    <tbody>
      {#each projects as p (p.id)}
        <tr>
          <td><a href={href(`/projects/${p.id}`)}>{p.name}</a></td>
          <td class="muted">{p.description}</td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

<style>
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }
  .head h1 {
    margin: 0;
  }
  .form {
    display: grid;
    gap: 0.8rem;
    max-width: 32rem;
    margin-bottom: 1.25rem;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
    justify-content: flex-end;
  }
</style>
