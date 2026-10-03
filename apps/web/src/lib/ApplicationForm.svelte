<script lang="ts">
  import { session } from './session.svelte'
  import { untrack } from 'svelte'
  import { api, ApiError } from './api'
  import Field from './Field.svelte'
  import type { ApplicationInput, BuildPack, HealthCheck, ResourceLimits, Server, Storage } from './types'

  let {
    initial,
    submitLabel,
    domainPlaceholder = '<slug>.localhost',
    onsubmit,
    oncancel,
  }: {
    initial?: ApplicationInput & { registry_username?: string; has_registry_password?: boolean; server_id?: number }
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
    docker_image: '',
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
    { value: 'dockerimage', label: 'Docker Image', hint: 'Run a prebuilt Docker image from a registry; nothing is cloned or built.' },
  ]
  let name = $state(start.name)
  let build_pack = $state<BuildPack>(start.build_pack || 'dockerfile')
  let docker_image = $state(start.docker_image ?? '')
  let publish_directory = $state(start.publish_directory || '.')
  let registryUsername = $state(untrack(() => initial?.registry_username) ?? '')
  let registryPassword = $state('')
  const hadPassword = untrack(() => initial?.has_registry_password) ?? false
  const fromGit = $derived(build_pack !== 'dockerimage')
  let git_url = $state(start.git_url)
  let git_branch = $state(start.git_branch || 'main')
  let dockerfile_path = $state(start.dockerfile_path || 'Dockerfile')
  let port = $state(String(start.port))
  // Rows carry a key so removing one keeps the others' inputs in place.
  let nextKey = 0
  const startDomains = start.domains.length > 0 ? start.domains : ['']
  let domains = $state(startDomains.map((value) => ({ key: nextKey++, value })))
  let storages = $state((start.storages ?? []).map((s: Storage) => ({ key: nextKey++, ...s })))
  const limits: ResourceLimits = start.resource_limits ?? { memory_mb: null, cpus: null }
  // The Target server is chosen once, when the application is created.
  const creating = untrack(() => initial) === undefined
  let servers = $state.raw<Server[]>([])
  let server_id = $state(untrack(() => initial?.server_id) ?? 0)
  const currentServer = $derived(servers.find((s) => s.id === server_id))
  api<{ servers: Server[] }>('GET', '/servers')
    .then((r) => {
      servers = r.servers
      if (server_id === 0) server_id = r.servers.find((s) => s.kind === 'local')?.id ?? 0
    })
    .catch(() => {})
  let memoryMB = $state(limits.memory_mb == null ? '' : String(limits.memory_mb))
  let cpus = $state(limits.cpus == null ? '' : String(limits.cpus))

  function makePrimary(key: number) {
    const i = domains.findIndex((d) => d.key === key)
    domains.unshift(...domains.splice(i, 1))
  }
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
        docker_image,
        publish_directory,
        git_url,
        git_branch,
        dockerfile_path,
        port: build_pack === 'static' ? 80 : Number(port),
        domains: domains.map((d) => d.value.trim()).filter((d) => d !== ''),
        health_check,
        storages: storages
          .filter((s) => s.name.trim() !== '' || s.mount_path.trim() !== '')
          .map((s) => ({ name: s.name.trim(), mount_path: s.mount_path.trim() })),
        resource_limits: {
          memory_mb: memoryMB.trim() === '' ? null : Number(memoryMB),
          cpus: cpus.trim() === '' ? null : Number(cpus),
        },
      }
      if (creating && server_id !== 0) input.server_id = server_id
      if (build_pack === 'dockerimage') input.registry_credentials = { username: registryUsername, password: registryPassword }
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
  <!-- A viewer sees the values and cannot change them. -->
  <fieldset class="contents" disabled={!session.canWrite}>
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
  {#if servers.length > 1 || !creating}
    <fieldset>
      <legend>Server</legend>
      {#if creating}
        <label class="field">
          <span>Where it is built and runs</span>
          <select bind:value={server_id} aria-invalid={errors.server_id ? 'true' : undefined}>
            {#each servers as s (s.id)}
              <option value={s.id}>
                {s.name}{s.kind === 'remote' ? ` (${s.host})` : ''}{s.status !== 'reachable' ? ` · ${s.status}` : ''}
              </option>
            {/each}
          </select>
        </label>
        {#if currentServer && currentServer.status !== 'reachable'}
          <p class="muted">This server is {currentServer.status}; validate it on its page, or the deploy will fail.</p>
        {/if}
        <p class="muted">The server cannot be changed once the application exists.</p>
      {:else}
        <p>
          {currentServer?.name ?? `server ${server_id}`}{currentServer?.kind === 'remote' ? ` (${currentServer.host})` : ''}
          <span class="muted">· cannot be changed</span>
        </p>
      {/if}
      {#if errors.server_id}<small class="error">{errors.server_id}</small>{/if}
    </fieldset>
  {/if}
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
      label="Docker Image"
      bind:value={docker_image}
      error={errors.docker_image}
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
  <fieldset>
    <legend>Domains</legend>
    <p class="muted">The first is the primary domain. Changes reach the proxy at once, without a deploy.</p>
    {#each domains as d, i (d.key)}
      <div class="line">
        <input
          aria-label={i === 0 ? 'Primary domain' : `Domain ${i + 1}`}
          bind:value={d.value}
          placeholder={i === 0 ? `${domainPlaceholder} (the default)` : 'www.example.com'}
          aria-invalid={errors.domains ? 'true' : undefined}
        />
        {#if i === 0}
          <span class="tag">primary</span>
        {:else}
          <button type="button" onclick={() => makePrimary(d.key)}>Make primary</button>
        {/if}
        {#if domains.length > 1}
          <button type="button" aria-label="Remove domain" onclick={() => (domains = domains.filter((x) => x.key !== d.key))}>×</button>
        {/if}
      </div>
    {/each}
    {#if errors.domains}<small class="error">{errors.domains}</small>{/if}
    {#if domains.length < 10}
      <div><button type="button" onclick={() => domains.push({ key: nextKey++, value: '' })}>Add domain</button></div>
    {/if}
  </fieldset>
  <fieldset>
    <legend>Health check</legend>
    <label class="check">
      <input type="checkbox" bind:checked={checkEnabled} />
      Wait until the new container answers before it takes traffic
    </label>
    {#if checkEnabled}
      <p class="muted">
        The Bakery requests the path inside the new container until it answers 2xx or 3xx. The image needs <code>curl</code> or
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
  <fieldset>
    <legend>Persistent storage</legend>
    <p class="muted">
      A volume per name, mounted at its path in every container, so what the app writes there survives deploys and rollbacks. Removing
      a row keeps the data until the application is deleted.
    </p>
    {#each storages as s (s.key)}
      <div class="line">
        <input aria-label="Storage name" bind:value={s.name} placeholder="data" />
        <input aria-label="Mount path" bind:value={s.mount_path} placeholder="/data" />
        <button type="button" aria-label="Remove storage" onclick={() => (storages = storages.filter((x) => x.key !== s.key))}>×</button>
      </div>
    {/each}
    {#if errors.storages}<small class="error">{errors.storages}</small>{/if}
    {#if storages.length < 10}
      <div>
        <button type="button" onclick={() => storages.push({ key: nextKey++, name: '', mount_path: '' })}>Add storage</button>
      </div>
    {/if}
  </fieldset>
  <fieldset>
    <legend>Resource limits</legend>
    <p class="muted">Empty is unlimited. Limits apply from the next deploy.</p>
    <div class="row">
      <Field label="Memory (MB)" type="number" bind:value={memoryMB} error={errors['resource_limits.memory_mb']} placeholder="unlimited" />
      <Field label="CPU (cores)" bind:value={cpus} error={errors['resource_limits.cpus']} placeholder="unlimited" />
    </div>
  </fieldset>
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
  .field {
    display: grid;
    gap: 0.3rem;
  }
  .field span {
    font-size: 0.8rem;
    color: var(--muted);
  }
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
  fieldset:not(.contents) p {
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
  .line {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  .line input {
    flex: 1;
    min-width: 0;
  }
  .tag {
    font-size: 0.8rem;
    color: var(--muted);
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
