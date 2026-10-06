<script lang="ts">
  // Coolify's Dashboard (resources/views/livewire/dashboard.blade.php and
  // app/Livewire/Dashboard.php, Apache-2.0, see NOTICE): the first eight
  // Projects and Servers by name, each as a card, under a section heading
  // with "View all".
  //
  // Left out for now: the active-deployments strip (dashboard.active-deployments)
  // and the per-server metrics chart (dashboard.server-metrics-chart). They come
  // with the live updates of the page ports. The Servers empty state skips "A
  // private key is required": every Server here gets its own key when it is
  // added.
  import { api } from '../lib/api'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import Icon from '../lib/Icon.svelte'
  import { projectCounts, type ProjectCounts } from '../lib/projectCounts'
  import { href, serverPath } from '../lib/router.svelte'
  import { session } from '../lib/session.svelte'
  import type { Project, Server } from '../lib/types'
  import Empty from '../lib/ui/Empty.svelte'
  import SectionHeading from '../lib/ui/SectionHeading.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'

  const itemLimit = 8
  const byName = (a: { name: string }, b: { name: string }) =>
    a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' })

  let projects = $state.raw<Project[] | null>(null)
  let servers = $state.raw<Server[] | null>(null)
  // Per Project id: its Environments and Resources, filled in after the list.
  let counts = $state.raw<Record<number, ProjectCounts>>({})
  let projectsError = $state('')
  let serversError = $state('')

  api<{ projects: Project[] }>('GET', '/projects')
    .then((r) => {
      projects = [...r.projects].sort(byName).slice(0, itemLimit)
      for (const p of projects) {
        projectCounts(p.id).then((c) => {
          if (c) counts = { ...counts, [p.id]: c }
        })
      }
    })
    .catch((e) => (projectsError = e.message))

  api<{ servers: Server[] }>('GET', '/servers')
    .then((r) => (servers = [...r.servers].sort(byName).slice(0, itemLimit)))
    .catch((e) => (serversError = e.message))

  const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`

  // Coolify's server states, in its order, in this context's words: a Server
  // not yet validated is not ready, one that failed validation is unreachable.
  function serverStatus(s: Server): { label: string; type: 'success' | 'warning' | 'error' } {
    if (s.status === 'unreachable') return { label: 'Unreachable', type: 'error' }
    if (s.status === 'unvalidated') return { label: 'Not ready', type: 'warning' }
    return { label: 'Ready', type: 'success' }
  }

  const card =
    'group relative flex min-h-28 min-w-0 flex-col rounded-xl border border-neutral-200 bg-white p-3 shadow-sm transition-all hover:-translate-y-px hover:border-neutral-300 hover:shadow-md dark:border-white/[0.08] dark:bg-white/[0.05] dark:hover:border-white/[0.14]'
  const iconBox =
    'flex size-8 shrink-0 items-center justify-center rounded-lg border border-neutral-200 bg-neutral-50 text-neutral-500 dark:bg-white/[0.04] dark:text-fg-dim'

  $effect(() => breadcrumb.set({ label: 'Dashboard' }))
</script>

<div class="chrome w-full">
  <div class="flex min-w-0 flex-col gap-8">
    <section class="mb-0! min-w-0">
      <SectionHeading title="Projects" subtitle="Your deployment workspaces" href={href('/projects')} />

      {#if projectsError}
        <p class="text-sm text-error">{projectsError}</p>
      {:else if projects === null}
        <Spinner text="Loading…" />
      {:else if projects.length === 0}
        <Empty title="No projects yet" description="Use New project to create your first deployment workspace." icon="projects" size="sm">
          {#if session.can('manage_applications')}
            <a href={href('/projects')} class="button button-highlighted">
              <Icon name="plus" class="size-3.5" />
              New project
            </a>
          {/if}
        </Empty>
      {:else}
        <div class="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {#each projects as project (project.id)}
            {@const c = counts[project.id]}
            <article class={card}>
              <a href={href(`/project/${project.id}`)} class="absolute inset-0 rounded-xl" aria-label="Open {project.name}"></a>

              <div class="flex min-w-0 items-start gap-3">
                <div class={[iconBox, 'dark:border-white/[0.08]']}>
                  <Icon name="projects" class="size-4" />
                </div>
                <div class="min-w-0 flex-1">
                  <h3 class="truncate text-[13px]! leading-4! font-semibold! text-black dark:text-fg">{project.name}</h3>
                  <p class="mt-0.5 truncate text-[11px] text-neutral-500 dark:text-fg-faint">{project.description}</p>
                </div>
              </div>

              <div class="mt-auto flex items-center justify-between gap-3 pt-4">
                <p class="min-w-0 truncate text-[11px] text-neutral-500 dark:text-fg-dim">
                  {#if c}
                    {plural(c.environments, 'env')}
                    <span class="px-1 text-neutral-300 dark:text-white/15">·</span>
                    {plural(c.resources, 'resource')}
                  {/if}
                </p>

                <div class="relative z-10 flex shrink-0 items-center gap-0.5">
                  {#if c?.firstEnvironment && session.can('manage_applications')}
                    <a
                      href={href(`/project/${project.id}/environment/${c.firstEnvironment}/new`)}
                      class="flex size-6.5 items-center justify-center rounded-md text-neutral-400 transition-colors hover:bg-neutral-100 hover:text-black dark:text-fg-faint dark:hover:bg-white/[0.06] dark:hover:text-fg"
                      title="Add resource"
                      aria-label="Add resource to {project.name}"
                    >
                      <Icon name="plus" class="size-3" />
                    </a>
                  {/if}
                  {#if session.can('manage_applications')}
                    <a
                      href={href(`/project/${project.id}/edit`)}
                      class="flex size-6.5 items-center justify-center rounded-md text-neutral-400 transition-colors hover:bg-neutral-100 hover:text-black dark:text-fg-faint dark:hover:bg-white/[0.06] dark:hover:text-fg"
                      title="Project settings"
                      aria-label="Open settings for {project.name}"
                    >
                      <Icon name="settings" class="size-3" />
                    </a>
                  {/if}
                </div>
              </div>
            </article>
          {/each}
        </div>
      {/if}
    </section>

    <section class="mb-0! min-w-0">
      <SectionHeading title="Servers" subtitle="Infrastructure available for deployments" href={href('/servers')} />

      {#if serversError}
        <p class="text-sm text-error">{serversError}</p>
      {:else if servers === null}
        <Spinner text="Loading…" />
      {:else if servers.length === 0}
        <Empty title="No servers yet" description="Connect infrastructure for your deployments." icon="servers" size="sm">
          {#if session.can('manage_servers')}
            <a href={href('/servers')} class="button button-highlighted">
              <Icon name="plus" class="size-3.5" />
              New server
            </a>
          {/if}
        </Empty>
      {:else}
        <div class="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {#each servers as server (server.id)}
            {@const status = serverStatus(server)}
            <a href={href(serverPath(server.id))} aria-label="Open {server.name}" class={card}>
              <div class="relative z-10 flex min-w-0 items-start gap-3">
                <div class={[iconBox, 'dark:border-white/[0.1]']}>
                  <Icon name="servers" class="size-4" />
                </div>
                <div class="min-w-0 flex-1">
                  <h3 class="truncate text-[13px]! leading-4! font-semibold! text-black dark:text-fg">{server.name}</h3>
                  <p class="mt-0.5 truncate text-[11px] text-neutral-500 dark:text-fg-faint">
                    {server.kind === 'local' ? 'This machine' : server.host}
                  </p>
                </div>
                {#if status.type !== 'success'}
                  <span
                    data-tooltip={status.label}
                    title={status.label}
                    aria-label="Server status: {status.label}"
                    class={[
                      'flex size-6 shrink-0 items-center justify-center rounded-md',
                      status.type === 'warning' && 'text-orange-500 dark:text-warning',
                      status.type === 'error' && 'text-red-500 dark:text-red-400',
                    ]}
                  >
                    <Icon name="alert-triangle" class="size-4" />
                  </span>
                {/if}
              </div>
            </a>
          {/each}
        </div>
      {/if}
    </section>
  </div>
</div>
