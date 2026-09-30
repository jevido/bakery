<script lang="ts">
  import { api, ApiError } from '../lib/api'
  import CopyButton from '../lib/CopyButton.svelte'
  import Field from '../lib/Field.svelte'
  import { percent, size } from '../lib/format'
  import { go, href } from '../lib/router.svelte'
  import ServerMeters from '../lib/ServerMeters.svelte'
  import StatusBadge from '../lib/StatusBadge.svelte'
  import type { ContainerMetrics, Metrics, Server } from '../lib/types'

  let { id }: { id: number } = $props()

  let server = $state.raw<Server | null>(null)
  let loadError = $state('')
  let metrics = $state.raw<Metrics | null>(null)
  let metricsError = $state('')
  let busy = $state('')
  let actionError = $state('')
  let cleaned = $state('')

  let editing = $state(false)
  let name = $state('')
  let host = $state('')
  let port = $state('')
  let user = $state('')
  let errors = $state<Record<string, string>>({})

  async function load() {
    const r = await api<{ server: Server }>('GET', `/servers/${id}`)
    server = r.server
  }

  async function loadMetrics() {
    try {
      metrics = await api<Metrics>('GET', `/servers/${id}/metrics`)
      metricsError = ''
    } catch (e) {
      metricsError = e instanceof Error ? e.message : String(e)
    }
  }

  $effect(() => {
    server = null
    metrics = null
    loadError = ''
    load().catch((e) => (loadError = e.message))
  })

  // Metrics every 5 s while the Server is reachable.
  let reachable = $derived(server?.status === 'reachable')
  $effect(() => {
    if (!reachable) return
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

  function validate() {
    return run('validate', async () => {
      const r = await api<{ server: Server }>('POST', `/servers/${id}/validate`)
      server = r.server
    })
  }

  function forgetHostKey() {
    if (!confirm('Forget the host key? The next connection trusts whatever key the server then presents.')) return
    return run('forget', async () => {
      const r = await api<{ server: Server }>('DELETE', `/servers/${id}/host-key`)
      server = r.server
    })
  }

  function cleanUp() {
    return run('cleanup', async () => {
      const r = await api<{ cleanup: Server['last_cleanup'] }>('POST', `/servers/${id}/cleanup`)
      cleaned = `Freed ${size(r.cleanup.reclaimed_bytes)}.`
      await load()
      await loadMetrics()
    })
  }

  function startEditing(s: Server) {
    name = s.name
    host = s.host
    port = String(s.port)
    user = s.user
    errors = {}
    editing = true
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    errors = {}
    try {
      const r = await api<{ server: Server }>('PATCH', `/servers/${id}`, { name, host, port: Number(port) || 0, user })
      server = r.server
      editing = false
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    }
  }

  let removeError = $state('')

  async function remove() {
    if (!server || !confirm(`Remove ${server.name} from Bakery? Nothing on the server itself is touched.`)) return
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
  const checkLabels: Record<string, string> = {
    ssh: 'SSH',
    socket: 'Podman socket',
    podman: 'Podman version',
    linger: 'Linger',
    ports: 'Ports 80 and 443',
  }
</script>

{#if loadError}
  <p class="error">{loadError}</p>
{:else if !server}
  <p class="muted">Loading…</p>
{:else}
  <p class="crumbs"><a href={href('/servers')}>Servers</a> /</p>
  <div class="head">
    <div class="title">
      <h1>{server.name}</h1>
      <StatusBadge status={server.status} />
      <p class="muted mono">{server.kind === 'local' ? 'The machine Bakery runs on' : `${server.user}@${server.host}:${server.port}`}</p>
    </div>
    <div class="buttons">
      <button class="primary" disabled={!!busy} onclick={validate}>{busy === 'validate' ? 'Validating…' : 'Validate'}</button>
      {#if server.status === 'reachable'}
        <button disabled={!!busy} onclick={cleanUp}>{busy === 'cleanup' ? 'Cleaning up…' : 'Clean up now'}</button>
      {/if}
    </div>
  </div>
  {#if actionError}<p class="error">{actionError}</p>{/if}

  {#if server.kind === 'remote' && server.public_key}
    <section class="card">
      <h2>Server key</h2>
      <p class="muted">
        Bakery logs in with this key. Add it for <span class="mono">{server.user}</span> on the server, then press Validate:
      </p>
      <pre class="mono key" data-testid="server-key">{server.public_key}</pre>
      <pre class="mono key">echo '{server.public_key}' &gt;&gt; ~/.ssh/authorized_keys</pre>
      <div class="actions">
        <CopyButton text={server.public_key} label="Copy public key" />
        <CopyButton text={`echo '${server.public_key}' >> ~/.ssh/authorized_keys`} label="Copy command" />
      </div>
      {#if server.host_key_fingerprint}
        <p class="muted">
          Host key <span class="mono" data-testid="host-key">{server.host_key_fingerprint}</span>, pinned on the first connection.
          <button class="danger" disabled={!!busy} onclick={forgetHostKey}>Forget host key</button>
        </p>
      {/if}
    </section>
  {/if}

  <section>
    <h2>Validation</h2>
    {#if server.validation.checks.length === 0}
      <p class="muted">Not validated yet.</p>
    {:else}
      <ul class="checks" data-testid="checks">
        {#each server.validation.checks as c (c.name)}
          <li class={[c.ok ? 'ok' : c.required ? 'bad' : 'warn']}>
            <span class="mark" aria-hidden="true">{c.ok ? '✓' : c.required ? '✗' : '!'}</span>
            <div>
              <strong>{checkLabels[c.name] ?? c.name}</strong>
              {#if !c.required}<span class="muted">(not required)</span>{/if}
              <div class="muted detail">{c.detail}</div>
            </div>
          </li>
        {/each}
      </ul>
      {#if server.validation.checked_at}
        <p class="muted">Checked {when.format(new Date(server.validation.checked_at))}.</p>
      {/if}
    {/if}
  </section>

  {#if server.status === 'reachable'}
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
          <p class="muted">No Bakery containers run here.</p>
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

    <section>
      <h2>Cleanup</h2>
      <p class="muted">
        Every night at 03:00 Bakery removes images nothing needs any more: dangling Bakery images, and the images of
        deployments older than the five newest of each application that runs here.
      </p>
      {#if cleaned}<p data-testid="cleaned">{cleaned}</p>{/if}
      {#if server.last_cleanup.at}
        <p class="muted">
          Last cleanup {when.format(new Date(server.last_cleanup.at))}, freed {size(server.last_cleanup.reclaimed_bytes)}.
        </p>
      {/if}
    </section>
  {/if}

  {#if server.kind === 'remote'}
    <section>
      <h2>Settings</h2>
      {#if editing}
        <form class="card form" onsubmit={save}>
          <Field label="Name" bind:value={name} error={errors.name} required />
          <div class="row">
            <Field label="Host" bind:value={host} error={errors.host} required />
            <Field label="SSH port" type="number" bind:value={port} error={errors.port} />
          </div>
          <Field label="User" bind:value={user} error={errors.user} required />
          <p class="muted">A new host, port or user forgets the host key; validate again afterwards.</p>
          <div class="actions">
            <button type="button" onclick={() => (editing = false)}>Cancel</button>
            <button class="primary">Save</button>
          </div>
        </form>
      {:else}
        <div class="actions">
          <button onclick={() => server && startEditing(server)}>Edit</button>
          <button class="danger" onclick={remove}>Remove server</button>
        </div>
        {#if removeError}<p class="error" data-testid="remove-error">{removeError}</p>{/if}
      {/if}
    </section>
  {/if}
{/if}

<style>
  .crumbs {
    margin: 0 0 0.5rem;
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
    flex-wrap: wrap;
    margin-bottom: 1rem;
  }
  .title {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
  }
  .title h1,
  .title p {
    margin: 0;
  }
  .buttons,
  .actions {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  section {
    margin-bottom: 1.5rem;
  }
  .key {
    white-space: pre-wrap;
    word-break: break-all;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.6rem 0.75rem;
    font-size: 0.85rem;
  }
  .checks {
    list-style: none;
    padding: 0;
    display: grid;
    gap: 0.5rem;
  }
  .checks li {
    display: flex;
    gap: 0.6rem;
  }
  .mark {
    font-weight: 700;
    width: 1rem;
  }
  .ok .mark {
    color: var(--ok);
  }
  .bad .mark {
    color: var(--danger);
  }
  .warn .mark {
    color: var(--warn);
  }
  .detail {
    font-size: 0.85rem;
    word-break: break-word;
  }
  .form {
    display: grid;
    gap: 0.8rem;
    max-width: 32rem;
  }
  .form .actions {
    justify-content: flex-end;
  }
  .row {
    display: grid;
    grid-template-columns: 1fr 8rem;
    gap: 0.8rem;
  }
</style>
