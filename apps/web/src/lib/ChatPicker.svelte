<script lang="ts">
  // Paperclip's AgentChatPicker (ui/src/components/AgentChatPicker.tsx; MIT,
  // see NOTICE): "Chat with an agent", a search by name, Job or Title over
  // the Agents that can answer, and picking one opens the asking Member's
  // chat with it. Arrow keys move through the list and Enter opens the
  // highlighted Agent, as cmdk's Command does there. The list is the
  // caller's, already narrowed to the Agents Chats.svelte offers.
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import * as Dialog from '@bakery/ui/components/ui/dialog'
  import type { Agent } from './agents'
  import { go } from './router.svelte'
  import { sidebar } from './sidebar.svelte'

  let {
    open = $bindable(false),
    agents,
    error = false,
    onretry,
  }: {
    open?: boolean
    /** The chattable Agents; null while they load. */
    agents: Agent[] | null
    error?: boolean
    onretry?: () => void
  } = $props()

  let query = $state('')
  let highlighted = $state(0)

  const matches = $derived.by(() => {
    const q = query.trim().toLowerCase()
    if (!agents) return []
    if (!q) return agents
    return agents.filter((a) => [a.name, a.title, a.job_label].some((s) => s?.toLowerCase().includes(q)))
  })

  function reset() {
    query = ''
    highlighted = 0
  }

  function pick(a: Agent) {
    open = false
    reset()
    sidebar.closeDrawer()
    go(`/chats/${a.id}`)
  }

  function keydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      if (matches.length === 0) return
      highlighted = (highlighted + (e.key === 'ArrowDown' ? 1 : -1) + matches.length) % matches.length
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const a = matches[highlighted]
      if (a) pick(a)
    }
  }
</script>

<Dialog.Root
  bind:open
  onOpenChange={(o) => {
    if (!o) reset()
  }}
>
  <Dialog.Content class="gap-0 overflow-hidden p-0 sm:max-w-md" data-testid="chat-picker">
    <div class="px-4 pt-4 pb-3">
      <Dialog.Title>Chat with an agent</Dialog.Title>
    </div>
    <input
      class="w-full border-0 border-y border-border bg-transparent px-4 py-3 text-sm shadow-none outline-none placeholder:text-muted-foreground focus:ring-0"
      placeholder="Search by name or role…"
      aria-label="Search agents by name or role"
      aria-controls="chat-picker-agents"
      aria-activedescendant={matches[highlighted] ? `chat-picker-agent-${matches[highlighted].id}` : undefined}
      value={query}
      oninput={(e) => {
        query = e.currentTarget.value
        highlighted = 0
      }}
      onkeydown={keydown}
    />
    {#if error}
      <div role="alert" class="flex flex-col items-start gap-2 p-4 text-sm">
        <p>Couldn’t load agents. Try again.</p>
        {#if onretry}<Button variant="outline" size="sm" onclick={onretry}>Retry</Button>{/if}
      </div>
    {:else if agents === null}
      <p role="status" class="p-4 text-sm text-muted-foreground">Loading agents…</p>
    {:else if matches.length === 0}
      <div class="flex flex-col items-center gap-2 px-4 py-6 text-center text-sm">
        {#if agents.length}
          <span>No agents match “{query}”</span>
          <span class="text-xs text-muted-foreground">Try another name or role.</span>
          <Button variant="ghost" size="sm" onclick={reset}>Clear search</Button>
        {:else}
          <span>No agents yet.</span>
          <span class="text-xs text-muted-foreground">Create an agent from the Agents page to start chatting.</span>
        {/if}
      </div>
    {:else}
      <div id="chat-picker-agents" role="listbox" aria-label="Agents" class="max-h-80 overflow-y-auto overscroll-contain p-1">
        {#each matches as a, i (a.id)}
          <button
            type="button"
            role="option"
            id="chat-picker-agent-{a.id}"
            aria-selected={i === highlighted}
            data-agent={a.id}
            tabindex="-1"
            class={['flex w-full items-center gap-3 rounded-sm px-3 py-3 text-left text-sm', i === highlighted && 'bg-accent text-accent-foreground']}
            onmousemove={() => (highlighted = i)}
            onclick={() => pick(a)}
          >
            <AgentIcon icon={a.icon} class="size-4 shrink-0" />
            <span class="flex min-w-0 flex-1 flex-col gap-0.5">
              <span class="truncate font-medium">{a.name}</span>
              <span class="truncate text-xs text-muted-foreground">{a.title || a.job_label}</span>
            </span>
            {#if a.status === 'paused'}<span class="text-xs text-muted-foreground">Paused</span>{/if}
          </button>
        {/each}
      </div>
    {/if}
  </Dialog.Content>
</Dialog.Root>
