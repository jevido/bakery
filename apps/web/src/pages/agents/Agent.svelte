<script lang="ts">
  // Paperclip's AgentDetail (ui/src/pages/AgentDetail.tsx) with only what
  // has meaning without Runs (MIT, see NOTICE): the header with the Agent
  // icon, name and Job · Title, and AgentActionButtons
  // (ui/src/components/AgentActionButtons.tsx: Pause / Resume and the
  // overflow menu with Copy Agent ID and Terminate, and Run heartbeat),
  // after a Chat button to the asking Member's Conversation with it;
  // then AgentOverview's cards (Identity, Capabilities) with
  // AgentProperties's rows, and AgentConfigForm's Run Policy card trimmed to
  // the Heartbeat policy (Heartbeat on interval, Wake on demand). Clear
  // error, Reset sessions, Duplicate and Assign Task are left out, and so
  // are Paperclip's other tabs, so the page has no tab bar. The Roles card is
  // The Bakery's, in the place of Paperclip's Governance permissions: the
  // Roles the Agent holds as chips, and only Roles below both the asker's
  // highest and the Hirer's highest can be added. Whoever may manage the
  // Agent (can_manage) edits it in place while it is idle or paused; the
  // server stays the judge of every change. A Runs card under the others
  // lists the Agent's last 20 Runs as Paperclip's Runs tab does, each
  // linking to its Issue and unfolding to its Transcript. The Skills card
  // under Roles is Paperclip's Skills tab (AgentSkills), switched by the same
  // people who may give Roles.
  import { CircleHelp, Copy, Heart, MessageCircle, MoreHorizontal, Pause, Play, Plus, Trash2, X } from '@lucide/svelte'
  import * as AlertDialog from '@bakery/ui/components/ui/alert-dialog'
  import { Button as UiButton } from '@bakery/ui/components/ui/button'
  import * as Popover from '@bakery/ui/components/ui/popover'
  import { Separator } from '@bakery/ui/components/ui/separator'
  import { Switch } from '@bakery/ui/components/ui/switch'
  import * as Tooltip from '@bakery/ui/components/ui/tooltip'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import AgentIconPicker from '../../lib/AgentIconPicker.svelte'
  import AgentSkills from '../../lib/AgentSkills.svelte'
  import {
    addAgentRole,
    agentStatusText,
    agentStatusTones,
    editAgent,
    getAgent,
    jobLabels,
    jobs,
    listAgents,
    pauseAgent,
    removeAgentRole,
    resumeAgent,
    terminateAgent,
    type Agent,
    type AgentPatch,
  } from '../../lib/agents'
  import { api, ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import { ago, formatDate } from '../../lib/format'
  import { myRank, rankOf } from '../../lib/hierarchy'
  import InlineEditor from '../../lib/InlineEditor.svelte'
  import OptionPopover from '../../lib/OptionPopover.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import RunLedger from '../../lib/RunLedger.svelte'
  import { listRuns, runHeartbeat, type Run } from '../../lib/runs'
  import BudgetPolicyCard from '../../lib/BudgetPolicyCard.svelte'
  import { budgetOverview, type Budget } from '../../lib/costs'
  import { href } from '../../lib/router.svelte'
  import { session, type Member } from '../../lib/session.svelte'
  import type { GuildRole } from '../../lib/types'
  import InlineBanner from '@bakery/ui/InlineBanner.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import NotFound from '../NotFound.svelte'

  let { id }: { id: number } = $props()

  let agent = $state.raw<Agent | null>(null)
  let others = $state.raw<Agent[]>([])
  let roles = $state.raw<GuildRole[]>([])
  let members = $state.raw<Member[]>([])
  let missing = $state(false)
  let loadError = $state('')
  let busy = $state(false)
  let moreOpen = $state(false)
  let addingRole = $state(false)
  let terminating = $state(false)
  let reportsToError = $state('')
  let runs = $state.raw<Run[]>([])
  let budgets = $state.raw<Budget[]>([])
  // The interval as typed, committed on blur or Enter; null shows the Agent's.
  let intervalDraft = $state<number | null>(null)
  let intervalError = $state('')

  // The page is keyed by id, so loading once is enough.
  // The Agent's own Budgets, from the Guild's Budget overview.
  function loadBudgets() {
    budgetOverview()
      .then((o) => (budgets = o.budgets.filter((b) => b.scope.type === 'agent' && b.scope.id === id)))
      .catch(() => {})
  }

  function load() {
    loadBudgets()
    getAgent(id)
      .then((a) => (agent = a))
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) missing = true
        else loadError = e.message
      })
    listRuns({ agent: id, limit: 20 })
      .then((rs) => (runs = rs))
      .catch(() => {})
    // The Guild's live Agents, for Reports to and the direct reports.
    listAgents('all')
      .then((as) => (others = as))
      .catch(() => {})
    api<{ roles: GuildRole[] }>('GET', '/roles')
      .then((r) => (roles = r.roles))
      .catch(() => {})
    api<{ members: Member[] }>('GET', '/members')
      .then((r) => (members = r.members))
      .catch(() => {})
  }
  load()

  $effect(() => breadcrumb.set({ label: 'Agents', href: href('/agents/all') }, { label: agent?.name ?? 'Agent' }))

  const live = $derived(agent !== null && (agent.status === 'idle' || agent.status === 'error' || agent.status === 'paused'))
  const editable = $derived(!!agent?.can_manage && live)
  // Resume of an Agent paused by its Budget waits until no Budget of its own is at its Hard stop.
  const budgetHeld = $derived(agent?.pause_reason === 'budget' && budgets.some((b) => b.status === 'hard_stop' && b.hard_stop))
  const reports = $derived(others.filter((a) => a.reports_to?.id === id))
  // Reports to cannot be the Agent itself; a deeper cycle is the server's to refuse.
  const managerOptions = $derived([
    { value: null as number | null, label: 'No manager' },
    ...others.filter((a) => a.id !== id).map((a) => ({ value: a.id as number | null, label: a.name })),
  ])
  // A Role can be added below the asker's highest and below the Hirer's
  // highest (CanAssignToAgent), as the guilds context checks it.
  const hirerRank = $derived.by(() => {
    const hirer = members.find((m) => m.id === agent?.hirer?.id)
    return hirer ? rankOf(hirer, roles) : 0
  })
  const addable = $derived(
    roles.filter((r) => !r.base && !agent?.roles.some((h) => h.id === r.id) && r.position < myRank(roles) && r.position < hirerRank),
  )

  const message = (e: unknown) => (e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))

  async function act(f: () => Promise<Agent>) {
    busy = true
    try {
      agent = await f()
      others = others.map((a) => (a.id === id ? agent! : a))
    } catch (e) {
      toast.error(message(e))
    } finally {
      busy = false
    }
  }

  async function save(patch: AgentPatch) {
    try {
      agent = await editAgent(id, patch)
      others = others.map((a) => (a.id === id ? agent! : a))
      if ('reports_to' in patch) reportsToError = ''
      if (patch.heartbeat) intervalError = ''
    } catch (e) {
      if ('reports_to' in patch && e instanceof ApiError && e.errors.reports_to) reportsToError = e.errors.reports_to
      else if (patch.heartbeat && e instanceof ApiError && e.errors['heartbeat.interval_sec']) intervalError = e.errors['heartbeat.interval_sec']
      else toast.error(message(e))
    }
  }

  async function commitInterval() {
    if (intervalDraft === null || intervalDraft === agent?.heartbeat.interval_sec) {
      intervalDraft = null
      return
    }
    await save({ heartbeat: { interval_sec: intervalDraft } })
    if (!intervalError) intervalDraft = null
  }

  async function heartbeat() {
    busy = true
    try {
      const r = await runHeartbeat(id)
      runs = [r, ...runs.filter((x) => x.id !== r.id)]
      toast.success(r.wake_count > 1 ? 'Joined the queued run.' : 'Heartbeat queued.')
    } catch (e) {
      toast.error(message(e))
    } finally {
      busy = false
    }
  }

  async function copyId() {
    moreOpen = false
    try {
      await navigator.clipboard.writeText(String(id))
      toast.success('Agent ID copied.')
    } catch {
      toast.error('Clipboard access is unavailable.')
    }
  }
</script>

{#snippet row(label: string, value: import('svelte').Snippet)}
  <div class="flex items-start gap-3 py-1.5" data-property-row={label}>
    <span class="mt-0.5 w-24 shrink-0 text-xs text-muted-foreground">{label}</span>
    <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">{@render value()}</div>
  </div>
{/snippet}

{#snippet hint(label: string, text: string)}
  <Tooltip.Root>
    <Tooltip.Trigger>
      {#snippet child({ props })}
        <button {...props} type="button" aria-label={`About ${label}`} class="inline-flex text-muted-foreground/50 transition-colors hover:text-muted-foreground"><CircleHelp class="size-3" /></button>
      {/snippet}
    </Tooltip.Trigger>
    <Tooltip.Content side="top" class="max-w-xs">{text}</Tooltip.Content>
  </Tooltip.Root>
{/snippet}

{#snippet muted(text: string)}<span class="text-sm text-muted-foreground">{text}</span>{/snippet}

{#if missing}
  <NotFound title="Agent not found" description="This agent does not exist or you cannot see it." />
{:else if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if agent === null}
  <PageSkeleton />
{:else}
  {@const a = agent}
  <div class="mx-auto max-w-5xl space-y-8">
    <header class="flex flex-wrap items-center justify-between gap-5 border-b border-border pb-6">
      <div class="flex min-w-0 items-center gap-4">
        {#snippet avatar()}
          <span class="flex size-16 items-center justify-center rounded-full bg-accent"><AgentIcon icon={a.icon} class="size-8" /></span>
        {/snippet}
        {#if editable}
          <AgentIconPicker value={a.icon} label="Change agent icon" onchange={(icon) => save({ icon })}>{@render avatar()}</AgentIconPicker>
        {:else}
          <span role="img" aria-label={`${a.name} icon`} class="shrink-0">{@render avatar()}</span>
        {/if}
        <div class="min-w-0 space-y-1">
          <InlineEditor label="Name" value={a.name} {editable} as="h2" class="truncate text-2xl font-semibold tracking-tight" onsave={(name) => save({ name })} />
          <div class="flex items-center gap-1 text-xs text-muted-foreground">
            <span class="px-1">{a.job_label}</span>
            {#if editable || a.title}
              <span>·</span>
              <InlineEditor label="Title" value={a.title} {editable} placeholder="Add a title..." class="text-xs not-italic" onsave={(title) => save({ title })} />
            {/if}
          </div>
        </div>
      </div>
      {#if a.status !== 'terminated'}
        <div class="flex flex-wrap items-center gap-2">
          {#if session.can('manage_work') && a.status !== 'pending_approval'}
            <UiButton variant="outline" size="sm" href={href(`/chats/${a.id}`)}><MessageCircle class="size-3.5" />Chat</UiButton>
          {/if}
          {#if a.can_manage}
            <Tooltip.Root>
              <Tooltip.Trigger>
                {#snippet child({ props })}
                  <span {...props} class="inline-flex">
                    <UiButton
                      variant="outline"
                      size="sm"
                      disabled={busy || !a.heartbeat.wake_on_demand || a.status === 'pending_approval'}
                      onclick={heartbeat}><Play class="size-3.5" />Run heartbeat</UiButton
                    >
                  </span>
                {/snippet}
              </Tooltip.Trigger>
              {#if !a.heartbeat.wake_on_demand}
                <Tooltip.Content side="bottom">Wake on demand is off.</Tooltip.Content>
              {/if}
            </Tooltip.Root>
            {#if a.status === 'paused'}
              <UiButton
                variant="outline"
                size="sm"
                disabled={busy || budgetHeld}
                title={budgetHeld ? "Agent is paused because its budget's hard stop was reached." : undefined}
                onclick={() => act(() => resumeAgent(id))}><Play class="size-3.5" />Resume</UiButton
              >
            {:else}
              <UiButton variant="outline" size="sm" disabled={busy || a.status === 'pending_approval'} onclick={() => act(() => pauseAgent(id))}><Pause class="size-3.5" />Pause</UiButton>
            {/if}
          {/if}
          <Popover.Root bind:open={moreOpen}>
            <Popover.Trigger>
              {#snippet child({ props })}
                <UiButton {...props} variant="ghost" size="icon-xs" aria-label={`Open actions for ${a.name}`}><MoreHorizontal class="size-4" /></UiButton>
              {/snippet}
            </Popover.Trigger>
            <Popover.Content class="w-44 p-1" align="end">
              <button type="button" class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs hover:bg-accent/50" onclick={copyId}>
                <Copy class="size-3" />Copy Agent ID
              </button>
              {#if a.can_manage}
                <button
                  type="button"
                  class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-destructive hover:bg-accent/50"
                  onclick={() => {
                    moreOpen = false
                    terminating = true
                  }}
                >
                  <Trash2 class="size-3" />Terminate
                </button>
              {/if}
            </Popover.Content>
          </Popover.Root>
        </div>
      {:else}
        <StatusBadge type="error" label="Terminated" />
      {/if}
    </header>

    {#if a.status === 'pending_approval'}
      <InlineBanner tone="warning" title="Waiting for approval" data-testid="pending-approval">
        This agent is pending board approval and cannot be invoked yet.
        {#snippet actions()}
          {#if a.approval_id}<a href={href(`/approvals/${a.approval_id}`)} class="text-sm font-medium underline-offset-4 hover:underline">View approval</a>{/if}
        {/snippet}
      </InlineBanner>
    {/if}

    <div class="grid gap-4 md:grid-cols-2">
      <section class="rounded-lg border border-border p-4" aria-labelledby="agent-identity-heading">
        <div class="mb-4 flex items-center justify-between gap-3">
          <h3 id="agent-identity-heading" class="text-sm font-medium">Identity</h3>
          <StatusBadge type={agentStatusTones[a.status]} label={agentStatusText(a)} />
        </div>
        <div class="space-y-1">
          {#snippet job()}
            {#if editable}
              <OptionPopover align="end" label="Job" value={a.job} options={jobs.map((j) => ({ value: j as string, label: jobLabels[j] }))} onpick={(job) => save({ job })}>
                <span class="text-sm">{a.job_label}</span>
              </OptionPopover>
            {:else}
              <span class="text-sm">{a.job_label}</span>
            {/if}
          {/snippet}
          {@render row('Job', job)}
          {#snippet title()}
            {#if a.title}<span class="text-sm">{a.title}</span>{:else}{@render muted('Not set')}{/if}
          {/snippet}
          {@render row('Title', title)}
          {#snippet reportsTo()}
            {#if editable}
              <OptionPopover align="end" label="Reports to" value={a.reports_to?.id ?? null} options={managerOptions} onpick={(reports_to) => save({ reports_to })}>
                {#if a.reports_to}<span class="text-sm">{a.reports_to.name}</span>{:else}{@render muted('Board')}{/if}
              </OptionPopover>
            {:else if a.reports_to}
              <a href={href(`/agents/${a.reports_to.id}`)} class="text-sm hover:underline">{a.reports_to.name}</a>
            {:else}
              {@render muted('Board')}
            {/if}
            {#if reportsToError}<p class="w-full text-xs text-destructive" data-testid="reports-to-error">{reportsToError}</p>{/if}
          {/snippet}
          {@render row('Reports to', reportsTo)}
          {#snippet directReports()}
            {#each reports as r (r.id)}
              <a href={href(`/agents/${r.id}`)} class="inline-flex items-center gap-1 rounded-full border border-border px-2 py-0.5 text-xs no-underline hover:bg-accent/50" data-testid="direct-report">
                <AgentIcon icon={r.icon} class="size-3" />{r.name}
              </a>
            {:else}
              {@render muted('None')}
            {/each}
          {/snippet}
          {@render row('Direct reports', directReports)}
        </div>
        <Separator class="my-3" />
        <div class="space-y-1">
          {#snippet hirer()}
            {#if a.hirer}<span class="text-sm">{a.hirer.name}</span>{:else}{@render muted('Unknown')}{/if}
          {/snippet}
          {@render row('Hirer', hirer)}
          {#snippet created()}<span class="text-sm">{formatDate(a.created_at)}</span>{/snippet}
          {@render row('Created', created)}
          {#if a.terminated_at}
            {#snippet terminated()}<span class="text-sm">{formatDate(a.terminated_at!)}</span>{/snippet}
            {@render row('Terminated', terminated)}
          {/if}
        </div>
      </section>

      <section class="rounded-lg border border-border p-4" aria-labelledby="agent-capabilities-heading">
        <h3 id="agent-capabilities-heading" class="mb-3 text-sm font-medium">Capabilities</h3>
        {#if !editable && !a.capabilities}
          <p class="text-sm text-muted-foreground">No capability summary has been added.</p>
        {:else}
        <InlineEditor
          label="Capabilities"
          value={a.capabilities}
          {editable}
          multiline
          placeholder="No capability summary has been added."
          class="text-sm"
          onsave={(capabilities) => save({ capabilities })}
        />
        {/if}
      </section>

      <section class="rounded-lg border border-border md:col-span-2" aria-labelledby="agent-run-policy-heading" data-testid="run-policy">
        <h3 id="agent-run-policy-heading" class="flex items-center gap-2 px-4 pt-4 pb-3 text-sm font-medium"><Heart class="size-3" />Run Policy</h3>
        <div class="space-y-3 px-4 pb-4">
          <div class="space-y-2">
            <div class="flex items-center justify-between gap-3">
              <div class="flex items-center gap-1.5">
                <span class="text-xs text-muted-foreground">Heartbeat on interval</span>
                {@render hint('Heartbeat on interval', 'Run this agent automatically on a timer while it has open issues assigned.')}
              </div>
              <Switch
                checked={a.heartbeat.enabled}
                disabled={!editable}
                aria-label="Heartbeat on interval"
                onCheckedChange={(enabled) => save({ heartbeat: { enabled } })}
              />
            </div>
            {#if a.heartbeat.enabled}
              <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span>Run heartbeat every</span>
                <input
                  type="number"
                  min="60"
                  max="86400"
                  aria-label="Run heartbeat every"
                  class="w-20 rounded-md border border-border bg-transparent px-2 py-0.5 text-center font-mono text-xs outline-none disabled:opacity-60"
                  disabled={!editable}
                  value={intervalDraft ?? a.heartbeat.interval_sec}
                  oninput={(e) => (intervalDraft = e.currentTarget.valueAsNumber)}
                  onblur={commitInterval}
                  onkeydown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
                />
                <span>sec</span>
                {@render hint('the interval', 'Seconds between automatic heartbeat invocations, 60 to 86400.')}
              </div>
              {#if intervalError}<p class="text-xs text-destructive" data-testid="interval-error">{intervalError}</p>{/if}
              <p class="text-xs text-muted-foreground" data-testid="last-heartbeat">
                {a.heartbeat.last_heartbeat_at ? `Last heartbeat ${ago(a.heartbeat.last_heartbeat_at)}` : 'No heartbeat yet.'}
              </p>
            {/if}
          </div>
          <div class="flex items-center justify-between gap-3">
            <div class="flex items-center gap-1.5">
              <span class="text-xs text-muted-foreground">Wake on demand</span>
              {@render hint('Wake on demand', 'Allow this agent to be woken by assignments, comments and Run heartbeat.')}
            </div>
            <Switch
              checked={a.heartbeat.wake_on_demand}
              disabled={!editable}
              aria-label="Wake on demand"
              onCheckedChange={(wake_on_demand) => save({ heartbeat: { wake_on_demand } })}
            />
          </div>
        </div>
      </section>

      {#if a.status !== 'terminated' && a.status !== 'pending_approval'}
      <section class="space-y-3 md:col-span-2" aria-label="Budget" data-testid="agent-budget">
        <h3 class="text-sm font-medium">Budget</h3>
        <div class="grid gap-4 xl:grid-cols-2">
          {#each budgets as b (b.id)}
            <BudgetPolicyCard budget={b} scope={b.scope} onsaved={load} />
          {:else}
            <BudgetPolicyCard budget={null} scope={{ type: 'agent', id, name: a.name }} onsaved={load} />
          {/each}
        </div>
      </section>
      {/if}

      <section class="rounded-lg border border-border p-4 md:col-span-2" aria-labelledby="agent-roles-heading">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h3 id="agent-roles-heading" class="text-sm font-medium">Roles</h3>
          {#if editable}
            <Popover.Root bind:open={addingRole}>
              <Popover.Trigger>
                {#snippet child({ props })}
                  <UiButton {...props} variant="outline" size="xs"><Plus />Add role</UiButton>
                {/snippet}
              </Popover.Trigger>
              <Popover.Content align="end" class="max-h-72 w-52 overflow-y-auto p-1">
                <div role="listbox" aria-label="Add role">
                  {#each addable as r (r.id)}
                    <button
                      type="button"
                      role="option"
                      aria-selected="false"
                      class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-accent/50"
                      onclick={() => {
                        addingRole = false
                        act(() => addAgentRole(id, r.id))
                      }}
                    >
                      <span class="inline-block size-2.5 shrink-0 rounded-full" style:background-color={r.color}></span>{r.name}
                    </button>
                  {:else}
                    <p class="px-2 py-1.5 text-xs text-muted-foreground">No roles you can give.</p>
                  {/each}
                </div>
              </Popover.Content>
            </Popover.Root>
          {/if}
        </div>
        <div class="flex flex-wrap items-center gap-1.5" data-testid="agent-roles">
          {#each a.roles as r (r.id)}
            <span class="inline-flex items-center gap-1.5 rounded-full border border-border px-2 py-0.5 text-xs" data-role={r.name}>
              <span class="inline-block size-2 shrink-0 rounded-full" style:background-color={r.color}></span>{r.name}
              {#if editable}
                <button
                  type="button"
                  aria-label={`Remove ${r.name}`}
                  class="-mr-1 inline-flex size-4 items-center justify-center rounded-full text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                  disabled={busy}
                  onclick={() => act(() => removeAgentRole(id, r.id))}
                >
                  <X class="size-3" />
                </button>
              {/if}
            </span>
          {:else}
            {@render muted('Only @everyone.')}
          {/each}
        </div>
        <p class="mt-3 text-xs text-muted-foreground">An agent holds @everyone too, and only ever roles below the one who hired it.</p>
      </section>

      <section class="rounded-lg border border-border p-4 md:col-span-2" aria-labelledby="agent-skills-heading" data-testid="agent-skills">
        <h3 id="agent-skills-heading" class="mb-3 text-sm font-medium">Skills</h3>
        <AgentSkills agentId={id} {editable} />
      </section>

      <section class="rounded-lg border border-border p-4 md:col-span-2" aria-labelledby="agent-runs-heading">
        <h3 id="agent-runs-heading" class="mb-3 text-sm font-medium">Runs</h3>
        <RunLedger {runs} show="issue" onstatus={(r) => (runs = runs.map((x) => (x.id === r.id ? r : x)))} />
      </section>
    </div>
  </div>

  <AlertDialog.Root bind:open={terminating}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>Terminate {a.name}?</AlertDialog.Title>
        <AlertDialog.Description>A terminated agent stops for good and cannot be resumed. It stays listed under Terminated.</AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
        <AlertDialog.Action
          onclick={() => {
            terminating = false
            act(() => terminateAgent(id))
          }}>Terminate</AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>
{/if}
