<script lang="ts">
  // Paperclip's RoutineHistoryTab (ui/src/components/RoutineHistoryTab.tsx;
  // MIT, see NOTICE): the Routine revisions on the left, newest first, and
  // the picked one on the right, its fields marked where they differ from
  // the current Routine, its description, triggers and variables. An older
  // one has the banner with Compare and Restore: Compare shows the changed
  // fields and the description line by line, Restore asks first, names the
  // Webhook triggers that come back with a new URL and secret, and shows
  // those secrets once after. Left out: Paperclip's env and secret diffs
  // (no Routine secrets here), its "Show older" (the API answers at most
  // 100), its Change summary field on Restore (the API writes "Restored
  // from revision N") and its ConflictBanner (leaving the Overview ends an
  // edit, so History never has unsaved edits beside it).
  import { RotateCcw, Search } from '@lucide/svelte'
  import Markdown from '@bakery/ui/Markdown.svelte'
  import { Badge } from '@bakery/ui/components/ui/badge'
  import { Button } from '@bakery/ui/components/ui/button'
  import * as Dialog from '@bakery/ui/components/ui/dialog'
  import { untrack } from 'svelte'
  import Actor from '../../lib/Actor.svelte'
  import { listAgents, type Agent } from '../../lib/agents'
  import { api, ApiError } from '../../lib/api'
  import DocumentDiff from '../../lib/DocumentDiff.svelte'
  import { ago } from '../../lib/format'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import {
    restoreRoutineRevision,
    routineRevisions,
    signingModeHelp,
    type RoutineDetail,
    type RoutineRevision,
    type RoutineSnapshot,
    type SecretMaterial,
    type SnapshotTrigger,
  } from '../../lib/routines'
  import { toast } from '../../lib/ui/toast.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import { workLabel } from '../../lib/work'
  import RoutineWebhookSecret from './RoutineWebhookSecret.svelte'

  // `version` changes when the Routine changed, so the list asks again.
  let {
    routine,
    version,
    editable,
    onrestored,
  }: { routine: RoutineDetail; version: number; editable: boolean; onrestored: () => Promise<unknown> } = $props()

  let revisions = $state.raw<RoutineRevision[] | null>(null)
  let loadError = $state('')
  let picked = $state<number | null>(null)
  let comparing = $state(false)
  let confirming = $state(false)
  let restoring = $state(false)
  let agents = $state.raw<Agent[]>([])
  let projects = $state.raw<{ id: number; name: string }[]>([])
  /** The secrets of the Webhook triggers the last restore recreated, shown once. */
  let revealed = $state.raw<(SecretMaterial & { trigger_id: number; label: string })[]>([])

  untrack(() => {
    listAgents().then((as) => (agents = as)).catch(() => {})
    api<{ projects: { id: number; name: string }[] }>('GET', '/projects').then((r) => (projects = r.projects)).catch(() => {})
  })

  $effect(() => {
    void version
    routineRevisions(routine.id)
      .then((rs) => (revisions = rs))
      .catch((e) => (loadError = e.message))
  })

  const current = $derived(revisions?.[0] ?? null)
  const selected = $derived(revisions?.find((r) => r.id === picked) ?? current)
  const historical = $derived(!!selected && !!current && selected.id !== current.id)

  const agentName = (id: number) =>
    id === 0 ? 'No default agent' : (agents.find((a) => a.id === id)?.name ?? (routine.assignee_agent?.id === id ? routine.assignee_agent.name : 'Removed'))
  const projectName = (id: number) =>
    id === 0 ? 'No project' : (projects.find((p) => p.id === id)?.name ?? (routine.project?.id === id ? routine.project.name : 'Removed'))

  type Field = { label: string; value: (s: RoutineSnapshot['routine']) => string }
  const fields: Field[] = [
    { label: 'Title', value: (s) => s.title },
    { label: 'Priority', value: (s) => workLabel(s.priority) },
    { label: 'Status', value: (s) => workLabel(s.status) },
    { label: 'Default agent', value: (s) => agentName(s.assignee_agent_id) },
    { label: 'Project', value: (s) => projectName(s.project_id) },
    { label: 'Concurrency', value: (s) => workLabel(s.concurrency_policy) },
    { label: 'Catch-up', value: (s) => workLabel(s.catch_up_policy) },
  ]

  /** Paperclip's summarizeTriggerSnapshot. */
  function triggerSummary(t: SnapshotTrigger): string {
    if (t.kind === 'schedule') return [t.cron_expression, t.timezone].filter(Boolean).join(' · ')
    if (t.kind === 'webhook') {
      const mode = t.signing_mode ? signingModeHelp[t.signing_mode].label : ''
      return [mode, t.signing_mode === 'hmac_sha256' ? `replay ${t.replay_window_sec}s` : ''].filter(Boolean).join(' · ')
    }
    return 'API'
  }
  const triggerName = (t: SnapshotTrigger) => t.label || workLabel(t.kind)
  const variablesSummary = (s: RoutineSnapshot['routine']) => (s.variables.length ? s.variables.map((v) => v.name).join(', ') : 'None')

  /** Paperclip's computeFieldChanges: the fields, variables and triggers that differ. */
  function changes(before: RoutineSnapshot, after: RoutineSnapshot) {
    const out: { field: string; old: string; new: string }[] = []
    for (const f of fields) {
      const [o, n] = [f.value(before.routine), f.value(after.routine)]
      if (o !== n) out.push({ field: f.label, old: o, new: n })
    }
    if (JSON.stringify(before.routine.variables) !== JSON.stringify(after.routine.variables))
      out.push({ field: 'Variables', old: variablesSummary(before.routine), new: variablesSummary(after.routine) })
    for (const t of before.triggers) {
      const now = after.triggers.find((a) => a.id === t.id)
      const was = `${triggerSummary(t)}${t.enabled ? '' : ' (disabled)'}`
      if (!now) out.push({ field: `Trigger ${triggerName(t)}`, old: was, new: '—' })
      else if (JSON.stringify(now) !== JSON.stringify(t))
        out.push({ field: `Trigger ${triggerName(t)}`, old: was, new: `${triggerSummary(now)}${now.enabled ? '' : ' (disabled)'}` })
    }
    for (const t of after.triggers.filter((a) => !before.triggers.some((b) => b.id === a.id)))
      out.push({ field: `Trigger ${triggerName(t)}`, old: '—', new: `${triggerSummary(t)}${t.enabled ? '' : ' (disabled)'}` })
    return out
  }

  /** The Webhook triggers in the picked revision that the Routine no longer has. */
  const recreated = $derived(
    selected ? selected.snapshot.triggers.filter((t) => t.kind === 'webhook' && !routine.triggers.some((r) => r.id === t.id)) : [],
  )

  async function restore() {
    if (!selected || !current || restoring) return
    restoring = true
    const from = selected
    try {
      const res = await restoreRoutineRevision(routine.id, from.id)
      confirming = false
      picked = null
      revealed = res.secret_materials.map((m) => ({
        ...m,
        label: triggerName(from.snapshot.triggers.find((t) => t.id === m.trigger_id) ?? ({ kind: 'webhook', label: '' } as SnapshotTrigger)),
      }))
      toast.success(`Restored revision ${from.revision_number} as revision ${res.revision.revision_number}`, res.routine.title)
      await onrestored()
    } catch (e) {
      toast.error('Revision not restored', e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
    } finally {
      restoring = false
    }
  }
</script>

{#snippet caps(text: string)}
  <p class="text-xs font-medium tracking-(--tracking-caps) text-muted-foreground uppercase">{text}</p>
{/snippet}

{#snippet amber(text: string)}
  <Badge variant="outline" class="border-amber-500/40 bg-amber-500/10 px-1.5 text-(length:--text-nano) tracking-(--tracking-eyebrow) text-amber-800 uppercase dark:text-amber-200">{text}</Badge>
{/snippet}

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if revisions === null}
  <PageSkeleton />
{:else}
  <div class="space-y-4">
    {#each revealed as m (m.trigger_id)}
      <RoutineWebhookSecret
        title="Webhook trigger {m.label} recreated"
        url={location.origin + m.webhook_path}
        secret={m.webhook_secret}
        ondone={() => (revealed = revealed.filter((r) => r.trigger_id !== m.trigger_id))}
      />
    {/each}

    <div class="grid gap-5 md:grid-cols-[300px_minmax(0,1fr)]">
      <aside class="space-y-1" aria-label="Routine revisions">
        <header class="flex items-center justify-between pb-2">
          {@render caps('Revisions')}
          <span class="text-(length:--text-micro) text-muted-foreground">{revisions.length} total</span>
        </header>
        {#each revisions as revision (revision.id)}
          {@const isSelected = revision.id === selected?.id}
          {@const isCurrent = revision.id === current?.id}
          <button
            type="button"
            onclick={() => (picked = revision.id)}
            aria-pressed={isSelected}
            data-revision={revision.revision_number}
            class={[
              'w-full rounded-md border px-3 py-2 text-left transition-colors',
              isSelected && !isCurrent ? 'border-amber-500/40 bg-amber-500/10' : isSelected ? 'border-border bg-accent/40' : 'border-border/60 bg-transparent hover:bg-accent/40',
            ]}
          >
            <span class="flex items-center gap-2 text-sm font-medium">
              <span>Revision {revision.revision_number}</span>
              {#if isCurrent}
                <Badge variant="outline" class="border-border px-1.5 text-(length:--text-nano) tracking-(--tracking-eyebrow) text-muted-foreground uppercase">Current</Badge>
              {/if}
              {#if revision.restored_from_revision_id}{@render amber('Restored')}{/if}
            </span>
            {#if revision.change_summary}<span class="block truncate text-xs text-muted-foreground">{revision.change_summary}</span>{/if}
            <span class="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
              <Actor
                member={revision.author?.kind === 'member' ? revision.author : null}
                agent={revision.author?.kind === 'agent' ? { ...revision.author, icon: revision.author.icon ?? '' } : null}
                fallback="System"
                size="xs"
                class="min-w-0 truncate"
              />
              <span class="shrink-0">· {ago(revision.created_at)}</span>
            </span>
          </button>
        {/each}
      </aside>

      <div class="min-w-0 space-y-4">
        {#if revisions.length <= 1}
          <div class="space-y-2 py-12">
            <Empty title="No edits yet" icon="routines" />
            <p class="text-center text-xs text-muted-foreground">Revision 1 is the only history this routine has. Saving an edit creates the first additional revision.</p>
          </div>
        {:else if selected && current}
          {@const snapshot = selected.snapshot.routine}
          {#if historical}
            <div class="rounded-md border border-amber-500/30 bg-amber-500/5 px-4 py-3" role="status">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="space-y-1">
                  <p class="text-sm font-medium text-amber-800 dark:text-amber-200">You're viewing revision {selected.revision_number} (read-only)</p>
                  <p class="text-xs text-muted-foreground">
                    Restoring this revision creates a new revision {current.revision_number + 1} with the same content. History stays append-only.
                  </p>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <Button variant="outline" size="sm" onclick={() => (picked = null)}>Return to current</Button>
                  <Button variant="outline" size="sm" onclick={() => (comparing = true)}><Search class="size-3.5" />Compare</Button>
                  {#if editable}
                    <Button size="sm" disabled={restoring} onclick={() => (confirming = true)}><RotateCcw class="size-3.5" />Restore as new revision</Button>
                  {/if}
                </div>
              </div>
            </div>
          {/if}

          <header class="space-y-1 rounded-md border border-border p-4">
            <p class="text-sm font-medium">Revision {selected.revision_number}</p>
            <p class="truncate text-xs text-muted-foreground">
              Saved {ago(selected.created_at)} by {selected.author?.name ?? 'System'}{selected.change_summary ? ` · ${selected.change_summary}` : ''}
            </p>
          </header>

          <div class="rounded-md border border-border p-3">
            <div class="pb-2">{@render caps('Structured fields')}</div>
            <div class="grid gap-3 divide-y divide-border md:grid-cols-2 md:divide-y-0">
              {#each fields as field (field.label)}
                {@const value = field.value(snapshot)}
                <div class="space-y-1 p-2" data-revision-field={field.label}>
                  <p class="text-(length:--text-micro) tracking-wide text-muted-foreground uppercase">{field.label}</p>
                  <p class="text-sm">
                    {value || '—'}
                    {#if historical && value !== field.value(current.snapshot.routine)}<span class="ml-2">{@render amber('differs from current')}</span>{/if}
                  </p>
                </div>
              {/each}
            </div>
          </div>

          <div class="space-y-2 rounded-md border border-border p-3">
            {@render caps('Description')}
            <div class="rounded-md bg-background/40 p-3 text-sm leading-7">
              {#if snapshot.description.trim()}
                <Markdown source={snapshot.description} class="text-sm" />
              {:else}
                <span class="text-muted-foreground">No description</span>
              {/if}
            </div>
          </div>

          <div class="space-y-2 rounded-md border border-border p-3">
            {@render caps(`Triggers (${selected.snapshot.triggers.length})`)}
            {#if selected.snapshot.triggers.length === 0}
              <p class="text-sm text-muted-foreground">No triggers in this revision.</p>
            {:else}
              <ul class="divide-y divide-border">
                {#each selected.snapshot.triggers as t (t.id)}
                  <li class="flex flex-wrap items-center gap-2 py-2 text-sm">
                    <Badge variant="outline" class="border-border text-(length:--text-nano) tracking-(--tracking-eyebrow) text-muted-foreground uppercase">{t.kind}</Badge>
                    <span class="font-medium">{triggerName(t)}</span>
                    <span class="text-xs text-muted-foreground">{triggerSummary(t)}</span>
                    <span class={['ml-auto text-xs', t.enabled ? 'text-emerald-400' : 'text-muted-foreground']}>{t.enabled ? 'enabled' : 'disabled'}</span>
                  </li>
                {/each}
              </ul>
            {/if}
            <p class="text-xs text-muted-foreground">
              Webhook secrets are not kept in revisions. A Webhook trigger a restore recreates gets a new URL and secret.
            </p>
          </div>

          {#if snapshot.variables.length > 0}
            <div class="space-y-2 rounded-md border border-border p-3">
              {@render caps(`Variables (${snapshot.variables.length})`)}
              <ul class="divide-y divide-border">
                {#each snapshot.variables as v (v.name)}
                  <li class="flex items-center justify-between py-2 text-sm">
                    <span class="font-mono text-xs">{v.name}</span>
                    <span class="text-xs text-muted-foreground">default: {v.default_value ?? '—'}</span>
                  </li>
                {/each}
              </ul>
            </div>
          {/if}
        {/if}
      </div>
    </div>
  </div>

  {#if selected && current}
    {@const diff = changes(selected.snapshot, current.snapshot)}
    <DocumentDiff
      bind:open={comparing}
      old={{ label: `Revision ${selected.revision_number}`, body: selected.snapshot.routine.description }}
      new={{ label: `Revision ${current.revision_number} (current)`, body: current.snapshot.routine.description }}
    >
      {#snippet heading()}Compare routine revisions{/snippet}
      <section class="shrink-0 space-y-2">
        {@render caps('Field changes')}
        {#if diff.length === 0}
          <p class="text-sm text-muted-foreground">No structural field changes.</p>
        {:else}
          <table class="w-full overflow-hidden rounded-md border border-border text-sm" data-testid="revision-field-changes">
            <thead>
              <tr class="bg-muted/30 text-xs tracking-wide text-muted-foreground uppercase">
                <th class="px-3 py-2 text-left">Field</th>
                <th class="px-3 py-2 text-left">Old value</th>
                <th class="px-3 py-2 text-left">New value</th>
              </tr>
            </thead>
            <tbody>
              {#each diff as change (change.field)}
                <tr class="border-t border-border/60">
                  <td class="px-3 py-2 align-top text-xs font-medium">{change.field}</td>
                  <td class="px-3 py-2 align-top text-xs text-red-700 dark:text-red-300">{change.old}</td>
                  <td class="px-3 py-2 align-top text-xs text-emerald-700 dark:text-emerald-300">{change.new}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
        <div class="pt-2">{@render caps('Description diff')}</div>
      </section>
      {#snippet footer()}
        <Button variant="outline" onclick={() => (comparing = false)}>Close</Button>
        {#if editable}
          <Button
            onclick={() => {
              comparing = false
              confirming = true
            }}
          >
            <RotateCcw class="size-3.5" />Restore revision {selected?.revision_number} as new revision
          </Button>
        {/if}
      {/snippet}
    </DocumentDiff>

    <Dialog.Root bind:open={confirming}>
      <Dialog.Content class="sm:max-w-md">
        <Dialog.Header>
          <Dialog.Title>Restore revision {selected.revision_number}?</Dialog.Title>
          <Dialog.Description>
            This creates a new revision {current.revision_number + 1} with the same content as revision {selected.revision_number}. Revisions
            {selected.revision_number}–{current.revision_number} stay in history and are not changed.
          </Dialog.Description>
        </Dialog.Header>
        <ul class="space-y-2 text-sm">
          {#each ['Routine field values, variables and schedule crons will revert.', 'Previous run history is kept.'] as line (line)}
            <li class="flex items-start gap-2"><span class="mt-1.5 inline-block size-1.5 shrink-0 rounded-full bg-emerald-400"></span>{line}</li>
          {/each}
          {#each recreated as t (t.id)}
            <li class="flex items-start gap-2 text-amber-800 dark:text-amber-200" data-recreated-webhook={t.id}>
              <span class="mt-1.5 inline-block size-1.5 shrink-0 rounded-full bg-amber-400"></span>
              The webhook trigger {triggerName(t)} will be recreated with a new URL and secret. The secret is shown once after the restore; copy it before closing.
            </li>
          {/each}
        </ul>
        <Dialog.Footer>
          <Button variant="outline" disabled={restoring} onclick={() => (confirming = false)}>Cancel</Button>
          <Button disabled={restoring} onclick={restore}>
            <RotateCcw class="size-3.5" />{restoring ? 'Restoring…' : `Restore as revision ${current.revision_number + 1}`}
          </Button>
        </Dialog.Footer>
      </Dialog.Content>
    </Dialog.Root>
  {/if}
{/if}
