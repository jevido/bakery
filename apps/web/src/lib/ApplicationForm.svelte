<script lang="ts">
  import { untrack } from 'svelte'
  import { ApiError } from './api'
  import Field from './Field.svelte'
  import type { ApplicationInput } from './types'

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
  let errors = $state<Record<string, string>>({})
  let message = $state('')
  let busy = $state(false)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    message = ''
    try {
      await onsubmit({ name, git_url, git_branch, dockerfile_path, port: Number(port), domain })
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
  .actions {
    display: flex;
    gap: 0.5rem;
    justify-content: flex-end;
  }
</style>
