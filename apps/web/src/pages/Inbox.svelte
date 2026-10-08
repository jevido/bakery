<script lang="ts">
  // Paperclip's Inbox (ui/src/pages/Inbox.tsx, its issue rows from
  // components/IssueRow.tsx with InboxArchiveButton; MIT, see NOTICE): the
  // Issues the Member takes part in, most recently changed first, under the
  // tabs Mine (Touched, minus their Inbox archives), Recent (Touched,
  // archived ones dimmed) and Unread (Touched and unread, as Paperclip's
  // unreadTouchedIssues). The blue dot marks an unread Issue; clicking it, or
  // the empty slot on a read one, toggles the Member's Read mark. Archive
  // takes a row out of Mine with a toast to undo it. #/inbox opens the last
  // tab used. Approvals join the Issues as Paperclip's getApprovalsForTab and
  // getInboxWorkItems (ui/src/lib/inbox.ts) have them: Mine shows the
  // Actionable ones and those the Member requested or decided, Recent every
  // one, Unread the Actionable ones, merged with the Issues by when they last
  // changed. An Approval row is Paperclip's ApprovalInboxRow, with Approve and
  // Reject for someone with the approve Permission while it is Actionable.
  // Archive and the read dot are for Issues only: Paperclip keeps Approval
  // dismissals in the browser, which is left out. Also left out: the Blocked
  // and All tabs, failed runs and join requests (they wait for agents),
  // grouping, columns, nesting, keyboard navigation and swipe to archive.
  import { Archive, ArchiveRestore } from '@lucide/svelte'
  import { untrack } from 'svelte'
  import * as AlertDialog from '@bakery/ui/components/ui/alert-dialog'
  import { Button as UiButton } from '@bakery/ui/components/ui/button'
  import * as Tabs from '@bakery/ui/components/ui/tabs'
  import { ApiError } from '../lib/api'
  import { approvalLabel, approvalTypeIcon, decide, isActionable, listApprovals, type Approval } from '../lib/approvals'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import { refreshBadges } from '../lib/inbox.svelte'
  import CollectionToolbar from '../lib/CollectionToolbar.svelte'
  import { ago } from '../lib/format'
  import PageSkeleton from '../lib/PageSkeleton.svelte'
  import { go, href, inboxLastTabKey, inboxPath, inboxTabs, type InboxTab } from '../lib/router.svelte'
  import SearchField from '../lib/SearchField.svelte'
  import { session } from '../lib/session.svelte'
  import StatusIcon from '../lib/StatusIcon.svelte'
  import Empty from '../lib/ui/Empty.svelte'
  import { toast } from '../lib/ui/toast.svelte'
  import { archiveFromInbox, listIssues, markRead, markUnread, unarchiveFromInbox, type Issue, type IssueFilter } from '../lib/work'

  let { tab }: { tab: InboxTab } = $props()

  const pageSize = 200
  const labels: Record<InboxTab, string> = { mine: 'Mine', recent: 'Recent', unread: 'Unread' }
  const empty: Record<InboxTab, string> = { mine: 'Inbox zero.', recent: 'No recent inbox items.', unread: 'No new inbox items.' }
  const asks: Record<InboxTab, IssueFilter> = {
    mine: { inbox: 'me' },
    recent: { touched: 'me' },
    unread: { touched: 'me', unread: 'me' },
  }

  $effect(() => {
    try {
      localStorage.setItem(inboxLastTabKey, tab)
    } catch {
      // Private mode: #/inbox opens Mine.
    }
  })

  let searchText = $state('')
  let search = $state('')
  // The search asks 150 ms after the last key, as the other list pages do.
  $effect(() => {
    const value = searchText
    const t = setTimeout(() => (search = value.trim()), 150)
    return () => clearTimeout(t)
  })

  let issues = $state.raw<Issue[] | null>(null)
  // Every Approval in the Guild; the tab and the search pick from them here.
  let approvals = $state.raw<Approval[] | null>(null)
  let deciding = $state<{ id: number; action: 'approve' | 'reject' } | null>(null)
  const canDecide = $derived(session.can('approve'))
  let more = $state(false)
  let loadingMore = $state(false)
  let loadError = $state('')
  let confirming = $state(false)
  let marking = $state(false)

  // A change of tab or search asks again from the first page; a late answer
  // for one already left behind is dropped.
  let asked = 0
  function load(offset = 0) {
    const ask = ++asked
    return listIssues({ ...asks[tab], q: search, limit: pageSize, offset })
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
  function loadApprovals() {
    return listApprovals()
      .then((as) => (approvals = as))
      .catch((e) => (loadError = e.message))
  }
  // A new tab shows the skeleton and reads the Approvals again; a new search
  // keeps the old rows until it answers.
  let shownTab: InboxTab | null = null
  $effect(() => {
    void [tab, search]
    untrack(() => {
      if (shownTab !== tab) {
        issues = null
        loadApprovals()
      }
      shownTab = tab
      load()
    })
  })

  const mine = (a: Approval) =>
    isActionable(a) || (session.member != null && (a.requester?.id === session.member.id || a.decided_by?.id === session.member.id))
  const approvalsShown = $derived.by(() => {
    const q = search.toLowerCase()
    return (approvals ?? []).filter(
      (a) =>
        (tab === 'recent' || (tab === 'unread' ? isActionable(a) : mine(a))) &&
        (!q || approvalLabel(a.type, a.payload).toLowerCase().includes(q)),
    )
  })

  type Row = { kind: 'issue'; at: string; issue: Issue } | { kind: 'approval'; at: string; approval: Approval }
  // Issues come newest change first, a page at a time; an Approval older than
  // the last Issue loaded waits for the page that reaches it. On a tie the
  // Approval goes first, as Paperclip's.
  const rows = $derived.by((): Row[] | null => {
    if (issues === null || approvals === null) return null
    const last = more ? issues.at(-1)?.updated_at : undefined
    const as = approvalsShown.filter((a) => !last || a.updated_at >= last)
    return [
      ...issues.map((issue): Row => ({ kind: 'issue', at: issue.updated_at, issue })),
      ...as.map((approval): Row => ({ kind: 'approval', at: approval.updated_at, approval })),
    ].sort((a, b) => (a.at === b.at ? (a.kind === 'approval' ? -1 : b.kind === 'approval' ? 1 : 0) : Date.parse(b.at) - Date.parse(a.at)))
  })

  async function decideApproval(a: Approval, action: 'approve' | 'reject') {
    deciding = { id: a.id, action }
    try {
      const after = await decide(a.id, action)
      approvals = (approvals ?? []).map((x) => (x.id === a.id ? after : x))
      refreshBadges()
    } catch (e) {
      toast.error(action === 'approve' ? 'Could not approve it' : 'Could not reject it', e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
    } finally {
      deciding = null
    }
  }

  function loadMore() {
    loadingMore = true
    load(issues?.length ?? 0).finally(() => (loadingMore = false))
  }

  const unreadShown = $derived((issues ?? []).filter((i) => i.unread))

  function patch(id: number, change: Partial<Issue>) {
    issues = (issues ?? []).map((i) => (i.id === id ? { ...i, ...change } : i))
  }

  // Changes show at once and roll back with a toast when the API says no.
  async function toggleRead(issue: Issue) {
    const unread = !issue.unread
    const before = issues
    // On Unread, a row marked read leaves the tab, as Paperclip's fades out.
    if (tab === 'unread' && !unread) issues = (issues ?? []).filter((i) => i.id !== issue.id)
    else patch(issue.id, { unread })
    try {
      await (unread ? markUnread(issue.id) : markRead(issue.id))
      refreshBadges()
    } catch (e) {
      issues = before
      toast.error(unread ? 'Could not mark it unread' : 'Could not mark it read', (e as Error).message)
    }
  }

  async function archive(issue: Issue) {
    const before = issues
    issues = (issues ?? []).filter((i) => i.id !== issue.id)
    try {
      await archiveFromInbox(issue.id)
      refreshBadges()
      toast.show(`Archived ${issue.identifier}`, '', { label: 'Undo', onclick: () => unarchive(issue) })
    } catch (e) {
      issues = before
      toast.error('Could not archive it', (e as Error).message)
    }
  }

  async function unarchive(issue: Issue) {
    const back = { ...issue, archived: false }
    const before = issues
    if (tab === 'mine') {
      // Undo puts the row back where the list's order has it.
      const rest = (issues ?? []).filter((i) => i.id !== issue.id)
      const at = rest.findIndex((i) => i.updated_at < issue.updated_at)
      issues = at === -1 ? [...rest, back] : [...rest.slice(0, at), back, ...rest.slice(at)]
    } else patch(issue.id, { archived: false })
    try {
      await unarchiveFromInbox(issue.id)
      refreshBadges()
    } catch (e) {
      issues = before
      toast.error('Could not unarchive it', (e as Error).message)
    }
  }

  async function markAllRead() {
    confirming = false
    marking = true
    const ids = unreadShown.map((i) => i.id)
    const results = await Promise.allSettled(ids.map((id) => markRead(id)))
    const failed = new Set(ids.filter((_, n) => results[n].status === 'rejected'))
    const done = new Set(ids.filter((id) => !failed.has(id)))
    issues = (issues ?? []).filter((i) => tab !== 'unread' || !done.has(i.id)).map((i) => (done.has(i.id) ? { ...i, unread: false } : i))
    refreshBadges()
    if (failed.size) toast.error('Could not mark everything as read', `${failed.size} ${failed.size === 1 ? 'item' : 'items'} stayed unread.`)
    marking = false
  }

  $effect(() => breadcrumb.set({ label: 'Inbox' }))
</script>

<div class="chrome space-y-6">
  <CollectionToolbar ariaLabel="Inbox controls">
    {#snippet context()}
      <Tabs.Root value={tab} onValueChange={(v) => go(inboxPath(v as InboxTab))}>
        <Tabs.List variant="line" class="justify-start">
          {#each inboxTabs as t (t)}
            <Tabs.Trigger value={t}>{labels[t]}</Tabs.Trigger>
          {/each}
        </Tabs.List>
      </Tabs.Root>
    {/snippet}
    {#snippet search()}
      <div class="sm:ml-auto sm:w-56">
        <SearchField bind:value={searchText} label="Search inbox…" />
      </div>
    {/snippet}
    {#snippet actions()}
      {#if unreadShown.length > 0}
        <UiButton variant="outline" size="sm" class="h-8 shrink-0" disabled={marking} onclick={() => (confirming = true)}>
          {marking ? 'Marking…' : 'Mark all as read'}
        </UiButton>
      {/if}
    {/snippet}
  </CollectionToolbar>

  {#if loadError}<p class="text-sm text-destructive">{loadError}</p>{/if}
  {#if rows === null && !loadError}
    <PageSkeleton />
  {:else if rows && rows.length === 0}
    <Empty title={search ? 'No inbox items match your search.' : empty[tab]} icon={search ? 'search' : 'inbox'} />
  {:else if rows}
    <div class="-mx-2 sm:mx-0" aria-label="Inbox" role="list">
      {#each rows as row (row.kind === 'issue' ? `issue:${row.issue.id}` : `approval:${row.approval.id}`)}
        {#if row.kind === 'approval'}
          {@const a = row.approval}
          {@const Icon = approvalTypeIcon(a.type)}
          <div
            role="listitem"
            data-slot="approval-row"
            data-approval={a.id}
            data-status={a.status}
            class="group relative flex min-w-0 items-center gap-2 rounded-lg border-b border-border py-2 pr-3 pl-1 text-sm last:border-b-0 hover:bg-accent/50 [&_button]:relative [&_button]:z-10"
          >
            <a
              href={href(`/approvals/${a.id}`)}
              class="absolute inset-0 rounded-lg text-inherit no-underline focus-visible:z-10 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
            >
              <span class="sr-only">Open {approvalLabel(a.type, a.payload)}</span>
            </a>
            <span class="inline-flex size-4 shrink-0" aria-hidden="true"></span>
            <span class="shrink-0 rounded-md bg-muted p-1"><Icon class="size-3.5 text-muted-foreground" /></span>
            <span class="min-w-0 flex-1">
              <span class="block truncate font-medium">{approvalLabel(a.type, a.payload)}</span>
              <span class="mt-0.5 flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
                <span class="capitalize">{a.status.replaceAll('_', ' ')}</span>
                {#if a.requester_agent || a.requester}<span>requested by {a.requester_agent?.name ?? a.requester?.name}</span>{/if}
                <span>updated {ago(a.updated_at)}</span>
              </span>
            </span>
            {#if canDecide && isActionable(a)}
              <span class="ml-auto flex shrink-0 items-center gap-2">
                <UiButton size="sm" class="h-8 min-w-16 bg-green-700 px-3 text-white hover:bg-green-600" disabled={!!deciding} onclick={() => decideApproval(a, 'approve')}>
                  {deciding?.id === a.id && deciding.action === 'approve' ? 'Approving…' : 'Approve'}
                </UiButton>
                <UiButton variant="destructive" size="sm" class="h-8 min-w-16 px-3" disabled={!!deciding} onclick={() => decideApproval(a, 'reject')}>
                  {deciding?.id === a.id && deciding.action === 'reject' ? 'Rejecting…' : 'Reject'}
                </UiButton>
              </span>
            {/if}
          </div>
        {:else}
          {@const issue = row.issue}
          <div
            role="listitem"
            data-slot="task-row"
            data-issue={issue.identifier}
            data-unread={issue.unread ? 'true' : undefined}
            data-archived={issue.archived ? 'true' : undefined}
            class={[
              'group relative flex min-w-0 items-center gap-2 rounded-lg border-b border-border py-2 pr-3 pl-1 text-sm last:border-b-0 hover:bg-accent/50 [&_button]:relative [&_button]:z-10',
              issue.archived && 'opacity-50',
            ]}
          >
            <a
              href={href(`/issues/${issue.identifier}`)}
              class="absolute inset-0 rounded-lg text-inherit no-underline focus-visible:z-10 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
            >
              <span class="sr-only">Open {issue.identifier}: {issue.title}</span>
            </a>
            <span class="inline-flex size-4 shrink-0 items-center justify-center" data-testid="issue-row-unread-slot">
              <button
                type="button"
                class={['inline-flex size-4 items-center justify-center rounded-full transition-colors hover:bg-blue-500/20', !issue.unread && 'opacity-0 group-hover:opacity-100 focus-visible:opacity-100']}
                aria-label={issue.unread ? 'Mark as read' : 'Mark as unread'}
                title={issue.unread ? 'Mark as read' : 'Mark as unread'}
                onclick={() => toggleRead(issue)}
              >
                <span class={['block size-2 rounded-full', issue.unread ? 'bg-blue-600 dark:bg-blue-400' : 'border border-muted-foreground/60']}></span>
              </button>
            </span>
            <StatusIcon status={issue.status} />
            <span class="shrink-0 font-mono text-xs text-muted-foreground">{issue.identifier}</span>
            <span class={['min-w-0 flex-1 truncate', issue.unread && 'font-semibold']}>{issue.title}</span>
            <span class="ml-auto flex shrink-0 items-center gap-3">
              {#if tab === 'mine'}
                <button
                  type="button"
                  class="inline-flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 hover:bg-accent hover:text-foreground focus-visible:opacity-100"
                  aria-label="Archive"
                  onclick={() => archive(issue)}
                >
                  <Archive class="size-3.5" />Archive
                </button>
              {:else if issue.archived}
                <button
                  type="button"
                  class="inline-flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium text-muted-foreground hover:bg-accent hover:text-foreground"
                  aria-label="Unarchive"
                  onclick={() => unarchive(issue)}
                >
                  <ArchiveRestore class="size-3.5" />Unarchive
                </button>
              {/if}
              <span class="w-24 shrink-0 truncate text-right text-xs text-muted-foreground">{ago(issue.updated_at)}</span>
            </span>
          </div>
        {/if}
      {/each}
    </div>
    {#if more}
      <div class="flex justify-center">
        <UiButton variant="outline" size="sm" disabled={loadingMore} onclick={loadMore}>{loadingMore ? 'Loading…' : 'Load more'}</UiButton>
      </div>
    {/if}
  {/if}
</div>

<AlertDialog.Root bind:open={confirming}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>Mark all as read?</AlertDialog.Title>
      <AlertDialog.Description>
        This will mark {unreadShown.length} unread {unreadShown.length === 1 ? 'item' : 'items'} as read.
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
      <AlertDialog.Action onclick={markAllRead}>Mark all as read</AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
