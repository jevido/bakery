<script lang="ts">
  // Paperclip's BoardChat (ui/src/pages/BoardChat.tsx; MIT, see NOTICE), its
  // chat pane: the Guild's Board chat, where the whole Board talks to the
  // Guild's CEO. The header names the CEO over the Guild, the welcome bubble
  // names the Guild's mission (its first active guild-level Goal) and offers
  // Paperclip's four suggestion chips until a Member has written in the
  // session, and the
  // composer floats over the thread with Paperclip's soft fade. Visiting opens
  // nothing: the first message opens the Board chat and then writes itself
  // into it, as on the Chat page. The CEO answers from its Hirer's Desktop,
  // so while its Run waits the status line says whose Desktop it waits for,
  // and while it runs it counts the seconds as Paperclip's does. The Board
  // chat is shared, so the thread and its Runs are read again every 5
  // seconds while the page is open, not only while a Run is live: another
  // Member's message lands without a reload. Left out, as the work document
  // records: Paperclip's history button, feedback votes, the reveal timer
  // and streaming text (a Run's reply lands as a Comment).
  import { Activity, MessageSquarePlus, SendHorizontal, UserPlus } from '@lucide/svelte'
  import { runIsFinal } from '@bakery/ui/runStatus'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import Markdown from '@bakery/ui/Markdown.svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import * as Sheet from '@bakery/ui/components/ui/sheet'
  import ActivityFeed from '../../lib/ActivityFeed.svelte'
  import { ApiError } from '../../lib/api'
  import { listAgents, type Agent } from '../../lib/agents'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import ChatThread from '../../lib/ChatThread.svelte'
  import { getBoardChat, openBoardChat } from '../../lib/chats'
  import HireAgentDialog from '../../lib/HireAgentDialog.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { followRun, listRuns, type Run } from '../../lib/runs'
  import { session } from '../../lib/session.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import { listComments, listGoals, writeComment, type Comment, type IssueDetail } from '../../lib/work'

  // Paperclip's split: the chat pane at 2/3 and the Activity feed in the
  // rest, behind a 12 px divider that drags, each pane at least 280 px. The
  // chat's share is kept per Guild in localStorage. Below 768 px the feed is
  // a sheet behind a floating button.
  const dividerPx = 12
  const minPanePx = 280
  const splitKey = () => `bakery.boardChatSplit.${session.guild?.id ?? 0}`
  let containerWidth = $state(0)
  let chatFraction = $state(Number(localStorage.getItem(splitKey())) || 2 / 3)
  let feedOpen = $state(false)
  const inner = $derived(Math.max(0, containerWidth - dividerPx))
  const split = $derived(inner > 0 && containerWidth >= 2 * minPanePx + dividerPx)
  const chatWidth = $derived(Math.round(Math.min(inner - minPanePx, Math.max(minPanePx, inner * chatFraction))))

  function dragStart(down: PointerEvent) {
    down.preventDefault()
    const divider = down.currentTarget as HTMLElement
    divider.setPointerCapture(down.pointerId)
    const startX = down.clientX
    const startWidth = chatWidth
    const move = (e: PointerEvent) => {
      if (inner <= 0) return
      chatFraction = Math.min(inner - minPanePx, Math.max(minPanePx, startWidth + e.clientX - startX)) / inner
    }
    const up = () => {
      divider.removeEventListener('pointermove', move)
      divider.removeEventListener('pointerup', up)
      divider.removeEventListener('pointercancel', up)
      localStorage.setItem(splitKey(), String(chatFraction))
    }
    divider.addEventListener('pointermove', move)
    divider.addEventListener('pointerup', up)
    divider.addEventListener('pointercancel', up)
  }

  let agents = $state.raw<Agent[] | undefined>(undefined)
  let issue = $state.raw<IssueDetail | null | undefined>(undefined)
  let mission = $state<string | null>(null)
  let comments = $state.raw<Comment[]>([])
  let runs = $state.raw<Run[]>([])
  let loadError = $state('')
  let body = $state('')
  let sending = $state(false)
  let hiring = $state(false)
  let now = $state(Date.now())
  let textarea = $state<HTMLTextAreaElement | null>(null)
  let scroller = $state<HTMLDivElement | null>(null)

  const fail = (e: unknown) => (loadError = e instanceof Error ? e.message : String(e))
  const loadAgents = () =>
    listAgents()
      .then((as) => (agents = as))
      .catch(fail)
  loadAgents()
  getBoardChat()
    .then((i) => (issue = i))
    .catch(fail)
  listGoals()
    .then((gs) => (mission = gs.find((g) => g.status === 'active' && g.level === 'guild')?.title ?? null))
    .catch(() => {})

  $effect(() => breadcrumb.set({ label: 'Conference Room' }))

  /**
   * The Guild's CEO: the Board chat's own once it exists, else picked the
   * way the API does, its oldest CEO that is neither pending approval nor
   * terminated.
   */
  const ceo = $derived.by((): Agent | null => {
    if (!agents) return null
    const id = issue?.conversation?.agent?.id
    if (id) return agents.find((a) => a.id === id) ?? null
    return (
      agents
        .filter((a) => a.job === 'ceo' && a.status !== 'pending_approval' && a.status !== 'terminated')
        .toSorted((a, b) => a.id - b.id)[0] ?? null
    )
  })
  const guildName = $derived(session.guild?.name ?? 'Your guild')

  const issueId = $derived(issue?.id)
  /** The newest Run still waiting or running; Runs are listed newest first. */
  const liveRun = $derived(runs.find((r) => !runIsFinal(r.status)) ?? null)
  /** Whether the newest Run failed with no reply written after it. */
  const failed = $derived.by(() => {
    const last = runs[0]
    if (!last || (last.status !== 'failed' && last.status !== 'lost')) return false
    const since = Date.parse(last.created_at)
    return !comments.some((c) => c.author_agent && Date.parse(c.created_at) >= since)
  })
  // The chips come back with each New session, so New chat starts the room
  // over as Paperclip's first visit does.
  const memberWrote = $derived.by(() => {
    const isNew = (c: Comment) => !c.author_agent && !c.deleted && c.body.trim() === '/new'
    return comments.slice(comments.findLastIndex(isNew) + 1).some((c) => !c.author_agent)
  })

  const follow = (id: number) => {
    listComments(id)
      .then((cs) => (comments = cs))
      .catch(() => {})
    listRuns({ issue: id })
      .then((rs) => (runs = rs))
      .catch(() => {})
  }
  $effect(() => {
    if (!issueId) return
    const id = issueId
    follow(id)
    const t = setInterval(() => follow(id), 5000)
    return () => clearInterval(t)
  })
  // The live Run's stream says the moment its Desktop claims it and the
  // moment it ends, when the reply is read at once instead of on the next
  // poll.
  const liveId = $derived(liveRun?.id)
  $effect(() => {
    if (!liveId || !issueId) return
    const id = issueId
    return followRun(
      liveId,
      () => {},
      (r) => {
        runs = runs.map((x) => (x.id === r.id ? r : x))
        if (runIsFinal(r.status)) follow(id)
      },
    )
  })
  // A Board chat someone else opened shows up too.
  $effect(() => {
    if (issue !== null) return
    const t = setInterval(() => getBoardChat().then((i) => i && (issue = i)), 5000)
    return () => clearInterval(t)
  })

  // The status line's seconds since the Run started.
  $effect(() => {
    if (liveRun?.status !== 'running') return
    const t = setInterval(() => (now = Date.now()), 100)
    return () => clearInterval(t)
  })
  const elapsed = $derived(liveRun?.started_at ? Math.max(0, (now - Date.parse(liveRun.started_at)) / 1000) : 0)

  // New messages scroll into view, as Paperclip's do.
  $effect(() => {
    void comments.length
    void liveRun
    void failed
    if (scroller) requestAnimationFrame(() => scroller?.scrollTo({ top: scroller.scrollHeight }))
  })

  const canWrite = $derived(session.can('manage_work'))

  async function write(text: string) {
    if (!text || sending || !canWrite) return
    sending = true
    try {
      const i = issue ?? (issue = await openBoardChat())
      const c = await writeComment(i.id, text)
      comments = [...comments, c]
      if (text === body.trim()) body = ''
      // The message woke the CEO with a Run; /new did not.
      follow(i.id)
      // The first message may have picked the CEO.
      getBoardChat()
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
      write(body.trim())
    }
  }

  // The textarea grows with what is typed, up to 200 pixels, as Paperclip's,
  // and is measured again whenever its width changes (it mounts narrow
  // before the composer is laid out, and the divider moves it).
  let textareaWidth = $state(0)
  $effect(() => {
    void [body, textareaWidth]
    if (!textarea) return
    textarea.style.height = 'auto'
    textarea.style.height = `${Math.min(textarea.scrollHeight, 200)}px`
  })

  function pick(prompt: string) {
    body = prompt
    textarea?.focus()
  }

  const chips = $derived([
    {
      label: 'Draft an Organization Brief',
      prompt: `Draft a one-page Organization Brief for ${guildName} — include our mission, team roster, and first priorities.`,
    },
    {
      label: 'Create a hiring plan',
      prompt: `Create a hiring plan for ${guildName}. List the next roles to hire, in priority order, with a short rationale for each.`,
    },
    { label: 'Outline our first 30 days', prompt: 'Outline our first 30 days. Break it into weekly priorities with who owns what.' },
    { label: 'Write an intro pitch', prompt: `Write a short intro pitch for ${guildName} that I could reuse for investors, customers, or recruits.` },
  ])
  const welcome = $derived(
    ceo
      ? `Welcome to **${guildName}**! I'm ${ceo.name}, your team lead${mission ? ` — your mission is "${mission}".` : '.'}\n\n` +
          "Here are a few things I can help you put on paper right now. Pick one below and I'll draft it for you."
      : '',
  )
</script>

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if agents === undefined || issue === undefined}
  <PageSkeleton />
{:else}
  <div class="-m-4 flex h-[calc(100%+2rem)] min-h-0 flex-row md:-m-6 md:h-[calc(100%+3rem)]" data-testid="board-chat" bind:clientWidth={containerWidth}>
    <div
      class={['relative flex min-h-0 min-w-0 shrink-0 flex-col bg-background', 'w-full md:w-auto', !split && 'md:w-2/3']}
      style:width={split ? `${chatWidth}px` : undefined}
      data-testid="board-chat-pane"
    >
      <header class="chrome relative flex shrink-0 items-center justify-between gap-2 border-b border-border px-4 py-3">
        <div class="min-w-0 flex-1">
          <h3 class="truncate text-sm font-semibold" data-testid="board-chat-title">{ceo?.name ?? 'Conference Room'}</h3>
          <p class="truncate text-xs text-muted-foreground">{guildName}</p>
        </div>
        <div class="flex shrink-0 items-center gap-0.5">
          <Button
            variant="ghost"
            size="icon-sm"
            class="text-muted-foreground"
            aria-label="new chat"
            title="new chat"
            disabled={!issue || liveRun !== null || sending || !canWrite}
            onclick={() => write('/new')}><MessageSquarePlus /></Button
          >
        </div>
      </header>

      <div class="relative min-h-0 min-w-0 flex-1">
        <div bind:this={scroller} class="scrollbar-auto-hide absolute inset-0 overflow-x-hidden overflow-y-auto">
          <div class="flex flex-col gap-4 px-6 pt-3 pb-32">
            {#if ceo}
              <div class="flex flex-col items-start" data-testid="board-welcome">
                <div class="mb-1 flex items-center gap-1.5 pl-1">
                  <span class="flex size-6 shrink-0 items-center justify-center rounded-full bg-muted"><AgentIcon icon={ceo.icon} class="size-3.5" /></span>
                  <span class="text-sm font-medium text-foreground">{ceo.name}</span>
                </div>
                <div class="max-w-[85%] min-w-0 border border-border bg-card px-3 py-2 text-sm break-words text-foreground [border-radius:14px_14px_14px_4px]">
                  <Markdown source={welcome} class="text-sm [&>*:first-child]:mt-0 [&>*:last-child]:mb-0" />
                </div>
              </div>
              {#if !memberWrote && canWrite}
                <div class="flex flex-wrap gap-2 pl-1" data-testid="board-chips">
                  {#each chips as chip (chip.label)}
                    <button
                      type="button"
                      onclick={() => pick(chip.prompt)}
                      class="rounded-full border border-border bg-card px-3 py-1.5 text-xs text-muted-foreground transition-colors duration-150 hover:bg-accent hover:text-foreground"
                    >
                      {chip.label}
                    </button>
                  {/each}
                </div>
              {/if}
            {:else}
              <div class="chrome flex flex-col items-center gap-3 py-16 text-center" data-testid="board-no-ceo">
                <span class="flex size-12 items-center justify-center rounded-full bg-muted"><UserPlus class="size-6 text-muted-foreground" /></span>
                <p class="text-sm font-medium">This guild has no CEO yet</p>
                <p class="max-w-sm text-sm text-muted-foreground">The Conference Room is where the Board talks to the guild's CEO. Hire one to get answers here.</p>
                {#if session.can('hire_agents')}
                  <Button size="sm" onclick={() => (hiring = true)}><UserPlus />Hire a CEO</Button>
                {/if}
              </div>
            {/if}

            {#if ceo || comments.length > 0}
              <ChatThread {comments} agent={ceo ?? { id: 0, name: 'CEO', icon: '' }} boundary={issue?.conversation?.boundary_comment_id ?? null} board viewer={session.member?.id ?? null} />
            {/if}

            {#if liveRun}
              <div class="flex justify-start">
                <div class="border border-border bg-card px-3 py-2 text-sm text-foreground [border-radius:14px_14px_14px_4px]">
                  <span class="typing-dots" aria-label="typing"><span></span><span></span><span></span></span>
                </div>
              </div>
              <div class="flex items-center gap-2 pl-1 text-xs text-muted-foreground" data-testid="board-status" data-status={liveRun.status}>
                {#if liveRun.status === 'queued'}
                  <span>Waiting for {ceo?.hirer ? `${ceo.hirer.name}'s` : "the hirer's"} Desktop…</span>
                {:else}
                  <span>Thinking…</span>
                  {#if elapsed > 0}<span class="opacity-50">{elapsed.toFixed(1)}s</span>{/if}
                {/if}
              </div>
            {:else if failed}
              <div role="alert" class="flex justify-start" data-testid="board-error">
                <div class="max-w-[85%] border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive [border-radius:14px_14px_14px_4px]">
                  {ceo?.name ?? 'The CEO'} couldn't answer. Please try again.
                </div>
              </div>
            {/if}
          </div>
        </div>
      </div>

      <div class="chrome pointer-events-none absolute inset-x-0 bottom-0 z-10 bg-linear-to-t from-background via-background/95 to-background/0 px-6 pt-6 pb-5" data-testid="chat-composer">
        {#if !canWrite}
          <p class="pointer-events-auto rounded-lg border border-dashed bg-background px-4 py-3 text-sm text-muted-foreground">You can read the Conference Room but not write in it.</p>
        {:else}
          {#if !ceo}
            <p class="mb-2 text-xs text-muted-foreground" data-testid="board-no-ceo-note">Nobody will answer until the guild hires a CEO.</p>
          {/if}
          <div class="pointer-events-auto flex items-end gap-2 rounded-xl border border-border bg-background/80 px-3 py-2 backdrop-blur focus-within:ring-2 focus-within:ring-ring">
            <textarea
              bind:this={textarea}
              bind:clientWidth={textareaWidth}
              data-slot="chat-composer-input"
              bind:value={body}
              onkeydown={keydown}
              rows="1"
              aria-label="Message the Conference Room"
              placeholder="Ask anything about your guild..."
              class="max-h-50 min-h-6 flex-1 resize-none border-0 bg-transparent px-0 py-1 shadow-none focus:ring-0 text-sm outline-none placeholder:text-muted-foreground"
            ></textarea>
            <Button size="icon-sm" disabled={!body.trim() || sending} onclick={() => write(body.trim())} aria-label="Send message" title="Send message"><SendHorizontal /></Button>
          </div>
        {/if}
      </div>
    </div>
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
      role="separator"
      aria-orientation="vertical"
      aria-label="Resize the Conference Room and the activity feed"
      class="group relative hidden w-3 shrink-0 cursor-col-resize touch-none bg-background md:flex"
      onpointerdown={dragStart}
      data-testid="board-divider"
    >
      <div class="pointer-events-none absolute top-0 bottom-0 left-0 w-px bg-border transition-colors group-hover:bg-foreground/20" aria-hidden="true"></div>
    </div>
    <div class="hidden md:flex md:min-h-0 md:min-w-0 md:flex-1">
      <ActivityFeed />
    </div>
  </div>
  <div class="md:hidden">
    <Sheet.Root bind:open={feedOpen}>
      <Sheet.Trigger>
        {#snippet child({ props })}
          <Button {...props} size="icon" variant="secondary" class="fixed right-4 bottom-20 z-20 size-10 rounded-full shadow-lg" aria-label="Open agent feed"
            ><Activity class="size-4" /></Button
          >
        {/snippet}
      </Sheet.Trigger>
      <Sheet.Content side="bottom" class="h-[70vh] rounded-t-xl p-0">
        <ActivityFeed />
      </Sheet.Content>
    </Sheet.Root>
  </div>
  <HireAgentDialog bind:open={hiring} initialJob="ceo" onhired={() => loadAgents()} />
{/if}

<style>
  /* Paperclip's typing indicator (ui/src/index.css). */
  .typing-dots {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    padding: 4px 0;
  }
  .typing-dots span {
    width: 5px;
    height: 5px;
    border-radius: 999px;
    background: var(--muted-foreground);
    animation: typing-bounce 1.2s ease-in-out infinite;
  }
  .typing-dots span:nth-child(2) {
    animation-delay: 0.15s;
  }
  .typing-dots span:nth-child(3) {
    animation-delay: 0.3s;
  }
  @keyframes typing-bounce {
    0%,
    60%,
    100% {
      transform: translateY(0);
      opacity: 0.4;
    }
    30% {
      transform: translateY(-3px);
      opacity: 1;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .typing-dots span {
      animation: none;
      opacity: 0.6;
    }
  }
</style>
