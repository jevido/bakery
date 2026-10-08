<script lang="ts">
  // Paperclip's Agents page in its streamlined mode (ui/src/pages/Agents.tsx;
  // MIT, see NOTICE): the filter tabs in the path, the List / Org chart
  // toggle and Hire agent in one row, the count, then one EntityRow per
  // Agent with its icon, name, Job and Title, its Manager and Hirer, and its
  // status as AgentStatusBadge draws it. Paperclip's Error and Built-in tabs
  // wait for Runs and built-in agents; Terminated is The Bakery's, since it
  // keeps terminated Agents as records. Paperclip's environment, model and
  // live-run columns belong to the runtime and are left out.
  import { Bot, List, Network, Plus } from '@lucide/svelte'
  import { Button as UiButton } from '$lib/components/ui/button'
  import * as Tabs from '$lib/components/ui/tabs'
  import AgentIcon from '../../lib/AgentIcon.svelte'
  import { agentStatusLabel, listAgents, type Agent, type AgentStatus } from '../../lib/agents'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import HireAgentDialog from '../../lib/HireAgentDialog.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { agentsTabs, go, href, type AgentsTab } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { StatusType } from '../../lib/statusColors'
  import Button from '../../lib/ui/Button.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import EntityRow from '../../lib/EntityRow.svelte'

  let { tab }: { tab: AgentsTab } = $props()

  const tabLabels: Record<AgentsTab, string> = { all: 'All', active: 'Active', paused: 'Paused', terminated: 'Terminated' }
  // Paperclip's agent status hues: idle grey, paused amber; a waiting hire
  // is amber too and a terminated Agent red.
  const statusTones: Record<AgentStatus, StatusType> = { idle: 'neutral', paused: 'warning', pending_approval: 'warning', terminated: 'error' }

  let agents = $state.raw<Agent[] | null>(null)
  let loadError = $state('')
  let view = $state<'list' | 'org'>('list')
  let hiring = $state(false)

  const load = () =>
    listAgents(tab)
      .then((as) => {
        agents = as
        loadError = ''
      })
      .catch((e) => (loadError = e.message))

  $effect(() => {
    void tab
    load()
  })
  $effect(() => breadcrumb.set({ label: 'Agents' }))

  const canHire = $derived(session.can('hire_agents'))
</script>

<div class="chrome space-y-4">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <Tabs.Root value={tab} onValueChange={(v) => go(`/agents/${v}`)}>
      <Tabs.List variant="line" class="justify-start">
        {#each agentsTabs as t (t)}<Tabs.Trigger value={t}>{tabLabels[t]}</Tabs.Trigger>{/each}
      </Tabs.List>
    </Tabs.Root>
    <div class="flex items-center gap-2">
      <div class="flex items-center overflow-hidden rounded-md border border-border" role="group" aria-label="Agent view">
        <UiButton type="button" size="icon-sm" variant={view === 'list' ? 'secondary' : 'ghost'} class="rounded-none" onclick={() => (view = 'list')} title="List view" aria-label="List view" aria-pressed={view === 'list'}>
          <List class="size-3.5" />
        </UiButton>
        <UiButton type="button" size="icon-sm" variant={view === 'org' ? 'secondary' : 'ghost'} class="rounded-none border-l border-border" onclick={() => (view = 'org')} title="Org chart view" aria-label="Org chart view" aria-pressed={view === 'org'}>
          <Network class="size-3.5" />
        </UiButton>
      </div>
      {#if canHire}
        <UiButton size="sm" variant="outline" onclick={() => (hiring = true)}><Plus class="mr-1.5 size-3.5" />Hire agent</UiButton>
      {/if}
    </div>
  </div>

  {#if agents && agents.length > 0}
    <p class="text-xs text-muted-foreground">{agents.length} agent{agents.length !== 1 ? 's' : ''}</p>
  {/if}

  {#if loadError}<p class="text-sm text-destructive">{loadError}</p>{/if}

  {#if agents === null && !loadError}
    <PageSkeleton />
  {:else if agents && view === 'org'}
    <p class="py-8 text-center text-sm text-muted-foreground">No organizational hierarchy defined.</p>
  {:else if agents && agents.length === 0 && tab === 'all'}
    <div class="flex flex-col items-center justify-center py-16 text-center">
      <div class="mb-4 rounded-md bg-muted/50 p-4"><Bot class="size-10 text-muted-foreground/50" /></div>
      <p class="mb-4 text-sm text-muted-foreground">No agents yet. Hire your first agent to get started.</p>
      {#if canHire}<Button variant="highlighted" onclick={() => (hiring = true)}><Plus class="size-4" />Hire agent</Button>{/if}
    </div>
  {:else if agents && agents.length === 0}
    <p class="py-8 text-center text-sm text-muted-foreground">No agents match the selected status.</p>
  {:else if agents}
    <div class="rounded-md border border-border" aria-label="Agents">
      {#each agents as agent (agent.id)}
        <EntityRow
          title={agent.name}
          subtitle={`${agent.job_label}${agent.title ? ` - ${agent.title}` : ''}`}
          href={href(`/agents/${agent.id}`)}
          class={['group py-3', agent.status === 'paused' && tab !== 'paused' && 'opacity-50'].filter(Boolean).join(' ')}
          data-testid="agent-row"
        >
          {#snippet leading()}
            <span class="flex size-8 items-center justify-center rounded-full bg-accent"><AgentIcon icon={agent.icon} class="size-4" /></span>
          {/snippet}
          {#snippet trailing()}
            <div class="hidden items-center gap-3 text-xs text-muted-foreground lg:flex">
              <span class="w-36 truncate" title="Reports to">{agent.reports_to ? `Reports to ${agent.reports_to.name}` : ''}</span>
              <span class="w-36 truncate" title="Hirer">{agent.hirer ? `Hired by ${agent.hirer.name}` : ''}</span>
            </div>
            <span class="flex w-32 justify-end"><StatusBadge type={statusTones[agent.status]} label={agentStatusLabel(agent.status)} /></span>
          {/snippet}
        </EntityRow>
      {/each}
    </div>
  {/if}
</div>

<HireAgentDialog bind:open={hiring} onhired={() => load()} />
