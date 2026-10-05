<script lang="ts">
  // Coolify's Application page (resources/views/livewire/project/application/configuration.blade.php,
  // Apache-2.0, see NOTICE): the heading, the configuration sidebar and the
  // sub-page the URL names.
  import { api, ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import EnvironmentVariables from '../../lib/EnvironmentVariables.svelte'
  import { applicationPath, go, href, type ApplicationPage } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Application, ApplicationInput, Deployment, Environment, Server } from '../../lib/types'
  import Callout from '../../lib/ui/Callout.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import Advanced from './Advanced.svelte'
  import ConfigurationSidebar from './ConfigurationSidebar.svelte'
  import Danger from './Danger.svelte'
  import DeploymentPage from './Deployment.svelte'
  import DeploymentHistory from './DeploymentHistory.svelte'
  import Domains from './Domains.svelte'
  import General from './General.svelte'
  import Heading, { type Action } from './Heading.svelte'
  import Healthcheck from './Healthcheck.svelte'
  import PersistentStorage from './PersistentStorage.svelte'
  import Previews from './Previews.svelte'
  import ResourceLimits from './ResourceLimits.svelte'
  import Rollback from './Rollback.svelte'
  import RuntimeLogs from './RuntimeLogs.svelte'
  import Servers from './Servers.svelte'
  import Source from './Source.svelte'
  import Webhooks from './Webhooks.svelte'
  import { applicationInput } from './applicationInput'

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
  let deploying = $state(false)

  // Coolify's status string from the Application's own Containers, whether
  // one (running or not) is there to remove, and the running one's name.
  type ApplicationStatus = { status: string; container_present: boolean; container: string }
  let appStatus = $state.raw<ApplicationStatus | null>(null)

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
    appStatus = await api<ApplicationStatus>('GET', `/applications/${id}/status`)
  }

  $effect(() => {
    application = null
    server = null
    deployments = []
    appStatus = null
    loadError = ''
    // The person may have moved on while it was on its way; a late answer
    // must not send them back.
    let gone = false
    api<{ application: Application }>('GET', `/applications/${id}`)
      .then((r) => {
        if (gone) return
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
      .catch((e) => {
        if (!gone) loadError = e.message
      })
    return () => (gone = true)
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

  // Saves part of the Application, the rest as it is now; the sub-pages that
  // take an onsave throw the API's ApiError back to show it.
  async function patch(change: Partial<ApplicationInput>) {
    if (!application) return
    const r = await api<{ application: Application }>('PATCH', `/applications/${application.id}`, {
      ...applicationInput(application),
      ...change,
    })
    application = r.application
  }

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
      appStatus = await api<ApplicationStatus>('POST', `/applications/${id}/stop`)
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

  const serverNames = $derived<Record<number, string>>(server ? { [server.id]: server.name } : {})

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
          <div class="chrome mb-6">
            <Callout title="Deploy key">
              Private repository: add the deploy key from <a class="underline" href={href(applicationPath(application, 'source'))}>Git Source</a> to the
              repository before the first deploy.
            </Callout>
          </div>
        {/if}

        {#if page === ''}
          <General {application} onchange={(a) => (application = a)} />
        {:else if page === 'domains'}
          <Domains {application} {server} onchange={(a) => (application = a)} />
        {:else if page === 'environment-variables'}
          <EnvironmentVariables path={`/applications/${application.id}/environment-variables`} />
        {:else if page === 'deployment'}
          {#if deploymentId}
            <DeploymentPage {application} {deploymentId} {serverNames} onchange={() => loadDeployments().catch(() => {})} />
          {:else}
            <DeploymentHistory {application} {serverNames} />
          {/if}
        {:else if page === 'logs'}
          <RuntimeLogs url={`/api/applications/${application.id}/logs`} container={appStatus ? appStatus.container : null} />
        {:else if page === 'source' && application.build_pack !== 'dockerimage'}
          <Source {application} onchange={(a) => (application = a)} />
        {:else if page === 'webhooks'}
          <Webhooks {application} />
        {:else if page === 'preview-deployments'}
          <Previews {application} onchange={() => loadDeployments().catch(() => {})} />
        {:else if page === 'servers'}
          <Servers {server} {status} />
        {:else if page === 'persistent-storage'}
          <PersistentStorage storages={application.storages} onsave={(storages) => patch({ storages })} />
        {:else if page === 'resource-limits'}
          <ResourceLimits
            limits={application.resource_limits}
            onsave={(resource_limits) => patch({ resource_limits })}
            applied="Redeploy to apply them."
          />
        {:else if page === 'healthcheck'}
          <Healthcheck {application} {status} onchange={(a) => (application = a)} />
        {:else if page === 'rollback'}
          <Rollback {application} />
        {:else if page === 'advanced'}
          <Advanced {application} />
        {:else if page === 'danger' && session.canWrite}
          <Danger
            label="application"
            name={application.name}
            url={`/applications/${application.id}`}
            checkboxes={[
              { id: 'delete_volumes', label: 'Permanently delete all volumes associated with this resource.', checked: true },
              // In place of Coolify's Docker cleanup.
              { id: 'delete_images', label: 'Remove the images its deployments built or pulled.', checked: true },
            ]}
            back={`/project/${application.project_id}/environment/${application.environment_id}`}
          />
        {:else}
          <div class="chrome">
            <h2 class="text-[15px]! font-semibold! text-black dark:text-fg">Not available</h2>
            <p class="mt-2 text-[13px] text-neutral-600 dark:text-fg-dim">This Application has no such page.</p>
          </div>
        {/if}
      </div>
    </div>
  </section>
{/if}
