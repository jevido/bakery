<script lang="ts">
  // Paperclip's Issues page in its streamlined presentation (ui/src/pages/
  // Issues.tsx over components/IssuesList.tsx, IssueRow.tsx,
  // IssueGroupHeader.tsx and IssueFiltersPopover.tsx; MIT, see NOTICE): New
  // Issue, search and Filter in the collection toolbar, then the rows grouped
  // by status in Paperclip's order, each group collapsible with its count
  // and a "+" for a new Issue in it, and pages of 100 under "Load more".
  // Status, priority and search ask the server; Assignee and Project filter
  // the loaded rows, as Paperclip's do. The filters live in the hash query
  // (#/issues?status=todo&q=…), so a reload keeps them. Left out: the board
  // view, nesting, columns and live runs, which wait for agents.
  import { ChevronRight, OctagonAlert, Plus } from '@lucide/svelte'
  import { untrack } from 'svelte'
  import { Button as UiButton } from '@bakery/ui/components/ui/button'
  import { api } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import CollectionToolbar from '../../lib/CollectionToolbar.svelte'
  import FilterPopover from '../../lib/FilterPopover.svelte'
  import { ago } from '../../lib/format'
  import Identity from '../../lib/Identity.svelte'
  import NewIssueDialog from '../../lib/NewIssueDialog.svelte'
  import OptionPopover from '../../lib/OptionPopover.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import PriorityIcon from '../../lib/PriorityIcon.svelte'
  import { href } from '../../lib/router.svelte'
  import SearchField from '../../lib/SearchField.svelte'
  import { session, type Member } from '../../lib/session.svelte'
  import StatusIcon from '../../lib/StatusIcon.svelte'
  import Button from '../../lib/ui/Button.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import { issueStatuses, listIssues, priorities, updateIssue, workLabel, type Issue, type IssueStatus } from '../../lib/work'

  const pageSize = 100
  type Grouping = 'status' | 'priority' | 'none'

  // The filters as the hash query holds them, read again when a link or Back
  // changes the hash under the open page (the page's own changes fire no
  // hashchange).
  type Filters = { search: string; selected: string[]; grouping: Grouping }
  function fromHash(): Filters {
    const query = new URLSearchParams(location.hash.split('?')[1] ?? '')
    const listed = (key: string) => (query.get(key) ?? '').split(',').filter(Boolean)
    return {
      search: (query.get('q') ?? '').trim(),
      selected: ['status', 'priority', 'assignee', 'project'].flatMap((kind) => listed(kind).map((v) => `${kind}:${v}`)),
      grouping: (['status', 'priority', 'none'] as const).find((g) => g === query.get('group')) ?? 'status',
    }
  }
  const initial = fromHash()
  let searchText = $state(initial.search)
  let search = $state(initial.search)
  let selected = $state<string[]>(initial.selected)
  let grouping = $state<Grouping>(initial.grouping)
  $effect(() => {
    const follow = () => {
      if (!location.hash.startsWith('#/issues')) return
      const f = fromHash()
      searchText = search = f.search
      selected = f.selected
      grouping = f.grouping
    }
    window.addEventListener('hashchange', follow)
    return () => window.removeEventListener('hashchange', follow)
  })
  let collapsed = $state<string[]>([])

  let issues = $state.raw<Issue[] | null>(null)
  let more = $state(false)
  let loadingMore = $state(false)
  let loadError = $state('')
  let members = $state.raw<Member[]>([])
  let projects = $state.raw<{ id: number; name: string }[]>([])
  let creating = $state(false)
  let createStatus = $state<IssueStatus>('todo')

  const editable = $derived(session.can('manage_work'))
  const picked = (kind: string) => selected.filter((v) => v.startsWith(`${kind}:`)).map((v) => v.slice(kind.length + 1))
  const statuses = $derived(picked('status'))
  const priorityKeys = $derived(picked('priority'))
  const assignees = $derived(picked('assignee'))
  const projectKeys = $derived(picked('project'))

  api<{ members: Member[] }>('GET', '/members').then((r) => (members = r.members)).catch(() => {})
  api<{ projects: { id: number; name: string }[] }>('GET', '/projects').then((r) => (projects = r.projects)).catch(() => {})

  // The search asks 150 ms after the last key, as the other list pages do.
  $effect(() => {
    const value = searchText
    const t = setTimeout(() => (search = value.trim()), 150)
    return () => clearTimeout(t)
  })

  // Each change of what the server filters asks again from the first page;
  // a late answer for filters already left behind is dropped.
  let asked = 0
  function load(offset = 0) {
    const ask = ++asked
    return listIssues({ status: statuses, priority: priorityKeys, q: search, limit: pageSize, offset })
      .then((page) => {
        if (ask !== asked) return
        const seen = new Set(offset ? (issues ?? []).map((i) => i.id) : [])
        issues = [...(offset ? (issues ?? []) : []), ...page.filter((i) => !seen.has(i.id))]
        more = page.length >= pageSize
        loadError = ''
      })
      .catch((e) => {
        if (ask === asked) loadError = e.message
      })
  }
  // Compared as a string, so picking an Assignee or Project asks nothing.
  const asking = $derived(JSON.stringify([statuses, priorityKeys, search]))
  $effect(() => {
    void asking
    untrack(() => load())
  })

  function loadMore() {
    loadingMore = true
    load(issues?.length ?? 0).finally(() => (loadingMore = false))
  }

  // The filters into the hash query, in place: no history entry per key.
  $effect(() => {
    const q = new URLSearchParams()
    for (const [key, values] of [
      ['status', statuses],
      ['priority', priorityKeys],
      ['assignee', assignees],
      ['project', projectKeys],
    ] as const)
      if (values.length) q.set(key, values.join(','))
    if (search) q.set('q', search)
    if (grouping !== 'status') q.set('group', grouping)
    const next = `#/issues${q.size ? `?${q}` : ''}`
    if (location.hash !== next) history.replaceState(history.state, '', next)
  })

  const me = $derived(session.member?.id)
  const shown = $derived(
    (issues ?? []).filter((i) => {
      if (assignees.length) {
        const key = i.assignee ? String(i.assignee.id) : 'none'
        if (!assignees.includes(key) && !(assignees.includes('me') && i.assignee?.id === me)) return false
      }
      if (projectKeys.length && !projectKeys.includes(i.project ? String(i.project.id) : 'none')) return false
      return true
    }),
  )

  const groups = $derived.by(() => {
    if (grouping === 'none') return [{ key: '__all', label: null as string | null, status: undefined as IssueStatus | undefined, items: shown }]
    const order: readonly string[] = grouping === 'status' ? issueStatuses : priorities
    return order
      .map((key) => ({
        key,
        label: workLabel(key),
        status: grouping === 'status' ? (key as IssueStatus) : undefined,
        items: shown.filter((i) => (grouping === 'status' ? i.status : i.priority) === key),
      }))
      .filter((g) => g.items.length > 0)
  })

  const filterGroups = $derived([
    { label: 'Status', options: issueStatuses.map((s) => ({ value: `status:${s}`, label: workLabel(s) })) },
    { label: 'Priority', options: priorities.map((p) => ({ value: `priority:${p}`, label: workLabel(p) })) },
    {
      label: 'Assignee',
      options: [
        { value: 'assignee:me', label: 'Me' },
        { value: 'assignee:none', label: 'No assignee' },
        ...members.filter((m) => m.id !== me).map((m) => ({ value: `assignee:${m.id}`, label: m.name })),
      ],
    },
    { label: 'Project', options: [{ value: 'project:none', label: 'No project' }, ...projects.map((p) => ({ value: `project:${p.id}`, label: p.name }))] },
  ])

  async function setStatus(issue: Issue, status: IssueStatus) {
    try {
      const changed = await updateIssue(issue.id, { status })
      issues = (issues ?? []).map((i) => (i.id === issue.id ? { ...i, ...changed } : i))
    } catch (e) {
      toast.error('Could not change the status', (e as Error).message)
    }
  }

  function newIssue(status: IssueStatus = 'todo') {
    createStatus = status
    creating = true
  }

  $effect(() => breadcrumb.set({ label: 'Issues' }))
</script>

<div class="chrome space-y-4">
  <CollectionToolbar ariaLabel="Issue controls">
    {#snippet context()}
      {#if editable}
        <UiButton variant="outline" size="sm" aria-label="New Issue" onclick={() => newIssue()}><Plus class="size-4" /><span class="hidden sm:inline">New Issue</span></UiButton>
      {/if}
    {/snippet}
    {#snippet search()}
      <SearchField bind:value={searchText} label="Search issues..." />
    {/snippet}
    {#snippet controls()}
      <FilterPopover bind:selected groups={filterGroups} label="Filter issues" />
      <OptionPopover
        label="Group"
        align="end"
        value={grouping}
        options={[
          { value: 'status', label: 'Status' },
          { value: 'priority', label: 'Priority' },
          { value: 'none', label: 'None' },
        ]}
        onpick={(v) => (grouping = v)}
      >
        <span class="inline-flex h-8 items-center rounded-md px-3 text-xs hover:bg-accent">Group: {grouping === 'none' ? 'None' : workLabel(grouping)}</span>
      </OptionPopover>
    {/snippet}
  </CollectionToolbar>

  {#if loadError}<p class="text-sm text-destructive">{loadError}</p>{/if}
  {#if issues === null && !loadError}
    <PageSkeleton />
  {:else if issues && issues.length === 0 && !search && selected.length === 0}
    <Empty title="No issues yet." icon="issues">
      {#if editable}
        <Button variant="highlighted" onclick={() => newIssue()}><Plus class="size-4" />New Issue</Button>
      {/if}
    </Empty>
  {:else if shown.length === 0}
    <p class="py-10 text-center text-sm text-muted-foreground">No issues match the current filters or search.</p>
  {:else}
    <div class="-mx-2 sm:mx-0">
      {#each groups as group (group.key)}
        {@const open = !collapsed.includes(group.key)}
        {#if group.label}
          <div class="flex items-center rounded-lg py-1.5 pr-3 pl-1 hover:bg-accent/50 sm:pr-4" data-issue-group={group.key}>
            <button
              type="button"
              class="flex min-w-0 items-center gap-2 text-left"
              aria-expanded={open}
              onclick={() => (collapsed = open ? [...collapsed, group.key] : collapsed.filter((k) => k !== group.key))}
            >
              <span class="inline-flex w-4 shrink-0 items-center justify-center">
                <ChevronRight class={['size-3.5 shrink-0 text-muted-foreground transition-transform', open && 'rotate-90']} />
              </span>
              <span class="truncate text-sm font-semibold tracking-wide uppercase">{group.label}</span>
              <span class="text-xs text-muted-foreground tabular-nums" data-count>{group.items.length}</span>
            </button>
            {#if editable}
              <button
                type="button"
                class="-mr-2 ml-auto inline-flex size-6 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                title="New issue in {group.label}"
                aria-label="New issue in {group.label}"
                onclick={() => newIssue(group.status)}
              >
                <Plus class="size-3" />
              </button>
            {/if}
          </div>
        {/if}
        {#if open}
          {#each group.items as issue (issue.id)}
            <div
              data-slot="task-row"
              data-issue={issue.identifier}
              class="group relative flex min-w-0 items-start gap-2 rounded-lg py-2.5 pr-2 pl-2 text-sm hover:bg-accent/50 sm:items-center sm:py-2 sm:pl-4 [&_button]:relative [&_button]:z-10"
            >
              <a
                href={href(`/issues/${issue.identifier}`)}
                class="absolute inset-0 rounded-lg text-inherit no-underline focus-visible:z-10 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
              >
                <span class="sr-only">Open {issue.identifier}: {issue.title}</span>
              </a>
              <span class="flex shrink-0 items-center gap-1 pt-px sm:pt-0">
                <span class="size-4 shrink-0" aria-hidden="true"></span>
                <StatusIcon status={issue.status} onchange={editable ? (s) => setStatus(issue, s) : undefined} />
                <PriorityIcon priority={issue.priority} class="ml-1" />
              </span>
              <span class="flex min-w-0 flex-1 items-center gap-2">
                <span class={['min-w-0 truncate', (issue.status === 'done' || issue.status === 'cancelled') && 'text-muted-foreground']}>{issue.title}</span>
                {#if issue.unresolved_blockers}
                  {@const label = `Blocked by ${issue.unresolved_blockers} ${issue.unresolved_blockers === 1 ? 'issue' : 'issues'}`}
                  <span class="relative z-10 shrink-0 text-amber-600 dark:text-amber-300" title={label} aria-label={label} role="img" data-blocked-marker>
                    <OctagonAlert class="size-3.5" />
                  </span>
                {/if}
              </span>
              <span class="ml-auto hidden min-w-0 shrink-0 items-center gap-3 sm:flex">
                {#if issue.project}
                  <span class="max-w-40 truncate text-xs text-muted-foreground" data-project>{issue.project.name}</span>
                {/if}
                <span class="flex w-36 justify-end">
                  {#if issue.assignee}
                    <Identity name={issue.assignee.name} size="sm" />
                  {/if}
                </span>
                <span class="w-20 shrink-0 text-right font-mono text-xs text-muted-foreground">{issue.identifier}</span>
                <span class="w-24 shrink-0 truncate text-right text-xs text-muted-foreground">{ago(issue.updated_at)}</span>
              </span>
            </div>
          {/each}
        {/if}
      {/each}
    </div>
    {#if more && !search}
      <div class="flex justify-center">
        <UiButton variant="outline" size="sm" disabled={loadingMore} onclick={loadMore}>{loadingMore ? 'Loading…' : 'Load more'}</UiButton>
      </div>
    {/if}
  {/if}
</div>

<NewIssueDialog bind:open={creating} status={createStatus} oncreated={() => load()} />
