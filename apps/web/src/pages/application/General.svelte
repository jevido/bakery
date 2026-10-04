<script lang="ts">
  // Coolify's Application General page (resources/views/livewire/project/application/general.blade.php,
  // app/Livewire/Project/Application/General.php; Apache-2.0, see NOTICE),
  // with only the sections and fields The Bakery has a setting for: the
  // details, the domain summary, the build pipeline, the container image,
  // the exposed port and HTTP Basic Authentication. Saving sends the whole
  // Application as loaded with these fields changed, so nothing else is
  // touched; Authentication goes to the routing settings, which keep their
  // redirect and response headers.
  import { untrack } from 'svelte'
  import { api, ApiError } from '../../lib/api'
  import Icon from '../../lib/Icon.svelte'
  import { applicationPath, href } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import { scrollToPendingSettingsSection } from '../../lib/settingsSection.svelte'
  import type { Application, ApplicationInput, BuildPack, RouteSettings } from '../../lib/types'
  import Input from '../../lib/ui/Input.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'

  let { application, onchange }: { application: Application; onchange: (a: Application) => void } = $props()

  // Coolify keeps a Docker image as an image name and a tag; The Bakery
  // stores the one reference. A digest is a tag of "sha256:…" (or Coolify's
  // "sha256-…" spelling) and joins with "@".
  function splitImage(reference: string): { image: string; tag: string } {
    const at = reference.indexOf('@')
    if (at !== -1) return { image: reference.slice(0, at), tag: reference.slice(at + 1) }
    const colon = reference.lastIndexOf(':')
    if (colon > reference.lastIndexOf('/')) return { image: reference.slice(0, colon), tag: reference.slice(colon + 1) }
    return { image: reference, tag: '' }
  }

  function joinImage(image: string, tag: string): string {
    const i = image.trim()
    const t = tag.trim()
    if (!t) return i
    if (/^sha256[:-]/.test(t)) return `${i}@sha256:${t.slice(7)}`
    return `${i}:${t}`
  }

  const gitBased = $derived(application.build_pack !== 'dockerimage')
  const canUpdate = $derived(session.canWrite)

  let name = $state('')
  let description = $state('')
  let buildPack = $state<BuildPack>('dockerfile')
  let dockerfilePath = $state('')
  let publishDirectory = $state('')
  let image = $state('')
  let tag = $state('')
  let registryUsername = $state('')
  let registryPassword = $state('')
  let port = $state('')
  let routing = $state.raw<RouteSettings | null>(null)
  let authEnabled = $state(false)
  let authUsername = $state('')
  let authPassword = $state('')
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)

  function reset() {
    const a = application
    name = a.name
    description = a.description ?? ''
    buildPack = a.build_pack
    dockerfilePath = a.dockerfile_path
    publishDirectory = a.publish_directory
    ;({ image, tag } = splitImage(a.docker_image ?? ''))
    registryUsername = a.registry_username ?? ''
    registryPassword = ''
    port = String(a.port)
    authEnabled = routing?.basic_auth.enabled ?? false
    authUsername = routing?.basic_auth.username ?? ''
    authPassword = ''
    errors = {}
  }

  $effect(() => {
    void application.id
    untrack(() => {
      routing = null
      reset()
    })
    api<{ routing: RouteSettings }>('GET', `/applications/${application.id}/routing`)
      .then((r) => {
        routing = r.routing
        authEnabled = r.routing.basic_auth.enabled
        authUsername = r.routing.basic_auth.username
      })
      .catch(() => {})
  })

  $effect(scrollToPendingSettingsSection)

  const static_ = $derived(buildPack === 'static')
  const applicationDirty = $derived(
    name !== application.name ||
      description !== (application.description ?? '') ||
      buildPack !== application.build_pack ||
      dockerfilePath !== application.dockerfile_path ||
      publishDirectory !== application.publish_directory ||
      (!gitBased && joinImage(image, tag) !== application.docker_image) ||
      (!gitBased && (registryUsername !== (application.registry_username ?? '') || registryPassword !== '')) ||
      (!static_ && port !== String(application.port)),
  )
  const authDirty = $derived(
    !!routing &&
      (authEnabled !== routing.basic_auth.enabled ||
        (authEnabled && (authUsername !== routing.basic_auth.username || authPassword !== ''))),
  )
  const dirty = $derived(canUpdate && (applicationDirty || authDirty))

  async function putAuthentication(enabled: boolean): Promise<RouteSettings> {
    if (!routing) throw new Error('routing settings not loaded')
    const r = await api<{ routing: RouteSettings }>('PUT', `/applications/${application.id}/routing`, {
      redirect: routing.redirect,
      response_headers: routing.response_headers,
      basic_auth: { enabled, username: enabled ? authUsername : routing.basic_auth.username, password: enabled ? authPassword : '' },
    })
    return r.routing
  }

  async function save() {
    if (saving || !dirty) return
    saving = true
    errors = {}
    const changedApplication = applicationDirty
    const changedAuthentication = authDirty
    try {
      if (changedApplication) {
        const input: ApplicationInput = {
          name,
          description,
          build_pack: buildPack,
          docker_image: gitBased ? '' : joinImage(image, tag),
          publish_directory: publishDirectory,
          git_url: application.git_url,
          git_branch: application.git_branch,
          dockerfile_path: dockerfilePath,
          port: static_ ? 80 : Number(port),
          domains: application.domains,
        }
        if (!gitBased) input.registry_credentials = { username: registryUsername, password: registryPassword }
        onchange((await api<{ application: Application }>('PATCH', `/applications/${application.id}`, input)).application)
      }
      if (changedAuthentication) routing = await putAuthentication(authEnabled)
      reset()
      // Authentication reaches the proxy at once; the rest with the next
      // Deployment, as Coolify says after a save.
      toast.success('Application settings updated!', changedApplication ? 'Redeploy to apply the changes.' : '')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      saving = false
    }
  }

  // Coolify's Authentication listbox saves at once (instantSave). Turning it
  // off here does too; turning it on needs a username and password first,
  // so that waits for Save.
  async function authenticationChanged() {
    if (authEnabled || !routing?.basic_auth.enabled) return
    try {
      routing = await putAuthentication(false)
      toast.success('Settings saved.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      authEnabled = true
      toast.error('Settings not saved', err.message)
    }
  }

  const primaryDomain = $derived(application.domains[0] ?? '')
  const additionalDomains = $derived(Math.max(0, application.domains.length - 1))
  const domainsHref = $derived(href(applicationPath(application, 'domains')))
</script>

<form
  class="application-settings-form flex flex-col"
  onsubmit={(e) => {
    e.preventDefault()
    save()
  }}
>
  {#if canUpdate}
    <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
  {/if}
  <div class="flex flex-col gap-6">
    <SettingsSection
      id="application-details-section"
      title="Application details"
      helper="Name the application and choose the build strategy The Bakery should use to deploy it."
    >
      <div class="grid gap-4">
        <Input label="Name" bind:value={name} error={errors.name} required disabled={!canUpdate} />
        <Input label="Description" bind:value={description} error={errors.description} disabled={!canUpdate} />
      </div>
    </SettingsSection>

    <SettingsSection id="access-section" title="Access" helper="Manage how this application is reached publicly.">
      <section id="public-access-section">
        <h3 class="mb-3 text-sm font-semibold text-black dark:text-fg">Public access</h3>
        <div
          class="group relative flex items-center gap-3 rounded-lg border border-neutral-200 bg-neutral-50/60 px-4 py-3 transition-colors focus-within:ring-2 focus-within:ring-coollabs/40 hover:bg-neutral-100 dark:border-white/[0.07] dark:bg-white/[0.05] dark:focus-within:ring-warning/40 dark:hover:bg-white/[0.08]"
        >
          <a
            class="flex min-w-0 flex-1 items-center gap-3 after:absolute after:inset-0 after:content-[''] focus-visible:outline-none"
            aria-label={primaryDomain ? 'Manage application domains' : 'Add an application domain'}
            href={domainsHref}
          >
            <div
              class="flex size-9 shrink-0 items-center justify-center rounded-md bg-neutral-200/70 text-neutral-600 dark:bg-white/[0.07] dark:text-fg-dim"
            >
              <Icon name="globe" class="size-4" />
            </div>
            <div class="min-w-0">
              <p class="text-sm font-medium text-black dark:text-fg">
                {#if primaryDomain}<span class="block truncate">{primaryDomain}</span>{:else}No public domain configured{/if}
              </p>
              <p class="text-xs text-neutral-500 dark:text-fg-dim">
                {#if additionalDomains > 0}
                  +{additionalDomains} more {additionalDomains === 1 ? 'domain' : 'domains'}
                {:else if !primaryDomain}
                  Make this application available from a URL
                {:else}
                  Manage DNS checks and redirect settings
                {/if}
              </p>
            </div>
          </a>
          <a
            class="button relative z-10 ml-auto shrink-0"
            aria-label={primaryDomain ? 'Manage application domains' : 'Add an application domain'}
            href={domainsHref}
          >
            {primaryDomain ? 'Manage domains' : 'Add domain'}
            <Icon name="arrow-right" class="size-4" />
          </a>
        </div>
      </section>
    </SettingsSection>

    <SettingsSection
      id="build-pipeline-section"
      title="Build pipeline"
      helper="Commands, directories and options used while building the application."
    >
      {#if gitBased}
        <div class="application-build-pack-options mb-5 border-b border-neutral-200 pb-5 dark:border-white/[0.07]">
          <div class="grid gap-4 sm:grid-cols-2">
            <Select label="Build strategy" bind:value={buildPack} error={errors.build_pack} disabled={!canUpdate}>
              <option value="nixpacks">Nixpacks</option>
              <option value="static">Static</option>
              <option value="dockerfile">Dockerfile</option>
            </Select>
          </div>
        </div>
      {/if}
      <div class="flex flex-col gap-5">
        {#if !gitBased}
          <p class="text-sm text-neutral-500 dark:text-fg-dim">Nothing to build. This application deploys a prebuilt Docker image.</p>
        {:else}
          <div class="grid gap-4 lg:grid-cols-2">
            {#if buildPack === 'dockerfile'}
              <Input
                label="Dockerfile location"
                bind:value={dockerfilePath}
                error={errors.dockerfile_path}
                placeholder="Dockerfile"
                helper="Relative to the repository root."
                disabled={!canUpdate}
              />
            {:else if buildPack === 'static'}
              <Input
                label="Publish directory"
                bind:value={publishDirectory}
                error={errors.publish_directory}
                placeholder="dist"
                helper="Relative to the repository root."
                required
                disabled={!canUpdate}
              />
            {/if}
          </div>
        {/if}
      </div>
    </SettingsSection>

    {#if !gitBased}
      <SettingsSection id="container-image-section" title="Container image" helper="Configure the Docker image used for this application.">
        <div class="grid gap-4 lg:grid-cols-2">
          <Input label="Image" bind:value={image} error={errors.docker_image} placeholder="nginx" required disabled={!canUpdate} />
          <Input
            label="Tag"
            bind:value={tag}
            placeholder="alpine"
            helper="Enter a tag (e.g., 'latest', 'v1.2.3') or SHA256 hash (e.g., 'sha256-59e02939b1bf39f16c93138a28727aec520bb916da021180ae502c61626b3cf0')"
            disabled={!canUpdate}
          />
        </div>
        <div class="mt-5 grid gap-4 border-t border-neutral-200 pt-5 sm:grid-cols-2 dark:border-white/[0.07]">
          <Input
            label="Registry username"
            bind:value={registryUsername}
            error={errors.registry_credentials}
            helper="Only for a private image. Clear the username to remove the stored credentials."
            disabled={!canUpdate}
          />
          <Input
            label="Registry password or token"
            type="password"
            bind:value={registryPassword}
            autocomplete="new-password"
            placeholder={application.has_registry_password ? 'unchanged' : ''}
            disabled={!canUpdate}
          />
        </div>
      </SettingsSection>
    {/if}

    <SettingsSection
      id="networking-section"
      title="Networking"
      helper="The port the container exposes; the proxy sends the domains' traffic to it."
    >
      <div class="grid gap-4 lg:grid-cols-[14rem_minmax(0,1fr)]">
        <div class="min-w-0">
          <Input
            label="Ports exposes"
            value={static_ ? '80' : port}
            oninput={(e) => (port = e.currentTarget.value)}
            readonly={static_}
            error={errors.port}
            placeholder="3000"
            inputmode="numeric"
            helper={static_
              ? 'A static site is served on port 80.'
              : 'The port your application listens on. The healthcheck uses it too. Be sure to set this correctly.'}
            disabled={!canUpdate}
          />
        </div>
      </div>
    </SettingsSection>

    <SettingsSection id="security-section" title="Security" helper="Protect this application with authentication at the proxy level.">
      <Select
        label="Authentication"
        value={authEnabled ? 'basic' : 'none'}
        onchange={(e) => {
          authEnabled = e.currentTarget.value === 'basic'
          authenticationChanged()
        }}
        helper="HTTP Basic Authentication makes the proxy ask for a username and password before any request reaches the application. The Bakery supports a single username and password."
        disabled={!canUpdate || !routing}
      >
        <option value="none">None</option>
        <option value="basic">HTTP Basic Authentication</option>
      </Select>
      {#if authEnabled}
        <div class="mt-5 grid w-full gap-4 border-t border-neutral-200 pt-5 sm:grid-cols-2 dark:border-white/[0.07]">
          <Input label="Username" bind:value={authUsername} error={errors['basic_auth.username']} required disabled={!canUpdate} />
          <Input
            label="Password"
            type="password"
            bind:value={authPassword}
            error={errors['basic_auth.password']}
            autocomplete="new-password"
            placeholder={routing?.basic_auth.password_set ? 'unchanged' : ''}
            required={!routing?.basic_auth.password_set}
            disabled={!canUpdate}
          />
        </div>
      {/if}
    </SettingsSection>
  </div>
</form>
