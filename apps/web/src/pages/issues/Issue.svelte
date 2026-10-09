<script lang="ts">
  // Paperclip's IssueDetail (ui/src/pages/IssueDetail.tsx; MIT, see NOTICE)
  // as its classic layout draws it, without the task chat: the parent chain,
  // the status, priority, identifier and Project over the title and the
  // Markdown description, both edited in place, the Sub-issues, then two
  // tabs as Paperclip's Chat and Activity: Comments and Activity, the one
  // open kept in the address (?tab=activity); the properties panel on the right. Each change is one PATCH,
  // and the panel says "Saving..." until it answered. Without manage_work
  // the page only reads. Delete sits in the More actions menu with Add
  // sub-issue, and is The Bakery's own: Paperclip hides an Issue instead.
  // While a Blocker is not done, IssueBlockedNotice sits above the
  // description; the Documents follow the description. The Issue's Approvals
  // sit as cards above the tabs, as Paperclip's linkedApprovals above its
  // thread; "Request approval" in the More actions menu asks for a new one
  // (The Bakery's own: only Paperclip's agents ask). When an Agent is the
  // Assignee, "Run" queues a Run of it (for whoever may manage it), the
  // newest queued or running Run shows live under the description as
  // Paperclip's LiveRunWidget, and every Run of the Issue is listed below
  // the tabs as its IssueRunLedger. The Work products (its Pull request and
  // Preview) follow the Documents as Paperclip's artifact cards. Left out
  // until the Issue has them: checkout and attachments. A Conversation is
  // its owner's chat: they are moved to it in place, anyone else reads its
  // thread under a notice, as Paperclip drops the issue header for one.
  import { Activity, ChevronRight, Ellipsis, MessageSquare, Play, Plus, ShieldCheck, Trash2 } from '@lucide/svelte'
  import { runIsFinal } from '@bakery/ui/runStatus'
  import * as AlertDialog from '@bakery/ui/components/ui/alert-dialog'
  import { Button, buttonVariants } from '@bakery/ui/components/ui/button'
  import * as DropdownMenu from '@bakery/ui/components/ui/dropdown-menu'
  import * as Tabs from '@bakery/ui/components/ui/tabs'
  import { api, ApiError } from '../../lib/api'
  import { listAgents, type Agent } from '../../lib/agents'
  import ApprovalCard from '../../lib/ApprovalCard.svelte'
  import Assignee from '../../lib/Assignee.svelte'
  import { decide, listIssueApprovals, type Approval } from '../../lib/approvals'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import ChatThread from '../../lib/ChatThread.svelte'
  import CommentThread from '../../lib/CommentThread.svelte'
  import { ago } from '../../lib/format'
  import Actor from '../../lib/Actor.svelte'
  import IssueActivity from '../../lib/IssueActivity.svelte'
  import InlineEditor from '../../lib/InlineEditor.svelte'
  import IssueDocuments from '../../lib/IssueDocuments.svelte'
  import IssueWorkProducts from '../../lib/IssueWorkProducts.svelte'
  import IssueBlockedNotice from '../../lib/IssueBlockedNotice.svelte'
  import IssueProperties from '../../lib/IssueProperties.svelte'
  import LiveRun from '../../lib/LiveRun.svelte'
  import RunLedger from '../../lib/RunLedger.svelte'
  import { cancelRun, listRuns, startRun, type Run } from '../../lib/runs'
  import NewIssueDialog from '../../lib/NewIssueDialog.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import RequestApprovalDialog from '../../lib/RequestApprovalDialog.svelte'
  import PriorityIcon from '../../lib/PriorityIcon.svelte'
  import { go, href, replace } from '../../lib/router.svelte'
  import { session, type Member } from '../../lib/session.svelte'
  import StatusIcon from '../../lib/StatusIcon.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import NotFound from '../NotFound.svelte'
  import { refreshBadges } from '../../lib/inbox.svelte'
  import { deleteIssue, getIssue, listComments, listGoals, listIssues, markRead, updateIssue, type Comment, type Goal, type Issue, type IssueDetail, type IssueInput } from '../../lib/work'

  /** The Issue's identifier (DEF-12) or id, as the address holds it. */
  let { key }: { key: string } = $props()

  let issue = $state.raw<IssueDetail | null>(null)
  let missing = $state(false)
  let loadError = $state('')
  let saving = $state(0)
  let members = $state.raw<Member[]>([])
  let agents = $state.raw<Agent[]>([])
  let projects = $state.raw<{ id: number; name: string }[]>([])
  let goals = $state.raw<Goal[]>([])
  let issues = $state.raw<Issue[]>([])
  let creating = $state(false)
  let deleting = $state(false)
  let approvals = $state.raw<Approval[]>([])
  let requesting = $state(false)
  let deciding = $state<{ id: number; action: 'approve' | 'reject' } | null>(null)
  let runs = $state.raw<Run[]>([])
  let starting = $state(false)
  let stopping = $state(false)

  const editable = $derived(session.can('manage_work'))
  const canDecide = $derived(session.can('approve'))

  // The open tab as the hash query holds it, read again when a link or Back
  // changes the hash under the open page; Comments is the default.
  const tabFromHash = () => (new URLSearchParams(location.hash.split('?')[1] ?? '').get('tab') === 'activity' ? 'activity' : 'comments')
  let tab = $state(tabFromHash())
  $effect(() => {
    const follow = () => {
      if (location.hash.startsWith('#/issues/')) tab = tabFromHash()
    }
    window.addEventListener('hashchange', follow)
    return () => window.removeEventListener('hashchange', follow)
  })
  $effect(() => {
    const path = location.hash.split('?')[0]
    const next = tab === 'activity' ? `${path}?tab=activity` : path
    if (location.hash !== next) history.replaceState(history.state, '', next)
  })
  /** Bumped after every change, so the Activity tab loads again. */
  let activityVersion = $state(0)

  // Opening the Issue marks it read once per visit, as Paperclip's
  // IssueDetail does; a failed Read mark is not worth a toast.
  let marked = false
  function load() {
    getIssue(key)
      .then((i) => {
        if (i.conversation?.board && session.member) return replace('/board-chat')
        if (i.conversation?.agent && i.conversation.member_id === session.member?.id) return replace(`/chats/${i.conversation.agent.id}`)
        issue = i
        if (marked) return
        marked = true
        markRead(i.id).then(refreshBadges, () => {})
      })
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) missing = true
        else loadError = e.message
      })
  }
  load()
  const loadApprovals = () =>
    listIssueApprovals(key)
      .then((as) => (approvals = as))
      .catch(() => {})
  loadApprovals()
  api<{ members: Member[] }>('GET', '/members').then((r) => (members = r.members)).catch(() => {})
  api<{ projects: { id: number; name: string }[] }>('GET', '/projects').then((r) => (projects = r.projects)).catch(() => {})
  listAgents().then((as) => (agents = as)).catch(() => {})
  listGoals().then((gs) => (goals = gs)).catch(() => {})
  listIssues().then((is) => (issues = is)).catch(() => {})

  // The Agent the Issue is assigned to, as the Agents list has it (its
  // status, Hirer and whether the asker may manage it).
  const agent = $derived(issue?.assignee?.kind === 'agent' ? (agents.find((a) => a.id === issue!.assignee!.id) ?? null) : null)
  const unrunnable: Record<string, string> = {
    paused: 'is paused',
    pending_approval: 'waits for its hire to be approved',
    terminated: 'is terminated',
  }
  const runBlocked = $derived(agent ? unrunnable[agent.status] : undefined)
  /** The newest Run still waiting or running; Runs are listed newest first. */
  const liveRun = $derived(runs.find((r) => !runIsFinal(r.status)) ?? null)

  const loadRuns = (id: number) =>
    listRuns({ issue: id })
      .then((rs) => (runs = rs))
      .catch(() => {})
  const issueId = $derived(issue?.id)
  const running = $derived(liveRun !== null)
  $effect(() => {
    if (issueId) loadRuns(issueId)
  })
  // While a Run waits or runs, the list, the Issue and its Comments are read
  // again every 5 seconds, so a Desktop's claim, what the Agent does through
  // The Bakery's API (its Checkout, status and Comments) and the end show
  // without a reload; the end reads them once more.
  let commentsVersion = $state(0)
  function follow(id: number) {
    loadRuns(id)
    getIssue(id)
      .then((i) => (issue = i))
      .catch(() => {})
    commentsVersion++
    activityVersion++
  }
  $effect(() => {
    if (!running || !issueId) return
    const id = issueId
    const t = setInterval(() => follow(id), 5000)
    return () => {
      clearInterval(t)
      follow(id)
    }
  })
  /** Scrolls to the Run among the Issue's Runs. */
  function showRun(id: number) {
    document.querySelector(`[data-run="${id}"]`)?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }
  /** A Run as its live stream last reported it. */
  const updated = (r: Run) => (runs = runs.map((x) => (x.id === r.id ? r : x)))

  async function run() {
    starting = true
    try {
      // A Run already queued on the Issue is joined, not listed twice.
      const r = await startRun(agent!.id, issue!.id)
      runs = [r, ...runs.filter((x) => x.id !== r.id)]
      activityVersion++
    } catch (e) {
      toast.error(e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
    } finally {
      starting = false
    }
  }

  async function cancel(r: Run) {
    stopping = true
    try {
      updated(await cancelRun(r.id))
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : String(e))
    } finally {
      stopping = false
    }
  }

  // A Conversation's messages, for the read-only thread.
  let chat = $state.raw<Comment[]>([])
  $effect(() => {
    if (issue?.conversation) listComments(issue.id).then((cs) => (chat = cs)).catch(() => {})
  })
  const owner = $derived(issue?.conversation ? (members.find((m) => m.id === issue!.conversation!.member_id)?.name ?? 'Someone') : '')

  $effect(() => breadcrumb.set({ label: 'Issues', href: href('/issues') }, { label: issue?.identifier ?? key }))

  // The parent chain, top first, from the Guild's Issues; the direct parent
  // is on the Issue itself, so it shows before the list has loaded.
  const ancestors = $derived.by(() => {
    if (!issue?.parent) return []
    const chain = [issue.parent]
    const seen = new Set([issue.id, issue.parent.id])
    let next = issues.find((i) => i.id === issue!.parent!.id)?.parent
    while (next && !seen.has(next.id)) {
      chain.unshift(next)
      seen.add(next.id)
      next = issues.find((i) => i.id === next!.id)?.parent
    }
    return chain
  })

  async function save(patch: IssueInput) {
    saving++
    try {
      issue = await updateIssue(issue!.id, patch)
      activityVersion++
      // Assigning the Issue to an Agent wakes it with a Run.
      loadRuns(issue.id)
      if (patch.parent_id !== undefined || patch.blocked_by_ids !== undefined) listIssues().then((is) => (issues = is)).catch(() => {})
    } catch (e) {
      toast.error(e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
    } finally {
      saving--
    }
  }

  async function remove() {
    try {
      await deleteIssue(issue!.id)
      toast.success(`${issue!.identifier} deleted.`)
      go('/issues')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  async function decideApproval(id: number, action: 'approve' | 'reject') {
    deciding = { id, action }
    try {
      await decide(id, action)
      refreshBadges()
      activityVersion++
      await loadApprovals()
    } catch (e) {
      toast.error(e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
    } finally {
      deciding = null
    }
  }

  function refresh() {
    activityVersion++
    // A comment wakes the Agent assignee or joins its queued Run.
    loadRuns(issue!.id)
    getIssue(issue!.id)
      .then((i) => (issue = i))
      .catch(() => {})
  }
</script>

{#if missing}
  <NotFound title="Issue not found" description="This issue does not exist or you cannot see it." />
{:else if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if issue === null}
  <PageSkeleton />
{:else if issue.conversation}
  {@const agent = issue.conversation.agent ?? { id: 0, name: 'the CEO', icon: '' }}
  <div class="mx-auto flex max-w-3xl flex-col gap-6">
    <p class="rounded-lg border border-dashed px-4 py-3 text-sm text-muted-foreground" data-testid="conversation-notice">
      {#if issue.conversation.board}This is the Board's chat with {agent.name}.{:else}This is {owner}'s chat with {agent.name}.{/if}
    </p>
    <ChatThread comments={chat} {agent} boundary={issue.conversation.boundary_comment_id} />
  </div>
{:else}
  {@const i = issue}
  <div class="flex flex-col gap-6 lg:flex-row">
    <div class="max-w-3xl min-w-0 flex-1 space-y-6">
      {#if ancestors.length > 0}
        <nav class="flex flex-wrap items-center gap-1 text-xs text-muted-foreground" aria-label="Parent issues">
          {#each ancestors as a, n (a.id)}
            <span class="flex items-center gap-1">
              {#if n > 0}<ChevronRight class="size-3 shrink-0" />{/if}
              <a href={href(`/issues/${a.identifier}`)} class="max-w-50 truncate transition-colors hover:text-foreground" title={a.title}>{a.title}</a>
            </span>
          {/each}
          <ChevronRight class="size-3 shrink-0" />
          <span class="max-w-50 truncate text-foreground/60">{i.title}</span>
        </nav>
      {/if}

      <div class="space-y-3" data-testid="issue-detail-header">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <StatusIcon status={i.status} size="lg" onchange={editable ? (status) => save({ status }) : undefined} />
          <PriorityIcon priority={i.priority} onchange={editable ? (priority) => save({ priority }) : undefined} />
          <span class="shrink-0 font-mono text-sm text-muted-foreground">{i.identifier}</span>
          {#if i.project}
            <a
              href={href(`/project/${i.project.id}`)}
              class="-mx-1 inline-flex min-w-0 items-center gap-1 rounded px-1 py-0.5 text-xs text-muted-foreground transition-colors hover:text-foreground"
            >
              <span class="truncate">{i.project.name}</span>
            </a>
          {:else}
            <span class="-mx-1 inline-flex items-center gap-1 px-1 py-0.5 text-xs text-muted-foreground opacity-50">No project</span>
          {/if}
          {#if i.created_by || i.created_by_agent}
            <span class="inline-flex items-center gap-1 text-xs text-muted-foreground">
              <Actor member={i.created_by} agent={i.created_by_agent} size="xs" /> opened {ago(i.created_at)}
            </span>
          {/if}
          <div class="ml-auto flex shrink-0 items-center gap-1">
            {#if agent?.can_manage}
              <span title={runBlocked ? `${agent.name} ${runBlocked}` : `Run ${agent.name} on this issue on ${agent.hirer?.name ?? 'its hirer'}'s desktop`}>
                <Button size="xs" variant="outline" class="shadow-none" disabled={!!runBlocked || starting} onclick={run}><Play class="size-3" />Run</Button>
              </span>
            {/if}
            {#if editable}
              <DropdownMenu.Root>
                <DropdownMenu.Trigger class={buttonVariants({ variant: 'ghost', size: 'icon-xs' })} aria-label="More issue actions" title="More issue actions">
                  <Ellipsis class="size-4" />
                </DropdownMenu.Trigger>
                <DropdownMenu.Content align="end" class="w-52">
                  <DropdownMenu.Item onSelect={() => (creating = true)}><Plus />Add sub-issue</DropdownMenu.Item>
                  <DropdownMenu.Item onSelect={() => (requesting = true)}><ShieldCheck />Request approval</DropdownMenu.Item>
                  <DropdownMenu.Item variant="destructive" onSelect={() => (deleting = true)}><Trash2 />Delete issue</DropdownMenu.Item>
                </DropdownMenu.Content>
              </DropdownMenu.Root>
            {/if}
          </div>
        </div>

        <InlineEditor label="Title" value={i.title} {editable} as="h2" class="text-xl font-bold" onsave={(title) => save({ title })} />
        <IssueBlockedNotice blockers={i.blocked_by} />
        <InlineEditor
          label="Description"
          value={i.description}
          {editable}
          multiline
          placeholder="Add a description..."
          class="text-sm leading-7 text-foreground"
          onsave={(description) => save({ description })}
        />
      </div>

      {#if liveRun}
        <LiveRun run={liveRun} hirer={liveRun.agent.id === agent?.id ? (agent.hirer?.name ?? null) : (agents.find((a) => a.id === liveRun.agent.id)?.hirer?.name ?? null)} {stopping} oncancel={() => cancel(liveRun)} onstatus={updated} />
      {/if}

      <IssueDocuments issue={i.id} {editable} onchange={refresh} />

      <IssueWorkProducts products={i.work_products ?? []} canViewApplication={session.can('view_resources')} />

      <section class="space-y-3" aria-label="Sub-issues">
        <div class="flex items-center justify-between gap-2">
          <h3 class="text-sm font-medium text-muted-foreground">Sub-issues</h3>
          {#if editable}
            <Button variant="outline" size="sm" class="shadow-none" onclick={() => (creating = true)}><Plus class="size-3.5" />Add sub-issue</Button>
          {/if}
        </div>
        {#if i.children.length === 0}
          <p class="text-sm text-muted-foreground">No sub-issues.</p>
        {:else}
          <div class="rounded-lg border">
            {#each i.children as child (child.id)}
              <a
                href={href(`/issues/${child.identifier}`)}
                data-issue={child.identifier}
                class="flex min-w-0 items-center gap-2 border-b px-3 py-2 text-sm text-inherit no-underline transition-colors last:border-b-0 hover:bg-accent/50"
              >
                <StatusIcon status={child.status} />
                <PriorityIcon priority={child.priority} class="ml-1" />
                <span class={['min-w-0 flex-1 truncate', (child.status === 'done' || child.status === 'cancelled') && 'text-muted-foreground']}>{child.title}</span>
                {#if child.assignee}<Assignee assignee={child.assignee} />{/if}
                <span class="w-20 shrink-0 text-right font-mono text-xs text-muted-foreground">{child.identifier}</span>
              </a>
            {/each}
          </div>
        {/if}
      </section>

      {#if approvals.length > 0}
        <section class="space-y-3" aria-label="Approvals">
          {#each approvals as a (a.id)}
            <ApprovalCard
              approval={a}
              {canDecide}
              onapprove={() => decideApproval(a.id, 'approve')}
              onreject={() => decideApproval(a.id, 'reject')}
              detailLink={href(`/approvals/${a.id}`)}
              pending={deciding?.id === a.id ? deciding.action : null}
            />
          {/each}
        </section>
      {/if}

      <Tabs.Root bind:value={tab}>
        <Tabs.List variant="line" class="w-full justify-start gap-1">
          <Tabs.Trigger value="comments" class="flex-none gap-1.5"><MessageSquare class="size-3.5" />Comments</Tabs.Trigger>
          <Tabs.Trigger value="activity" class="flex-none gap-1.5"><Activity class="size-3.5" />Activity</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="comments" class="pt-3">
          <CommentThread issue={i.id} {editable} onchange={refresh} version={commentsVersion} />
        </Tabs.Content>
        <Tabs.Content value="activity" class="pt-3">
          {#if tab === 'activity'}<IssueActivity issue={i.id} version={activityVersion} />{/if}
        </Tabs.Content>
      </Tabs.Root>

      {#if runs.length > 0 || i.assignee?.kind === 'agent'}
        <section class="space-y-3" aria-label="Runs">
          <h3 class="text-sm font-medium text-muted-foreground">Runs</h3>
          <RunLedger {runs} onstatus={updated} />
        </section>
      {/if}
    </div>

    <aside class="w-full shrink-0 border-t pt-4 lg:w-80 lg:border-t-0 lg:border-l lg:pt-0 lg:pl-6" aria-label="Properties">
      <div class="mb-2 flex h-6 items-center justify-between">
        <h3 class="text-sm font-medium">Properties</h3>
        {#if saving > 0}<span class="text-xs text-muted-foreground" role="status">Saving...</span>{/if}
      </div>
      <IssueProperties issue={i} {members} {agents} {projects} {goals} {issues} {editable} onsave={save} onrun={showRun} />
    </aside>
  </div>

  <AlertDialog.Root open={deleting} onOpenChange={(open) => (deleting = open)}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>Delete {i.identifier}?</AlertDialog.Title>
        <AlertDialog.Description>
          "{i.title}" and its comments are deleted for good. Its sub-issues stay, without a parent.
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
        <AlertDialog.Action class={buttonVariants({ variant: 'destructive' })} data-testid="confirm-delete-issue" onclick={remove}>Delete</AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>

  <RequestApprovalDialog
    bind:open={requesting}
    issueId={i.id}
    onsaved={(a) => {
      toast.success('Approval requested', a.payload.title, { label: 'Open approval', href: href(`/approvals/${a.id}`) })
      activityVersion++
      loadApprovals()
    }}
  />
  <NewIssueDialog bind:open={creating} parentId={i.id} projectId={i.project?.id ?? null} goalId={i.goal?.id ?? null} oncreated={refresh} />
{/if}
