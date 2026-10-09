<script lang="ts">
  // Paperclip's AgentChat (ui/src/pages/AgentChat.tsx; MIT, see NOTICE): the
  // asking Member's Conversation with one Agent as a chat thread, the live
  // Run under it, and a composer at the bottom (Paperclip's ChatComposer,
  // ui/src/components/ChatComposer.tsx, trimmed to a growing textarea where
  // Enter sends and Shift+Enter is a new line). Visiting opens nothing: the
  // first message opens the Conversation and then writes itself into it, as
  // Paperclip's ensureIssue does. Sending /new starts a New session. While a
  // Run waits or runs, the thread and the Run are read again every 5
  // seconds, as on the Issue page, so the Agent's reply lands without a
  // reload.
  import { SendHorizontal, Settings } from '@lucide/svelte'
  import { runIsFinal } from '@bakery/ui/runStatus'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import { ApiError } from '../../lib/api'
  import { listAgents, type Agent } from '../../lib/agents'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import ChatThread from '../../lib/ChatThread.svelte'
  import { getChat, openChat } from '../../lib/chats'
  import { recordChatVisit } from '../../lib/recentChats.svelte'
  import LiveRun from '../../lib/LiveRun.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { href } from '../../lib/router.svelte'
  import { cancelRun, listRuns, type Run } from '../../lib/runs'
  import { session } from '../../lib/session.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import { listComments, writeComment, type Comment, type IssueDetail } from '../../lib/work'

  let { agentId }: { agentId: number } = $props()

  let agent = $state.raw<Agent | null | undefined>(undefined)
  let issue = $state.raw<IssueDetail | null | undefined>(undefined)
  let comments = $state.raw<Comment[]>([])
  let runs = $state.raw<Run[]>([])
  let loadError = $state('')
  let body = $state('')
  let sending = $state(false)
  let stopping = $state(false)
  let textarea = $state<HTMLTextAreaElement | null>(null)

  // The page is keyed by its Agent, so both load once.
  function load() {
    listAgents()
      .then((as) => {
        agent = as.find((a) => a.id === agentId) ?? null
        // The sidebar's Chats list it from now on.
        if (agent) recordChatVisit(agent.id)
      })
      .catch((e) => (loadError = e instanceof Error ? e.message : String(e)))
    getChat(agentId)
      .then((i) => (issue = i))
      .catch((e) => (loadError = e instanceof Error ? e.message : String(e)))
  }
  load()

  $effect(() => breadcrumb.set({ label: 'Chat', href: href('/chats') }, { label: agent?.name ?? 'Conversation' }))

  const issueId = $derived(issue?.id)
  /** The newest Run still waiting or running; Runs are listed newest first. */
  const liveRun = $derived(runs.find((r) => !runIsFinal(r.status)) ?? null)
  const running = $derived(liveRun !== null)

  const loadComments = (id: number) =>
    listComments(id)
      .then((cs) => (comments = cs))
      .catch(() => {})
  const loadRuns = (id: number) =>
    listRuns({ issue: id })
      .then((rs) => (runs = rs))
      .catch(() => {})
  $effect(() => {
    if (!issueId) return
    loadComments(issueId)
    loadRuns(issueId)
  })
  function follow(id: number) {
    loadRuns(id)
    loadComments(id)
    getChat(agentId)
      .then((i) => i && (issue = i))
      .catch(() => {})
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

  // Why the composer is off, if it is.
  const blocked = $derived.by(() => {
    if (!session.can('manage_work')) return 'You cannot chat in this guild.'
    if (agent?.status === 'paused') return `${agent.name} is paused.`
    if (agent?.status === 'terminated') return `${agent.name} is terminated.`
    if (agent?.status === 'pending_approval') return `${agent.name} waits for its hire to be approved.`
    return ''
  })

  async function send() {
    const text = body.trim()
    if (!text || sending || blocked) return
    sending = true
    try {
      const i = issue ?? (issue = await openChat(agentId))
      const c = await writeComment(i.id, text)
      comments = [...comments, c]
      body = ''
      // The message woke the Agent with a Run; /new did not.
      loadRuns(i.id)
      getChat(agentId)
        .then((x) => x && (issue = x))
        .catch(() => {})
    } catch (e) {
      toast.error(e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
    } finally {
      sending = false
      textarea?.focus()
    }
  }

  function keydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      e.preventDefault()
      send()
    }
  }

  // The textarea grows with what is typed, up to 200 pixels, as Paperclip's.
  $effect(() => {
    void body
    if (!textarea) return
    textarea.style.height = 'auto'
    textarea.style.height = `${Math.min(textarea.scrollHeight, 200)}px`
  })

  async function cancel(r: Run) {
    stopping = true
    try {
      const c = await cancelRun(r.id)
      runs = runs.map((x) => (x.id === c.id ? c : x))
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : String(e))
    } finally {
      stopping = false
    }
  }
</script>

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if agent === null}
  <p class="text-sm text-destructive">Agent not found.</p>
{:else if agent === undefined || issue === undefined}
  <PageSkeleton />
{:else}
  {@const a = agent}
  <div class="mx-auto flex max-w-3xl flex-col gap-6">
    <header class="flex items-center gap-3 border-b pb-4">
      <span class="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted"><AgentIcon icon={a.icon} class="size-4" /></span>
      <div class="min-w-0 flex-1">
        <h1 class="truncate text-base font-semibold">{a.name}</h1>
        <p class="truncate text-xs text-muted-foreground">
          {a.title || a.job_label}{#if issue}
            · <span data-testid="chat-state">{issue.conversation?.state === 'active' ? 'Active' : 'Waiting'}</span>{/if}
        </p>
      </div>
      <Button variant="ghost" size="icon-sm" href={href(`/agents/${a.id}`)} aria-label="{a.name}'s settings" title="{a.name}'s settings"><Settings /></Button>
    </header>

    <ChatThread {comments} agent={a} boundary={issue?.conversation?.boundary_comment_id ?? null} />

    {#if liveRun}
      <LiveRun run={liveRun} hirer={a.hirer?.name ?? null} {stopping} oncancel={() => cancel(liveRun)} onstatus={(r) => (runs = runs.map((x) => (x.id === r.id ? r : x)))} />
    {/if}

    <div class="sticky bottom-0 bg-background pb-4" data-testid="chat-composer">
      {#if blocked}
        <p class="rounded-lg border border-dashed px-4 py-3 text-sm text-muted-foreground">{blocked}</p>
      {:else}
        <div class="flex items-end gap-2 rounded-xl border border-border bg-background px-3 py-2 focus-within:ring-2 focus-within:ring-ring">
          <textarea
            bind:this={textarea}
            bind:value={body}
            onkeydown={keydown}
            rows="1"
            aria-label="Message {a.name}"
            placeholder="Message {a.name}… (/new starts a new session)"
            class="max-h-50 min-h-6 flex-1 resize-none bg-transparent py-1 text-sm outline-none placeholder:text-muted-foreground"
          ></textarea>
          <Button size="icon-sm" disabled={!body.trim() || sending} onclick={send} aria-label="Send" title="Send"><SendHorizontal /></Button>
        </div>
      {/if}
    </div>
  </div>
{/if}
