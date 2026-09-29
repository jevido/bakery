<script lang="ts">
  import { api } from '../lib/api'
  import ApplicationForm from '../lib/ApplicationForm.svelte'
  import EnvEditor from '../lib/EnvEditor.svelte'
  import { go, href } from '../lib/router.svelte'
  import type { Application, ApplicationInput } from '../lib/types'

  let { id }: { id: number } = $props()

  type Tab = 'general' | 'env'

  let application = $state.raw<Application | null>(null)
  let loadError = $state('')
  let tab = $state<Tab>('general')
  let saved = $state(false)

  $effect(() => {
    application = null
    loadError = ''
    api<{ application: Application }>('GET', `/applications/${id}`)
      .then((r) => (application = r.application))
      .catch((e) => (loadError = e.message))
  })

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
    <div>
      <h1>{application.name}</h1>
      <p class="muted mono">{application.domain}</p>
    </div>
  </div>

  <div class="tabs" role="tablist">
    <button role="tab" aria-selected={tab === 'general'} onclick={() => (tab = 'general')}>General</button>
    <button role="tab" aria-selected={tab === 'env'} onclick={() => (tab = 'env')}>Environment variables</button>
  </div>

  {#if tab === 'general'}
    {#key application.id}
      <ApplicationForm initial={application} submitLabel="Save" domainPlaceholder={`${application.slug}.localhost`} onsubmit={update} />
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
    margin-bottom: 1rem;
  }
  .head h1,
  .head p {
    margin: 0;
  }
  .tabs {
    display: flex;
    gap: 0.25rem;
    border-bottom: 1px solid var(--border);
    margin-bottom: 1.25rem;
  }
  .tabs button {
    background: none;
    border: 0;
    border-bottom: 2px solid transparent;
    border-radius: 0;
    color: var(--muted);
    padding: 0.5rem 0.75rem;
  }
  .tabs button[aria-selected='true'] {
    color: var(--text);
    border-bottom-color: var(--accent);
  }
  .ok {
    color: var(--ok);
  }
</style>
