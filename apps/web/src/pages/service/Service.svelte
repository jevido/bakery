<script lang="ts">
  // Coolify's Service page (resources/views/livewire/project/service/configuration.blade.php
  // and heading.blade.php, Apache-2.0, see NOTICE): the heading with the
  // Service's Links and Actions, the configuration sidebar and the sub-page
  // the URL names.
  import { api, ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import { href, servicePath, type ServicePage } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Environment, Service } from '../../lib/types'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import Danger from '../application/Danger.svelte'
  import Heading, { type Action } from '../application/Heading.svelte'
  import ConfigurationSidebar from './ConfigurationSidebar.svelte'
  import Domains from './Domains.svelte'
  import General from './General.svelte'
  import Interim from './Interim.svelte'

  let { projectId, environmentId, id, page }: { projectId: number; environmentId: number; id: number; page: ServicePage } = $props()

  let service = $state.raw<Service | null>(null)
  let environment = $state.raw<Environment | null>(null)
  let loadError = $state('')
  let acting = $state(false)

  async function load() {
    const r = await api<{ service: Service }>('GET', `/services/${id}`)
    service = r.service
  }

  $effect(() => {
    service = null
    loadError = ''
    api<{ service: Service }>('GET', `/services/${id}`)
      .then(({ service: s }) => {
        // A link with another Project or Environment in it still opens the
        // Service, at its own path.
        if (s.project_id !== projectId || s.environment_id !== environmentId) {
          location.replace('#' + servicePath(s, page))
          return
        }
        service = s
        api<{ environment: Environment }>('GET', `/environments/${s.environment_id}`)
          .then((e) => (environment = e.environment))
          .catch(() => {})
      })
      .catch((e) => (loadError = e.message))
  })

  // Quickly while an action runs, slower otherwise so a crash shows up.
  let deploying = $derived(service?.status === 'deploying')
  $effect(() => {
    const t = setInterval(() => load().catch(() => {}), deploying ? 2000 : 5000)
    return () => clearInterval(t)
  })

  async function act(action: 'start' | 'stop' | 'restart' | 'redeploy') {
    acting = true
    if (action === 'stop' || action === 'restart') toast.info('Gracefully stopping service.', 'It could take a while depending on the service.')
    if (action === 'redeploy') toast.info('Pulling new images and restarting service.')
    try {
      const r = await api<{ service: Service }>('POST', `/services/${id}/${action}`)
      service = r.service
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error(`Service not ${action === 'stop' ? 'stopped' : action === 'start' ? 'deployed' : 'restarted'}`, err.message)
    } finally {
      acting = false
    }
  }

  // The Service status in Coolify's words: its Components have passed their
  // readiness checks once all of them run, so a running Service reads as
  // `running:healthy`, and a stopped one as Coolify's `exited`.
  const status = $derived.by(() => {
    if (!service) return null
    switch (service.status) {
      case 'running':
        return 'running:healthy'
      case 'degraded':
        return 'degraded:unhealthy'
      case 'deploying':
        return 'starting'
      case 'stopped':
        return 'exited'
      default:
        return service.status
    }
  })

  const urls = $derived(service ? service.components.filter((c) => c.public).flatMap((c) => c.domains.map((d) => `https://${d}`)) : [])

  // Coolify opens these modals by clicking their hidden triggers.
  const openModal = (id: string) => () => document.getElementById(id)?.click()

  // Coolify's heading.blade.php: a running or degraded Service is restarted or
  // stopped, any other is deployed.
  const busy = $derived(acting || !!service?.busy)
  const actions = $derived<Action[]>(
    !session.canWrite || !service
      ? []
      : service.status === 'running' || service.status === 'degraded'
        ? [
            { label: 'Restart', icon: 'restart', run: openModal('service-restart-trigger'), disabled: busy },
            ...(service.status === 'running'
              ? [{ label: 'Restart (pull latest)', icon: 'refresh', run: () => act('redeploy'), disabled: busy } satisfies Action]
              : []),
            { label: 'Stop', icon: 'stop-circle', run: openModal('service-stop-trigger'), danger: true, disabled: busy },
          ]
        : [{ label: 'Deploy', icon: 'play-circle', run: () => act('start'), disabled: busy }],
  )

  // Projects › Project › Environment › Service, with its status.
  const crumbs = $derived(
    service && environment ? { project: environment.project_name ?? 'Project', environment: environment.name, name: service.name } : null,
  )
  $effect(() => {
    if (crumbs)
      breadcrumb.set(
        { label: 'Projects', href: href('/projects') },
        { label: crumbs.project, href: href(`/project/${projectId}`) },
        { label: crumbs.environment, href: href(`/project/${projectId}/environment/${environmentId}`) },
        { label: crumbs.name, summary: status ?? undefined, summaryTitle: 'Service status' },
      )
  })
</script>

{#if loadError}
  <p class="chrome text-sm text-error">{loadError}</p>
{:else if !service}
  <div class="chrome"><Spinner text="Loading…" /></div>
{:else}
  <Heading name={service.name} {urls} {status} {actions} resource="service" error={service.last_error} />
  {#if session.canWrite}
    <div class="hidden" aria-hidden="true">
      <ConfirmationModal
        title="Confirm Service Restart?"
        buttonTitle="Restart"
        actions={['This service will be restarted.']}
        confirmWithText={false}
        onconfirm={() => act('restart')}
      >
        {#snippet trigger(show)}
          <button id="service-restart-trigger" type="button" onclick={show}>Restart</button>
        {/snippet}
      </ConfirmationModal>
      <ConfirmationModal
        title="Confirm Service Stopping?"
        buttonTitle="Stop"
        actions={['This service will be stopped.', 'All non-persistent data will be deleted.']}
        confirmWithText={false}
        onconfirm={() => act('stop')}
      >
        {#snippet trigger(show)}
          <button id="service-stop-trigger" type="button" onclick={show}>Stop</button>
        {/snippet}
      </ConfirmationModal>
    </div>
  {/if}

  <section class="mt-4 w-full max-w-none lg:mt-0">
    <div class="grid min-w-0 gap-8 xl:grid-cols-[210px_minmax(0,1fr)] xl:gap-8">
      <ConfigurationSidebar {service} {page} />

      <div class="min-w-0">
        {#if page === ''}
          <General {service} {environment} onchange={(s) => (service = s)} />
        {:else if page === 'domains'}
          <Domains {service} onchange={(s) => (service = s)} />
        {:else if page === 'environment-variables' || page === 'logs'}
          <Interim {service} {page} onchange={(s) => (service = s)} />
        {:else if page === 'danger' && session.canWrite}
          <Danger
            label="service"
            name={service.name}
            url={`/services/${service.id}`}
            checkboxes={[]}
            back={`/project/${projectId}/environment/${environmentId}`}
          />
        {:else}
          <div class="chrome">
            <h2 class="text-[15px]! font-semibold! text-black dark:text-fg">Not available</h2>
            <p class="mt-2 text-[13px] text-neutral-600 dark:text-fg-dim">This Service has no such page.</p>
          </div>
        {/if}
      </div>
    </div>
  </section>
{/if}
