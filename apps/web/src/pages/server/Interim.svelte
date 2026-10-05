<script lang="ts">
  // The sections of the Server page from before Coolify's look, each under
  // the sidebar item that names it, until that sub-page is ported.
  import { api, ApiError } from '../../lib/api'
  import { percent, size } from '../../lib/format'
  import { go, href, type ServerPage } from '../../lib/router.svelte'
  import ServerMeters from '../../lib/ServerMeters.svelte'
  import { session } from '../../lib/session.svelte'
  import type { ContainerMetrics, Metrics, Server } from '../../lib/types'

  let { server, page, onchange }: { server: Server; page: ServerPage; onchange: (s: Server) => void } = $props()
  const id = $derived(server.id)

  let metrics = $state.raw<Metrics | null>(null)
  let metricsError = $state('')
  let busy = $state('')
  let actionError = $state('')
  let cleaned = $state('')

  async function load() {
    const r = await api<{ server: Server }>('GET', `/servers/${id}`)
    onchange(r.server)
  }

  async function loadMetrics() {
    try {
      metrics = await api<Metrics>('GET', `/servers/${id}/metrics`)
      metricsError = ''
    } catch (e) {
      metricsError = e instanceof Error ? e.message : String(e)
    }
  }

  // Metrics every 5 s while the Server is reachable and Metrics is open.
  let reachable = $derived(server.status === 'reachable')
  $effect(() => {
    if (!reachable || page !== 'metrics') return
    loadMetrics()
    const t = setInterval(loadMetrics, 5000)
    return () => clearInterval(t)
  })

  async function run(what: string, f: () => Promise<void>) {
    busy = what
    actionError = ''
    try {
      await f()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      actionError = err.message
    } finally {
      busy = ''
    }
  }

  function cleanUp() {
    return run('cleanup', async () => {
      const r = await api<{ cleanup: Server['last_cleanup'] }>('POST', `/servers/${id}/cleanup`)
      cleaned = `Freed ${size(r.cleanup.reclaimed_bytes)}.`
      await load()
      await loadMetrics()
    })
  }

  let removeError = $state('')

  async function remove() {
    if (!confirm(`Remove ${server.name} from The Bakery? Nothing on the server itself is touched.`)) return
    removeError = ''
    try {
      await api('DELETE', `/servers/${id}`)
      go('/servers')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      removeError = err.message
    }
  }

  function ownerHref(c: ContainerMetrics): string | null {
    switch (c.owner) {
      case 'application':
        return href(`/applications/${c.owner_id}`)
      case 'database':
        return href(`/databases/${c.owner_id}`)
      case 'service':
        return href(`/services/${c.owner_id}`)
      default:
        return null
    }
  }

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })
</script>

{#if page === 'metrics' && server.status === 'reachable'}
  <section>
    <h2>Usage</h2>
    {#if metrics}
      <div class="card"><ServerMeters metrics={metrics.server} /></div>
      <p class="muted">
        Podman: images {size(metrics.server.images_bytes)}, containers {size(metrics.server.containers_bytes)}, volumes
        {size(metrics.server.volumes_bytes)}.
      </p>
      <h3>Containers</h3>
      {#if metrics.containers.length === 0}
        <p class="muted">The Bakery runs no containers here.</p>
      {:else}
        <table data-testid="containers">
          <thead><tr><th>Container</th><th>Belongs to</th><th>CPU</th><th>Memory</th></tr></thead>
          <tbody>
            {#each metrics.containers as c (c.name)}
              {@const link = ownerHref(c)}
              <tr>
                <td class="mono">{c.name}</td>
                <td>
                  {#if link}<a href={link}>{c.owner} {c.owner_id}</a>{:else}<span class="muted">{c.owner || '—'}</span>{/if}
                </td>
                <td>{percent(c.cpu_percent)}</td>
                <td>
                  {size(c.memory_used_bytes)}
                  {#if c.memory_limit_bytes && metrics.server.memory_total_bytes && c.memory_limit_bytes < metrics.server.memory_total_bytes}
                    <span class="muted">/ {size(c.memory_limit_bytes)}</span>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    {:else if metricsError}
      <p class="error">{metricsError}</p>
    {:else}
      <p class="muted">Reading metrics…</p>
    {/if}
  </section>
{:else if page === 'docker-cleanup' && server.status === 'reachable'}
  <section>
    <div class="head">
      <h2>Docker Cleanup</h2>
      {#if session.isAdmin}
        <button disabled={!!busy} onclick={cleanUp}>{busy === 'cleanup' ? 'Cleaning up…' : 'Clean up now'}</button>
      {/if}
    </div>
    {#if actionError}<p class="error">{actionError}</p>{/if}
    <p class="muted">
      Every night at 03:00 The Bakery removes images nothing needs any more: dangling images of The Bakery, and the images of
      deployments older than the five newest of each application that runs here.
    </p>
    {#if cleaned}<p data-testid="cleaned">{cleaned}</p>{/if}
    {#if server.last_cleanup.at}
      <p class="muted">
        Last cleanup {when.format(new Date(server.last_cleanup.at))}, freed {size(server.last_cleanup.reclaimed_bytes)}.
      </p>
    {/if}
  </section>
{:else if page === 'danger' && server.kind === 'remote' && session.isAdmin}
  <section>
    <h2>Danger</h2>
    <div class="actions">
      <button class="danger" onclick={remove}>Remove server</button>
    </div>
    {#if removeError}<p class="error" data-testid="remove-error">{removeError}</p>{/if}
  </section>
{:else if page === 'resources'}
  <section>
    <h2>Resources</h2>
    <p class="muted">The Applications, Databases and Services on this Server.</p>
  </section>
{:else}
  <section>
    <h2>Not available</h2>
    <p class="muted">This Server has no such page.</p>
  </section>
{/if}

<style>
  .head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
    flex-wrap: wrap;
    margin-bottom: 1rem;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  section {
    margin-bottom: 1.5rem;
  }
</style>
