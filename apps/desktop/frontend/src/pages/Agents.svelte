<script lang="ts">
  // The Guild's Agents, as the dashboard's Agents page lists them
  // (apps/web/src/pages/agents/Agents.svelte, from Paperclip's
  // ui/src/pages/Agents.tsx; MIT, see NOTICE): the filter tabs, the count,
  // then one AgentRow per Agent. "Runs on this desktop" marks the Agents the
  // person hired, which this desktop runs. No Hire agent and no Org chart:
  // both stay in the dashboard. The list follows the `agents` events of the
  // Go side's 15-second refresh.
  import { Bot } from '@lucide/svelte'
  import * as Tabs from '@bakery/ui/components/ui/tabs'
  import AgentRow from '@bakery/ui/AgentRow.svelte'
  import { agents as listAgents, onEvent, type Agent, type AgentsEvent, type AgentsTab } from '../lib/desktop'
  import { shownGuilds } from '../lib/guilds.svelte'
  import { agentsPath, agentsTabs, go, href } from '../lib/router.svelte'

  let { address, bakery, guild, tab }: { address: string; bakery: number; guild: number; tab: AgentsTab } = $props()

  const tabLabels: Record<AgentsTab, string> = { all: 'All', active: 'Active', paused: 'Paused', terminated: 'Terminated' }

  let agents = $state.raw<Agent[] | null>(null)
  let loadError = $state('')

  $effect(() => {
    const [a, g, t] = [address, guild, tab]
    agents = null
    loadError = ''
    listAgents(a, g, t).then(
      (as) => {
        if (a === address && g === guild && t === tab) agents = as
      },
      (e: Error) => (loadError = e.message),
    )
  })

  $effect(() =>
    onEvent<AgentsEvent>('agents', (e) => {
      if (e.address === address && e.guild_id === guild && e.status === tab) agents = e.agents ?? []
    }),
  )

  const meID = $derived(shownGuilds.member?.id)
</script>

<div class="chrome space-y-4">
  <Tabs.Root value={tab} onValueChange={(v) => go(agentsPath(bakery, guild, v as AgentsTab))}>
    <Tabs.List variant="line" class="justify-start">
      {#each agentsTabs as t (t)}<Tabs.Trigger value={t}>{tabLabels[t]}</Tabs.Trigger>{/each}
    </Tabs.List>
  </Tabs.Root>

  {#if agents && agents.length > 0}
    <p class="text-xs text-muted-foreground">{agents.length} agent{agents.length !== 1 ? 's' : ''}</p>
  {/if}

  {#if loadError}
    <p class="text-sm text-destructive" role="alert">{loadError}</p>
  {:else if agents === null}
    <p class="py-8 text-center text-sm text-muted-foreground">Loading agents…</p>
  {:else if agents.length === 0 && tab === 'all'}
    <div class="flex flex-col items-center justify-center py-16 text-center">
      <div class="mb-4 rounded-md bg-muted/50 p-4"><Bot class="size-10 text-muted-foreground/50" /></div>
      <p class="text-sm text-muted-foreground">No agents yet. Hire one in The Bakery to run it on this desktop.</p>
    </div>
  {:else if agents.length === 0}
    <p class="py-8 text-center text-sm text-muted-foreground">No agents match the selected status.</p>
  {:else}
    <div class="rounded-md border border-border" aria-label="Agents">
      {#each agents as agent (agent.id)}
        <AgentRow {agent} href={href(agentsPath(bakery, guild, agent.id))} dimmed={agent.status === 'paused' && tab !== 'paused'}>
          {#snippet badges()}
            {#if meID !== undefined && agent.hirer?.id === meID}
              <span class="inline-flex items-center rounded-full border border-border px-2 py-0.5 text-xs font-medium whitespace-nowrap text-muted-foreground" data-testid="runs-here">
                Runs on this desktop
              </span>
            {/if}
          {/snippet}
        </AgentRow>
      {/each}
    </div>
  {/if}
</div>
