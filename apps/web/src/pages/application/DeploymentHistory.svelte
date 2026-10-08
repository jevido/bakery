<script lang="ts" module>
  import type { Deployment } from '../../lib/types'
  import { statusType, type StatusType } from '@bakery/ui/statusColors'

  const deploymentLabels: Partial<Record<Deployment['status'], string>> = {
    finished: 'Success',
    failed: 'Failed',
    queued: 'Queued',
    cancelled: 'Cancelled',
  }

  /** Coolify's status label for a Deployment, coloured from the status palette. */
  export function deploymentStatus(d: Pick<Deployment, 'status' | 'active'>): { label: string; type: StatusType } {
    return { label: deploymentLabels[d.status] ?? 'In progress', type: statusType(d.status) }
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
  //
  // Laid out as Paperclip's list pattern (ui/src/pages/ProjectDetail.tsx,
  // MIT), as the Environment page is: a CollectionToolbar, the Deployments
  // as EntityRows in one card, pagination under it.
  import { untrack } from 'svelte'
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import { Card } from '@bakery/ui/components/ui/card'
  import { api } from '../../lib/api'
  import CollectionToolbar from '../../lib/CollectionToolbar.svelte'
  import EntityRow from '@bakery/ui/EntityRow.svelte'
  import FilterPopover from '../../lib/FilterPopover.svelte'
  import { ago, duration } from '../../lib/format'
  import Icon from '../../lib/Icon.svelte'
  import { applicationPath, href } from '../../lib/router.svelte'
  import SearchField from '../../lib/SearchField.svelte'
  import SortPopover from '../../lib/SortPopover.svelte'
  import type { Application } from '../../lib/types'
  import Empty from '../../lib/ui/Empty.svelte'
  import PageSizeSelect from '../../lib/ui/PageSizeSelect.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'

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

  let searchText = $state('')
  let query = $state('')
  // Status and Source, as `status:…` and `source:…` values; the Pull request
  // filter is kept apart since only one applies at a time.
  let picked = $state<string[]>([])
  let prPicked = $state<string[]>([])
  let sort = $state<'newest' | 'oldest'>('newest')
  let page = $state(1)
  let take = $state(untrack(() => (embedded ? 3 : 10)))

  const skip = $derived((page - 1) * take)
  const lastPage = $derived(Math.max(1, Math.ceil(count / take)))
  const firstRow = $derived(count === 0 ? 0 : skip + 1)
  const lastRow = $derived(Math.min(skip + deployments.length, count))
  const pullRequest = $derived(prPicked[0] ? Number(prPicked[0]) : 0)
  const activeCount = $derived(picked.length + prPicked.length)
  const hasQuery = $derived(query !== '' || activeCount > 0)

  // The search box loads 300 ms after the last key, as Coolify's debounce does.
  $effect(() => {
    const value = searchText.trim()
    const timer = setTimeout(() => {
      if (value !== untrack(() => query)) {
        query = value
        page = 1
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
      if (r.deployments.length === 0 && page > 1 && r.count > 0) page = Math.max(1, Math.ceil(r.count / take))
    } finally {
      if (mine === sequence) loading = false
    }
  }

  $effect(() => {
    void [application.id, page, take, sort, query, picked.length, prPicked.length, reload]
    untrack(() => load().catch(() => {}))
  })

  // Coolify polls the first page every five seconds; later pages hold still.
  $effect(() => {
    if (page !== 1) return
    const t = setInterval(() => load().catch(() => {}), 5000)
    return () => clearInterval(t)
  })

  function clearFilter() {
    picked = []
    prPicked = []
    page = 1
  }

  const deploymentHref = (d: Deployment) => href(`${applicationPath(application, 'deployment')}/${d.id}`)

  function durationOf(d: Deployment): string {
    if (d.status === 'queued') return 'Waiting'
    if (d.active) return duration(d.created_at, new Date())
    return d.finished_at ? duration(d.created_at, d.finished_at) : '-'
  }

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' })
  const serverName = (id: number) => serverNames[id] ?? `Server #${id}`

  const filterGroups = $derived<{ label: string; options: { value: string; label: string }[] }[]>([
    {
      label: 'Status',
      options: Object.keys(statusLabels)
        .filter((s) => options.statuses.includes(s))
        .map((s) => ({ value: `status:${s}`, label: statusLabels[s] })),
    },
    {
      label: 'Source',
      options: Object.keys(sourceLabels)
        .filter((s) => options.sources.includes(s))
        .map((s) => ({ value: `source:${s}`, label: sourceLabels[s] })),
    },
    { label: 'Server', options: options.server_ids.map((id) => ({ value: `server:${id}`, label: serverName(id) })) },
  ])
  const pullRequestGroups = $derived([{ options: options.pull_requests.map((n) => ({ value: String(n), label: `Pull request #${n}` })) }])

  const selectedChips = $derived([
    ...filterGroups.flatMap((g) => g.options.filter((o) => picked.includes(o.value)).map((o) => ({ value: o.value, label: o.label, pr: false }))),
    ...prPicked.map((v) => ({ value: v, label: `Pull request #${v}`, pr: true })),
  ])

  function removeChip(chip: { value: string; pr: boolean }) {
    if (chip.pr) prPicked = prPicked.filter((v) => v !== chip.value)
    else picked = picked.filter((v) => v !== chip.value)
    page = 1
  }

  const listCard = 'block gap-0 overflow-hidden py-0'
</script>

<div class="chrome flex flex-col gap-3">
  <CollectionToolbar ariaLabel="Deployment history controls">
    {#snippet search()}
      <SearchField bind:value={searchText} label="Search deployments" oninput={() => (page = 1)} />
    {/snippet}
    {#snippet controls()}
      <FilterPopover bind:selected={picked} groups={filterGroups} label="Filter deployments" onchange={() => (page = 1)} />
      {#if options.pull_requests.length > 0}
        <FilterPopover
          bind:selected={prPicked}
          groups={pullRequestGroups}
          label="Filter by pull request"
          clearLabel="Clear pull request filter"
          onchange={() => {
            if (prPicked.length > 1) prPicked = [prPicked[prPicked.length - 1]]
            page = 1
          }}
        />
      {/if}
      <SortPopover
        bind:value={sort}
        options={[
          { value: 'newest', label: 'Newest first' },
          { value: 'oldest', label: 'Oldest first' },
        ]}
        label="Sort deployments"
        onchange={() => (page = 1)}
      />
    {/snippet}
    {#snippet feedback()}
      {#if selectedChips.length > 0}
        <div class="flex flex-wrap items-center gap-1.5">
          {#each selectedChips as chip (`${chip.pr}-${chip.value}`)}
            <span class="inline-flex items-center gap-1 rounded-full border border-border bg-muted/50 py-0.5 pr-1 pl-2 text-xs">
              {chip.label}
              <button
                type="button"
                class="flex size-4 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground"
                aria-label="Remove filter {chip.label}"
                onclick={() => removeChip(chip)}
              >
                <Icon name="x" class="size-2.5" />
              </button>
            </span>
          {/each}
          <button type="button" class="px-1 text-xs text-muted-foreground hover:text-foreground hover:underline" onclick={clearFilter}>
            Clear all
          </button>
        </div>
      {/if}
    {/snippet}
  </CollectionToolbar>

  {#if deployments.length > 0}
    <Card class={[listCard, loading && 'opacity-90']}>
      {#each deployments as d (d.id)}
        {@const status = deploymentStatus(d)}
        {@const commitUrl = d.commit_sha ? commitLink(application.git_url, d.commit_sha) : null}
        {@const message = d.commit_message.split('\n')[0]}
        {@const title = d.commit_sha ? d.commit_sha.slice(0, 7) : d.source_image ? (d.source_image.split('/').pop() ?? '-') : '-'}
        {@const subtitle = message ? `${message} · ${ago(d.created_at)}` : ago(d.created_at)}
        <EntityRow href={deploymentHref(d)} selected={selected === d.id} {title} {subtitle} reserveSubtitleSpace data-testid="deployment-row">
          {#snippet trailing()}
            {#if commitUrl}
              <a
                href={commitUrl}
                target="_blank"
                rel="noopener noreferrer"
                class="hidden items-center justify-center rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground lg:flex"
                title="Open commit on the git host"
                aria-label="Open commit on the git host"
              >
                <Icon name="external-link" class="size-3.5" />
              </a>
            {/if}
            <span class="hidden w-24 truncate text-right text-xs text-muted-foreground sm:inline" title={serverName(d.server_id)}
              >{serverName(d.server_id)}</span
            >
            <span class="hidden w-14 shrink-0 text-right text-xs text-muted-foreground tabular-nums sm:inline" title={when.format(new Date(d.created_at))}
              >{durationOf(d)}</span
            >
            <StatusBadge status={status.label} type={status.type} />
          {/snippet}
        </EntityRow>
      {/each}

      <footer class="flex min-h-11 items-center justify-between border-t border-border px-4 text-xs text-muted-foreground">
        <div class="flex items-center gap-3">
          <span class="inline-flex h-7 items-center whitespace-nowrap tabular-nums">{firstRow}–{lastRow} of {count}</span>
          {#if !embedded}<PageSizeSelect bind:value={take} storageKey="bakery.page-size.deployments" onchange={() => (page = 1)} />{/if}
        </div>
        <div class="flex items-center gap-1">
          <button
            type="button"
            class={buttonVariants({ variant: 'outline', size: 'icon-sm', class: 'size-7 text-muted-foreground' })}
            aria-label="Previous page"
            disabled={page <= 1}
            onclick={() => (page = Math.max(1, page - 1))}
          >
            <Icon name="arrow-right" class="size-3.5 rotate-180" />
          </button>
          <button
            type="button"
            class={buttonVariants({ variant: 'outline', size: 'icon-sm', class: 'size-7 text-muted-foreground' })}
            aria-label="Next page"
            disabled={page >= lastPage}
            onclick={() => (page = Math.min(lastPage, page + 1))}
          >
            <Icon name="arrow-right" class="size-3.5" />
          </button>
        </div>
      </footer>
    </Card>
  {:else if loaded}
    <Card class={listCard}>
      <Empty
        size="sm"
        title="No deployments found"
        description={hasQuery ? 'No deployments match the current search and filters.' : 'Deploy the application to create its first deployment record.'}
        icon="layers"
      />
    </Card>
  {/if}
</div>
