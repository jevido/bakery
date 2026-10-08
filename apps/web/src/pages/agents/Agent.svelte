<script lang="ts">
  // Paperclip's AgentDetail (ui/src/pages/AgentDetail.tsx) with only what
  // has meaning without Runs (MIT, see NOTICE): the header with the Agent
  // icon, name and Job · Title, and AgentActionButtons
  // (ui/src/components/AgentActionButtons.tsx: Pause / Resume and the
  // overflow menu with Copy Agent ID and Terminate); then AgentOverview's
  // cards (Identity, Capabilities) with AgentProperties's rows. Run, Clear
  // error, Reset sessions, Duplicate and Assign Task wait for Runs, and so
  // do Paperclip's other tabs, so the page has no tab bar. The Roles card is
  // The Bakery's, in the place of Paperclip's Governance permissions: the
  // Roles the Agent holds as chips, and only Roles below both the asker's
  // highest and the Hirer's highest can be added. Whoever may manage the
  // Agent (can_manage) edits it in place while it is idle or paused; the
  // server stays the judge of every change.
  import { Copy, MoreHorizontal, Pause, Play, Plus, Trash2, X } from '@lucide/svelte'
  import * as AlertDialog from '@bakery/ui/components/ui/alert-dialog'
  import { Button as UiButton } from '@bakery/ui/components/ui/button'
  import * as Popover from '@bakery/ui/components/ui/popover'
  import { Separator } from '@bakery/ui/components/ui/separator'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import AgentIconPicker from '../../lib/AgentIconPicker.svelte'
  import {
    addAgentRole,
    agentStatusLabel,
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
  import { formatDate } from '../../lib/format'
  import { myRank, rankOf } from '../../lib/hierarchy'
  import InlineEditor from '../../lib/InlineEditor.svelte'
  import OptionPopover from '../../lib/OptionPopover.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { href } from '../../lib/router.svelte'
  import type { Member } from '../../lib/session.svelte'
  import type { GuildRole } from '../../lib/types'
  import InlineBanner from '../../lib/ui/InlineBanner.svelte'
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

  // The page is keyed by id, so loading once is enough.
  function load() {
    getAgent(id)
      .then((a) => (agent = a))
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) missing = true
        else loadError = e.message
      })
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

  const live = $derived(agent !== null && (agent.status === 'idle' || agent.status === 'paused'))
  const editable = $derived(!!agent?.can_manage && live)
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
    } catch (e) {
      if ('reports_to' in patch && e instanceof ApiError && e.errors.reports_to) reportsToError = e.errors.reports_to
      else toast.error(message(e))
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
          {#if a.can_manage}
            {#if a.status === 'paused'}
              <UiButton variant="outline" size="sm" disabled={busy} onclick={() => act(() => resumeAgent(id))}><Play class="size-3.5" />Resume</UiButton>
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
          <StatusBadge type={agentStatusTones[a.status]} label={agentStatusLabel(a.status)} />
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
