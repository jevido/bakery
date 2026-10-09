<script lang="ts">
  // A Conversation's messages as a chat thread, from Paperclip's
  // IssueChatThread (ui/src/components/IssueChatThread.tsx; MIT, see NOTICE)
  // trimmed to its message layout: the Conversation owner's messages on the
  // right in a muted bubble, the Agent's on the left with its icon and name,
  // each with when it was written and its Markdown. A /new is the Session
  // boundary: it shows as a "New session" divider, and what came before the
  // newest one is dimmed, since the Agent no longer sees it. Left out with
  // what a Conversation does not have: queued and pending messages, feedback
  // votes, confirmation cards and attachments.
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import Markdown from '@bakery/ui/Markdown.svelte'
  import { ago, formatDate } from './format'
  import type { Comment, WorkAgent } from './work'

  let {
    comments,
    agent,
    boundary = null,
  }: {
    /** The Conversation's Comments, oldest first. */
    comments: Comment[]
    agent: WorkAgent
    /** The newest /new Comment's id: what precedes it is dimmed. */
    boundary?: number | null
  } = $props()

  const isNew = (c: Comment) => !c.deleted && !c.author_agent && c.body.trim() === '/new'
  // The boundary as the Comments hold it, so a /new just written dims the
  // thread before the Conversation is read again.
  const newest = $derived(comments.findLast(isNew)?.id ?? boundary)
  const before = $derived.by(() => {
    const at = comments.findIndex((c) => c.id === newest)
    return new Set(at < 0 ? [] : comments.slice(0, at).map((c) => c.id))
  })
</script>

<div class="flex flex-col gap-4" data-testid="chat-thread">
  {#if comments.length === 0}
    <div class="flex flex-col items-center gap-3 py-16 text-center" data-testid="chat-empty">
      <span class="flex size-12 items-center justify-center rounded-full bg-muted"><AgentIcon icon={agent.icon} class="size-6" /></span>
      <p class="text-sm text-muted-foreground">Say hello to {agent.name}.</p>
    </div>
  {/if}
  {#each comments as c (c.id)}
    {#if isNew(c)}
      <div class={['flex items-center gap-3 py-1 text-xs text-muted-foreground', before.has(c.id) && 'opacity-50']} data-testid="chat-new-session" data-comment={c.id}>
        <span class="h-px flex-1 bg-border"></span>
        <span title={formatDate(c.created_at)}>New session</span>
        <span class="h-px flex-1 bg-border"></span>
      </div>
    {:else}
      {@const mine = !c.author_agent}
      <div
        id="comment-{c.id}"
        class={['flex gap-2.5', mine ? 'justify-end' : 'justify-start', before.has(c.id) && 'opacity-50']}
        data-testid="chat-message"
        data-comment={c.id}
        data-from={mine ? 'member' : 'agent'}
      >
        {#if !mine}
          <span class="mt-6 flex size-8 shrink-0 items-center justify-center rounded-full bg-muted"><AgentIcon icon={c.author_agent?.icon ?? agent.icon} class="size-4" /></span>
        {/if}
        <div class={['flex max-w-[85%] min-w-0 flex-col', mine && 'items-end']}>
          <div class={['mb-1 flex items-center gap-2 px-1', mine ? 'justify-end' : 'justify-start']}>
            <span class="text-sm font-medium text-foreground">{c.author_agent?.name ?? c.author?.name ?? 'Someone'}</span>
            <span class="text-xs text-muted-foreground" title={formatDate(c.created_at)}>{ago(c.created_at)}</span>
          </div>
          <div
            class={[
              'max-w-full min-w-0 overflow-hidden rounded-2xl px-4 py-2.5 break-words',
              mine ? 'rounded-br-sm bg-muted' : 'rounded-bl-sm border border-border bg-background',
              c.deleted && 'bg-muted/50 text-muted-foreground',
            ]}
          >
            {#if c.deleted}
              <span class="text-sm italic">Message deleted</span>
            {:else}
              <Markdown source={c.body} class="text-sm" />
            {/if}
          </div>
        </div>
      </div>
    {/if}
  {/each}
</div>
