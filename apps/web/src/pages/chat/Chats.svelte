<script lang="ts">
  // Paperclip's AgentChats (ui/src/pages/AgentChats.tsx; MIT, see NOTICE):
  // "Who would you like to talk to?" with the first six of the Guild's
  // Agents that can answer, each opening the asking Member's Conversation
  // with it. Visiting opens nothing: the chat page opens the Conversation
  // with its first message. Without manage_work the Agents are listed but
  // none opens. Left out: Paperclip's experimental switch (Chat is always
  // on here).
  import { MessageCircle } from '@lucide/svelte'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import { listAgents, type Agent } from '../../lib/agents'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import { go, href } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'

  let agents = $state.raw<Agent[] | null>(null)
  let loadError = $state('')
  let opening = $state<number | null>(null)

  const canChat = $derived(session.can('manage_work'))

  $effect(() => breadcrumb.set({ label: 'Chat' }))

  function load() {
    loadError = ''
    listAgents()
      .then((as) => (agents = as.filter((a) => a.status !== 'terminated' && a.status !== 'pending_approval')))
      .catch(() => (loadError = 'Couldn’t load your agents.'))
  }
  load()

  function open(a: Agent) {
    opening = a.id
    go(`/chats/${a.id}`)
  }
</script>

<div class="mx-auto flex h-full max-w-xl flex-col justify-center gap-6 px-4 py-12">
  <div class="flex flex-col gap-3">
    <MessageCircle class="size-6 text-muted-foreground" />
    <h1 class="text-xl font-semibold">Who would you like to talk to?</h1>
    <p class="text-sm leading-relaxed text-muted-foreground">Ask a question, think through an idea, or plan the next step with your team.</p>
  </div>
  {#if loadError}
    <div role="alert" class="flex flex-col items-start gap-3">
      <p class="text-sm">{loadError}</p>
      <Button variant="outline" onclick={load}>Try again</Button>
    </div>
  {:else if agents === null}
    <p role="status" class="text-sm text-muted-foreground">Loading agents…</p>
  {:else}
    <div class="grid grid-cols-1 gap-2 sm:grid-cols-2" data-testid="chat-agents">
      {#each agents.slice(0, 6) as a (a.id)}
        <button
          type="button"
          disabled={!canChat || opening !== null}
          onclick={() => open(a)}
          data-agent={a.id}
          class="flex items-center gap-3 rounded-lg border border-border p-4 text-left hover:bg-accent/50 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-50"
        >
          <span class="flex size-8 shrink-0 items-center justify-center rounded-full bg-muted"><AgentIcon icon={a.icon} class="size-4" /></span>
          <span class="flex min-w-0 flex-col gap-1">
            <span class="truncate text-sm font-medium">{a.name}</span>
            <span class="text-xs text-muted-foreground">{opening === a.id ? 'Opening chat…' : a.title || a.job_label}</span>
          </span>
        </button>
      {/each}
    </div>
    {#if agents.length === 0}
      <p class="text-sm text-muted-foreground">Add an agent to start a conversation.</p>
    {:else if !canChat}
      <p class="text-sm text-muted-foreground">You cannot start a chat in this guild.</p>
    {/if}
  {/if}
  <Button variant="ghost" class="self-start" href={href('/agents/all')}>Browse all agents</Button>
</div>
