<script lang="ts">
  import LogView from './LogView.svelte'
  import StatusBadge from './StatusBadge.svelte'
  import type { Deployment } from './types'

  let {
    deployments,
    selected = $bindable(null),
    onchange,
  }: {
    deployments: Deployment[]
    selected?: number | null
    /** Called when the open deployment's status changes. */
    onchange: () => void
  } = $props()

  let current = $derived(deployments.find((d) => d.id === selected) ?? null)

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
    {#if current.commit_sha}<span class="mono muted">{current.commit_sha.slice(0, 12)}</span>{/if}
  </div>
  {#if current.error}<p class="error">{current.error}</p>{/if}
  {#key current.id}
    <LogView url={`/api/deployments/${current.id}/log`} onstatus={onchange} onend={onchange} />
  {/key}
{:else if deployments.length === 0}
  <p class="muted">No deployments yet. Press Deploy to build and start this application.</p>
{:else}
  <table>
    <thead><tr><th>#</th><th>Status</th><th>Commit</th><th>Started</th><th>Duration</th></tr></thead>
    <tbody>
      {#each deployments as d (d.id)}
        <tr>
          <td><button class="link" onclick={() => (selected = d.id)}>#{d.id}</button></td>
          <td><StatusBadge status={d.status} /></td>
          <td class="mono muted">{d.commit_sha.slice(0, 12)}</td>
          <td class="muted">{when.format(new Date(d.started_at ?? d.created_at))}</td>
          <td class="muted">{duration(d)}</td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

<style>
  .head {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-bottom: 0.75rem;
  }
  .link {
    background: none;
    border: 0;
    padding: 0;
    color: var(--accent);
    text-decoration: underline;
  }
</style>
