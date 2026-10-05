<script lang="ts">
  // The add form from before Coolify's look, at Coolify's #/servers/new,
  // until the New server page is ported.
  import { api, ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import Field from '../../lib/Field.svelte'
  import { go, href, serverPath } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Server } from '../../lib/types'

  let name = $state('')
  let host = $state('')
  let port = $state('22')
  let user = $state('')
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)

  async function add(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    try {
      const { server } = await api<{ server: Server }>('POST', '/servers', { name, host, port: Number(port) || 0, user })
      go(serverPath(server.id, 'private-key'))
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      busy = false
    }
  }

  $effect(() => breadcrumb.set({ label: 'Servers', href: href('/servers') }, { label: 'New server' }))
</script>

<h1>New server</h1>
{#if !session.isAdmin}
  <p class="muted">Only an Admin or the Owner adds servers.</p>
{:else}
<form class="card form" onsubmit={add}>
  <p class="muted">
    A server The Bakery reaches over SSH as a user with rootless Podman. The Bakery generates a key for it; you add that key to the
    user's <span class="mono">~/.ssh/authorized_keys</span> next.
  </p>
  <Field label="Name" bind:value={name} error={errors.name} required />
  <div class="row">
    <Field label="Host" bind:value={host} error={errors.host} placeholder="203.0.113.10" required />
    <Field label="SSH port" type="number" bind:value={port} error={errors.port} />
  </div>
  <Field label="User" bind:value={user} error={errors.user} placeholder="bakery" required />
  <div class="actions">
    <a class="button" href={href('/servers')}>Cancel</a>
    <button class="primary" disabled={busy}>Add server</button>
  </div>
</form>
{/if}

<style>
  .form {
    display: grid;
    gap: 0.8rem;
    max-width: 32rem;
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
</style>
