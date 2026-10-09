<script lang="ts">
  // Paperclip's Routines page (ui/src/pages/Routines.tsx and RoutineListRow
  // in ui/src/components/RoutineList.tsx; MIT, see NOTICE): Create routine
  // over the Routines and Recent Runs tabs. A row shows the Routine's
  // Project, Agent and last run, with Run now, the On/Off toggle and a menu
  // to archive it; only a Member with manage_work gets those. Recent Runs
  // lists the Guild's Routine runs with their Execution Issues, where
  // Paperclip lists the Execution Issues themselves. Left out: folders,
  // grouping, sorting and bulk selection.
  import { MoreHorizontal, Play, Plus } from '@lucide/svelte'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import * as AlertDialog from '@bakery/ui/components/ui/alert-dialog'
  import { Button } from '@bakery/ui/components/ui/button'
  import * as DropdownMenu from '@bakery/ui/components/ui/dropdown-menu'
  import { Switch } from '@bakery/ui/components/ui/switch'
  import * as Tabs from '@bakery/ui/components/ui/tabs'
  import { ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import { ago } from '../../lib/format'
  import NewRoutineDialog from '../../lib/NewRoutineDialog.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { go, href } from '../../lib/router.svelte'
  import {
    listRoutines,
    nextRoutineStatus,
    recentRoutineRuns,
    routineRunStatusLabel,
    runRoutine,
    updateRoutine,
    type Routine,
    type RoutineRun,
  } from '../../lib/routines'
  import { session } from '../../lib/session.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import { workLabel } from '../../lib/work'

  let { tab }: { tab: 'routines' | 'runs' } = $props()

  let routines = $state.raw<Routine[] | null>(null)
  let runs = $state.raw<RoutineRun[] | null>(null)
  let loadError = $state('')
  let creating = $state(false)
  let running = $state<number | null>(null)
  let changing = $state<number | null>(null)
  let archiving = $state<Routine | null>(null)

  const canManage = $derived(session.can('manage_work'))
  const message = (e: unknown) => (e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))

  const loadRoutines = () =>
    listRoutines()
      .then((rs) => (routines = rs))
      .catch((e) => (loadError = message(e)))
  const loadRuns = () =>
    recentRoutineRuns()
      .then((rs) => (runs = rs))
      .catch((e) => (loadError = message(e)))

  $effect(() => {
    loadError = ''
    if (tab === 'runs') loadRuns()
    else loadRoutines()
  })
  $effect(() => breadcrumb.set({ label: 'Routines' }))

  // Archived Routines stay out of the list, as Paperclip's default view.
  const visible = $derived(routines?.filter((r) => r.status !== 'archived') ?? null)

  async function runNow(r: Routine) {
    running = r.id
    try {
      const run = await runRoutine(r.id)
      toast.success(
        `Routine run ${routineRunStatusLabel(run.status)}`,
        run.issue ? `${run.issue.identifier} ${run.issue.title}` : r.title,
        run.issue ? { label: `Open ${run.issue.identifier}`, href: href(`/issues/${run.issue.identifier}`) } : undefined,
      )
      await loadRoutines()
    } catch (e) {
      toast.error('Routine not run', message(e))
    } finally {
      running = null
    }
  }

  async function setStatus(r: Routine, status: Routine['status']) {
    changing = r.id
    try {
      await updateRoutine(r.id, { status })
      await loadRoutines()
    } catch (e) {
      toast.error('Routine not changed', message(e))
    } finally {
      changing = null
    }
  }
</script>

<div class="space-y-6">
  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
    <div class="space-y-1">
      <h1 class="text-xl font-bold">Routines</h1>
      <p class="text-sm text-muted-foreground">Recurring work definitions that materialize into auditable execution issues.</p>
    </div>
    {#if canManage}
      <Button onclick={() => (creating = true)}><Plus class="size-4" />Create routine</Button>
    {/if}
  </div>

  <Tabs.Root value={tab} onValueChange={(v) => go(v === 'runs' ? '/routines/runs' : '/routines')}>
    <Tabs.List variant="line" class="justify-start">
      <Tabs.Trigger value="routines">Routines</Tabs.Trigger>
      <Tabs.Trigger value="runs">Recent Runs</Tabs.Trigger>
    </Tabs.List>
  </Tabs.Root>

  {#if loadError}<p class="text-sm text-destructive">{loadError}</p>{/if}

  {#if tab === 'routines'}
    {#if visible === null && !loadError}
      <PageSkeleton />
    {:else if visible && visible.length === 0}
      <div class="py-12">
        <Empty title="No active routines. Use Create routine to define the first recurring workflow." icon="routines" />
      </div>
    {:else if visible}
      <p class="text-sm text-muted-foreground">{visible.length} routine{visible.length === 1 ? '' : 's'}</p>
      <div class="rounded-lg border" role="list" aria-label="Routines">
        {#each visible as r (r.id)}
          {@const enabled = r.status === 'active'}
          {@const draft = !r.assignee_agent}
          <div role="listitem" class="group relative flex flex-col gap-3 border-b px-3 py-3 transition-colors last:border-b-0 hover:bg-accent/50 sm:flex-row sm:items-center">
            <div class="min-w-0 flex-1 space-y-1.5">
              <div class="flex flex-wrap items-center gap-2">
                <a href={href(`/routines/${r.id}`)} class="truncate text-sm font-medium text-inherit no-underline after:absolute after:inset-0">{r.title}</a>
                {#if r.status === 'paused' || draft}
                  <span class="text-xs text-muted-foreground">{draft ? 'draft' : 'paused'}</span>
                {/if}
              </div>
              <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                <span class="flex items-center gap-2">
                  <span class="size-2.5 shrink-0 rounded-sm bg-muted-foreground/40"></span>
                  <span>{r.project?.name ?? 'No project'}</span>
                </span>
                <span class="flex items-center gap-2">
                  {#if r.assignee_agent}<AgentIcon icon={r.assignee_agent.icon} class="size-3.5 shrink-0" />{/if}
                  <span>{r.assignee_agent?.name ?? 'No default agent'}</span>
                </span>
                <span title={r.last_run ? new Date(r.last_run.triggered_at).toLocaleString() : undefined}>
                  {r.last_run ? `${ago(r.last_run.triggered_at)} · ${routineRunStatusLabel(r.last_run.status)}` : 'Never'}
                </span>
              </div>
            </div>
            {#if canManage}
              <div class="relative flex items-center gap-3">
                <Button variant="outline" size="sm" disabled={running === r.id} onclick={() => runNow(r)}>
                  <Play class="size-3.5" />
                  {running === r.id ? 'Running...' : 'Run now'}
                </Button>
                <div class="flex items-center gap-3">
                  <Switch
                    checked={enabled}
                    disabled={changing === r.id}
                    onCheckedChange={(on) => setStatus(r, nextRoutineStatus(r.status, on))}
                    aria-label={enabled ? `Disable ${r.title}` : `Enable ${r.title}`}
                  />
                  <span class="w-12 text-xs text-muted-foreground">{draft ? 'Draft' : enabled ? 'On' : 'Off'}</span>
                </div>
                <DropdownMenu.Root>
                  <DropdownMenu.Trigger>
                    {#snippet child({ props })}
                      <Button {...props} variant="ghost" size="icon-sm" aria-label="More actions for {r.title}"><MoreHorizontal class="size-4" /></Button>
                    {/snippet}
                  </DropdownMenu.Trigger>
                  <DropdownMenu.Content align="end">
                    <DropdownMenu.Item onSelect={() => go(`/routines/${r.id}`)}>Edit</DropdownMenu.Item>
                    <DropdownMenu.Item disabled={running === r.id} onSelect={() => runNow(r)}>Run now</DropdownMenu.Item>
                    <DropdownMenu.Separator />
                    <DropdownMenu.Item disabled={changing === r.id} onSelect={() => setStatus(r, nextRoutineStatus(r.status, !enabled))}>
                      {enabled ? 'Pause' : 'Enable'}
                    </DropdownMenu.Item>
                    <DropdownMenu.Item disabled={changing === r.id} onSelect={() => (archiving = r)}>Archive</DropdownMenu.Item>
                  </DropdownMenu.Content>
                </DropdownMenu.Root>
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  {:else if runs === null && !loadError}
    <PageSkeleton />
  {:else if runs && runs.length === 0}
    <div class="py-12">
      <Empty title="No routine runs yet." icon="routines" />
    </div>
  {:else if runs}
    <div class="rounded-lg border" role="list" aria-label="Recent Runs">
      {#each runs as run (run.id)}
        <div role="listitem" class="flex flex-col gap-1 border-b px-3 py-2.5 text-sm last:border-b-0 sm:flex-row sm:items-center sm:gap-4">
          <a href={href(`/routines/${run.routine.id}`)} class="min-w-0 flex-1 truncate font-medium hover:underline">{run.routine.title}</a>
          <span class="w-20 text-xs text-muted-foreground">{workLabel(run.source)}</span>
          <span class="w-28 text-xs capitalize">{routineRunStatusLabel(run.status)}</span>
          <span class="w-28 text-xs text-muted-foreground" title={new Date(run.triggered_at).toLocaleString()}>{ago(run.triggered_at)}</span>
          <span class="w-56 truncate text-xs">
            {#if run.issue}
              <a href={href(`/issues/${run.issue.identifier}`)} class="hover:underline"><span class="font-mono text-muted-foreground">{run.issue.identifier}</span> {run.issue.title}</a>
            {:else}
              <span class="text-muted-foreground">{run.failure_reason ?? 'No issue'}</span>
            {/if}
          </span>
        </div>
      {/each}
    </div>
  {/if}
</div>

<NewRoutineDialog bind:open={creating} />

<AlertDialog.Root open={archiving !== null} onOpenChange={(o) => !o && (archiving = null)}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>Archive routine?</AlertDialog.Title>
      <AlertDialog.Description>
        "{archiving?.title}" stops running and leaves the list. Its runs and issues stay.
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
      <AlertDialog.Action
        onclick={() => {
          const r = archiving
          archiving = null
          if (r) setStatus(r, 'archived')
        }}>Archive</AlertDialog.Action
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
