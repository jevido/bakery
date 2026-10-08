<script lang="ts">
  // Paperclip's Agents page in its streamlined mode (ui/src/pages/Agents.tsx;
  // MIT, see NOTICE): the filter tabs in the path, the List / Org chart
  // toggle and Hire agent in one row, the count, then one EntityRow per
  // Agent with its icon, name, Job and Title, its Manager and Hirer, and its
  // status as AgentStatusBadge draws it. Paperclip's Error and Built-in tabs
  // wait for Runs and built-in agents; Terminated is The Bakery's, since it
  // keeps terminated Agents as records. Paperclip's environment, model and
  // live-run columns belong to the runtime and are left out. The Org chart
  // view shows GET /api/org through OrgChart, filtered by the tab as
  // Paperclip's filterOrgTree does; the view lives in the hash query
  // (?view=org) so a reload and the tabs keep it.
  import { Bot, List, Network, Plus } from '@lucide/svelte'
  import { Button as UiButton } from '@bakery/ui/components/ui/button'
  import * as Tabs from '@bakery/ui/components/ui/tabs'
  import { getOrg, listAgents, type Agent, type AgentStatus, type OrgNode } from '../../lib/agents'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import HireAgentDialog from '../../lib/HireAgentDialog.svelte'
  import OrgChart from '../../lib/OrgChart.svelte'
  import { filterOrgTree } from '../../lib/orgLayout'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { agentsTabs, go, href, type AgentsTab } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import Button from '../../lib/ui/Button.svelte'
  import AgentRow from '@bakery/ui/AgentRow.svelte'

  let { tab }: { tab: AgentsTab } = $props()

  const tabLabels: Record<AgentsTab, string> = { all: 'All', active: 'Active', paused: 'Paused', terminated: 'Terminated' }

  let agents = $state.raw<Agent[] | null>(null)
  let loadError = $state('')
  // The statuses each tab shows, as GET /api/agents?status= filters them.
  const tabStatuses: Record<AgentsTab, AgentStatus[]> = {
    all: ['pending_approval', 'idle', 'paused'],
    active: ['idle'],
    paused: ['paused'],
    terminated: ['terminated'],
  }

  const viewInHash = () => (new URLSearchParams(location.hash.split('?')[1] ?? '').get('view') === 'org' ? 'org' : 'list')
  let org = $state.raw<OrgNode[] | null>(null)
  let view = $state<'list' | 'org'>(viewInHash())
  let hiring = $state(false)

  const load = () =>
    Promise.all([listAgents(tab), getOrg()])
      .then(([as, tree]) => {
        agents = as
        org = tree
        loadError = ''
      })
      .catch((e) => (loadError = e.message))

  const viewQuery = $derived(view === 'org' ? '?view=org' : '')
  // The view into the hash query, in place: no history entry per switch.
  $effect(() => {
    const next = `#/agents/${tab}${viewQuery}`
    if (location.hash !== next) history.replaceState(history.state, '', next)
  })

  const shownOrg = $derived(filterOrgTree(org ?? [], tabStatuses[tab]))

  $effect(() => {
    void tab
    load()
  })
  $effect(() => breadcrumb.set({ label: 'Agents' }))

  const canHire = $derived(session.can('hire_agents'))
</script>

<div class="chrome space-y-4">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <Tabs.Root value={tab} onValueChange={(v) => go(`/agents/${v}${viewQuery}`)}>
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
  {:else if org && view === 'org'}
    {#if shownOrg.length > 0}
      <OrgChart org={shownOrg} />
    {:else if org.length > 0 || tab === 'terminated'}
      <p class="py-8 text-center text-sm text-muted-foreground">No agents match the selected status.</p>
    {:else}
      <p class="py-8 text-center text-sm text-muted-foreground">No organizational hierarchy defined.</p>
    {/if}
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
        <AgentRow {agent} href={href(`/agents/${agent.id}`)} dimmed={agent.status === 'paused' && tab !== 'paused'} />
      {/each}
    </div>
  {/if}
</div>

<HireAgentDialog bind:open={hiring} onhired={() => load()} />
