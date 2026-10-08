<script lang="ts">
  // Paperclip's ApprovalDetail (ui/src/pages/ApprovalDetail.tsx; MIT, see
  // NOTICE): the "Approval confirmed" banner after Approve, the request with
  // its type icon, label, status and Requester, "See full request" as JSON,
  // the Decision note, the Linked issues, the Decision buttons, and the
  // comment thread. Deliberate differences: no agent is woken, so the banner
  // says the Requester can go ahead and the line about the requesting agent
  // under the Linked issues is left out; a "Decision note" field sits above
  // Approve, Reject and Request revision, since Paperclip's board cannot
  // write one from its page; Resubmit is the Requester's alone and opens the
  // request dialog prefilled, so the request can change with it. A hire
  // offers "Open hired agent" after Approve and never Request revision; its
  // rejected Agent is terminated, so there is no "Delete disapproved agent".
  // Deciding needs approve, commenting manage_work.
  import { ChevronRight, CircleCheck, Sparkles } from '@lucide/svelte'
  import { refreshBadges } from '../../lib/inbox.svelte'
  import { Button } from '$lib/components/ui/button'
  import { Textarea } from '$lib/components/ui/textarea'
  import { ApiError } from '../../lib/api'
  import ApprovalPayload from '../../lib/ApprovalPayload.svelte'
  import {
    approvalLabel,
    approvalTypeIcon,
    decide,
    getApproval,
    hirePayload,
    isActionable,
    listApprovalComments,
    listApprovalIssues,
    writeApprovalComment,
    type Approval,
    type ApprovalComment,
    type Decision,
  } from '../../lib/approvals'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import { ago, formatDate } from '../../lib/format'
  import Identity from '../../lib/Identity.svelte'
  import Markdown from '../../lib/Markdown.svelte'
  import MarkdownField from '../../lib/MarkdownField.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import RequestApprovalDialog from '../../lib/RequestApprovalDialog.svelte'
  import { go, href } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import type { Issue } from '../../lib/work'
  import NotFound from '../NotFound.svelte'

  let { id }: { id: number } = $props()

  let approval = $state.raw<Approval | null>(null)
  let issues = $state.raw<Issue[]>([])
  let comments = $state.raw<ApprovalComment[]>([])
  let missing = $state(false)
  let error = $state('')
  let busy = $state<Decision | null>(null)
  let resubmitting = $state(false)
  let note = $state('')
  let body = $state('')
  let posting = $state(false)
  let showRaw = $state(false)

  const message = (e: unknown, fallback: string) =>
    e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : e instanceof Error ? e.message : fallback

  function load() {
    getApproval(id)
      .then((a) => (approval = a))
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) missing = true
        else error = message(e, 'Could not load the approval')
      })
    listApprovalIssues(id).then((is) => (issues = is)).catch(() => {})
    listApprovalComments(id).then((cs) => (comments = cs)).catch(() => {})
  }
  load()

  $effect(() => breadcrumb.set({ label: 'Approvals', href: href('/approvals') }, { label: `#${id}` }))

  // The banner shows after Approve, here or on the list (?resolved=approved).
  let resolved = $state(new URLSearchParams(location.hash.split('?')[1] ?? '').get('resolved') === 'approved')
  const showBanner = $derived(resolved && approval?.status === 'approved')
  const canDecide = $derived(session.can('approve'))
  const canComment = $derived(session.can('manage_work'))
  const isRequester = $derived(!!approval?.requester && approval.requester.id === session.member?.id)
  const TypeIcon = $derived(approvalTypeIcon(approval?.type ?? ''))
  const hire = $derived(approval ? hirePayload(approval) : null)
  const cta = $derived(
    issues.length > 0
      ? { label: issues.length > 1 ? 'Review linked issues' : 'Review linked issue', to: `/issues/${issues[0].identifier}` }
      : hire
        ? { label: 'Open hired agent', to: `/agents/${hire.agent_id}` }
        : { label: 'Back to approvals', to: '/approvals' },
  )

  async function run(decision: Decision) {
    busy = decision
    try {
      approval = await decide(id, decision, note.trim())
      refreshBadges()
      note = ''
      error = ''
      if (decision === 'approve') {
        resolved = true
        history.replaceState(history.state, '', `#/approvals/${id}?resolved=approved`)
      }
    } catch (e) {
      error = message(e, 'The decision failed')
    } finally {
      busy = null
    }
  }

  async function post() {
    if (!body.trim() || posting) return
    posting = true
    try {
      comments = [...comments, await writeApprovalComment(id, body.trim())]
      body = ''
      error = ''
    } catch (e) {
      error = message(e, 'Comment failed')
    } finally {
      posting = false
    }
  }
</script>

{#if missing}
  <NotFound />
{:else if !approval}
  {#if error}<p class="text-sm text-destructive">{error}</p>{:else}<PageSkeleton />{/if}
{:else}
  <div class="chrome max-w-3xl space-y-6" data-approval={approval.id} data-status={approval.status}>
    {#if showBanner}
      <div class="rounded-lg border border-green-300 bg-green-50 px-4 py-3 dark:border-green-700/40 dark:bg-green-900/20" data-testid="approval-confirmed">
        <div class="flex items-start justify-between gap-3">
          <div class="flex items-start gap-2">
            <div class="relative mt-0.5">
              <CircleCheck class="size-4 text-green-600 dark:text-green-300" />
              <Sparkles class="absolute -top-1 -right-2 size-3 animate-pulse text-green-500 dark:text-green-200" />
            </div>
            <div>
              <p class="text-sm font-medium text-green-800 dark:text-green-100">Approval confirmed</p>
              <p class="text-xs text-green-700 dark:text-green-200/90">The requester can now go ahead.</p>
            </div>
          </div>
          <Button
            size="sm"
            variant="outline"
            class="border-green-400 text-green-800 hover:bg-green-100 dark:border-green-600/50 dark:text-green-100 dark:hover:bg-green-900/30"
            onclick={() => go(cta.to)}>{cta.label}</Button
          >
        </div>
      </div>
    {/if}

    <div class="space-y-3 rounded-lg border border-border p-4">
      <div class="flex items-center justify-between gap-3">
        <div class="flex min-w-0 items-center gap-2">
          <TypeIcon class="size-5 shrink-0 text-muted-foreground" />
          <div class="min-w-0">
            <h2 class="text-lg font-semibold">{approvalLabel(approval.type, approval.payload)}</h2>
            <p class="font-mono text-xs text-muted-foreground">#{approval.id}</p>
          </div>
        </div>
        <StatusBadge status={approval.status}>{approval.status.replace(/_/g, ' ')}</StatusBadge>
      </div>
      <div class="space-y-1 text-sm">
        {#if approval.requester}
          <div class="flex items-center gap-2">
            <span class="text-xs text-muted-foreground">Requested by</span>
            <Identity name={approval.requester.name} size="sm" />
          </div>
        {/if}
        <ApprovalPayload type={approval.type} payload={approval.payload} />
        <button
          type="button"
          class="mt-2 flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
          aria-expanded={showRaw}
          onclick={() => (showRaw = !showRaw)}
        >
          <ChevronRight class={['size-3 transition-transform', showRaw && 'rotate-90']} />
          See full request
        </button>
        {#if showRaw}
          <pre class="overflow-x-auto rounded-md bg-muted/40 p-3 text-xs" data-testid="approval-raw">{JSON.stringify(approval.payload, null, 2)}</pre>
        {/if}
        {#if approval.decision_note}
          <p class="text-xs text-muted-foreground" data-testid="decision-note">Decision note: {approval.decision_note}</p>
        {/if}
      </div>
      {#if error}<p class="text-sm text-destructive">{error}</p>{/if}
      {#if issues.length > 0}
        <div class="border-t border-border/60 pt-2">
          <p class="mb-1.5 text-xs text-muted-foreground">Linked issues</p>
          <div class="space-y-1.5" data-testid="linked-issues">
            {#each issues as issue (issue.id)}
              <a href={href(`/issues/${issue.identifier}`)} class="block rounded border border-border/70 px-2 py-1.5 text-xs text-inherit no-underline hover:bg-accent/20">
                <span class="mr-2 font-mono text-muted-foreground">{issue.identifier}</span>
                <span>{issue.title}</span>
              </a>
            {/each}
          </div>
        </div>
      {/if}
      {#if canDecide && isActionable(approval)}
        <Textarea bind:value={note} rows={2} placeholder="Decision note (optional)" aria-label="Decision note" class="text-sm" />
      {/if}
      <div class="flex flex-wrap items-center gap-2">
        {#if canDecide && isActionable(approval)}
          <Button size="sm" class="bg-green-700 text-white hover:bg-green-600" disabled={!!busy} onclick={() => run('approve')}>Approve</Button>
          <Button variant="destructive" size="sm" disabled={!!busy} onclick={() => run('reject')}>Reject</Button>
        {/if}
        {#if canDecide && approval.status === 'pending' && !hire}
          <Button size="sm" variant="outline" disabled={!!busy} onclick={() => run('request-revision')}>Request revision</Button>
        {/if}
        {#if isRequester && approval.status === 'revision_requested' && !hire}
          <Button size="sm" variant="outline" disabled={!!busy} onclick={() => (resubmitting = true)}>Resubmit</Button>
        {/if}
      </div>
    </div>

    <section class="space-y-3 rounded-lg border border-border p-4" aria-label="Comments">
      <h3 class="text-sm font-medium">Comments ({comments.length})</h3>
      <div class="space-y-2">
        {#each comments as c (c.id)}
          <div class="rounded-md border border-border/60 p-3" data-comment={c.id}>
            <div class="mb-1 flex items-center justify-between gap-2">
              <Identity name={c.author ? (c.author.id === session.member?.id ? 'You' : c.author.name) : 'Someone'} size="sm" />
              <span class="text-xs text-muted-foreground" title={formatDate(c.created_at)}>{ago(c.created_at)}</span>
            </div>
            <Markdown source={c.body} class="text-sm" />
          </div>
        {/each}
      </div>
      {#if canComment}
        <MarkdownField bind:value={body} label="Comment" placeholder="Add a comment..." rows={3} onsubmit={post} />
        <div class="flex justify-end">
          <Button size="sm" disabled={!body.trim() || posting} onclick={post}>{posting ? 'Posting…' : 'Post comment'}</Button>
        </div>
      {/if}
    </section>
  </div>
  <RequestApprovalDialog
    bind:open={resubmitting}
    {approval}
    onsaved={(a) => {
      approval = a
      error = ''
    }}
  />
{/if}
