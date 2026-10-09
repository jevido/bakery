<script lang="ts">
  // Paperclip's Routine page (ui/src/pages/RoutineDetail.tsx; MIT, see
  // NOTICE): the sections in a sub-sidebar on the left, and over the
  // current one a header with the title, "Edit routine" on the Overview,
  // Run and the automation toggle (active ↔ paused). Editing turns the
  // title into a field and the Overview into its form, with the save bar
  // under it. A Routine without an Agent is a Draft and an archived one
  // is read-only: neither has the toggle, and an archived one has no Run
  // or Edit, since archiving is final. Only a Member with manage_work gets
  // any control. Run on a Routine with variables opens the Run dialog to
  // ask for them. History lists the Routine revisions and restores one.
  // Left out: Secrets and Delivery.
  import { Pencil, Play, X } from '@lucide/svelte'
  import { Badge } from '@bakery/ui/components/ui/badge'
  import { Button } from '@bakery/ui/components/ui/button'
  import { Switch } from '@bakery/ui/components/ui/switch'
  import { untrack } from 'svelte'
  import { ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import { go, href, type RoutineSection } from '../../lib/router.svelte'
  import RunRoutineDialog from '../../lib/RunRoutineDialog.svelte'
  import {
    getRoutine,
    nextRoutineStatus,
    routineState,
    runRoutine,
    syncVariables,
    updateRoutine,
    type RoutineDetail,
    type RoutineRun,
  } from '../../lib/routines'
  import { session } from '../../lib/session.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import RoutineActivity from './RoutineActivity.svelte'
  import RoutineEditor, { type RoutineDraft } from './RoutineEditor.svelte'
  import RoutineHistory from './RoutineHistory.svelte'
  import RoutineOverview from './RoutineOverview.svelte'
  import RoutineRuns from './RoutineRuns.svelte'
  import RoutineSaveBar from './RoutineSaveBar.svelte'
  import RoutineSubSidebar from './RoutineSubSidebar.svelte'
  import RoutineTriggers from './RoutineTriggers.svelte'

  let { id, section }: { id: number; section: RoutineSection } = $props()

  const titles: Record<RoutineSection, string> = { '': 'Overview', triggers: 'Triggers', runs: 'Runs', activity: 'Activity', history: 'History' }

  let routine = $state.raw<RoutineDetail | null>(null)
  let loadError = $state('')
  /** Counts the changes a Run or a trigger made, so Runs and Activity ask again. */
  let version = $state(0)
  let editing = $state(false)
  let title = $state('')
  let draft = $state<RoutineDraft | null>(null)
  let saving = $state(false)
  let running = $state(false)
  let asking = $state(false)
  let errors = $state<Record<string, string>>({})
  let toggling = $state(false)

  const message = (e: unknown) => (e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
  const canManage = $derived(session.can('manage_work'))
  const archived = $derived(routine?.status === 'archived')
  const automation = $derived(routine ? routineState(routine) : 'paused')

  const load = () =>
    getRoutine(id)
      .then((r) => {
        routine = r
        version++
      })
      .catch((e) => (loadError = message(e)))
  untrack(load)

  // The shell clears the breadcrumb on every route, a change of section too.
  $effect(() => {
    void section
    breadcrumb.set({ label: 'Routines', href: href('/routines') }, { label: routine?.title ?? 'Routine' })
  })

  const draftOf = (r: RoutineDetail): RoutineDraft => ({
    title: r.title,
    description: r.description,
    priority: r.priority,
    concurrency_policy: r.concurrency_policy,
    catch_up_policy: r.catch_up_policy,
    project_id: r.project?.id ?? null,
    goal_id: r.goal?.id ?? null,
    parent_issue_id: r.parent_issue?.id ?? null,
    assignee_agent_id: r.assignee_agent?.id ?? null,
    variables: r.variables,
  })

  /** The draft's variables as they will be saved: one per placeholder. */
  const variables = $derived(draft ? syncVariables(title, draft.description, draft.variables) : [])

  /** The fields the form changed, by the name the save bar shows. */
  const dirty = $derived.by(() => {
    if (!editing || !routine || !draft) return []
    const was = draftOf(routine)
    const out: string[] = []
    if (title.trim() !== routine.title) out.push('title')
    const names: Record<keyof RoutineDraft, string> = {
      title: 'title',
      description: 'description',
      priority: 'priority',
      concurrency_policy: 'concurrency',
      catch_up_policy: 'catch-up',
      project_id: 'project',
      goal_id: 'goal',
      parent_issue_id: 'parent issue',
      assignee_agent_id: 'default agent',
      variables: 'variables',
    }
    for (const key of Object.keys(names) as (keyof RoutineDraft)[]) {
      if (key !== 'title' && key !== 'variables' && draft[key] !== was[key]) out.push(names[key])
    }
    if (JSON.stringify(variables) !== JSON.stringify(was.variables)) out.push(names.variables)
    return out
  })

  function startEditing() {
    if (!routine) return
    title = routine.title
    draft = draftOf(routine)
    errors = {}
    editing = true
  }

  function stopEditing() {
    editing = false
    draft = null
  }

  async function save() {
    if (!routine || !draft || !title.trim() || saving) return
    saving = true
    errors = {}
    try {
      routine = { ...routine, ...(await updateRoutine(routine.id, { ...draft, variables, title: title.trim() })) }
      version++
      toast.success('Routine saved', routine.title)
      stopEditing()
    } catch (e) {
      if (e instanceof ApiError) errors = e.errors
      toast.error('Routine not saved', message(e))
    } finally {
      saving = false
    }
  }

  /** Run asks for the Routine variables first, when it has any. */
  function run() {
    if (!routine) return
    if (routine.variables.length > 0) asking = true
    else runNow()
  }

  async function runNow() {
    if (!routine) return
    running = true
    try {
      await ran(await runRoutine(routine.id))
    } catch (e) {
      toast.error('Routine not run', message(e))
    } finally {
      running = false
    }
  }

  async function ran(r: RoutineRun) {
    if (!routine) return
    const open = r.issue ? { label: `Open ${r.issue.identifier}`, href: href(`/issues/${r.issue.identifier}`) } : undefined
    if (r.status === 'coalesced' && r.issue) toast.show(`Coalesced into ${r.issue.identifier}`, r.issue.title, open)
    else if (r.status === 'skipped') toast.show('Routine run skipped', r.failure_reason ?? 'A run is already active.', open)
    else toast.success('Routine run started', r.issue ? `${r.issue.identifier} ${r.issue.title}` : routine.title, open)
    await load()
    go(`/routines/${routine.id}/runs`)
  }

  async function toggle(on: boolean) {
    if (!routine) return
    toggling = true
    try {
      routine = { ...routine, ...(await updateRoutine(routine.id, { status: nextRoutineStatus(routine.status, on) })) }
      version++
    } catch (e) {
      toast.error('Routine not changed', message(e))
    } finally {
      toggling = false
    }
  }

  // The Overview is the only section with a form; leaving it ends the edit.
  $effect(() => {
    if (section !== '') untrack(stopEditing)
  })
</script>

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if !routine}
  <PageSkeleton />
{:else}
  <div class="-m-4 flex min-h-[calc(100%+2rem)] md:-m-6 md:min-h-[calc(100%+3rem)]">
    <RoutineSubSidebar id={routine.id} {section} dirty={dirty.length > 0} />
    <div class="flex min-w-0 flex-1 flex-col">
      <header class="flex min-h-14 shrink-0 flex-wrap items-center gap-3 border-b border-border bg-background px-4 py-2 md:px-6">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          {#if section === '' && editing}
            <!-- svelte-ignore a11y_autofocus -->
            <input
              class="min-w-0 flex-1 bg-transparent text-base leading-7 font-semibold outline-none placeholder:text-muted-foreground/50"
              placeholder="Routine title"
              aria-label="Routine title"
              bind:value={title}
              autofocus
            />
          {:else}
            <h1 class="min-w-0 flex-1 truncate text-xl font-bold">{routine.title}</h1>
          {/if}
          {#if archived}<Badge variant="outline" class="shrink-0">Archived</Badge>{/if}
        </div>
        {#if canManage && !archived}
          <div class="ml-auto flex shrink-0 items-center gap-2">
            {#if section === ''}
              {#if editing}
                <Button variant="outline" size="sm" onclick={stopEditing}><X class="size-3.5" />Cancel editing</Button>
              {:else}
                <Button variant="outline" size="sm" onclick={startEditing}><Pencil class="size-3.5" />Edit routine</Button>
              {/if}
            {/if}
            <Button size="sm" disabled={running} onclick={run}><Play class="size-3.5" />{running ? 'Running…' : 'Run'}</Button>
            <div class="flex items-center gap-2">
              <Switch
                checked={routine.status === 'active'}
                disabled={toggling || automation === 'draft'}
                onCheckedChange={toggle}
                aria-label={routine.status === 'active' ? 'Pause automatic triggers' : 'Enable automatic triggers'}
              />
              <span class={['text-sm font-medium', automation === 'active' ? 'text-emerald-400' : 'text-muted-foreground']}>
                {{ active: 'Active', paused: 'Paused', draft: 'Draft', archived: 'Archived' }[automation]}
              </span>
            </div>
          </div>
        {/if}
      </header>

      <main id="routine-section" class="min-w-0 flex-1 px-4 pt-8 pb-6 md:px-8">
        <section aria-labelledby="routine-section-title" class={section === '' ? (editing ? 'mx-auto w-full max-w-3xl' : 'mx-auto w-full max-w-5xl') : 'w-full'}>
          <h2 id="routine-section-title" class="mb-4 text-lg font-semibold">{titles[section]}</h2>
          {#if section === ''}
            {#if editing && draft}
              <RoutineEditor bind:draft {title} {errors} />
              <RoutineSaveBar {dirty} {saving} disabled={!title.trim()} onsave={save} ondiscard={stopEditing} />
            {:else}
              <RoutineOverview {routine} />
            {/if}
          {:else if section === 'triggers'}
            <RoutineTriggers {routine} editable={canManage && !archived} onchange={load} />
          {:else if section === 'runs'}
            <RoutineRuns id={routine.id} {version} />
          {:else if section === 'history'}
            <RoutineHistory {routine} {version} editable={canManage && !archived} onrestored={load} />
          {:else}
            <RoutineActivity id={routine.id} {version} />
          {/if}
        </section>
      </main>
    </div>
  </div>
{/if}

{#if routine}
  <RunRoutineDialog bind:open={asking} {routine} onrun={ran} />
{/if}
