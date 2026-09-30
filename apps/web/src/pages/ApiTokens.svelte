<script lang="ts">
  import { api, ApiError } from '../lib/api'
  import CopyButton from '../lib/CopyButton.svelte'
  import Field from '../lib/Field.svelte'

  type ApiToken = { id: number; name: string; read_only: boolean; created_at: string; last_used_at: string | null }

  let tokens = $state.raw<ApiToken[] | null>(null)
  let loadError = $state('')
  let name = $state('')
  let readOnly = $state(false)
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)
  // The value of the token just made; the API never shows it again.
  let created = $state.raw<{ name: string; token: string } | null>(null)

  async function load() {
    const r = await api<{ api_tokens: ApiToken[] }>('GET', '/api-tokens')
    tokens = r.api_tokens
  }
  load().catch((e) => (loadError = e.message))

  async function create(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    try {
      const r = await api<{ api_token: ApiToken; token: string }>('POST', '/api-tokens', { name, read_only: readOnly })
      created = { name: r.api_token.name, token: r.token }
      name = ''
      readOnly = false
      await load()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      busy = false
    }
  }

  async function revoke(t: ApiToken) {
    if (!confirm(`Revoke the API token "${t.name}"? Anything using it stops working at once.`)) return
    await api('DELETE', `/api-tokens/${t.id}`)
    if (created?.name === t.name) created = null
    await load()
  }

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })
  let example = $derived(created ? `curl -H "Authorization: Bearer ${created.token}" ${location.origin}/api/projects` : '')
</script>

<h1>API tokens</h1>
<p class="muted">
  Tokens let scripts and CI use Bakery's API as you, with your role. A read-only token can only read, like a viewer.
</p>

<form class="card form" onsubmit={create}>
  <Field label="Name" bind:value={name} error={errors.name} placeholder="ci" required />
  <label class="check">
    <input type="checkbox" bind:checked={readOnly} />
    Read-only
  </label>
  <div class="actions">
    <button class="primary" disabled={busy}>Create token</button>
  </div>
  {#if created}
    <div class="created" data-testid="new-token">
      <p>Copy the token <strong>{created.name}</strong> now; Bakery cannot show it again.</p>
      <div class="value">
        <input readonly value={created.token} aria-label="New API token" />
        <CopyButton text={created.token} />
      </div>
      <p class="muted small">Use it like this:</p>
      <pre class="mono small">{example}</pre>
    </div>
  {/if}
</form>

{#if loadError}
  <p class="error">{loadError}</p>
{:else if tokens === null}
  <p class="muted">Loading…</p>
{:else if tokens.length === 0}
  <p class="muted">No tokens yet.</p>
{:else}
  <table>
    <thead><tr><th>Name</th><th>Access</th><th>Created</th><th>Last used</th><th></th></tr></thead>
    <tbody>
      {#each tokens as t (t.id)}
        <tr data-testid="api-token">
          <td>{t.name}</td>
          <td>{t.read_only ? 'read-only' : 'full'}</td>
          <td class="muted">{when.format(new Date(t.created_at))}</td>
          <td class="muted">{t.last_used_at ? when.format(new Date(t.last_used_at)) : 'never'}</td>
          <td><button class="danger" onclick={() => revoke(t)}>Revoke</button></td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

<style>
  .form {
    display: grid;
    gap: 0.8rem;
    max-width: 36rem;
    margin-bottom: 1.5rem;
  }
  .check {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
  }
  .created p {
    margin: 0 0 0.4rem;
  }
  .value {
    display: flex;
    gap: 0.5rem;
  }
  .value input {
    flex: 1;
    font-family: var(--mono, monospace);
  }
  .small {
    font-size: 0.8rem;
  }
  pre {
    white-space: pre-wrap;
    word-break: break-all;
    margin: 0;
  }
</style>
