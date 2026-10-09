<script lang="ts">
  // Paperclip's Costs page (ui/src/pages/Costs.production.tsx with the
  // presets of hooks/useDateRange.ts; MIT, see NOTICE): the header with the
  // date range presets, four metric tiles, then the Overview and Budgets
  // tabs. Its tiles count what a subscription can count (tokens, Runs, run
  // time) and show the CLI's reported cost only as an equivalent, never
  // billed. The range lives in the hash query (?range=7d, or
  // ?range=custom&from=2026-10-01&to=2026-10-09), so a reload and a tab
  // switch keep it. Budgets lists every Budget by scope with Paperclip's
  // policy and incident cards, and also sets one for the guild or any Agent
  // or Project from a picker, where Paperclip sets agent and project Budgets
  // only from their pages. Left out: the Providers, Billers and Finance tabs, the
  // per-model breakdown under an Agent and the Inference ledger, which count
  // what an API key is billed (see the agents document).
  import { untrack } from 'svelte'
  import { ArrowUpRight, Coins, DollarSign, Hash, PauseCircle, Play, ReceiptText } from '@lucide/svelte'
  import type { Component } from 'svelte'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import * as Card from '@bakery/ui/components/ui/card'
  import * as Select from '@bakery/ui/components/ui/select'
  import * as Tabs from '@bakery/ui/components/ui/tabs'
  import { listAgents, type Agent } from '../lib/agents'
  import { api } from '../lib/api'
  import BudgetIncidentCard from '../lib/BudgetIncidentCard.svelte'
  import BudgetPolicyCard from '../lib/BudgetPolicyCard.svelte'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import {
    budgetAmount,
    budgetOverview,
    budgetUtilisation,
    costsByAgent,
    costsByProject,
    costSummary,
    type AgentCosts,
    type Budget,
    type BudgetOverview,
    type BudgetScope,
    type CostRange,
    type Figures,
    type ProjectCosts,
  } from '../lib/costs'
  import { runTime, tokens, usd } from '../lib/format'
  import PageSkeleton from '../lib/PageSkeleton.svelte'
  import { go, href, type CostsTab } from '../lib/router.svelte'
  import { session } from '../lib/session.svelte'

  let { tab }: { tab: CostsTab } = $props()

  const presets = [
    { key: 'mtd', label: 'Month to Date' },
    { key: '7d', label: 'Last 7 Days' },
    { key: '30d', label: 'Last 30 Days' },
    { key: 'ytd', label: 'Year to Date' },
    { key: 'all', label: 'All Time' },
    { key: 'custom', label: 'Custom' },
  ] as const
  type Preset = (typeof presets)[number]['key']
  const day = /^\d{4}-\d{2}-\d{2}$/

  // The range as the hash query holds it, read again when Back changes the
  // hash under the open page.
  function fromHash() {
    const q = new URLSearchParams(location.hash.split('?')[1] ?? '')
    const r = q.get('range')
    const preset: Preset = presets.some((p) => p.key === r) ? (r as Preset) : 'mtd'
    const from = q.get('from') ?? ''
    const to = q.get('to') ?? ''
    return { preset, from: day.test(from) ? from : '', to: day.test(to) ? to : '' }
  }
  const initial = fromHash()
  let preset = $state<Preset>(initial.preset)
  let customFrom = $state(initial.from)
  let customTo = $state(initial.to)
  $effect(() => {
    const follow = () => {
      if (!location.hash.startsWith('#/costs')) return
      const f = fromHash()
      preset = f.preset
      customFrom = f.from
      customTo = f.to
    }
    window.addEventListener('hashchange', follow)
    return () => window.removeEventListener('hashchange', follow)
  })
  const rangeQuery = $derived.by(() => {
    const q = new URLSearchParams()
    if (preset !== 'mtd') q.set('range', preset)
    if (preset === 'custom') {
      if (customFrom) q.set('from', customFrom)
      if (customTo) q.set('to', customTo)
    }
    return q.size ? `?${q}` : ''
  })
  $effect(() => {
    const next = `#/costs/${tab}${rangeQuery}`
    if (location.hash !== next) history.replaceState(history.state, '', next)
  })

  const customReady = $derived(preset !== 'custom' || (!!customFrom && !!customTo))

  // The range in RFC 3339 for the API, its end exclusive. A preset reaches
  // to now, so it has no end; a custom range runs from the first day's local
  // midnight up to the midnight after the last day.
  function rangeOf(p: Preset, from: string, to: string): CostRange {
    const now = new Date()
    const at = (d: Date) => ({ from: d.toISOString(), to: '' })
    switch (p) {
      case 'mtd':
        return at(new Date(now.getFullYear(), now.getMonth(), 1))
      case '7d':
        return at(new Date(now.getFullYear(), now.getMonth(), now.getDate() - 7))
      case '30d':
        return at(new Date(now.getFullYear(), now.getMonth(), now.getDate() - 30))
      case 'ytd':
        return at(new Date(now.getFullYear(), 0, 1))
      case 'all':
        return { from: '', to: '' }
      case 'custom': {
        const end = new Date(`${to}T00:00:00`)
        end.setDate(end.getDate() + 1)
        return { from: new Date(`${from}T00:00:00`).toISOString(), to: end.toISOString() }
      }
    }
  }

  let summary = $state.raw<Figures | null>(null)
  let byAgent = $state.raw<AgentCosts[]>([])
  let byProject = $state.raw<ProjectCosts[]>([])
  let loading = $state(false)
  let loadError = $state('')
  let overview = $state.raw<BudgetOverview | null>(null)
  let overviewError = $state('')
  function loadOverview() {
    budgetOverview()
      .then((o) => {
        overview = o
        overviewError = ''
      })
      .catch((e) => (overviewError = e.message))
  }
  loadOverview()

  // Each change of the range asks again; a late answer for a range already
  // left behind is dropped.
  let asked = 0
  $effect(() => {
    void [preset, customFrom, customTo, customReady]
    untrack(() => {
      if (!customReady) return
      const ask = ++asked
      const r = rangeOf(preset, customFrom, customTo)
      loading = true
      Promise.all([costSummary(r), costsByAgent(r), costsByProject(r)])
        .then(([s, a, p]) => {
          if (ask !== asked) return
          summary = s
          byAgent = a
          byProject = p
          loadError = ''
        })
        .catch((e) => {
          if (ask === asked) loadError = e.message
        })
        .finally(() => {
          if (ask === asked) loading = false
        })
    })
  })

  const exact = (n: number) => n.toLocaleString('en-US')
  const amountOf = (b: Pick<Budget, 'metric'>, n: number) => budgetAmount(b.metric, n)
  const utilisation = budgetUtilisation

  // The Budget tile: the open Budget incidents when there are any, else the
  // Guild Budget closest to its amount.
  const guildBudget = $derived(
    (overview?.budgets ?? []).filter((b) => b.scope.type === 'guild').toSorted((a, b) => utilisation(b) - utilisation(a))[0],
  )
  const openIncidents = $derived(overview?.incidents.length ?? 0)
  const budgetTile = $derived(
    openIncidents > 0
      ? {
          value: String(openIncidents),
          subtitle: `${openIncidents} open ${openIncidents === 1 ? 'incident' : 'incidents'} · ${overview!.paused_agents} agents paused · ${overview!.stopped_projects} projects paused`,
        }
      : guildBudget
        ? { value: `${Math.round(utilisation(guildBudget))}%`, subtitle: `${amountOf(guildBudget, guildBudget.observed)} of ${amountOf(guildBudget, guildBudget.amount)}` }
        : { value: 'Open', subtitle: 'No guild budget' },
  )

  // The hard incidents are the ones the Board resolves; a soft one shows
  // only in the count.
  const hardIncidents = $derived((overview?.incidents ?? []).filter((i) => i.threshold === 'hard' && i.status === 'open'))
  const pendingApprovals = $derived(hardIncidents.filter((i) => i.approval_id).length)
  const sections = [
    { type: 'guild', title: 'Guild budget', description: "The guild's monthly cap on what all its agents use." },
    { type: 'agent', title: 'Agent budgets', description: 'Monthly caps for individual agents.' },
    { type: 'project', title: 'Project budgets', description: 'Lifetime caps on the runs of issues in a project.' },
  ] as const

  // Setting a Budget for any scope: the guild, an Agent or a Project.
  let agents = $state.raw<Agent[]>([])
  let projects = $state.raw<{ id: number; name: string }[]>([])
  if (session.can('manage_budgets')) {
    listAgents()
      .then((a) => (agents = a))
      .catch(() => {})
    api<{ projects: { id: number; name: string }[] }>('GET', '/projects')
      .then((r) => (projects = r.projects))
      .catch(() => {})
  }
  const scopes = $derived<BudgetScope[]>([
    ...(session.guild ? [{ type: 'guild' as const, id: session.guild.id, name: session.guild.name }] : []),
    ...agents.filter((a) => a.status !== 'pending_approval').map((a) => ({ type: 'agent' as const, id: a.id, name: a.name })),
    ...projects.map((p) => ({ type: 'project' as const, id: p.id, name: p.name })),
  ])
  let newScope = $state('')
  const picked = $derived(scopes.find((s) => `${s.type}:${s.id}` === newScope) ?? null)

  $effect(() => breadcrumb.set({ label: 'Costs' }))
</script>

{#snippet tile(label: string, value: string, subtitle: string, Icon: Component<{ class?: string }>, title?: string)}
  <Card.Root class="block p-4" data-tile={label}>
    <div class="flex items-center justify-between gap-3">
      <div class="min-w-0">
        <div class="text-(length:--text-micro) tracking-(--tracking-eyebrow) text-muted-foreground uppercase">{label}</div>
        <div class="mt-2 text-2xl font-semibold tabular-nums" {title} data-value>{value}</div>
        <div class="mt-1 text-xs leading-5 text-muted-foreground">{subtitle}</div>
      </div>
      <div class="flex size-9 shrink-0 items-center justify-center rounded-md border border-border">
        <Icon class="size-4 text-muted-foreground" />
      </div>
    </div>
  </Card.Root>
{/snippet}

{#snippet figures(f: Figures)}
  <div class="text-right text-sm tabular-nums">
    <div class="font-medium" title="{exact(f.tokens)} tokens">{tokens(f.tokens)} tokens</div>
    <div class="text-xs text-muted-foreground">in {tokens(f.input_tokens + f.cached_input_tokens)} · out {tokens(f.output_tokens)}</div>
    <div class="text-xs text-muted-foreground">{exact(f.runs)} {f.runs === 1 ? 'run' : 'runs'} · {runTime(f.run_time_ms)} · {usd(f.cost_equivalent_usd)}</div>
  </div>
{/snippet}

<div class="chrome space-y-6">
  <div class="space-y-5">
    <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
      <div>
        <h1 class="text-3xl font-semibold tracking-tight">Costs</h1>
        <p class="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
          Tokens, runs and run time your agents used, with the cost the CLI reports as an equivalent, never billed.
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2" role="toolbar" aria-label="Date range">
        {#each presets as p (p.key)}
          <Button variant={preset === p.key ? 'secondary' : 'ghost'} size="sm" aria-pressed={preset === p.key} onclick={() => (preset = p.key)}>{p.label}</Button>
        {/each}
      </div>
    </div>

    {#if preset === 'custom'}
      <div class="flex flex-wrap items-center gap-2 border border-border p-3">
        <input
          type="date"
          aria-label="From"
          bind:value={customFrom}
          class="h-9 rounded-md border border-input bg-background px-3 text-sm text-foreground"
        />
        <span class="text-sm text-muted-foreground">to</span>
        <input type="date" aria-label="To" bind:value={customTo} class="h-9 rounded-md border border-input bg-background px-3 text-sm text-foreground" />
      </div>
    {/if}

    <div class="grid gap-3 lg:grid-cols-4">
      {@render tile(
        'Tokens',
        tokens(summary?.tokens ?? 0),
        `${tokens(summary?.input_tokens ?? 0)} in · ${tokens(summary?.output_tokens ?? 0)} out · ${tokens(summary?.cached_input_tokens ?? 0)} cached`,
        Hash,
        `${exact(summary?.tokens ?? 0)} tokens`,
      )}
      {@render tile('Runs', exact(summary?.runs ?? 0), `${runTime(summary?.run_time_ms ?? 0)} run time`, Play)}
      {@render tile('Cost equivalent', usd(summary?.cost_equivalent_usd ?? 0), 'As the CLI reports it, never billed', DollarSign)}
      {@render tile('Budget', budgetTile.value, budgetTile.subtitle, Coins)}
    </div>
  </div>

  <Tabs.Root value={tab} onValueChange={(v) => go(`/costs/${v}${rangeQuery}`)}>
    <Tabs.List variant="line" class="justify-start">
      <Tabs.Trigger value="overview">Overview</Tabs.Trigger>
      <Tabs.Trigger value="budgets">Budgets</Tabs.Trigger>
    </Tabs.List>
  </Tabs.Root>

  {#if tab === 'overview'}
    <div class="space-y-4">
      {#if !customReady}
        <p class="text-sm text-muted-foreground">Select a start and end date to load data.</p>
      {:else if loading && !summary}
        <PageSkeleton />
      {:else if loadError}
        <p class="text-sm text-destructive">{loadError}</p>
      {:else}
        {#if hardIncidents.length > 0}
          <div class="grid gap-4 xl:grid-cols-2">
            {#each hardIncidents.slice(0, 2) as incident (incident.id)}
              <BudgetIncidentCard {incident} onresolved={loadOverview} />
            {/each}
          </div>
        {/if}
        <div class="grid gap-4 xl:grid-cols-[1.25fr_0.95fr]">
          <Card.Root>
            <Card.Header class="px-5 pt-5 pb-2">
              <Card.Title class="text-base">By agent</Card.Title>
              <Card.Description>What each agent used in the selected period.</Card.Description>
            </Card.Header>
            <Card.Content class="space-y-2 px-5 pt-2 pb-5" aria-label="By agent">
              {#if byAgent.length === 0}
                <p class="text-sm text-muted-foreground">No runs yet.</p>
              {:else}
                {#each byAgent as row (row.agent.id)}
                  <div class="flex items-start justify-between gap-3 border border-border px-4 py-3" data-agent={row.agent.id}>
                    <div class="flex min-w-0 items-center gap-2">
                      <a href={href(`/agents/${row.agent.id}`)} class="flex min-w-0 items-center gap-2 text-sm text-inherit no-underline hover:underline">
                        <span class="inline-flex size-6 shrink-0 items-center justify-center rounded-full bg-muted" aria-hidden="true">
                          <AgentIcon icon={row.agent.icon} class="size-3.5" />
                        </span>
                        <span class="truncate">{row.agent.name}</span>
                      </a>
                      {#if row.agent.status === 'terminated'}<StatusBadge status="terminated" />{/if}
                    </div>
                    {@render figures(row)}
                  </div>
                {/each}
              {/if}
            </Card.Content>
          </Card.Root>

          <Card.Root>
            <Card.Header class="px-5 pt-5 pb-2">
              <Card.Title class="text-base">By project</Card.Title>
              <Card.Description>Runs on issues in each project.</Card.Description>
            </Card.Header>
            <Card.Content class="space-y-2 px-5 pt-2 pb-5" aria-label="By project">
              {#if byProject.length === 0}
                <p class="text-sm text-muted-foreground">No project-attributed runs yet.</p>
              {:else}
                {#each byProject as row (row.project?.id ?? 0)}
                  <div class="flex items-start justify-between gap-3 border border-border px-3 py-2 text-sm" data-project={row.project?.id ?? 0}>
                    {#if row.project}
                      <a href={href(`/project/${row.project.id}`)} class="truncate text-inherit no-underline hover:underline">{row.project.name}</a>
                    {:else}
                      <span class="truncate text-muted-foreground">No project</span>
                    {/if}
                    {@render figures(row)}
                  </div>
                {/each}
              {/if}
            </Card.Content>
          </Card.Root>
        </div>
      {/if}
    </div>
  {:else}
    <div class="space-y-4" aria-label="Budgets">
      {#if overviewError}
        <p class="text-sm text-destructive">{overviewError}</p>
      {:else if !overview}
        <PageSkeleton />
      {:else}
        <Card.Root class="gap-0 border-border/70 bg-[linear-gradient(180deg,rgba(255,255,255,0.05),rgba(255,255,255,0.02))] py-0">
          <Card.Header class="px-5 pt-5 pb-3">
            <Card.Title class="text-base">Budget control</Card.Title>
            <Card.Description>
              Hard stops on tokens, runs and run time for the guild, agents and projects. Your Claude subscription's own limits are separate.
            </Card.Description>
          </Card.Header>
          <Card.Content class="grid gap-3 px-5 pt-0 pb-5 md:grid-cols-4">
            {@render tile('Active incidents', String(overview.incidents.length), 'Open warnings and hard stops', ReceiptText)}
            {@render tile('Pending approvals', String(pendingApprovals), 'Budget overrides waiting for the board', ArrowUpRight)}
            {@render tile('Paused agents', String(overview.paused_agents), 'Agent runs blocked by budget', PauseCircle)}
            {@render tile('Paused projects', String(overview.stopped_projects), 'Project runs blocked by budget', DollarSign)}
          </Card.Content>
        </Card.Root>

        {#if hardIncidents.length > 0}
          <section class="space-y-3" aria-label="Active incidents">
            <div>
              <h2 class="text-lg font-semibold">Active incidents</h2>
              <p class="text-sm text-muted-foreground">Resolve hard stops here by raising the budget or keeping the scope paused.</p>
            </div>
            <div class="grid gap-4 xl:grid-cols-2">
              {#each hardIncidents as incident (incident.id)}
                <BudgetIncidentCard {incident} onresolved={loadOverview} />
              {/each}
            </div>
          </section>
        {/if}

        {#each sections as section (section.type)}
          {@const rows = overview.budgets.filter((b) => b.scope.type === section.type)}
          {#if rows.length > 0}
            <section class="space-y-3" aria-label={section.title}>
              <div>
                <h2 class="text-lg font-semibold">{section.title}</h2>
                <p class="text-sm text-muted-foreground">{section.description}</p>
              </div>
              <div class="grid gap-4 xl:grid-cols-2">
                {#each rows as b (b.id)}
                  <BudgetPolicyCard budget={b} scope={b.scope} onsaved={loadOverview} />
                {/each}
              </div>
            </section>
          {/if}
        {/each}

        {#if overview.budgets.length === 0}
          <Card.Root>
            <Card.Content class="px-5 py-8 text-sm text-muted-foreground">
              No budgets yet. Set one for the guild, an agent or a project here, or from the agent's or project's page.
            </Card.Content>
          </Card.Root>
        {/if}

        {#if session.can('manage_budgets')}
          <section class="space-y-3" aria-label="Add a budget">
            <div class="flex flex-wrap items-end justify-between gap-3">
              <div>
                <h2 class="text-lg font-semibold">Add a budget</h2>
                <p class="text-sm text-muted-foreground">Cap the guild, any agent or any project.</p>
              </div>
              <Select.Root type="single" bind:value={newScope}>
                <Select.Trigger class="w-64" aria-label="Budget scope">{picked ? `${picked.type}: ${picked.name}` : 'Choose a scope'}</Select.Trigger>
                <Select.Content>
                  {#each scopes as s (`${s.type}:${s.id}`)}
                    <Select.Item value="{s.type}:{s.id}" label="{s.type}: {s.name}" />
                  {/each}
                </Select.Content>
              </Select.Root>
            </div>
            {#if picked}
              {#key newScope}
                <div class="grid gap-4 xl:grid-cols-2">
                  <BudgetPolicyCard
                    budget={null}
                    scope={picked}
                    onsaved={() => {
                      newScope = ''
                      loadOverview()
                    }}
                  />
                </div>
              {/key}
            {/if}
          </section>
        {/if}
      {/if}
    </div>
  {/if}
</div>
