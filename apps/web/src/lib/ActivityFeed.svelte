<script lang="ts" module>
  import type { ActivityEvent } from './work'

  type Tier = 1 | 2 | 3

  // Paperclip's ACTION_TIER (ui/src/components/ActivityFeed.tsx; MIT, see
  // NOTICE) on The Bakery's Actions: 1 is a card, 2 a muted one-liner, 3 is
  // hidden unless "Show all activity". Its agent.created is agent.hired, its
  // issue.work_product_created the pull request and preview Actions, its
  // heartbeat.invoked and heartbeat.cancelled run.started and run.finished;
  // anything not listed is 3, as there.
  const actionTier: Record<string, Tier> = {
    'issue.created': 1,
    'issue.document_created': 1,
    'issue.pull_request_opened': 1,
    'issue.preview_ready': 1,
    'approval.created': 1,
    'approval.approved': 1,
    'approval.rejected': 1,
    'approval.revision_requested': 1,
    'agent.hired': 1,
    'issue.updated': 2,
    'issue.checked_out': 2,
    'issue.comment_added': 2,
    'issue.document_updated': 2,
    'issue.pull_request_merged': 2,
    'issue.pull_request_closed': 2,
    'issue.preview_failed': 2,
    'run.started': 2,
    'run.finished': 2,
    'agent.paused': 2,
    'agent.resumed': 2,
    'agent.updated': 2,
    'agent.terminated': 2,
    'goal.created': 2,
  }

  type Filter = 'all' | 'in-progress' | 'for-review' | 'completed'
  const filterOptions: { value: Filter; label: string }[] = [
    { value: 'all', label: 'All' },
    { value: 'in-progress', label: 'In Progress' },
    { value: 'for-review', label: 'In Review' },
    { value: 'completed', label: 'Done' },
  ]
  // Paperclip's FILTER_ACTIONS and STATUS_FILTER_MAP: a filter keeps these
  // Actions, and issue.updated events that moved an Issue to these statuses.
  const filterActions: Record<Filter, string[]> = {
    all: [],
    'in-progress': ['issue.created', 'issue.checked_out', 'run.started'],
    'for-review': ['approval.created', 'issue.document_created', 'issue.document_updated', 'issue.pull_request_opened'],
    completed: ['approval.approved', 'issue.pull_request_merged'],
  }
  const filterStatuses: Record<Filter, string[]> = { all: [], 'in-progress': ['in_progress'], 'for-review': ['in_review'], completed: ['done'] }

  // Paperclip's collapseSequential: three or more one-liners about the same
  // entity within five minutes of each other fold into "made N updates to".
  type Collapsed = { collapsed: true; key: string; events: ActivityEvent[]; latest: ActivityEvent }
  export type FeedItem = ActivityEvent | Collapsed
  const collapseWindow = 5 * 60 * 1000
</script>

<script lang="ts">
  // Paperclip's ActivityFeed and FeedCard (ui/src/components/ActivityFeed.tsx
  // and FeedCard.tsx; MIT, see NOTICE), beside the Conference Room: the
  // Guild's Activity, newest first, in three tiers, with Paperclip's filter
  // menu, its group-by-Issue toggle and the "Earlier" rule past the last five
  // minutes. It reads GET /api/activity as the Activity page does, a page at a
  // time under "Load more", and asks for the newest page again every 10
  // seconds while the tab is visible (Paperclip's useVisibilityRefetchInterval),
  // merging it in front. Left out: Paperclip's scroll-to-load (a button here,
  // as on the Activity page) and its live heartbeat spinner.
  import {
    ChevronDown,
    ChevronRight,
    CircleAlert,
    CircleCheck,
    CircleSlash,
    FileText,
    GitPullRequest,
    Layers,
    ListFilter,
    LoaderCircle,
    LogIn,
    MessageCircle,
    CirclePause,
    CirclePlay,
    PencilLine,
    Settings,
    User,
    UserPlus,
  } from '@lucide/svelte'
  import type { Component } from 'svelte'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import * as DropdownMenu from '@bakery/ui/components/ui/dropdown-menu'
  import { ago } from './format'
  import { href } from './router.svelte'
  import StatusIcon from './StatusIcon.svelte'
  import { activityEventVerb, activityPath, activityStatusTo, listActivity, workLabel, type IssueStatus } from './work'

  let { class: className = '' }: { class?: string } = $props()

  const pageSize = 50

  let events = $state.raw<ActivityEvent[] | null>(null)
  let more = $state(false)
  let loadingMore = $state(false)
  let loadError = $state('')
  let filter = $state<Filter>('all')
  let showAll = $state(false)
  let byIssue = $state(false)
  let open = $state<Record<string, boolean>>({})
  // Ids already shown, so only events that arrive by polling slide in.
  let seen = new Set<number>()

  // The newest page, merged in front of what is loaded: a poll never drops
  // the older pages "Load more" brought in.
  function refresh() {
    return listActivity({ limit: pageSize })
      .then((page) => {
        const oldest = page.at(-1)?.id ?? Infinity
        const kept = (events ?? []).filter((e) => e.id < oldest)
        if (events === null) {
          for (const e of page) seen.add(e.id)
          more = page.length >= pageSize
        }
        events = [...page, ...kept]
        loadError = ''
      })
      .catch((e) => (loadError = e.message))
  }
  function loadMore() {
    loadingMore = true
    listActivity({ before: events?.at(-1)?.id, limit: pageSize })
      .then((page) => {
        for (const e of page) seen.add(e.id)
        events = [...(events ?? []), ...page]
        more = page.length >= pageSize
      })
      .catch((e) => (loadError = e.message))
      .finally(() => (loadingMore = false))
  }
  $effect(() => {
    refresh()
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') refresh()
    }, 10_000)
    const back = () => document.visibilityState === 'visible' && refresh()
    document.addEventListener('visibilitychange', back)
    return () => {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', back)
    }
  })

  function tier(e: ActivityEvent): Tier {
    if (activityStatusTo(e) === 'in_review') return 1
    return actionTier[e.action] ?? 3
  }
  function matches(e: ActivityEvent): boolean {
    if (filter === 'all') return true
    if (filterActions[filter].includes(e.action)) return true
    const to = activityStatusTo(e)
    return to !== null && filterStatuses[filter].includes(to)
  }
  const entityKey = (e: ActivityEvent) => `${e.entity.type}:${e.entity.id}`

  function collapse(list: ActivityEvent[]): FeedItem[] {
    const out: FeedItem[] = []
    let group: ActivityEvent[] = []
    const flush = () => {
      if (group.length <= 2) out.push(...group)
      else out.push({ collapsed: true, key: `${entityKey(group[0])}:${group[0].id}`, events: group, latest: group[0] })
      group = []
    }
    for (const e of list) {
      const last = group.at(-1)
      const near = last && Math.abs(Date.parse(last.created_at) - Date.parse(e.created_at)) <= collapseWindow
      if (last && entityKey(last) === entityKey(e) && near && tier(e) >= 2) group.push(e)
      else {
        flush()
        // Unlike Paperclip's, a card never heads a group either: it would
        // fold the card under "made N updates to".
        if (tier(e) === 1) out.push(e)
        else group = [e]
      }
    }
    flush()
    return out
  }

  const items = $derived(collapse((events ?? []).filter((e) => (showAll || tier(e) < 3) && matches(e))))

  // Paperclip's by-task mode: every item of one Issue under its header, in
  // the order the Issues first appear; everything else under "Other activity".
  const groups = $derived.by(() => {
    const map = new Map<string, { label: string; items: FeedItem[] }>()
    for (const item of items) {
      const e = 'collapsed' in item ? item.latest : item
      const key = e.entity.type === 'issue' ? `issue:${e.entity.id}` : 'other'
      const label = key === 'other' ? 'Other activity' : `${e.entity.identifier ?? e.entity.id} — ${e.entity.title}`
      const group = map.get(key) ?? { label, items: [] }
      group.items.push(item)
      map.set(key, group)
    }
    return [...map.entries()].map(([key, g]) => ({ key, ...g }))
  })

  const recent = (at: string) => Date.now() - Date.parse(at) < collapseWindow
  const createdAt = (item: FeedItem) => ('collapsed' in item ? item.latest.created_at : item.created_at)
  const earlierBefore = (list: FeedItem[], i: number) => i > 0 && recent(createdAt(list[i - 1])) && !recent(createdAt(list[i]))
  function fresh(id: number) {
    if (seen.has(id)) return false
    seen.add(id)
    return true
  }

  // Paperclip's formatVerb where it says more than the Activity page's verb.
  function verb(e: ActivityEvent): string {
    const to = activityStatusTo(e)
    if (to) return `moved to ${workLabel(to).toLowerCase()}`
    if (e.action === 'issue.checked_out') return 'picked up'
    if (e.action === 'issue.document_created') return 'wrote doc on'
    if (e.action === 'issue.document_updated') return 'edited doc on'
    if (e.action === 'approval.created') return 'requested approval on'
    if (e.action === 'approval.revision_requested') return 'requested changes on'
    return activityEventVerb(e)
  }

  type Glyph = { icon: Component<{ class?: string }>; color: string } | { status: IssueStatus }
  // Paperclip's getIconSpec and deriveTaskStatus: approvals, agents,
  // documents, pull requests, comments and check-outs have their own icon;
  // any other Issue event shows the status it left the Issue in.
  function glyph(e: ActivityEvent): Glyph {
    switch (e.action) {
      case 'approval.created':
        return { icon: CircleAlert, color: 'text-amber-600 dark:text-amber-400' }
      case 'approval.approved':
        return { icon: CircleCheck, color: 'text-green-600 dark:text-green-400' }
      case 'approval.rejected':
        return { icon: CircleSlash, color: 'text-red-600 dark:text-red-400' }
      case 'approval.revision_requested':
        return { icon: PencilLine, color: 'text-amber-600 dark:text-amber-400' }
      case 'agent.hired':
        return { icon: UserPlus, color: 'text-purple-600 dark:text-purple-400' }
      case 'agent.paused':
        return { icon: CirclePause, color: 'text-muted-foreground' }
      case 'agent.resumed':
        return { icon: CirclePlay, color: 'text-muted-foreground' }
      case 'agent.updated':
      case 'agent.terminated':
        return { icon: Settings, color: 'text-muted-foreground' }
      case 'run.started':
      case 'run.finished':
        return { icon: LoaderCircle, color: 'text-muted-foreground' }
      case 'issue.document_created':
      case 'issue.document_updated':
        return { icon: FileText, color: 'text-blue-600 dark:text-blue-400' }
      case 'issue.pull_request_opened':
      case 'issue.pull_request_merged':
      case 'issue.pull_request_closed':
      case 'issue.preview_ready':
      case 'issue.preview_failed':
        return { icon: GitPullRequest, color: 'text-indigo-600 dark:text-indigo-400' }
      case 'issue.comment_added':
        return { icon: MessageCircle, color: 'text-muted-foreground' }
      case 'issue.checked_out':
        return { icon: LogIn, color: 'text-muted-foreground' }
      case 'issue.created':
        return { status: 'todo' }
    }
    if (e.entity.type === 'issue') return { status: activityStatusTo(e) ?? 'backlog' }
    return { icon: Settings, color: 'text-muted-foreground' }
  }

  const actorName = (e: ActivityEvent) => e.actor_agent?.name ?? e.actor?.name ?? 'Board'
  const cardClass =
    'group my-2 flex w-full items-center gap-2 rounded-lg border border-border bg-card px-4.5 py-3 text-left text-xs text-inherit no-underline transition-colors duration-150'
</script>

{#snippet actorGlyph(e: ActivityEvent)}
  {#if e.actor_agent}
    <AgentIcon icon={e.actor_agent.icon} class="size-3.5 shrink-0 text-muted-foreground" />
  {:else if e.actor}
    <User class="size-3.5 shrink-0 text-muted-foreground" />
  {:else}
    <Settings class="size-3.5 shrink-0 text-muted-foreground" />
  {/if}
{/snippet}

{#snippet card(e: ActivityEvent, muted: boolean)}
  {@const path = activityPath(e)}
  {@const g = glyph(e)}
  {@const text = muted ? 'text-muted-foreground/70' : 'text-muted-foreground group-hover:text-foreground'}
  <svelte:element
    this={path ? 'a' : 'div'}
    href={path ? href(path) : undefined}
    class={[cardClass, path && 'cursor-pointer hover:border-muted-foreground/30 hover:bg-accent']}
    data-feed-card={e.action}
    data-tier={tier(e)}
  >
    {#if 'status' in g}
      <StatusIcon status={g.status} size="sm" />
    {:else}
      <g.icon class="size-3.5 shrink-0 {g.color}" />
    {/if}
    {@render actorGlyph(e)}
    <span class="flex min-w-0 flex-1 items-baseline gap-1 overflow-hidden whitespace-nowrap">
      <span class="max-w-1/2 shrink-0 truncate font-medium {text}">{actorName(e)}</span>
      <span class="min-w-0 truncate {text}">{verb(e)}</span>
      {#if e.entity.type === 'issue' && e.entity.identifier}<span class="shrink-0 font-mono {text}">{e.entity.identifier}</span>{/if}
      <span class="min-w-0 shrink-[4] truncate {text}">{e.entity.title}</span>
    </span>
    <span class="shrink-0 text-muted-foreground">{ago(e.created_at)}</span>
  </svelte:element>
{/snippet}

{#snippet item(it: FeedItem, list: FeedItem[], i: number)}
  {#if earlierBefore(list, i)}
    <div class="flex items-center gap-2 py-1.5" data-testid="feed-earlier">
      <div class="h-px flex-1 bg-border"></div>
      <span class="text-(length:--text-nano) font-medium tracking-wider text-muted-foreground uppercase">Earlier</span>
      <div class="h-px flex-1 bg-border"></div>
    </div>
  {/if}
  {#if 'collapsed' in it}
    <div class={[fresh(it.latest.id) && 'feed-item-new']}>
      <button type="button" class="{cardClass} cursor-pointer hover:border-muted-foreground/30 hover:bg-accent" aria-expanded={!!open[it.key]} onclick={() => (open[it.key] = !open[it.key])} data-feed-collapsed>
        {#if open[it.key]}<ChevronDown class="size-3 shrink-0 text-muted-foreground" />{:else}<ChevronRight class="size-3 shrink-0 text-muted-foreground" />{/if}
        {@render actorGlyph(it.latest)}
        <span class="min-w-0 flex-1 truncate text-muted-foreground">
          <span class="font-medium group-hover:text-foreground">{actorName(it.latest)}</span>
          made {it.events.length} updates to
          <span class="group-hover:text-foreground">{it.latest.entity.identifier ?? it.latest.entity.title}</span>
        </span>
        <span class="shrink-0 text-muted-foreground">{ago(it.latest.created_at)}</span>
      </button>
      {#if open[it.key]}
        <div class="ml-5">
          {#each it.events as e (e.id)}{@render card(e, true)}{/each}
        </div>
      {/if}
    </div>
  {:else}
    <div class={[fresh(it.id) && 'feed-item-new']}>{@render card(it, tier(it) !== 1)}</div>
  {/if}
{/snippet}

<aside class="flex min-h-0 min-w-0 flex-1 flex-col bg-background {className}" aria-label="Activity feed" data-testid="activity-feed">
  <div class="chrome relative flex shrink-0 items-start justify-between gap-2 px-4 py-3">
    <div class="pointer-events-none absolute right-0 bottom-0 left-0 h-px bg-border md:-left-3" aria-hidden="true"></div>
    <div class="min-w-0 flex-1">
      <h3 class="text-sm font-semibold">Activity</h3>
      <p class="text-xs text-muted-foreground">What the guild is doing, live</p>
    </div>
    <div class="flex items-center gap-1">
      <Button
        variant={byIssue ? 'secondary' : 'ghost'}
        size="icon-sm"
        class="shrink-0 text-muted-foreground"
        aria-label="group by issue"
        aria-pressed={byIssue}
        title={byIssue ? 'Show flat' : 'Group by issue'}
        onclick={() => (byIssue = !byIssue)}><Layers class="size-3.5" /></Button
      >
      <DropdownMenu.Root>
        <DropdownMenu.Trigger>
          {#snippet child({ props })}
            <Button {...props} variant={filter !== 'all' || showAll ? 'secondary' : 'ghost'} size="icon-sm" class="shrink-0 text-muted-foreground" aria-label="filter by" title="Filter by"
              ><ListFilter class="size-3.5" /></Button
            >
          {/snippet}
        </DropdownMenu.Trigger>
        <DropdownMenu.Content align="end" class="w-48">
          <DropdownMenu.RadioGroup bind:value={() => filter, (v) => (filter = v as Filter)}>
            {#each filterOptions as o (o.value)}
              <DropdownMenu.RadioItem value={o.value}>{o.label}</DropdownMenu.RadioItem>
            {/each}
          </DropdownMenu.RadioGroup>
          <DropdownMenu.Separator />
          <DropdownMenu.CheckboxItem bind:checked={showAll}>Show all activity</DropdownMenu.CheckboxItem>
        </DropdownMenu.Content>
      </DropdownMenu.Root>
    </div>
  </div>

  <div class="scrollbar-auto-hide min-h-0 flex-1 overflow-y-auto">
    {#if loadError && events === null}
      <p class="p-4 text-sm text-destructive">{loadError}</p>
    {:else if events === null}
      <p class="p-6 text-center text-sm text-muted-foreground">Loading…</p>
    {:else if items.length === 0}
      <div class="flex h-full items-center justify-center p-6">
        <p class="text-center text-sm text-muted-foreground" data-testid="feed-empty">{events.length === 0 ? 'No activity yet.' : 'No activity matches this filter.'}</p>
      </div>
    {:else if byIssue}
      {#each groups as g (g.key)}
        <section class="mb-2" data-feed-group={g.key}>
          <button
            type="button"
            class="sticky top-0 z-10 flex w-full items-center gap-1.5 rounded-none border-0 border-b border-border bg-background/95 px-4 py-1.5 text-left backdrop-blur-sm"
            aria-expanded={!open[`group:${g.key}`]}
            onclick={() => (open[`group:${g.key}`] = !open[`group:${g.key}`])}
          >
            {#if open[`group:${g.key}`]}<ChevronRight class="size-3 shrink-0 text-muted-foreground" />{:else}<ChevronDown class="size-3 shrink-0 text-muted-foreground" />{/if}
            <span class="truncate text-xs font-medium text-muted-foreground">{g.label}</span>
            <span class="ml-auto shrink-0 text-xs text-muted-foreground">{g.items.length}</span>
          </button>
          {#if !open[`group:${g.key}`]}
            <div class="px-4">
              {#each g.items as it, i ('collapsed' in it ? it.key : it.id)}{@render item(it, g.items, i)}{/each}
            </div>
          {/if}
        </section>
      {/each}
    {:else}
      <div class="px-4 py-2">
        {#each items as it, i ('collapsed' in it ? it.key : it.id)}{@render item(it, items, i)}{/each}
      </div>
    {/if}
    {#if more && events}
      <div class="flex justify-center pb-4">
        <Button variant="outline" size="sm" disabled={loadingMore} onclick={loadMore}>{loadingMore ? 'Loading…' : 'Load more'}</Button>
      </div>
    {/if}
  </div>
</aside>

<style>
  @keyframes feed-slide-in {
    from {
      opacity: 0;
      transform: translateY(-8px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
  .feed-item-new {
    animation: feed-slide-in 300ms ease-out;
  }
</style>
