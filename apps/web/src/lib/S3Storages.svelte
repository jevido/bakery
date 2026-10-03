<script lang="ts">
  import { api, ApiError } from './api'
  import Field from './Field.svelte'
  import type { S3Storage, S3StorageInput } from './types'

  let storages = $state.raw<S3Storage[] | null>(null)
  let error = $state('')
  /** The storage being edited, 'new' for the add form, null when closed. */
  let editing = $state<S3Storage | 'new' | null>(null)

  let form = $state<S3StorageInput>(blank())
  let errors = $state<Record<string, string>>({})
  let message = $state('')
  let check = $state<{ ok: boolean; message?: string } | null>(null)
  let busy = $state(false)

  function blank(): S3StorageInput {
    return { name: '', endpoint: '', region: '', bucket: '', prefix: '', access_key: '', secret_key: '' }
  }

  async function load() {
    const r = await api<{ s3_storages: S3Storage[] }>('GET', '/s3-storages')
    storages = r.s3_storages
  }
  load().catch((e) => (error = e.message))

  function open(s: S3Storage | 'new') {
    editing = s
    form = s === 'new' ? blank() : { ...s, secret_key: '' }
    errors = {}
    message = ''
    check = null
  }

  function show(err: unknown) {
    if (!(err instanceof ApiError)) throw err
    errors = err.errors
    message = Object.keys(err.errors).length === 0 ? err.message : ''
  }

  async function test() {
    busy = true
    errors = {}
    message = ''
    check = null
    try {
      const id = editing && editing !== 'new' ? editing.id : undefined
      check = await api<{ ok: boolean; message?: string }>('POST', '/s3-storages/check', { ...form, id })
    } catch (err) {
      show(err)
    } finally {
      busy = false
    }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    message = ''
    try {
      if (editing === 'new') await api('POST', '/s3-storages', form)
      else if (editing) await api('PATCH', `/s3-storages/${editing.id}`, form)
      editing = null
      await load()
    } catch (err) {
      show(err)
    } finally {
      busy = false
    }
  }

  async function remove(s: S3Storage) {
    if (!confirm(`Delete the S3 storage ${s.name}? Backups already uploaded stay in the bucket.`)) return
    error = ''
    try {
      await api('DELETE', `/s3-storages/${s.id}`)
      await load()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      error = err.message
    }
  }
</script>

<p class="muted">
  S3-compatible buckets (AWS S3, Garage, Cloudflare R2, Backblaze B2 and the like) that database backups are uploaded to, next
  to the copy on this server.
</p>
{#if error}<p class="error">{error}</p>{/if}
{#if storages === null}
  {#if !error}<p class="muted">Loading…</p>{/if}
{:else if storages.length === 0}
  <p class="muted">None yet.</p>
{:else}
  <table>
    <thead><tr><th>Name</th><th>Endpoint</th><th>Bucket</th><th>Prefix</th><th></th></tr></thead>
    <tbody>
      {#each storages as s (s.id)}
        <tr>
          <td>{s.name}</td>
          <td class="mono">{s.endpoint}</td>
          <td class="mono">{s.bucket}</td>
          <td class="mono muted">{s.prefix || '—'}</td>
          <td>
            <div class="buttons">
              <button onclick={() => open(s)}>Edit</button>
              <button class="danger" onclick={() => remove(s)}>Delete</button>
            </div>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

{#if editing === null}
  <p><button onclick={() => open('new')}>Add S3 storage</button></p>
{:else}
  <form class="form" onsubmit={save}>
    <h3>{editing === 'new' ? 'Add S3 storage' : `Edit ${editing.name}`}</h3>
    <Field label="Name" bind:value={form.name} error={errors.name} required />
    <Field label="Endpoint" bind:value={form.endpoint} error={errors.endpoint} placeholder="https://s3.eu-west-1.amazonaws.com" required />
    <div class="row">
      <Field label="Region" bind:value={form.region} error={errors.region} placeholder="us-east-1" />
      <Field label="Bucket" bind:value={form.bucket} error={errors.bucket} required />
    </div>
    <Field label="Prefix (folder in the bucket, optional)" bind:value={form.prefix} error={errors.prefix} />
    <div class="row">
      <Field label="Access key" bind:value={form.access_key} error={errors.access_key} autocomplete="off" required />
      <Field
        label="Secret key"
        type="password"
        bind:value={form.secret_key}
        error={errors.secret_key}
        autocomplete="new-password"
        placeholder={editing !== 'new' ? 'unchanged' : ''}
        required={editing === 'new'}
      />
    </div>
    {#if check}
      <p class={check.ok ? 'ok' : 'error'} role="status">{check.ok ? 'Connected: The Bakery can reach the bucket.' : check.message}</p>
    {/if}
    {#if message}<p class="error">{message}</p>{/if}
    <div class="actions">
      <button type="button" onclick={test} disabled={busy}>Test connection</button>
      <span class="spacer"></span>
      <button type="button" onclick={() => (editing = null)}>Cancel</button>
      <button class="primary" disabled={busy}>Save</button>
    </div>
  </form>
{/if}

<style>
  .form {
    display: grid;
    gap: 0.8rem;
    max-width: 40rem;
    margin-top: 1rem;
  }
  h3,
  p {
    margin: 0;
  }
  p + table,
  table + p {
    margin-top: 0.8rem;
  }
  .row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
    gap: 0.8rem;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
  .spacer {
    flex: 1;
  }
  .buttons {
    display: flex;
    gap: 0.4rem;
    justify-content: flex-end;
  }
  .ok {
    color: var(--ok, #2e7d32);
  }
</style>
