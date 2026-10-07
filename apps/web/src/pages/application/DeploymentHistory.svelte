<script lang="ts" module>
  import type { Deployment } from '../../lib/types'
  import type { StatusType } from '../../lib/ui/StatusBadge.svelte'

  /** Coolify's status label and colour for a Deployment. */
  export function deploymentStatus(d: Pick<Deployment, 'status' | 'active'>): { label: string; type: StatusType } {
    if (d.status === 'finished') return { label: 'Success', type: 'success' }
    if (d.status === 'failed') return { label: 'Failed', type: 'error' }
    if (d.status === 'queued') return { label: 'Queued', type: 'warning' }
    if (d.status === 'cancelled') return { label: 'Cancelled', type: 'neutral' }
    return { label: 'In progress', type: 'warning' }
  }

  /** Coolify's Source column; The Bakery also names Restarts. */
  export function deploymentSource(d: Deployment): string {
    if (d.trigger === 'webhook' && d.preview > 0) return `Webhook · PR #${d.preview}`
    if (d.trigger === 'webhook') return 'Webhook'
    if (d.preview > 0) return `Pull request #${d.preview}`
    if (d.trigger === 'rollback') return 'Rollback'
    if (d.trigger === 'restart') return 'Restart'
    return 'Manual'
  }

  /** The commit's page on the git host, from an https or scp-style git URL. */
  export function commitLink(gitUrl: string, sha: string): string | null {
    const m = gitUrl.match(/^https?:\/\/([^/]+)\/(.+?)(?:\.git)?\/?$/) ?? gitUrl.match(/^(?:ssh:\/\/)?[^@]+@([^:/]+)[:/](.+?)(?:\.git)?$/)
    return m ? `https://${m[1]}/${m[2]}/commit/${sha}` : null
  }
</script>

<script lang="ts">
  // Coolify's Deployment history (resources/views/livewire/project/application/deployment/index.blade.php
  // and app/Livewire/Project/Application/Deployment/Index.php, Apache-2.0,
  // see NOTICE): search, the Status, Source, Server and Pull request
  // filters, Sort, the table and the pagination. Embedded above one
  // Deployment's log it shows three rows, as Coolify's does.
  import { untrack } from 'svelte'
  import { api } from '../../lib/api'
  import { ago, duration } from '../../lib/format'
  import Icon from '../../lib/Icon.svelte'
  import { applicationPath, href } from '../../lib/router.svelte'
  import type { Application } from '../../lib/types'
  import Empty from '../../lib/ui/Empty.svelte'
  import PageSizeSelect from '../../lib/ui/PageSizeSelect.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import TableDropdown from '../../lib/ui/TableDropdown.svelte'

  let {
    application,
    serverNames = {},
    selected = null,
    embedded = false,
    reload = 0,
  }: {
    application: Application
    /** Server names by id. */
    serverNames?: Record<number, string>
    /** The open Deployment, highlighted. */
    selected?: number | null
    embedded?: boolean
    /** Bumped by the page to load again at once (after a deploy). */
    reload?: number
  } = $props()

  type Filters = { statuses: string[]; sources: string[]; server_ids: number[]; pull_requests: number[] }

  const statusLabels: Record<string, string> = {
    finished: 'Success',
    failed: 'Failed',
    in_progress: 'In progress',
    queued: 'Queued',
    cancelled: 'Cancelled',
  }
  const sourceLabels: Record<string, string> = {
    manual: 'Manual',
    'pull-request': 'Pull requests',
    webhook: 'Webhooks',
    rollback: 'Rollbacks',
    restart: 'Restarts',
  }

  let deployments = $state.raw<Deployment[]>([])
  let count = $state(0)
  let options = $state.raw<Filters>({ statuses: [], sources: [], server_ids: [], pull_requests: [] })
  let loaded = $state(false)
  let loading = $state(false)

  let search = $state('')
  let query = $state('')
  // Status and Source filters as Coolify's "status:…" and "source:…" values.
  let picked = $state<string[]>([])
  let pullRequest = $state(0)
  let sort = $state<'newest' | 'oldest'>('newest')
  let skip = $state(0)
  let take = $state(untrack(() => (embedded ? 3 : 10)))

  const currentPage = $derived(Math.floor(skip / take) + 1)
  const lastPage = $derived(Math.max(1, Math.ceil(count / take)))
  const firstRow = $derived(count === 0 ? 0 : skip + 1)
  const lastRow = $derived(Math.min(skip + deployments.length, count))
  const activeCount = $derived(picked.length + (pullRequest ? 1 : 0))
  const hasQuery = $derived(query !== '' || activeCount > 0)

  // The search box loads 300 ms after the last key, as Coolify's debounce does.
  $effect(() => {
    const value = search.trim()
    const timer = setTimeout(() => {
      if (value !== untrack(() => query)) {
        query = value
        skip = 0
      }
    }, 300)
    return () => clearTimeout(timer)
  })

  function params(): string {
    const p = new URLSearchParams({ skip: String(skip), take: String(take), filters: '1' })
    if (sort === 'oldest') p.set('sort', 'oldest')
    if (query) p.set('search', query)
    const pick = (prefix: string) =>
      picked
        .filter((v) => v.startsWith(prefix))
        .map((v) => v.slice(prefix.length))
        .join(',')
    if (pick('status:')) p.set('status', pick('status:'))
    if (pick('source:')) p.set('source', pick('source:'))
    if (pick('server:')) p.set('server', pick('server:'))
    if (pullRequest) p.set('pull_request', String(pullRequest))
    return p.toString()
  }

  let sequence = 0
  async function load() {
    const mine = ++sequence
    loading = true
    try {
      const r = await api<{ deployments: Deployment[]; count: number; filters: Filters }>(
        'GET',
        `/applications/${application.id}/deployments?${params()}`,
      )
      if (mine !== sequence) return
      deployments = r.deployments
      count = r.count
      options = r.filters
      loaded = true
      // A page past the end after a filter or a deletion goes to the last one.
      if (r.deployments.length === 0 && skip > 0 && r.count > 0) skip = Math.max(0, (Math.ceil(r.count / take) - 1) * take)
    } finally {
      if (mine === sequence) loading = false
    }
  }

  $effect(() => {
    void [application.id, skip, take, sort, query, picked.length, pullRequest, reload]
    untrack(() => load().catch(() => {}))
  })

  // Coolify polls the first page every five seconds; later pages hold still.
  $effect(() => {
    if (skip !== 0) return
    const t = setInterval(() => load().catch(() => {}), 5000)
    return () => clearInterval(t)
  })

  function toggle(value: string) {
    picked = picked.includes(value) ? picked.filter((v) => v !== value) : [...picked, value]
    skip = 0
  }

  function clearFilter() {
    picked = []
    pullRequest = 0
    skip = 0
  }

  const deploymentHref = (d: Deployment) => href(`${applicationPath(application, 'deployment')}/${d.id}`)

  function durationOf(d: Deployment): string {
    if (d.status === 'queued') return 'Waiting'
    if (d.active) return duration(d.created_at, new Date())
    return d.finished_at ? duration(d.created_at, d.finished_at) : '-'
  }

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' })
  const serverName = (id: number) => serverNames[id] ?? `Server #${id}`
</script>

{#snippet check(selected: boolean)}
  <span
    class={[
      'flex size-4 shrink-0 items-center justify-center rounded-[5px] border',
      selected
        ? 'border-coollabs bg-coollabs text-primary-foreground'
        : 'border-neutral-300 bg-white dark:border-white/[0.14] dark:bg-white/[0.045]',
    ]}
  >
    {#if selected}
      <svg class="size-3" viewBox="0 0 12 12" fill="none" aria-hidden="true">
        <path d="m2.25 6.15 2.35 2.3 5.15-5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    {/if}
  </span>
{/snippet}

{#snippet tick()}
  <svg class="size-3.5 shrink-0" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <path d="m4.5 12.75 6 6 9-13.5" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
  </svg>
{/snippet}

{#snippet group(title: string, values: { value: string; label: string }[])}
  {#if values.length > 0}
    <span class="px-2 pt-2 pb-1 text-[10px] font-medium tracking-wider text-neutral-400 uppercase dark:text-fg-faint">{title}</span>
    {#each values as option (option.value)}
      <button
        type="button"
        class="listbox-option"
        role="option"
        aria-selected={picked.includes(option.value)}
        onclick={() => toggle(option.value)}
      >
        <span class="truncate">{option.label}</span>
        {@render check(picked.includes(option.value))}
      </button>
    {/each}
  {/if}
{/snippet}

<div class="chrome">
<SettingsSection
  id="deployment-history-section"
  title="Deployment history"
  helper="Search, filter, and open a deployment to inspect its build logs."
  flush
>
  <div class="table-toolbar flex flex-col gap-2 border-b border-neutral-200 p-3 sm:flex-row sm:flex-wrap sm:items-center dark:border-white/[0.08]">
    <div class="w-full min-w-0 flex-1 sm:max-w-md">
      <div class="table-search relative w-full min-w-0">
        <input type="search" placeholder="Search deployments" aria-label="Search deployments" class="input w-full pl-8!" bind:value={search} />
        <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-2.5">
          {#if loading && search.trim() !== query}
            <svg aria-hidden="true" class="size-3.5 animate-spin text-neutral-400 dark:text-fg-dim" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
            </svg>
          {:else}
            <Icon name="search" class="size-3.5 text-neutral-400 dark:text-fg-faint" />
          {/if}
        </div>
      </div>
    </div>
    <div class="flex flex-wrap items-center gap-2 sm:ml-auto">
      <div class="table-filter">
        <TableDropdown panelClass="w-44! overflow-hidden! p-0!">
          {#snippet trigger({ open, toggle: toggleOpen })}
            <button
              type="button"
              aria-haspopup="listbox"
              aria-expanded={open}
              title={pullRequest ? `Pull request #${pullRequest}` : undefined}
              class={['button max-w-80 min-w-0', activeCount > 0 && 'button-highlighted']}
              onclick={toggleOpen}
            >
              <Icon name="filter" class="size-3.5 shrink-0" />
              <span class="truncate">{pullRequest ? `Pull request #${pullRequest}` : 'Filter'}</span>
              {#if activeCount > 0}
                <span
                  class="shrink-0 rounded-full bg-neutral-100 px-1.5 py-0.5 text-[10px] font-medium text-neutral-500 dark:bg-white/[0.07] dark:text-fg-dim"
                  >{activeCount}</span
                >
              {/if}
            </button>
          {/snippet}
          {#snippet children(close)}
            <div class="max-h-80 overflow-y-auto p-1" aria-multiselectable="true">
              {@render group(
                'Status',
                Object.keys(statusLabels)
                  .filter((s) => options.statuses.includes(s))
                  .map((s) => ({ value: `status:${s}`, label: statusLabels[s] })),
              )}
              {@render group(
                'Source',
                Object.keys(sourceLabels)
                  .filter((s) => options.sources.includes(s))
                  .map((s) => ({ value: `source:${s}`, label: sourceLabels[s] })),
              )}
              {@render group(
                'Server',
                options.server_ids.map((id) => ({ value: `server:${id}`, label: serverName(id) })),
              )}
              {#if options.pull_requests.length > 0}
                <span class="px-2 pt-2 pb-1 text-[10px] font-medium tracking-wider text-neutral-400 uppercase dark:text-fg-faint"
                  >Pull request</span
                >
                {#each options.pull_requests as number (number)}
                  <button
                    type="button"
                    class="listbox-option"
                    role="option"
                    aria-selected={pullRequest === number}
                    onclick={() => {
                      pullRequest = pullRequest === number ? 0 : number
                      skip = 0
                      close()
                    }}
                  >
                    <span>Pull request #{number}</span>
                    {#if pullRequest === number}{@render tick()}{/if}
                  </button>
                {/each}
              {/if}
            </div>
            <div class="border-t border-neutral-200 bg-white p-1 dark:border-white/10 dark:bg-raised">
              <button
                type="button"
                class="listbox-option text-neutral-500 dark:text-fg-dim"
                disabled={activeCount === 0}
                onclick={() => {
                  clearFilter()
                  close()
                }}
              >
                <span>Reset filters</span>
                <Icon name="x" class="size-3.5" />
              </button>
            </div>
          {/snippet}
        </TableDropdown>
      </div>
      <div class="table-sort">
        <TableDropdown panelClass="w-44!">
          {#snippet trigger({ open, toggle: toggleOpen })}
            <button type="button" class="button" aria-haspopup="listbox" aria-expanded={open} onclick={toggleOpen}>
              <Icon name="sort-direction" class="size-3.5" />
              Sort
            </button>
          {/snippet}
          {#snippet children(close)}
            {#each [['newest', 'Newest first'], ['oldest', 'Oldest first']] as const as [value, label] (value)}
              <button
                type="button"
                class="listbox-option"
                role="option"
                aria-selected={sort === value}
                onclick={() => {
                  sort = value
                  skip = 0
                  close()
                }}
              >
                <span>{label}</span>
                {#if sort === value}{@render tick()}{/if}
              </button>
            {/each}
          {/snippet}
        </TableDropdown>
      </div>
    </div>
  </div>

  {#if deployments.length > 0}
    <div class={['data-table relative w-full transition-opacity', loading && 'opacity-90']}>
      <div class="deployment-table-scroll">
        <div class="data-table-header deployment-table-grid rounded-none!">
          <span>Status</span>
          <span>Source</span>
          <span>Commit</span>
          <span>Started</span>
          <span>Duration</span>
          <span>Server</span>
        </div>
        {#each deployments as d (d.id)}
          {@const status = deploymentStatus(d)}
          {@const link = deploymentHref(d)}
          {@const commitUrl = d.commit_sha ? commitLink(application.git_url, d.commit_sha) : null}
          {@const message = d.commit_message.split('\n')[0]}
          <div
            class={[
              'data-table-row deployment-table-grid border-b border-neutral-200 text-[13px] text-neutral-600 dark:border-white/[0.08] dark:text-fg-dim',
              selected === d.id && 'data-table-row-active',
            ]}
            data-testid="deployment-row"
          >
            <a href={link}><StatusBadge status={status.label} type={status.type} /></a>
            <a href={link}>{deploymentSource(d)}</a>
            <span class="min-w-0">
              {#if d.commit_sha}
                <span class="flex min-w-0 items-center gap-2">
                  {#if commitUrl}
                    <a
                      href={commitUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      class="shrink-0 font-mono text-xs text-neutral-950 underline decoration-neutral-400 underline-offset-2 dark:text-fg dark:decoration-neutral-600"
                      >{d.commit_sha.slice(0, 7)}</a
                    >
                  {:else}
                    <span class="shrink-0 font-mono text-xs text-neutral-950 dark:text-fg">{d.commit_sha.slice(0, 7)}</span>
                  {/if}
                  {#if message}
                    <a href={link} class="truncate text-neutral-500 dark:text-fg-faint" title={message}>{message}</a>
                  {/if}
                </span>
              {:else if d.source_image}
                <!-- An image Deployment has no commit: the digest it pulled stands in. -->
                <a href={link} class="block truncate font-mono text-xs text-neutral-500 dark:text-fg-faint" title={d.source_image}
                  >{d.source_image.split('/').pop()}</a
                >
              {:else}
                <span class="text-neutral-400 dark:text-fg-faint">-</span>
              {/if}
            </span>
            <a href={link} title={when.format(new Date(d.created_at))}>{ago(d.created_at)}</a>
            <a href={link} class="tabular-nums">{durationOf(d)}</a>
            <a href={link} class="truncate">{serverName(d.server_id)}</a>
          </div>
        {/each}
      </div>

      <footer
        class="flex min-h-11 items-center justify-between border-t border-neutral-200 px-4 text-[11px] text-neutral-500 dark:border-white/[0.08] dark:text-fg-faint"
      >
        <div class="flex items-center gap-3">
          <span class="inline-flex h-7 items-center whitespace-nowrap tabular-nums">{firstRow}–{lastRow} of {count}</span>
          {#if !embedded}
            <PageSizeSelect bind:value={take} storageKey="bakery.page-size.deployments" onchange={() => (skip = 0)} />
          {/if}
        </div>
        <div class="flex items-center gap-1">
          <button
            type="button"
            class="flex size-7 items-center justify-center rounded-md border border-neutral-200 text-neutral-500 transition-colors hover:bg-neutral-100 hover:text-black disabled:pointer-events-none disabled:opacity-35 dark:border-white/[0.08] dark:text-fg-dim dark:hover:bg-white/[0.06] dark:hover:text-fg"
            aria-label="Previous page"
            disabled={currentPage <= 1}
            onclick={() => (skip = Math.max(0, skip - take))}
          >
            <Icon name="arrow-right" class="size-3.5 rotate-180" />
          </button>
          <button
            type="button"
            class="flex size-7 items-center justify-center rounded-md border border-neutral-200 text-neutral-500 transition-colors hover:bg-neutral-100 hover:text-black disabled:pointer-events-none disabled:opacity-35 dark:border-white/[0.08] dark:text-fg-dim dark:hover:bg-white/[0.06] dark:hover:text-fg"
            aria-label="Next page"
            disabled={currentPage >= lastPage}
            onclick={() => (skip = skip + take)}
          >
            <Icon name="arrow-right" class="size-3.5" />
          </button>
        </div>
      </footer>
    </div>
  {:else if loaded}
    <Empty
      size="sm"
      title="No deployments found"
      description={hasQuery ? 'No deployments match the current search and filters.' : 'Deploy the application to create its first deployment record.'}
      icon="layers"
    />
  {/if}
</SettingsSection>
</div>
