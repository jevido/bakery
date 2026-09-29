<script lang="ts">
  import { untrack } from 'svelte'
  import { ApiError } from './api'
  import Field from './Field.svelte'
  import type { ApplicationInput, HealthCheck } from './types'

  let {
    initial,
    submitLabel,
    domainPlaceholder = '<slug>.localhost',
    onsubmit,
    oncancel,
  }: {
    initial?: ApplicationInput
    submitLabel: string
    domainPlaceholder?: string
    onsubmit: (input: ApplicationInput) => Promise<void>
    oncancel?: () => void
  } = $props()

  // The form edits its own copy; `initial` only seeds it (the parent remounts
  // the form with {#key} when it should start over).
  const start = untrack(() => initial) ?? { name: '', git_url: '', git_branch: 'main', dockerfile_path: 'Dockerfile', port: 3000, domain: '' }
  let name = $state(start.name)
  let git_url = $state(start.git_url)
  let git_branch = $state(start.git_branch)
  let dockerfile_path = $state(start.dockerfile_path)
  let port = $state(String(start.port))
  let domain = $state(start.domain)
  const check: HealthCheck = start.health_check ?? { enabled: false, path: '/', interval: 5, timeout: 5, retries: 10, start_period: 0 }
  let checkEnabled = $state(check.enabled)
  let checkPath = $state(check.path)
  let checkInterval = $state(String(check.interval))
  let checkTimeout = $state(String(check.timeout))
  let checkRetries = $state(String(check.retries))
  let checkStartPeriod = $state(String(check.start_period))
  let errors = $state<Record<string, string>>({})
  let message = $state('')
  let busy = $state(false)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    message = ''
    try {
      const health_check: HealthCheck = {
        enabled: checkEnabled,
        path: checkPath,
        interval: Number(checkInterval),
        timeout: Number(checkTimeout),
        retries: Number(checkRetries),
        start_period: Number(checkStartPeriod),
      }
      await onsubmit({ name, git_url, git_branch, dockerfile_path, port: Number(port), domain, health_check })
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
  <Field label="Name" bind:value={name} error={errors.name} required />
  <Field
    label="Git repository (https:// for public, SSH for private)"
    bind:value={git_url}
    error={errors.git_url}
    placeholder="https://github.com/you/app or git@github.com:you/app.git"
    required
  />
  <div class="row">
    <Field label="Branch" bind:value={git_branch} error={errors.git_branch} />
    <Field label="Dockerfile path" bind:value={dockerfile_path} error={errors.dockerfile_path} />
    <Field label="Port the app listens on" type="number" bind:value={port} error={errors.port} required />
  </div>
  <Field label="Domain (empty for the default)" bind:value={domain} error={errors.domain} placeholder={domainPlaceholder} />
  <fieldset>
    <legend>Health check</legend>
    <label class="check">
      <input type="checkbox" bind:checked={checkEnabled} />
      Wait until the new container answers before it takes traffic
    </label>
    {#if checkEnabled}
      <p class="muted">
        Bakery requests the path inside the new container until it answers 2xx or 3xx. The image needs <code>curl</code> or
        <code>wget</code>.
      </p>
      <Field label="Health check path" bind:value={checkPath} error={errors['health_check.path']} placeholder="/health" />
      <div class="row">
        <Field label="Interval (s)" type="number" bind:value={checkInterval} error={errors['health_check.interval']} />
        <Field label="Timeout (s)" type="number" bind:value={checkTimeout} error={errors['health_check.timeout']} />
        <Field label="Retries" type="number" bind:value={checkRetries} error={errors['health_check.retries']} />
        <Field label="Start period (s)" type="number" bind:value={checkStartPeriod} error={errors['health_check.start_period']} />
      </div>
    {/if}
  </fieldset>
  {#if message}<p class="error">{message}</p>{/if}
  <div class="actions">
    {#if oncancel}<button type="button" onclick={oncancel}>Cancel</button>{/if}
    <button class="primary" disabled={busy}>{submitLabel}</button>
  </div>
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
  fieldset {
    display: grid;
    gap: 0.8rem;
    border: 1px solid var(--border, #8884);
    border-radius: 0.4rem;
    padding: 0.8rem;
    margin: 0;
  }
  fieldset p {
    margin: 0;
  }
  .check {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
    justify-content: flex-end;
  }
</style>
