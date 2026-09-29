<script lang="ts">
  import { api } from '../lib/api'
  import type { KnownHost } from '../lib/types'

  let hosts = $state.raw<KnownHost[] | null>(null)
  let error = $state('')

  async function load() {
    const r = await api<{ known_hosts: KnownHost[] }>('GET', '/known-hosts')
    hosts = r.known_hosts
  }
  load().catch((e) => (error = e.message))

  async function forget(h: KnownHost) {
    if (!confirm(`Forget the host key of ${h.host}? The next clone trusts whatever key it then presents.`)) return
    await api('DELETE', `/known-hosts/${h.id}`)
    await load()
  }

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })
</script>

<h1>Settings</h1>

<h2>Known hosts</h2>
<p class="muted">
  The SSH host keys of git hosts, trusted on the first clone from each. A clone fails if a host later shows another key; forget
  the host only if you know why its key changed.
</p>
{#if error}
  <p class="error">{error}</p>
{:else if hosts === null}
  <p class="muted">Loading…</p>
{:else if hosts.length === 0}
  <p class="muted">None yet. They appear after the first clone of an SSH repository.</p>
{:else}
  <table>
    <thead><tr><th>Host</th><th>Keys</th><th>First seen</th><th></th></tr></thead>
    <tbody>
      {#each hosts as h (h.id)}
        <tr>
          <td class="mono">{h.host}</td>
          <td class="mono muted small">
            {#each h.fingerprints as f (f)}<div>{f}</div>{/each}
          </td>
          <td class="muted">{when.format(new Date(h.created_at))}</td>
          <td><button class="danger" onclick={() => forget(h)}>Forget</button></td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

<style>
  .small {
    font-size: 0.8rem;
  }
</style>
