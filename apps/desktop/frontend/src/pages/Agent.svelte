<script lang="ts">
  // One Agent, read-only: the header and properties of the dashboard's Agent
  // page (apps/web/src/pages/agents/Agent.svelte, from Paperclip's
  // AgentDetail; MIT, see NOTICE): the icon, name and Job · Title, the
  // Identity card (status, Job, Title, Reports to, Hirer) and the Roles. It
  // is changed in the dashboard, which "Open in The Bakery" opens.
  import { ExternalLink } from '@lucide/svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
  import { agentStatusText, agentStatusTones } from '@bakery/ui/agentStatus'
  import { agent as getAgent, onEvent, openInBrowser, runs as listRuns, type Agent, type Run, type RunsEvent } from '../lib/desktop'
  import { shownGuilds } from '../lib/guilds.svelte'
  import RunLedger from '../lib/RunLedger.svelte'
  import { agentsPath, href } from '../lib/router.svelte'

  let { address, bakery, guild, id }: { address: string; bakery: number; guild: number; id: number } = $props()

  let agent = $state.raw<Agent | null>(null)
  let loadError = $state('')
  let runs = $state.raw<Run[]>([])

  $effect(() => {
    const [a, g, i] = [address, guild, id]
    agent = null
    loadError = ''
    runs = []
    getAgent(a, g, i).then(
      (x) => {
        if (a === address && g === guild && i === id) agent = x
      },
      (e: Error) => (loadError = e.message === 'not found' ? 'This agent does not exist or you cannot see it.' : e.message),
    )
    listRuns(a, g, i).then((rs) => {
      if (a === address && g === guild && i === id) runs = rs
    })
  })

  // A Run started, reported or ended on this desktop refreshes the ledger
  // with no fixed delay; an issue page elsewhere may have started it.
  $effect(() =>
    onEvent<RunsEvent>('runs', (e) => {
      if (e.address === address && e.agent_id === id) listRuns(address, guild, id).then((rs) => (runs = rs))
    }),
  )

  const runsHere = $derived(agent !== null && shownGuilds.member !== null && agent.hirer?.id === shownGuilds.member.id)
</script>

{#snippet row(label: string, value: import('svelte').Snippet)}
  <div class="flex items-start gap-3 py-1.5" data-property-row={label}>
    <span class="mt-0.5 w-24 shrink-0 text-xs text-muted-foreground">{label}</span>
    <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">{@render value()}</div>
  </div>
{/snippet}

{#snippet muted(text: string)}<span class="text-sm text-muted-foreground">{text}</span>{/snippet}

{#if loadError}
  <p class="text-sm text-destructive" role="alert">{loadError}</p>
{:else if agent === null}
  <p class="py-8 text-center text-sm text-muted-foreground">Loading agent…</p>
{:else}
  {@const a = agent}
  <div class="mx-auto max-w-5xl space-y-8" data-testid="agent">
    <header class="flex flex-wrap items-center justify-between gap-5 border-b border-border pb-6">
      <div class="flex min-w-0 items-center gap-4">
        <span role="img" aria-label={`${a.name} icon`} class="flex size-16 shrink-0 items-center justify-center rounded-full bg-accent">
          <AgentIcon icon={a.icon} class="size-8" />
        </span>
        <div class="min-w-0 space-y-1">
          <h2 class="truncate text-2xl font-semibold tracking-tight">{a.name}</h2>
          <div class="flex items-center gap-1 text-xs text-muted-foreground">
            <span class="px-1">{a.job_label}</span>
            {#if a.title}<span>·</span><span>{a.title}</span>{/if}
          </div>
        </div>
      </div>
      <Button variant="outline" size="sm" onclick={() => openInBrowser(`${address}/#/agents/${a.id}`)}>
        <ExternalLink class="size-3.5" />
        Open in The Bakery
      </Button>
    </header>

    <div class="grid gap-4 md:grid-cols-2">
      <section class="rounded-lg border border-border p-4" aria-labelledby="agent-identity-heading">
        <div class="mb-4 flex items-center justify-between gap-3">
          <h3 id="agent-identity-heading" class="text-sm font-medium">Identity</h3>
          <div class="flex items-center gap-2">
            {#if runsHere}
              <span class="inline-flex items-center rounded-full border border-border px-2 py-0.5 text-xs font-medium text-muted-foreground" data-testid="runs-here">
                Runs on this desktop
              </span>
            {/if}
            <StatusBadge type={agentStatusTones[a.status]} label={agentStatusText(a)} />
          </div>
        </div>
        <div class="space-y-1">
          {#snippet job()}<span class="text-sm">{a.job_label}</span>{/snippet}
          {@render row('Job', job)}
          {#snippet title()}
            {#if a.title}<span class="text-sm">{a.title}</span>{:else}{@render muted('Not set')}{/if}
          {/snippet}
          {@render row('Title', title)}
          {#snippet reportsTo()}
            {#if a.reports_to}
              <a href={href(agentsPath(bakery, guild, a.reports_to.id))} class="text-sm hover:underline">{a.reports_to.name}</a>
            {:else}
              {@render muted('Board')}
            {/if}
          {/snippet}
          {@render row('Reports to', reportsTo)}
          {#snippet hirer()}
            {#if a.hirer}<span class="text-sm">{a.hirer.name}</span>{:else}{@render muted('Unknown')}{/if}
          {/snippet}
          {@render row('Hirer', hirer)}
        </div>
      </section>

      <section class="rounded-lg border border-border p-4" aria-labelledby="agent-capabilities-heading">
        <h3 id="agent-capabilities-heading" class="mb-3 text-sm font-medium">Capabilities</h3>
        {#if a.capabilities}
          <p class="text-sm whitespace-pre-wrap">{a.capabilities}</p>
        {:else}
          <p class="text-sm text-muted-foreground">No capability summary has been added.</p>
        {/if}
      </section>

      <section class="rounded-lg border border-border p-4 md:col-span-2" aria-labelledby="agent-roles-heading">
        <h3 id="agent-roles-heading" class="mb-3 text-sm font-medium">Roles</h3>
        <div class="flex flex-wrap items-center gap-1.5" data-testid="agent-roles">
          {#each a.roles ?? [] as r (r.id)}
            <span class="inline-flex items-center gap-1.5 rounded-full border border-border px-2 py-0.5 text-xs" data-role={r.name}>
              <span class="inline-block size-2 shrink-0 rounded-full" style:background-color={r.color}></span>{r.name}
            </span>
          {:else}
            {@render muted('Only @everyone.')}
          {/each}
        </div>
      </section>

      <section class="rounded-lg border border-border p-4 md:col-span-2" aria-labelledby="agent-runs-heading">
        <h3 id="agent-runs-heading" class="mb-3 text-sm font-medium">Runs</h3>
        <RunLedger {address} guildID={guild} {runs} />
      </section>
    </div>
  </div>
{/if}
