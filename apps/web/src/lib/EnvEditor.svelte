<script lang="ts">
  import { api, ApiError } from './api'
  import type { EnvVar } from './types'

  let { applicationId }: { applicationId: number } = $props()

  type Row = EnvVar & { key: number; revealed: boolean }

  let rows = $state<Row[]>([])
  let loaded = $state(false)
  let error = $state('')
  let saved = $state(false)
  let busy = $state(false)
  let nextKey = 0

  function toRows(vars: EnvVar[]): Row[] {
    return vars.map((v) => ({ ...v, key: nextKey++, revealed: false }))
  }

  $effect(() => {
    loaded = false
    api<{ env: EnvVar[] }>('GET', `/applications/${applicationId}/env`)
      .then((r) => {
        rows = toRows(r.env)
        loaded = true
      })
      .catch((e) => (error = e.message))
  })

  function add() {
    rows.push({ name: '', value: '', key: nextKey++, revealed: true })
    saved = false
  }

  function removeRow(key: number) {
    rows = rows.filter((r) => r.key !== key)
    saved = false
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    error = ''
    saved = false
    try {
      const env = rows.filter((r) => r.name.trim() !== '').map((r) => ({ name: r.name.trim(), value: r.value }))
      const res = await api<{ env: EnvVar[] }>('PUT', `/applications/${applicationId}/env`, { env })
      rows = toRows(res.env)
      saved = true
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      error = err.message
    } finally {
      busy = false
    }
  }
</script>

{#if !loaded && !error}
  <p class="muted">Loading…</p>
{:else}
  <form onsubmit={save}>
    <p class="muted">Passed to the container on the next deploy. Values are stored encrypted.</p>
    {#each rows as row (row.key)}
      <div class="row">
        <input
          class="mono"
          aria-label="Name"
          placeholder="NAME"
          bind:value={row.name}
          oninput={() => (saved = false)}
        />
        <input
          class="mono"
          aria-label="Value for {row.name || 'new variable'}"
          type={row.revealed ? 'text' : 'password'}
          autocomplete="off"
          bind:value={row.value}
          oninput={() => (saved = false)}
        />
        <button type="button" onclick={() => (row.revealed = !row.revealed)}>
          {row.revealed ? 'Hide' : 'Reveal'}
        </button>
        <button type="button" class="danger" aria-label="Remove {row.name}" onclick={() => removeRow(row.key)}>
          Remove
        </button>
      </div>
    {/each}
    {#if error}<p class="error">{error}</p>{/if}
    <div class="actions">
      <button type="button" onclick={add}>Add variable</button>
      <button class="primary" disabled={busy}>Save</button>
      {#if saved}<span class="ok">Saved</span>{/if}
    </div>
  </form>
{/if}

<style>
  form {
    display: grid;
    gap: 0.5rem;
    max-width: 48rem;
  }
  form > p {
    margin: 0 0 0.25rem;
  }
  .row {
    display: grid;
    grid-template-columns: minmax(8rem, 14rem) 1fr auto auto;
    gap: 0.5rem;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    margin-top: 0.5rem;
  }
  .ok {
    color: var(--ok);
  }
  @media (max-width: 640px) {
    .row {
      grid-template-columns: 1fr 1fr;
    }
  }
</style>
