<script lang="ts">
  // Coolify's Dashboard content (resources/views/livewire/dashboard.blade.php
  // and app/Livewire/Dashboard.php, Apache-2.0, see NOTICE) in Paperclip's
  // Dashboard layout (ui/src/pages/Dashboard.tsx, MIT): a row of metric cards,
  // then the first eight Projects and Servers by name as two list cards, each
  // under a section heading with "View all".
  //
  // Left out for now: the active-deployments strip (dashboard.active-deployments)
  // and the per-server metrics chart (dashboard.server-metrics-chart). They come
  // with the live updates of the page ports. The Servers empty state skips "A
  // private key is required": every Server here gets its own key when it is
  // added.
  import { buttonVariants } from '$lib/components/ui/button'
  import { Card } from '$lib/components/ui/card'
  import { api } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import Icon from '../lib/Icon.svelte'
  import MetricCard from '../lib/MetricCard.svelte'
  import PageSkeleton from '../lib/PageSkeleton.svelte'
  import { projectCounts, type ProjectCounts } from '../lib/projectCounts'
  import { href, serverPath } from '../lib/router.svelte'
  import { canIn, session } from '../lib/session.svelte'
  import type { Project, Server } from '../lib/types'
  import Empty from '../lib/ui/Empty.svelte'
  import StatusBadge from '../lib/ui/StatusBadge.svelte'
  import { statusType, type StatusType } from '../lib/statusColors'

  const itemLimit = 8
  const byName = (a: { name: string }, b: { name: string }) =>
    a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' })

  let projects = $state.raw<Project[] | null>(null)
  let servers = $state.raw<Server[] | null>(null)
  // Per Project id: its Environments and Resources, filled in after the list.
  // Every Project's, not only the eight listed: the Resources card sums them.
  let counts = $state.raw<Record<number, ProjectCounts>>({})
  let projectsError = $state('')
  let serversError = $state('')

  api<{ projects: Project[] }>('GET', '/projects')
    .then((r) => {
      projects = [...r.projects].sort(byName)
      for (const p of projects) {
        projectCounts(p.id).then((c) => {
          if (c) counts = { ...counts, [p.id]: c }
        })
      }
    })
    .catch((e) => (projectsError = e.message))

  api<{ servers: Server[] }>('GET', '/servers')
    .then((r) => (servers = [...r.servers].sort(byName)))
    .catch((e) => (serversError = e.message))

  const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`

  // Coolify's server states, in its order, in this context's words: a Server
  // not yet validated is not ready, one that failed validation is unreachable.
  function serverStatus(s: Server): { label: string; type: StatusType } {
    const label = s.status === 'unreachable' ? 'Unreachable' : s.status === 'unvalidated' ? 'Not ready' : 'Ready'
    return { label, type: statusType(s.status) }
  }

  // The Resources card waits until every Project's counts are in, so it never
  // shows a number that is still growing.
  const resources = $derived.by(() => {
    if (!projects) return null
    const loaded = projects.map((p) => counts[p.id]).filter(Boolean)
    if (loaded.length < projects.length) return null
    return loaded.reduce((n, c) => n + c.resources, 0)
  })
  const notReady = $derived(servers?.filter((s) => s.status === 'unvalidated').length ?? 0)
  const unreachable = $derived(servers?.filter((s) => s.status === 'unreachable').length ?? 0)

  const heading = 'text-sm font-semibold tracking-wide text-muted-foreground uppercase'
  const viewAll = 'text-xs text-muted-foreground no-underline transition-colors hover:text-foreground'
  const listCard = '@container block gap-0 divide-y divide-border overflow-hidden py-0'
  const rowAction =
    'flex size-6.5 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground'

  $effect(() => breadcrumb.set({ label: 'Dashboard' }))
</script>

<div class="chrome w-full space-y-6">
  {#if projects === null && servers === null && !projectsError && !serversError}
    <PageSkeleton variant="dashboard" />
  {:else}
    <div class="grid grid-cols-2 gap-1 sm:gap-2 xl:grid-cols-4">
      <MetricCard icon="projects" value={projects?.length ?? '–'} label="Projects" href={href('/projects')}>
        {#snippet description()}Deployment workspaces{/snippet}
      </MetricCard>
      <MetricCard icon="layers" value={resources ?? '–'} label="Resources" href={href('/projects')}>
        {#snippet description()}Applications, databases and services{/snippet}
      </MetricCard>
      <MetricCard icon="servers" value={servers?.length ?? '–'} label="Servers" href={href('/servers')}>
        {#snippet description()}Infrastructure for deployments{/snippet}
      </MetricCard>
      <MetricCard icon="alert-triangle" value={servers ? notReady + unreachable : '–'} label="Servers needing attention" href={href('/servers')}>
        {#snippet description()}{notReady} not ready, {unreachable} unreachable{/snippet}
      </MetricCard>
    </div>

    <div class="grid gap-4 md:grid-cols-2">
      <section class="mb-0! min-w-0">
        <div class="mb-3 flex items-baseline justify-between gap-3">
          <h3 class={heading}>Projects</h3>
          <a href={href('/projects')} class={viewAll}>View all</a>
        </div>

        {#if projectsError}
          <Card class="block p-4"><p class="text-sm text-destructive">{projectsError}</p></Card>
        {:else if projects === null}
          <PageSkeleton />
        {:else if projects.length === 0}
          <Card class="block py-0">
            <Empty title="No projects yet" description="Use New project to create your first deployment workspace." icon="projects" size="sm">
              {#if session.can('manage_applications')}
                <a href={href('/projects')} class={buttonVariants()}>
                  <Icon name="plus" class="size-3.5" />
                  New project
                </a>
              {/if}
            </Empty>
          </Card>
        {:else}
          <Card class={listCard}>
            {#each projects.slice(0, itemLimit) as project (project.id)}
              {@const c = counts[project.id]}
              <div class="dashboard-list-row relative flex items-start gap-2 text-sm transition-colors hover:bg-accent/50">
                <a href={href(`/project/${project.id}`)} class="absolute inset-0" aria-label="Open {project.name}"></a>
                <div class="flex min-w-0 flex-1 flex-col">
                  <span class="truncate text-sm leading-6" title={project.name}>{project.name}</span>
                  <span class="truncate text-(length:--text-micro) text-muted-foreground">
                    {[project.description, c && `${plural(c.environments, 'env')} · ${plural(c.resources, 'resource')}`].filter(Boolean).join(' · ')}
                  </span>
                </div>
                {#if c && canIn(c.permissions, 'manage_applications')}
                  <div class="relative z-10 flex shrink-0 items-center gap-0.5 self-center">
                    {#if c.firstEnvironment}
                      <a
                        href={href(`/project/${project.id}/environment/${c.firstEnvironment}/new`)}
                        class={rowAction}
                        title="Add resource"
                        aria-label="Add resource to {project.name}"
                      >
                        <Icon name="plus" class="size-3.5" />
                      </a>
                    {/if}
                    <a href={href(`/project/${project.id}/edit`)} class={rowAction} title="Project settings" aria-label="Open settings for {project.name}">
                      <Icon name="settings" class="size-3.5" />
                    </a>
                  </div>
                {/if}
              </div>
            {/each}
          </Card>
        {/if}
      </section>

      <section class="mb-0! min-w-0">
        <div class="mb-3 flex items-baseline justify-between gap-3">
          <h3 class={heading}>Servers</h3>
          <a href={href('/servers')} class={viewAll}>View all</a>
        </div>

        {#if serversError}
          <Card class="block p-4"><p class="text-sm text-destructive">{serversError}</p></Card>
        {:else if servers === null}
          <PageSkeleton />
        {:else if servers.length === 0}
          <Card class="block py-0">
            <Empty title="No servers yet" description="Connect infrastructure for your deployments." icon="servers" size="sm">
              {#if session.can('manage_servers')}
                <a href={href('/servers/new')} class={buttonVariants()}>
                  <Icon name="plus" class="size-3.5" />
                  New server
                </a>
              {/if}
            </Empty>
          </Card>
        {:else}
          <Card class={listCard}>
            {#each servers.slice(0, itemLimit) as server (server.id)}
              {@const status = serverStatus(server)}
              <a
                href={href(serverPath(server.id))}
                aria-label="Open {server.name}"
                class="dashboard-list-row flex items-center gap-2 text-sm text-inherit no-underline transition-colors hover:bg-accent/50"
              >
                <span class="flex min-w-0 flex-1 flex-col">
                  <span class="truncate text-sm leading-6" title={server.name}>{server.name}</span>
                  {#if server.kind === 'local'}
                    <span class="truncate text-(length:--text-micro) text-muted-foreground">This machine</span>
                  {:else}
                    <span class="truncate font-mono text-(length:--text-micro) text-muted-foreground">{server.host}</span>
                  {/if}
                </span>
                <StatusBadge label={status.label} type={status.type} />
              </a>
            {/each}
          </Card>
        {/if}
      </section>
    </div>
  {/if}
</div>
