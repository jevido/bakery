<script lang="ts">
  import { untrack } from 'svelte'
  import { ApiError } from './api'
  import Field from './Field.svelte'
  import type { ApplicationInput, BuildPack, HealthCheck } from './types'

  let {
    initial,
    submitLabel,
    domainPlaceholder = '<slug>.localhost',
    onsubmit,
    oncancel,
  }: {
    initial?: ApplicationInput & { registry_username?: string; has_registry_password?: boolean }
    submitLabel: string
    domainPlaceholder?: string
    onsubmit: (input: ApplicationInput) => Promise<void>
    oncancel?: () => void
  } = $props()

  // The form edits its own copy; `initial` only seeds it (the parent remounts
  // the form with {#key} when it should start over).
  const start = untrack(() => initial) ?? {
    name: '',
    build_pack: 'dockerfile' as BuildPack,
    image_reference: '',
    publish_directory: '.',
    git_url: '',
    git_branch: 'main',
    dockerfile_path: 'Dockerfile',
    port: 3000,
    domains: [] as string[],
  }
  const packs: { value: BuildPack; label: string; hint: string }[] = [
    { value: 'dockerfile', label: 'Dockerfile', hint: 'Build the Dockerfile in the repository.' },
    { value: 'nixpacks', label: 'Nixpacks', hint: 'Nixpacks detects the language and builds it; no Dockerfile needed.' },
    { value: 'static', label: 'Static site', hint: 'Serve a directory of the repository as files, on port 80.' },
    { value: 'image', label: 'Image', hint: 'Run a prebuilt image from a registry; nothing is cloned or built.' },
  ]
  let name = $state(start.name)
  let build_pack = $state<BuildPack>(start.build_pack || 'dockerfile')
  let image_reference = $state(start.image_reference ?? '')
  let publish_directory = $state(start.publish_directory || '.')
  let registryUsername = $state(untrack(() => initial?.registry_username) ?? '')
  let registryPassword = $state('')
  const hadPassword = untrack(() => initial?.has_registry_password) ?? false
  const fromGit = $derived(build_pack !== 'image')
  let git_url = $state(start.git_url)
  let git_branch = $state(start.git_branch || 'main')
  let dockerfile_path = $state(start.dockerfile_path || 'Dockerfile')
  let port = $state(String(start.port))
  let domain = $state(start.domains[0] ?? '')
  const otherDomains = start.domains.slice(1)
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
      const input: ApplicationInput = {
        name,
        build_pack,
        image_reference,
        publish_directory,
        git_url,
        git_branch,
        dockerfile_path,
        port: build_pack === 'static' ? 80 : Number(port),
        domains: [domain, ...otherDomains].filter((d) => d.trim() !== ''),
        health_check,
      }
      if (build_pack === 'image') input.registry_credentials = { username: registryUsername, password: registryPassword }
      await onsubmit(input)
      registryPassword = ''
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
  <fieldset class="packs">
    <legend>Build pack</legend>
    <div class="choices">
      {#each packs as p (p.value)}
        <label class={['choice', build_pack === p.value && 'selected']}>
          <input type="radio" name="build_pack" value={p.value} bind:group={build_pack} />
          {p.label}
        </label>
      {/each}
    </div>
    <p class="muted">{packs.find((p) => p.value === build_pack)?.hint}</p>
    {#if errors.build_pack}<small class="error">{errors.build_pack}</small>{/if}
  </fieldset>
  {#if fromGit}
    <Field
      label="Git repository (https:// for public, SSH for private)"
      bind:value={git_url}
      error={errors.git_url}
      placeholder="https://github.com/you/app or git@github.com:you/app.git"
      required
    />
    <div class="row">
      <Field label="Branch" bind:value={git_branch} error={errors.git_branch} />
      {#if build_pack === 'dockerfile'}
        <Field label="Dockerfile path" bind:value={dockerfile_path} error={errors.dockerfile_path} />
      {:else if build_pack === 'static'}
        <Field label="Publish directory" bind:value={publish_directory} error={errors.publish_directory} placeholder="public" />
      {/if}
      {#if build_pack !== 'static'}
        <Field label="Port the app listens on" type="number" bind:value={port} error={errors.port} required />
      {/if}
    </div>
  {:else}
    <Field
      label="Image reference"
      bind:value={image_reference}
      error={errors.image_reference}
      placeholder="docker.io/traefik/whoami:v1.10"
      required
    />
    <Field label="Port the app listens on" type="number" bind:value={port} error={errors.port} required />
    <fieldset>
      <legend>Registry credentials (private images only)</legend>
      <div class="row">
        <Field label="Username" bind:value={registryUsername} error={errors.registry_credentials} autocomplete="off" />
        <Field
          label="Password or token"
          type="password"
          bind:value={registryPassword}
          autocomplete="new-password"
          placeholder={hadPassword ? 'unchanged' : ''}
        />
      </div>
      {#if hadPassword}
        <p class="muted">Clear the username to remove the stored credentials.</p>
      {/if}
    </fieldset>
  {/if}
  <Field label="Domain (empty for the default)" bind:value={domain} error={errors.domains} placeholder={domainPlaceholder} />
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
