<script lang="ts">
  // The sections of the Server page from before Coolify's look, each under
  // the sidebar item that names it, until that sub-page is ported.
  import { api, ApiError } from '../../lib/api'
  import { size } from '../../lib/format'
  import { go, type ServerPage } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Server } from '../../lib/types'

  let { server, page, onchange }: { server: Server; page: ServerPage; onchange: (s: Server) => void } = $props()
  const id = $derived(server.id)

  let busy = $state('')
  let actionError = $state('')
  let cleaned = $state('')

  async function load() {
    const r = await api<{ server: Server }>('GET', `/servers/${id}`)
    onchange(r.server)
  }

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

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })
</script>

{#if page === 'docker-cleanup' && server.status === 'reachable'}
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
