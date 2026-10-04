<script lang="ts">
  import { api, ApiError } from './api'
  import { session } from './session.svelte'
  import type { EnvironmentVariable, InheritedVariable } from './types'
  import Button from './ui/Button.svelte'

  let {
    path,
    description = 'Values are stored encrypted and apply on the next deploy.',
  }: {
    /** The API path the variables are read from and saved to. */
    path: string
    description?: string
  } = $props()

  type Row = EnvironmentVariable & { key: number; revealed: boolean }

  let rows = $state<Row[]>([])
  let inherited = $state.raw<InheritedVariable[]>([])
  let loaded = $state(false)
  let error = $state('')
  let saved = $state(false)
  let busy = $state(false)
  let nextKey = 0

  let anyBuild = $derived(rows.some((r) => r.build))

  function toRows(vars: EnvironmentVariable[]): Row[] {
    return vars.map((v) => ({ ...v, key: nextKey++, revealed: false }))
  }

  function load() {
    return api<{ environment_variables: EnvironmentVariable[]; inherited?: InheritedVariable[] }>('GET', path).then((r) => {
      rows = toRows(r.environment_variables)
      inherited = r.inherited ?? []
      loaded = true
    })
  }

  $effect(() => {
    loaded = false
    // Variable values are Secrets; a viewer's request would be refused.
    if (!session.canSeeSecrets) return
    load().catch((e) => (error = e.message))
  })

  function add() {
    rows.push({ name: '', value: '', build: false, runtime: true, key: nextKey++, revealed: true })
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
      const variables = rows
        .filter((r) => r.name.trim() !== '')
        .map((r) => ({ name: r.name.trim(), value: r.value, build: r.build, runtime: r.runtime }))
      await api('PUT', path, { environment_variables: variables })
      // Read back, so overridden inherited variables are marked again.
      await load()
      saved = true
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      error = err.message
    } finally {
      busy = false
    }
  }

  function scope(v: EnvironmentVariable): string {
    return [v.build && 'build', v.runtime && 'runtime'].filter(Boolean).join(' + ')
  }
</script>

{#if !session.canSeeSecrets}
  <p class="muted">Variables are hidden for viewers.</p>
{:else if !loaded && !error}
  <p class="muted">Loading…</p>
{:else}
  <form onsubmit={save}>
    <p class="muted">{description}</p>
    {#each rows as row (row.key)}
      <div class="row">
        <input class="mono" aria-label="Name" placeholder="NAME" bind:value={row.name} oninput={() => (saved = false)} />
        <input
          class="mono"
          aria-label="Value for {row.name || 'new variable'}"
          type={row.revealed ? 'text' : 'password'}
          autocomplete="off"
          bind:value={row.value}
          oninput={() => (saved = false)}
        />
        <label class="scope">
          <input type="checkbox" bind:checked={row.build} onchange={() => (saved = false)} />
          Build
        </label>
        <label class="scope">
          <input type="checkbox" bind:checked={row.runtime} onchange={() => (saved = false)} />
          Runtime
        </label>
        <Button onclick={() => (row.revealed = !row.revealed)}>
          {row.revealed ? 'Hide' : 'Reveal'}
        </Button>
        <Button variant="error" aria-label="Remove {row.name}" onclick={() => removeRow(row.key)}>Remove</Button>
      </div>
    {/each}
    {#if anyBuild}
      <p class="muted small">
        Build variables are handed to the Dockerfile's <code>ARG</code>s and end up in the image's history; keep secrets
        runtime only.
      </p>
    {/if}
    {#if error}<p class="error">{error}</p>{/if}
    <div class="actions">
      <Button onclick={add}>Add variable</Button>
      <Button type="submit" variant="highlighted" loading={busy}>Save</Button>
      {#if saved}<span class="ok">Saved</span>{/if}
    </div>
  </form>

  {#if inherited.length > 0}
    <h3>Shared variables</h3>
    <p class="muted">Inherited from the environment and the project. The application's own variables win.</p>
    <table>
      <thead><tr><th>Name</th><th>From</th><th>Scope</th><th></th></tr></thead>
      <tbody>
        {#each inherited as v (v.from + v.name)}
          <tr class={{ overridden: v.overridden }}>
            <td class="mono">{v.name}</td>
            <td>{v.from}</td>
            <td class="muted">{scope(v)}</td>
            <td class="muted">{v.overridden ? 'overridden' : ''}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
{/if}

<style>
  form {
    display: grid;
    gap: 0.5rem;
    max-width: 56rem;
  }
  form > p {
    margin: 0 0 0.25rem;
  }
  .row {
    display: grid;
    grid-template-columns: minmax(8rem, 14rem) 1fr auto auto auto auto;
    gap: 0.5rem;
    align-items: center;
  }
  .scope {
    display: flex;
    gap: 0.25rem;
    align-items: center;
    font-size: 0.85rem;
  }
  .small {
    font-size: 0.85em;
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
  h3 {
    margin: 1.5rem 0 0.25rem;
  }
  .overridden td:first-child {
    text-decoration: line-through;
  }
  @media (max-width: 640px) {
    .row {
      grid-template-columns: 1fr 1fr;
    }
  }
</style>
