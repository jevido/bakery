<script lang="ts">
  // The Service page's sub-pages as they were on the old tabbed page, until
  // each is ported to Coolify's markup: Environment Variables and Runtime
  // Logs.
  import { api, ApiError } from '../../lib/api'
  import ContainerLogs from '../../lib/ContainerLogs.svelte'
  import CopyButton from '../../lib/CopyButton.svelte'
  import type { ServicePage } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Service } from '../../lib/types'

  let { service, page, onchange }: { service: Service; page: ServicePage; onchange: (s: Service) => void } = $props()

  const id = $derived(service.id)
  // Set after a save that only applies on the next redeploy, cleared once one runs.
  let pending = $state(false)
  $effect(() => {
    if (service.busy) pending = false
  })

  // --- Variables: the Owner's are editable, generated ones read-only.
  let revealed = $state<Record<string, boolean>>({})
  let variableDrafts = $state<Record<string, string>>({})
  let variablesError = $state('')

  async function saveVariables() {
    variablesError = ''
    try {
      const r = await api<{ service: Service }>('PATCH', `/services/${id}`, { variables: variableDrafts })
      onchange(r.service)
      variableDrafts = {}
      pending = true
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      variablesError = err.errors.variables ?? err.message
    }
  }

  // --- Logs.
  let logComponent = $state('')
  let logTarget = $derived(logComponent || service?.components[0]?.name || '')

</script>

{#if pending}<p class="muted">Saved. Redeploy to apply the change.</p>{/if}

{#if page === 'environment-variables'}
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
{:else if page === 'logs'}
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
{/if}

<style>
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
  .small {
    font-size: 0.8rem;
  }
</style>
