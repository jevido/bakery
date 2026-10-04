<script lang="ts">
  // Coolify's Application page (resources/views/livewire/project/application/configuration.blade.php,
  // Apache-2.0, see NOTICE): the heading, the configuration sidebar and the
  // sub-page the URL names. Sub-pages not yet in Coolify's markup still render
  // the components the old tabbed page used.
  import { api, ApiError } from '../../lib/api'
  import ApplicationForm from '../../lib/ApplicationForm.svelte'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import ContainerLogs from '../../lib/ContainerLogs.svelte'
  import DeployKey from '../../lib/DeployKey.svelte'
  import Deployments from '../../lib/Deployments.svelte'
  import EnvironmentVariables from '../../lib/EnvironmentVariables.svelte'
  import Previews from '../../lib/Previews.svelte'
  import { applicationPath, go, href, type ApplicationPage } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Application, ApplicationInput, Deployment, Environment, Server } from '../../lib/types'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import Webhook from '../../lib/Webhook.svelte'
  import Advanced from './Advanced.svelte'
  import ConfigurationSidebar from './ConfigurationSidebar.svelte'
  import Domains from './Domains.svelte'
  import General from './General.svelte'
  import Heading, { type Action } from './Heading.svelte'

  let {
    projectId,
    environmentId,
    id,
    page,
    deploymentId,
  }: { projectId: number; environmentId: number; id: number; page: ApplicationPage; deploymentId: number | null } = $props()

  let application = $state.raw<Application | null>(null)
  let environment = $state.raw<Environment | null>(null)
  let deployments = $state.raw<Deployment[]>([])
  // The Target server, for its name and, on a Remote server, where DNS must point.
  let server = $state.raw<Server | null>(null)
  let loadError = $state('')
  let saved = $state(false)
  let deploying = $state(false)

  // Coolify's status string from the Application's own Containers, and
  // whether one (running or not) is there to remove.
  let appStatus = $state.raw<{ status: string; container_present: boolean } | null>(null)

  // The Application's own Deployments; Previews' are listed with them but
  // have their own queue.
  const own = $derived(deployments.filter((d) => d.preview === 0))
  const active = $derived(deployments.some((d) => d.active))
  // One Deployment can wait behind the running one; a second cannot.
  const queued = $derived(own.some((d) => d.status === 'queued'))
  const status = $derived(appStatus?.status ?? null)
  const exited = $derived(status?.startsWith('exited') ?? false)

  async function loadDeployments() {
    const r = await api<{ deployments: Deployment[] }>('GET', `/applications/${id}/deployments`)
    deployments = r.deployments
  }

  async function loadStatus() {
    appStatus = await api<{ status: string; container_present: boolean }>('GET', `/applications/${id}/status`)
  }

  $effect(() => {
    application = null
    server = null
    deployments = []
    appStatus = null
    loadError = ''
    api<{ application: Application }>('GET', `/applications/${id}`)
      .then((r) => {
        const a = r.application
        // A link with another Project or Environment in it still opens the
        // Application, at its own path.
        if (a.project_id !== projectId || a.environment_id !== environmentId) {
          location.replace('#' + applicationPath(a, page) + (deploymentId ? `/${deploymentId}` : ''))
          return
        }
        application = a
        api<{ server: Server }>('GET', `/servers/${a.server_id}`)
          .then((s) => (server = s.server))
          .catch(() => {})
        api<{ environment: Environment }>('GET', `/environments/${a.environment_id}`)
          .then((e) => (environment = e.environment))
          .catch(() => {})
        loadStatus().catch(() => {})
        return loadDeployments()
      })
      .catch((e) => (loadError = e.message))
  })

  // Keeps the Deployments and the status current: quickly while a Deployment
  // is under way, slower otherwise so one started by a push shows up by itself.
  $effect(() => {
    const t = setInterval(() => {
      loadDeployments().catch(() => {})
      loadStatus().catch(() => {})
    }, active ? 3000 : 5000)
    return () => clearInterval(t)
  })

  // Deploy, Redeploy (without cache) and Restart all start a Deployment, and
  // Coolify then opens its log, as here.
  async function start(path: string, body?: unknown) {
    if (!application) return
    deploying = true
    try {
      const r = await api<{ deployment: Deployment }>('POST', `/applications/${id}/${path}`, body)
      await loadDeployments()
      go(`${applicationPath(application, 'deployment')}/${r.deployment.id}`)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error(path === 'restart' ? 'Restart not started' : 'Deployment not started', err.message)
    } finally {
      deploying = false
    }
  }

  const deploy = () => start('deploy')
  const deployWithoutCache = () => start('deploy', { force_rebuild: true })

  async function restart() {
    await start('restart')
  }

  async function stop() {
    toast.info('Gracefully stopping application.', 'It could take a while depending on the application.')
    try {
      appStatus = await api<{ status: string; container_present: boolean }>('POST', `/applications/${id}/stop`)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Application not stopped', err.message)
    }
  }

  // Coolify opens these modals by clicking their hidden triggers.
  const openModal = (id: string) => () => document.getElementById(id)?.click()

  // Coolify's heading.blade.php: what the menu offers depends on whether the
  // Application is exited, and the phone menu lists them in its own order.
  const busy = $derived(deploying || queued)
  const actions = $derived<Action[]>(
    !session.canWrite || !status
      ? []
      : exited
        ? [
            { label: 'Deploy', icon: 'play-circle', run: deploy, disabled: busy },
            { label: 'Deploy (without cache)', icon: 'refresh', run: deployWithoutCache, disabled: busy },
            ...(appStatus?.container_present
              ? [{ label: 'Remove container', icon: 'stop-circle', run: openModal('application-stop-trigger'), danger: true } satisfies Action]
              : []),
          ]
        : [
            { label: 'Redeploy', icon: 'refresh', run: deploy, disabled: busy },
            {
              label: status.startsWith('running') ? 'Redeploy (without cache)' : 'Deploy (without cache)',
              icon: 'refresh',
              run: deployWithoutCache,
              disabled: busy,
            },
            { label: 'Restart', icon: 'restart', run: openModal('application-restart-trigger'), disabled: busy },
            { label: 'Stop', icon: 'stop-circle', run: openModal('application-stop-trigger'), danger: true },
          ],
  )
  const mobileActions = $derived<Action[]>(
    !session.canWrite || !status || exited
      ? actions
      : [
          { label: 'Deploy', icon: 'refresh', run: deploy, disabled: busy },
          { label: 'Restart', icon: 'restart', run: openModal('application-restart-trigger'), disabled: busy },
          { label: 'Deploy (without cache)', icon: 'refresh', run: deployWithoutCache, disabled: busy },
          { label: 'Stop', icon: 'stop-circle', run: openModal('application-stop-trigger'), danger: true },
        ],
  )

  async function update(input: ApplicationInput) {
    saved = false
    const r = await api<{ application: Application }>('PATCH', `/applications/${id}`, input)
    application = r.application
    saved = true
  }

  async function remove() {
    if (!application) return
    await api('DELETE', `/applications/${id}`)
    go(`/project/${application.project_id}/environment/${application.environment_id}`)
  }

  // The open Deployment is the URL's: picking one opens its path, going back
  // to the list opens the list's.
  function selectDeployment(selected: number | null) {
    if (application) go(applicationPath(application, 'deployment') + (selected ? `/${selected}` : ''))
  }

  // Projects › Project › Environment › Application, with its status.
  const crumbs = $derived(
    application && environment
      ? { project: environment.project_name ?? 'Project', environment: environment.name, name: application.name }
      : null,
  )
  $effect(() => {
    if (crumbs)
      breadcrumb.set(
        { label: 'Projects', href: href('/projects') },
        { label: crumbs.project, href: href(`/project/${projectId}`) },
        { label: crumbs.environment, href: href(`/project/${projectId}/environment/${environmentId}`) },
        { label: crumbs.name, summary: status ?? undefined },
      )
  })
</script>

{#snippet later(title: string, text?: string)}
  <div class="chrome">
    <h2 class="text-[15px]! font-semibold! text-black dark:text-fg">{title}</h2>
    {#if text}<p class="mt-2 text-[13px] text-neutral-600 dark:text-fg-dim">{text}</p>{/if}
    <p class="mt-2 text-[13px] text-neutral-500 dark:text-fg-dim">This page is ported in a later task.</p>
  </div>
{/snippet}

<!-- Persistent Storage, Healthcheck and Resource Limits are edited in the
     old form until their pages are ported (tasks 05 and 06). -->
{#snippet legacyForm()}
  {#if application}
    {#key application.id}
      <div class="mt-6">
        <ApplicationForm
          initial={application}
          submitLabel="Save"
          domainPlaceholder={`${application.slug}.localhost`}
          onsubmit={update}
        />
      </div>
    {/key}
    {#if saved}<p class="ok">Saved. Domains apply at once; everything else on the next deploy.</p>{/if}
  {/if}
{/snippet}

{#if loadError}
  <p class="chrome text-sm text-error">{loadError}</p>
{:else if !application}
  <div class="chrome"><Spinner text="Loading…" /></div>
{:else}
  <Heading name={application.name} urls={application.public_urls} {status} {actions} {mobileActions} />
  {#if session.canWrite}
    <div class="hidden" aria-hidden="true">
      <ConfirmationModal
        title={exited ? 'Confirm Container Removal?' : 'Confirm Application Stopping?'}
        buttonTitle={exited ? 'Remove container' : 'Stop'}
        actions={[
          exited ? 'The exited application container will be removed.' : 'This application will be stopped.',
          exited ? 'Anonymous volumes may become eligible for cleanup.' : 'All non-persistent data of this application will be deleted.',
        ]}
        confirmWithText={false}
        onconfirm={stop}
      >
        {#snippet trigger(show)}
          <button id="application-stop-trigger" type="button" onclick={show}>Stop</button>
        {/snippet}
      </ConfirmationModal>
      <ConfirmationModal
        title="Confirm Application Restart?"
        buttonTitle="Restart"
        actions={['This application will be restarted without rebuilding.']}
        confirmWithText={false}
        onconfirm={restart}
      >
        {#snippet trigger(show)}
          <button id="application-restart-trigger" type="button" onclick={show}>Restart</button>
        {/snippet}
      </ConfirmationModal>
    </div>
  {/if}

  <section class="mt-4 w-full max-w-none lg:mt-0">
    <div class="grid min-w-0 gap-8 xl:grid-cols-[210px_minmax(0,1fr)] xl:gap-8">
      <ConfigurationSidebar {application} {page} />

      <div class="min-w-0">
        {#if application.deploy_key_public && deployments.length === 0 && page !== 'source'}
          <p class="muted">
            Private repository: add the deploy key from <a href={href(applicationPath(application, 'source'))}>Git Source</a> to the
            repository before the first deploy.
          </p>
        {/if}

        {#if page === ''}
          <General {application} onchange={(a) => (application = a)} />
        {:else if page === 'domains'}
          <Domains {application} {server} onchange={(a) => (application = a)} />
        {:else if page === 'environment-variables'}
          <EnvironmentVariables path={`/applications/${application.id}/environment-variables`} />
        {:else if page === 'deployment'}
          <Deployments
            {deployments}
            bind:selected={() => deploymentId, selectDeployment}
            serverNames={server ? { [server.id]: server.name } : {}}
            onchange={() => loadDeployments().catch(() => {})}
          />
        {:else if page === 'logs'}
          <ContainerLogs
            url={`/api/applications/${application.id}/logs`}
            empty="No running container. Deploy the application first."
            stopped="The container stopped (a new deployment may have replaced it)."
          />
        {:else if page === 'source' && application.build_pack !== 'dockerimage'}
          <dl class="source">
            <dt>Git repository</dt>
            <dd class="mono">{application.git_url}</dd>
            <dt>Branch</dt>
            <dd class="mono">{application.git_branch}</dd>
            {#if application.build_pack === 'static'}
              <dt>Publish directory</dt>
              <dd class="mono">{application.publish_directory}</dd>
            {:else if application.build_pack === 'dockerfile'}
              <dt>Dockerfile</dt>
              <dd class="mono">{application.dockerfile_path}</dd>
            {/if}
          </dl>
          {#if application.deploy_key_public}
            <DeployKey {application} onchange={(a) => (application = a)} />
          {:else}
            <p class="muted">A public repository needs no key. For a private one, use its SSH URL (git@host:owner/repo.git) under General.</p>
          {/if}
        {:else if page === 'webhooks'}
          <Webhook applicationId={application.id} />
        {:else if page === 'preview-deployments'}
          <Previews applicationId={application.id} onchange={() => loadDeployments().catch(() => {})} />
        {:else if page === 'servers'}
          {@render later(
            'Servers',
            server
              ? `Runs on ${server.name}${server.kind === 'remote' ? `; its domains must point at ${server.host}, where this server's proxy serves them` : ''}.`
              : undefined,
          )}
        {:else if page === 'persistent-storage'}
          {@render later(
            'Persistent Storage',
            application.storages.length > 0
              ? application.storages.map((s) => `${s.name} at ${s.mount_path}`).join(', ') + '.'
              : 'No persistent storage.',
          )}
          {@render legacyForm()}
        {:else if page === 'resource-limits'}
          {@render later(
            'Resource Limits',
            `Memory: ${application.resource_limits.memory_mb ? `${application.resource_limits.memory_mb} MB` : 'unlimited'}, CPU: ${application.resource_limits.cpus ?? 'unlimited'}.`,
          )}
          {@render legacyForm()}
        {:else if page === 'healthcheck'}
          {@render later('Healthcheck')}
          {@render legacyForm()}
        {:else if page === 'rollback'}
          {@render later('Rollback', 'Roll back from a finished Deployment under Deployment Logs.')}
        {:else if page === 'advanced'}
          <Advanced {application} />
        {:else if page === 'danger' && session.canWrite}
          <h2>Danger zone</h2>
          <button class="danger" onclick={remove}>Delete application</button>
        {:else}
          {@render later('Not available', 'This Application has no such page.')}
        {/if}
      </div>
    </div>
  </section>
{/if}

<style>
  .source {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: 0.3rem 1rem;
    margin: 0 0 1rem;
  }
  .source dt {
    color: var(--muted);
  }
  .source dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .ok {
    color: var(--ok);
  }
</style>
