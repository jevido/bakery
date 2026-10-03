<script lang="ts">
  import { session } from './session.svelte'
  import { untrack } from 'svelte'
  import { ApiError } from './api'
  import { databaseTypes } from './databaseTypes'
  import Field from './Field.svelte'
  import type { Database, DatabaseInput, DatabaseType } from './types'

  // Without a database it creates one (name, Database type, version); with one it
  // edits what may change (name, version, public port, resource limits).
  let {
    database,
    submitLabel,
    onsubmit,
    oncancel,
  }: {
    database?: Database
    submitLabel: string
    onsubmit: (input: DatabaseInput) => Promise<void>
    oncancel?: () => void
  } = $props()

  // The form keeps its own copy; the parent re-creates it to reset.
  const start = untrack(() => database)
  let name = $state(start?.name ?? '')
  let databaseType = $state<DatabaseType>(start?.type ?? 'postgresql')
  let version = $state(start?.version ?? '')
  let publicPort = $state(start?.public_port == null ? '' : String(start.public_port))
  let memoryMB = $state(start?.resource_limits.memory_mb == null ? '' : String(start.resource_limits.memory_mb))
  let cpus = $state(start?.resource_limits.cpus == null ? '' : String(start.resource_limits.cpus))
  let errors = $state<Record<string, string>>({})
  let message = $state('')
  let busy = $state(false)

  let defaultVersion = $derived(databaseTypes.find((t) => t.type === databaseType)?.defaultVersion ?? '')

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    message = ''
    try {
      const input: DatabaseInput = {
        name,
        version,
        public_port: publicPort.trim() === '' ? null : Number(publicPort),
        resource_limits: {
          memory_mb: memoryMB.trim() === '' ? null : Number(memoryMB),
          cpus: cpus.trim() === '' ? null : Number(cpus),
        },
      }
      if (!start) input.type = databaseType
      await onsubmit(input)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = err.errors
      if (Object.keys(err.errors).length === 0) message = err.message
    } finally {
      busy = false
    }
  }
</script>

<form class="form" onsubmit={submit}>
  <!-- A viewer sees the values and cannot change them. -->
  <fieldset class="contents" disabled={!session.canWrite}>
  <Field label="Name" bind:value={name} error={errors.name} required />
  {#if !start}
    <fieldset>
      <legend>Database type</legend>
      <div class="choices">
        {#each databaseTypes as t (t.type)}
          <label class={['choice', databaseType === t.type && 'selected']}>
            <input type="radio" name="type" value={t.type} bind:group={databaseType} />
            {t.label}
          </label>
        {/each}
      </div>
      {#if errors.type}<small class="error">{errors.type}</small>{/if}
    </fieldset>
  {/if}
  <Field label="Version (image tag)" bind:value={version} error={errors.version} placeholder={start ? '' : defaultVersion} />
  {#if start}
    <Field
      label="Public port (reach it from outside the server)"
      type="number"
      bind:value={publicPort}
      error={errors.public_port}
      placeholder="not published"
    />
    <div class="row">
      <Field label="Memory (MB)" type="number" bind:value={memoryMB} error={errors['resource_limits.memory_mb']} placeholder="unlimited" />
      <Field label="CPU (cores)" bind:value={cpus} error={errors['resource_limits.cpus']} placeholder="unlimited" />
    </div>
    <p class="muted">Changing the version, public port or limits restarts the database; its data stays.</p>
  {/if}
  {#if message}<p class="error">{message}</p>{/if}
  </fieldset>
  {#if session.canWrite}
  <div class="actions">
    {#if oncancel}<button type="button" onclick={oncancel}>Cancel</button>{/if}
    <button class="primary" disabled={busy}>{submitLabel}</button>
  </div>
  {/if}
</form>

<style>
  .form {
    display: grid;
    gap: 0.8rem;
    max-width: 40rem;
  }
  .row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
    gap: 0.8rem;
  }
  fieldset:not(.contents) {
    display: grid;
    gap: 0.8rem;
    border: 1px solid var(--border, #8884);
    border-radius: 0.4rem;
    padding: 0.8rem;
    margin: 0;
  }
  .choices {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  .choice {
    display: flex;
    gap: 0.4rem;
    align-items: center;
    padding: 0.35rem 0.7rem;
    border: 1px solid var(--border, #8884);
    border-radius: 0.4rem;
    cursor: pointer;
  }
  .choice.selected {
    border-color: currentColor;
    font-weight: 600;
  }
  p {
    margin: 0;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
    justify-content: flex-end;
  }
</style>
