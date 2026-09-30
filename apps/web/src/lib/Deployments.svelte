<script lang="ts">
  import { api, ApiError } from './api'
  import LogView from './LogView.svelte'
  import StatusBadge from './StatusBadge.svelte'
  import type { Deployment } from './types'

  let {
    deployments,
    selected = $bindable(null),
    serverNames = {},
    onchange,
  }: {
    deployments: Deployment[]
    selected?: number | null
    /** Server names by id, for the server a deployment ran on. */
    serverNames?: Record<number, string>
    /** Called when the open deployment's status changes. */
    onchange: () => void
  } = $props()

  let current = $derived(deployments.find((d) => d.id === selected) ?? null)
  // The one serving now: the newest finished deployment.
  let live = $derived(deployments.find((d) => d.status === 'finished') ?? null)
  let actionError = $state('')
  let busy = $state(false)

  async function act(path: string, confirmText?: string) {
    if (confirmText && !confirm(confirmText)) return
    busy = true
    actionError = ''
    try {
      const r = await api<{ deployment: Deployment }>('POST', path)
      onchange()
      return r.deployment
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      actionError = err.message
    } finally {
      busy = false
    }
  }

  function cancel(d: Deployment) {
    return act(`/deployments/${d.id}/cancel`)
  }

  async function rollback(d: Deployment) {
    const what = d.source_image ? shortImage(d.source_image) : d.commit_sha.slice(0, 7)
    const r = await act(`/deployments/${d.id}/rollback`, `Roll back to deployment #${d.id} (${what})?`)
    if (r) selected = r.id
  }

  /** docker.io/traefik/whoami@sha256:0123456789ab… → whoami@0123456789ab */
  function shortImage(ref: string): string {
    const [repo, digest = ''] = ref.split('@')
    return `${repo.split('/').pop()}@${digest.replace('sha256:', '').slice(0, 12)}`
  }

  function duration(d: Deployment): string {
    if (!d.started_at) return ''
    const end = d.finished_at ? new Date(d.finished_at) : new Date()
    const s = Math.max(0, Math.round((end.getTime() - new Date(d.started_at).getTime()) / 1000))
    return s < 60 ? `${s}s` : `${Math.floor(s / 60)}m ${s % 60}s`
  }

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'medium' })
</script>

{#if current}
  <div class="head">
    <button onclick={() => (selected = null)}>← All deployments</button>
    <span>Deployment #{current.id}</span>
    <StatusBadge status={current.status} />
    {@render trigger(current)}
    {#if current.branch}<span class="muted">{current.branch}</span>{/if}
    {#if current.commit_sha}<span class="mono muted">{current.commit_sha.slice(0, 12)}</span>{/if}
    {#if current.commit_message}<span class="subject" title={current.commit_message}>{current.commit_message}</span>{/if}
    {#if current.source_image}<span class="mono muted" title={current.source_image}>{shortImage(current.source_image)}</span>{/if}
    {#if current.server_id}<span class="muted">on {serverNames[current.server_id] ?? `server ${current.server_id}`}</span>{/if}
    {#if current.active}<button class="danger" disabled={busy} onclick={() => cancel(current)}>Cancel</button>{/if}
  </div>
  {#if actionError}<p class="error">{actionError}</p>{/if}
  {#if current.error}<p class="error">{current.error}</p>{/if}
  {#key current.id}
    <LogView url={`/api/deployments/${current.id}/log`} onstatus={onchange} onend={onchange} />
  {/key}
{:else if deployments.length === 0}
  <p class="muted">No deployments yet. Press Deploy to build and start this application.</p>
{:else}
  {#if actionError}<p class="error">{actionError}</p>{/if}
  <table>
    <thead><tr><th>#</th><th>Status</th><th>Source</th><th>Started</th><th>Duration</th><th></th></tr></thead>
    <tbody>
      {#each deployments as d (d.id)}
        <tr>
          <td><button class="link" onclick={() => (selected = d.id)}>#{d.id}</button></td>
          <td><StatusBadge status={d.status} /></td>
          <td class="commit">
            <div class="subject" title={d.source_image || d.commit_message}>
              {#if d.source_image}
                <span class="mono">{shortImage(d.source_image)}</span>
              {:else}
                <span class="mono muted">{d.commit_sha.slice(0, 7)}</span>
                {d.commit_message}
              {/if}
              {@render trigger(d)}
            </div>
            {#if d.branch || d.commit_author}
              <div class="muted small">{[d.commit_author, d.branch].filter(Boolean).join(' on ')}</div>
            {/if}
          </td>
          <td class="muted">{when.format(new Date(d.started_at ?? d.created_at))}</td>
          <td class="muted">{duration(d)}</td>
          <td class="actions">
            {#if d.active}
              <button disabled={busy} onclick={() => cancel(d)}>Cancel</button>
            {:else if d.status === 'finished' && d.id !== live?.id}
              <button disabled={busy} onclick={() => rollback(d)}>Roll back</button>
            {/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

{#snippet trigger(d: Deployment)}
  {#if d.trigger === 'webhook'}<span class="tag">webhook</span>{/if}
  {#if d.trigger === 'rollback'}<span class="tag">rollback of #{d.rollback_of}</span>{/if}
{/snippet}

<style>
  .actions {
    text-align: right;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-bottom: 0.75rem;
  }
  .commit {
    max-width: 28rem;
  }
  .subject {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
  .small {
    font-size: 0.85em;
  }
  .tag {
    font-size: 0.75em;
    padding: 0.05rem 0.4rem;
    border-radius: 0.6rem;
    border: 1px solid var(--accent);
    color: var(--accent);
  }
  .link {
    background: none;
    border: 0;
    padding: 0;
    color: var(--accent);
    text-decoration: underline;
  }
</style>
