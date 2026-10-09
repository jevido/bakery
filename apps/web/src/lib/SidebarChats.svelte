<script lang="ts">
  // Paperclip's SidebarAgentChats and AgentChatSidebar
  // (ui/src/components/SidebarAgentChats.tsx, AgentChatSidebar.tsx; MIT, see
  // NOTICE): the Chats section under the sidebar's sections, with the
  // asking Member's starred Agents, the Guild's first Agent and the ones
  // they chatted with recently (recentChats.ts orders them), each with a
  // star on hover, and "Chat with an agent" in its header opening the
  // picker. Only Agents that can answer are listed, as on Chats.svelte. On
  // the rail the header is a divider, as SidebarSection draws it, and the
  // star and the header button are left out.
  import { SquarePen, Star } from '@lucide/svelte'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import { cn } from '@bakery/ui/utils'
  import { listAgents, type Agent } from './agents'
  import ChatPicker from './ChatPicker.svelte'
  import { orderChatAgents } from './recentChats'
  import { chatLists, toggleChatStar } from './recentChats.svelte'
  import { href, router } from './router.svelte'
  import { session } from './session.svelte'
  import { sidebar } from './sidebar.svelte'
  import SidebarNavItem from './SidebarNavItem.svelte'

  let agents = $state.raw<Agent[] | null>(null)
  let loadError = $state(false)
  let pickerOpen = $state(false)

  function load() {
    loadError = false
    listAgents()
      .then((as) => (agents = as.filter((a) => a.status !== 'terminated' && a.status !== 'pending_approval')))
      .catch(() => (loadError = true))
  }

  // Read again on every Guild switch and route change, so a hired,
  // renamed or terminated Agent shows up as the sidebar's counts do.
  $effect(() => {
    void router.route
    void session.guild?.id
    load()
  })

  const ordered = $derived(agents ? orderChatAgents(agents, chatLists.starred, chatLists.recent) : [])
  const activeId = $derived(router.route.name === 'chat' ? router.route.agentId : null)
</script>

<section aria-label="Chats" class="group/chats flex flex-col" data-testid="sidebar-chats">
  <div class="relative px-3 py-1.5 pointer-coarse:py-1">
    <div class="flex min-h-6 min-w-0 items-center">
      {#if sidebar.rail}
        <span class="sr-only">Chats</span>
        <div class="h-px w-full bg-border/60" aria-hidden="true"></div>
      {:else}
        <div class="inline-flex max-w-full min-w-0 items-center px-1 py-0.5">
          <span class="font-mono text-(length:--text-nano) font-medium tracking-widest text-muted-foreground/60 uppercase">Chats</span>
        </div>
        <button
          type="button"
          aria-label="Chat with an agent"
          title="Chat with an agent"
          onclick={() => (pickerOpen = true)}
          class="absolute top-1/2 right-2 flex size-6 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground opacity-0 transition-opacity group-hover/chats:opacity-100 hover:bg-accent hover:text-foreground focus-visible:opacity-100 pointer-coarse:opacity-100"
        >
          <SquarePen class="size-3.5" aria-hidden="true" />
        </button>
      {/if}
    </div>
  </div>
  <div class="mt-0.5 flex flex-col gap-0.5">
    {#each ordered as a (a.id)}
      {@const pinned = chatLists.starred.includes(a.id)}
      <div class="group/agent-chat relative" data-chat-agent={a.id}>
        <SidebarNavItem href={href(`/chats/${a.id}`)} label={a.name} active={activeId === a.id} class={sidebar.rail ? undefined : 'pr-9'}>
          {#snippet iconNode()}<AgentIcon icon={a.icon} class="size-4 shrink-0" />{/snippet}
        </SidebarNavItem>
        {#if !sidebar.rail}
          <button
            type="button"
            aria-label="{pinned ? 'Unstar' : 'Star'} {a.name}"
            aria-pressed={pinned}
            title={pinned ? 'Unstar agent' : 'Star agent to pin'}
            onclick={() => toggleChatStar(a.id)}
            class={cn(
              'absolute top-1/2 right-4 flex size-6 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground opacity-0 transition-opacity group-hover/agent-chat:opacity-100 hover:text-foreground focus-visible:opacity-100 pointer-coarse:opacity-100',
              pinned && 'opacity-100',
            )}
          >
            <Star class={cn('size-3.5', pinned && 'fill-current')} aria-hidden="true" />
          </button>
        {/if}
      </div>
    {/each}
  </div>
</section>

<ChatPicker bind:open={pickerOpen} {agents} error={loadError} onretry={load} />
