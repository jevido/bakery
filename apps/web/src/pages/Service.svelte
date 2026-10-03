<script lang="ts">
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import { api, ApiError } from '../lib/api'
  import ContainerLogs from '../lib/ContainerLogs.svelte'
  import CopyButton from '../lib/CopyButton.svelte'
  import Field from '../lib/Field.svelte'
  import { go } from '../lib/router.svelte'
  import { session } from '../lib/session.svelte'
  import StatusBadge from '../lib/StatusBadge.svelte'
  import type { Service } from '../lib/types'

  let { id }: { id: number } = $props()

  type Tab = 'components' | 'variables' | 'compose' | 'logs' | 'settings'

  let service = $state.raw<Service | null>(null)
  let loadError = $state('')
  let tab = $state<Tab>('components')
  let acting = $state(false)
  let actionError = $state('')
  // Set after a save that only applies on the next redeploy.
  let pending = $state(false)

  async function load() {
    const r = await api<{ service: Service }>('GET', `/services/${id}`)
    service = r.service
  }

  $effect(() => {
    service = null
    loadError = ''
    load().catch((e) => (loadError = e.message))
  })

  // Quickly while an action runs, slower otherwise so a crash shows up.
  let deploying = $derived(service?.status === 'deploying')
  $effect(() => {
    const t = setInterval(() => load().catch(() => {}), deploying ? 2000 : 5000)
    return () => clearInterval(t)
  })

  async function act(action: 'start' | 'stop' | 'restart' | 'redeploy') {
    acting = true
    actionError = ''
    try {
      const r = await api<{ service: Service }>('POST', `/services/${id}/${action}`)
      service = r.service
      if (action !== 'stop') pending = false
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      actionError = err.message
    } finally {
      acting = false
    }
  }

  async function remove() {
    if (!service) return
    const ok = confirm(
      `Delete ${service.name}? Its containers, network and volumes with all their data are removed, and its domains stop answering. This cannot be undone.`,
    )
    if (!ok) return
    const projectId = service.project_id
    await api('DELETE', `/services/${id}`)
    go(`/projects/${projectId}`)
  }

  // --- Domains, one editor per public Component.
  let domainDrafts = $state<Record<string, string>>({})
  let domainErrors = $state<Record<string, string>>({})
  let domainSaved = $state('')

  function draft(component: string, domains: string[]): string {
    return domainDrafts[component] ?? domains.join('\n')
  }

  async function saveDomains(component: string) {
    domainErrors = {}
    domainSaved = ''
    const domains = (domainDrafts[component] ?? '')
      .split('\n')
      .map((d) => d.trim())
      .filter((d) => d !== '')
    try {
      const r = await api<{ service: Service }>('PATCH', `/services/${id}`, { domains: { [component]: domains } })
      service = r.service
      delete domainDrafts[component]
      domainSaved = component
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      domainErrors = { [component]: err.errors.domains ?? err.message }
    }
  }

  // --- Variables: the Owner's are editable, generated ones read-only.
  let revealed = $state<Record<string, boolean>>({})
  let variableDrafts = $state<Record<string, string>>({})
  let variablesError = $state('')

  async function saveVariables() {
    variablesError = ''
    try {
      const r = await api<{ service: Service }>('PATCH', `/services/${id}`, { variables: variableDrafts })
      service = r.service
      variableDrafts = {}
      pending = true
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      variablesError = err.errors.variables ?? err.message
    }
  }

  // --- Compose file.
  let composeDraft = $state<string | null>(null)
  let composeErrors = $state<string[]>([])

  async function saveCompose() {
    composeErrors = []
    try {
      const r = await api<{ service: Service }>('PATCH', `/services/${id}`, { compose: composeDraft })
      service = r.service
      composeDraft = null
      pending = true
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      composeErrors = (err.errors.compose ?? err.errors.domains ?? err.message).split('\n')
    }
  }

  // --- Settings.
  let nameDraft = $state<string | null>(null)
  let nameError = $state('')

  async function rename(e: SubmitEvent) {
    e.preventDefault()
    nameError = ''
    try {
      const r = await api<{ service: Service }>('PATCH', `/services/${id}`, { name: nameDraft })
      service = r.service
      nameDraft = null
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      nameError = err.errors.name ?? err.message
    }
  }

  // --- Logs.
  let logComponent = $state('')
  let logTarget = $derived(logComponent || service?.components[0]?.name || '')

  // The top bar's breadcrumb: Project › Service.
  const crumbProject = $derived(service?.project_id)
  const crumbName = $derived(service?.name)
  $effect(() => {
    if (crumbProject !== undefined && crumbName !== undefined) breadcrumb.resource(crumbProject, crumbName)
  })
</script>

{#if loadError}
  <p class="error">{loadError}</p>
{:else if !service}
  <p class="muted">Loading…</p>
{:else}
  <div class="head">
    <div class="heading">
      <h1>{service.name}</h1>
      <StatusBadge status={service.status} />
      <p class="muted">
        {service.components.length}
        {service.components.length === 1 ? 'component' : 'components'}{service.template ? ` · from the ${service.template} template` : ''}
      </p>
    </div>
    {#if session.canWrite}<div class="buttons">
      {#if service.desired_state === 'stopped'}
        <button class="primary" disabled={acting || service.busy} onclick={() => act('start')}>Start</button>
      {:else}
        <button disabled={acting || service.busy} onclick={() => act('restart')}>Restart</button>
        <button disabled={acting} onclick={() => act('stop')}>Stop</button>
      {/if}
      <button class={[pending && 'primary']} disabled={acting || service.busy} onclick={() => act('redeploy')}>Redeploy</button>
    </div>{/if}
  </div>
  {#if actionError}<p class="error">{actionError}</p>{/if}
  {#if service.last_error}<p class="error">{service.last_error}</p>{/if}
  {#if pending}<p class="muted">Saved. Redeploy to apply the change.</p>{/if}

  <div class="tabs" role="tablist">
    <button role="tab" aria-selected={tab === 'components'} onclick={() => (tab = 'components')}>Components</button>
    <button role="tab" aria-selected={tab === 'variables'} onclick={() => (tab = 'variables')}>Variables</button>
    <button role="tab" aria-selected={tab === 'compose'} onclick={() => (tab = 'compose')}>Compose file</button>
    <button role="tab" aria-selected={tab === 'logs'} onclick={() => (tab = 'logs')}>Logs</button>
    <button role="tab" aria-selected={tab === 'settings'} onclick={() => (tab = 'settings')}>Settings</button>
  </div>

  {#if tab === 'components'}
    <div class="components">
      {#each service.components as c (c.name)}
        <div class="card component">
          <div class="row">
            <strong>{c.name}</strong>
            <StatusBadge status={c.status} />
            <span class="mono muted">{c.image}</span>
          </div>
          {#if c.detail}<p class="error">{c.detail}</p>{/if}
          {#if c.public}
            <p>
              {#each c.domains as d (d)}
                <a class="mono" href={`https://${d}`} target="_blank" rel="noreferrer">{d}</a>
              {/each}
              <span class="muted">→ port {c.port}</span>
            </p>
            <label class="domains">
              <span class="muted">Domains, one per line (the first is the primary one)</span>
              <textarea
                readonly={!session.canWrite}
                rows={Math.max(2, c.domains.length + 1)}
                value={draft(c.name, c.domains)}
                oninput={(e) => (domainDrafts[c.name] = e.currentTarget.value)}
              ></textarea>
            </label>
            {#if domainErrors[c.name]}<p class="error">{domainErrors[c.name]}</p>{/if}
            {#if session.canWrite}<div class="actions">
              {#if domainSaved === c.name}<span class="ok">Saved.</span>{/if}
              <button disabled={domainDrafts[c.name] === undefined} onclick={() => saveDomains(c.name)}>Save domains</button>
            </div>{/if}
          {:else}
            <p class="muted">Only reachable by the other components of this service, as <span class="mono">{c.name}</span>.</p>
          {/if}
        </div>
      {/each}
    </div>
  {:else if tab === 'variables'}
    {@const vars = service.variables ?? []}
    {#if vars.length === 0}
      <p class="muted">The compose file uses no variables.</p>
    {:else}
      <table>
        <thead><tr><th>Name</th><th>Value</th><th></th></tr></thead>
        <tbody>
          {#each vars as v (v.name)}
            <tr>
              <td class="mono">{v.name}</td>
              {#if v.hidden}
                <td class="mono muted">••••••••</td>
                <td class="muted small">hidden for viewers</td>
              {:else if v.magic}
                <td class="mono">{revealed[v.name] ? v.value : '••••••••'}</td>
                <td class="cell-actions">
                  <button onclick={() => (revealed[v.name] = !revealed[v.name])}>{revealed[v.name] ? 'Hide' : 'Reveal'}</button>
                  <CopyButton text={v.value} />
                  <span class="muted small">generated</span>
                </td>
              {:else}
                <td>
                  <input
                    class="mono"
                    aria-label={v.name}
                    readonly={!session.canWrite}
                    value={variableDrafts[v.name] ?? v.value}
                    placeholder={v.default ?? ''}
                    oninput={(e) => (variableDrafts[v.name] = e.currentTarget.value)}
                  />
                </td>
                <td class="muted small">{v.default !== null ? `default ${v.default || '(empty)'}` : ''}</td>
              {/if}
            </tr>
          {/each}
        </tbody>
      </table>
      {#if variablesError}<p class="error">{variablesError}</p>{/if}
      {#if session.canWrite}
        <div class="actions">
          <button class="primary" disabled={Object.keys(variableDrafts).length === 0} onclick={saveVariables}>Save variables</button>
        </div>
      {/if}
    {/if}
  {:else if tab === 'compose'}
    <textarea
      class="compose"
      rows="20"
      spellcheck="false"
      aria-label="Compose file"
      readonly={!session.canWrite}
      value={composeDraft ?? service.compose ?? ''}
      oninput={(e) => (composeDraft = e.currentTarget.value)}
    ></textarea>
    {#if composeErrors.length > 0}
      <ul class="error lines">
        {#each composeErrors as line, i (i)}<li>{line}</li>{/each}
      </ul>
    {/if}
    {#if session.canWrite}
      <div class="actions">
        <button class="primary" disabled={composeDraft === null} onclick={saveCompose}>Save compose file</button>
      </div>
    {/if}
  {:else if tab === 'logs'}
    <div class="row">
      <label>
        Component
        <select value={logTarget} onchange={(e) => (logComponent = e.currentTarget.value)}>
          {#each service.components as c (c.name)}<option value={c.name}>{c.name}</option>{/each}
        </select>
      </label>
    </div>
    {#key `${logTarget}-${service.desired_state}`}
      <ContainerLogs
        url={`/api/services/${service.id}/components/${encodeURIComponent(logTarget)}/logs`}
        empty="No container: the service is stopped or this component has not started."
        stopped="The container stopped (a restart or a redeploy replaces it)."
      />
    {/key}
  {:else if tab === 'settings'}
    <form class="form" onsubmit={rename}>
      <fieldset class="contents" disabled={!session.canWrite}>
      <Field
        label="Name"
        bind:value={() => nameDraft ?? service?.name ?? '', (v) => (nameDraft = v)}
        error={nameError}
        required
      />
      </fieldset>
      {#if session.canWrite}<div class="actions"><button class="primary" disabled={nameDraft === null}>Save</button></div>{/if}
    </form>
    {#if session.canWrite}
      <h2>Danger zone</h2>
      <p class="muted">Deleting removes the containers, the network and every volume of this service with its data.</p>
      <button class="danger" onclick={remove}>Delete service</button>
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
  .buttons,
  .row,
  .actions,
  .cell-actions {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    flex-wrap: wrap;
  }
  .actions {
    justify-content: flex-end;
    margin-top: 0.5rem;
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
  .components {
    display: grid;
    gap: 0.75rem;
  }
  .component {
    display: grid;
    gap: 0.5rem;
  }
  .component p {
    margin: 0;
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
  }
  .domains {
    display: grid;
    gap: 0.3rem;
  }
  textarea {
    font-family: ui-monospace, monospace;
    font-size: 0.85rem;
    width: 100%;
  }
  .lines {
    padding-left: 1.2rem;
    font-family: ui-monospace, monospace;
    font-size: 0.8rem;
  }
  .form {
    display: grid;
    gap: 0.8rem;
    max-width: 40rem;
  }
  .small {
    font-size: 0.8rem;
  }
  .ok {
    color: var(--ok);
  }
</style>
