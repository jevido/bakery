<script lang="ts">
  import { api, ApiError } from '../lib/api'
  import Field from '../lib/Field.svelte'
  import { go, href } from '../lib/router.svelte'
  import ServerMeters from '../lib/ServerMeters.svelte'
  import StatusBadge from '../lib/StatusBadge.svelte'
  import type { Metrics, Server } from '../lib/types'

  let servers = $state.raw<Server[] | null>(null)
  // Metrics per Server id; missing while loading or when unreachable.
  let metrics = $state.raw<Record<number, Metrics>>({})
  let loadError = $state('')
  let adding = $state(false)
  let name = $state('')
  let host = $state('')
  let port = $state('22')
  let user = $state('')
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)

  async function load() {
    const r = await api<{ servers: Server[] }>('GET', '/servers')
    servers = r.servers
    for (const s of r.servers) {
      if (s.status !== 'reachable') continue
      api<Metrics>('GET', `/servers/${s.id}/metrics`)
        .then((m) => (metrics = { ...metrics, [s.id]: m }))
        .catch(() => {})
    }
  }

  $effect(() => {
    load().catch((e) => (loadError = e.message))
    const t = setInterval(() => load().catch(() => {}), 10000)
    return () => clearInterval(t)
  })

  async function add(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    try {
      const { server } = await api<{ server: Server }>('POST', '/servers', { name, host, port: Number(port) || 0, user })
      go(`/servers/${server.id}`)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      busy = false
    }
  }
</script>

<div class="head">
  <h1>Servers</h1>
  {#if !adding}
    <button class="primary" onclick={() => (adding = true)}>Add server</button>
  {/if}
</div>

{#if adding}
  <form class="card form" onsubmit={add}>
    <p class="muted">
      A server Bakery reaches over SSH as a user with rootless Podman. Bakery generates a key for it; you add that key to the
      user's <span class="mono">~/.ssh/authorized_keys</span> next.
    </p>
    <Field label="Name" bind:value={name} error={errors.name} required />
    <div class="row">
      <Field label="Host" bind:value={host} error={errors.host} placeholder="203.0.113.10" required />
      <Field label="SSH port" type="number" bind:value={port} error={errors.port} />
    </div>
    <Field label="User" bind:value={user} error={errors.user} placeholder="bakery" required />
    <div class="actions">
      <button type="button" onclick={() => (adding = false)}>Cancel</button>
      <button class="primary" disabled={busy}>Add server</button>
    </div>
  </form>
{/if}

{#if loadError}
  <p class="error">{loadError}</p>
{:else if servers === null}
  <p class="muted">Loading…</p>
{:else}
  <div class="list">
    {#each servers as s (s.id)}
      <a class="card server" href={href(`/servers/${s.id}`)} data-testid="server">
        <div class="title">
          <strong>{s.name}</strong>
          <StatusBadge status={s.status} />
          <span class="muted mono">{s.kind === 'local' ? 'this machine' : `${s.user}@${s.host}:${s.port}`}</span>
        </div>
        {#if metrics[s.id]}
          <ServerMeters metrics={metrics[s.id].server} />
        {:else if s.status === 'reachable'}
          <p class="muted">Reading metrics…</p>
        {:else if s.status === 'unvalidated'}
          <p class="muted">Not validated yet.</p>
        {:else}
          <p class="muted">Unreachable: open it to see why.</p>
        {/if}
      </a>
    {/each}
  </div>
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
  .form p {
    margin: 0;
  }
  .row {
    display: grid;
    grid-template-columns: 1fr 8rem;
    gap: 0.8rem;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
    justify-content: flex-end;
  }
  .list {
    display: grid;
    gap: 0.75rem;
  }
  .server {
    display: grid;
    gap: 0.75rem;
    color: inherit;
    text-decoration: none;
  }
  .server:hover {
    border-color: var(--muted);
  }
  .title {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
  }
  .server p {
    margin: 0;
  }
</style>
