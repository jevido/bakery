<script lang="ts">
  // Paperclip's TriggersSection (ui/src/components/routine-triggers/
  // RoutineTriggers.tsx; MIT, see NOTICE): a card per Routine trigger, then
  // Add trigger with its kind, Schedule or API. Delete hides the card and
  // offers Undo in a toast; the trigger is only deleted once the toast has
  // gone (or the page is left), as Paperclip's removal with Undo. Left out
  // with webhooks: signing, secrets and the trigger wizard's steps.
  import { Clock3, Plus, Zap } from '@lucide/svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import { Input } from '@bakery/ui/components/ui/input'
  import { onDestroy } from 'svelte'
  import { ApiError } from '../../lib/api'
  import { addTrigger, deleteTrigger, updateTrigger, type RoutineDetail, type RoutineTrigger, type RoutineTriggerKind } from '../../lib/routines'
  import { localTimeZone } from '../../lib/schedule'
  import ScheduleEditor from '../../lib/ScheduleEditor.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import RoutineTriggerCard from './RoutineTriggerCard.svelte'

  let { routine, editable, onchange }: { routine: RoutineDetail; editable: boolean; onchange: () => Promise<unknown> } = $props()

  /** How long Undo is offered, as the toast stays. */
  const undoFor = 5000

  let adding = $state<RoutineTriggerKind | null>(null)
  let label = $state('')
  let cron = $state('0 10 * * *')
  let timezone = $state(localTimeZone())
  let valid = $state(true)
  let saving = $state(false)
  /** Triggers deleted but still undoable, each with the timer that sends the delete. */
  let pending = $state(new Map<number, ReturnType<typeof setTimeout>>())

  const shown = $derived(routine.triggers.filter((t) => !pending.has(t.id)))
  const message = (e: unknown) => (e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))

  async function save(t: RoutineTrigger, patch: Parameters<typeof updateTrigger>[1]) {
    try {
      await updateTrigger(t.id, patch)
      await onchange()
      return true
    } catch (e) {
      toast.error('Trigger not saved', message(e))
      return false
    }
  }

  async function send(id: number) {
    pending.delete(id)
    pending = new Map(pending)
    try {
      await deleteTrigger(id)
    } catch (e) {
      toast.error('Trigger not deleted', message(e))
    }
    await onchange()
  }

  function remove(t: RoutineTrigger) {
    pending = new Map(pending).set(
      t.id,
      setTimeout(() => send(t.id), undoFor),
    )
    toast.show('Trigger deleted', t.label || (t.kind === 'schedule' ? 'Schedule' : 'API'), {
      label: 'Undo',
      onclick: () => {
        clearTimeout(pending.get(t.id))
        pending.delete(t.id)
        pending = new Map(pending)
      },
    })
  }

  // Leaving the page sends the deletes still waiting for their Undo.
  onDestroy(() => {
    for (const [id, timer] of pending) {
      clearTimeout(timer)
      deleteTrigger(id).catch(() => {})
    }
  })

  function start(kind: RoutineTriggerKind) {
    adding = kind
    label = ''
    cron = '0 10 * * *'
    timezone = localTimeZone()
  }

  async function add() {
    if (!adding) return
    saving = true
    try {
      await addTrigger(routine.id, adding === 'schedule' ? { kind: 'schedule', label, cron_expression: cron, timezone } : { kind: 'api', label })
      toast.success('Trigger added', adding === 'schedule' ? 'The routine schedule was saved.' : 'The routine can now be run through the API.')
      adding = null
      await onchange()
    } catch (e) {
      toast.error('Trigger not added', message(e))
    } finally {
      saving = false
    }
  }
</script>

<div class="space-y-4">
  {#if shown.length === 0 && !adding}
    <div class="py-6">
      <Empty title="No triggers yet. Add a schedule or an API trigger, or press Run." icon="routines" />
    </div>
  {/if}
  {#each shown as t (t.id)}
    <RoutineTriggerCard trigger={t} routineId={routine.id} {editable} onsave={(patch) => save(t, patch)} ondelete={() => remove(t)} />
  {/each}

  {#if adding}
    <div class="space-y-3 rounded-lg border border-dashed border-border p-4" role="group" aria-label="New trigger">
      <div class="flex gap-2">
        <Button variant={adding === 'schedule' ? 'secondary' : 'ghost'} size="sm" onclick={() => (adding = 'schedule')}><Clock3 class="size-3.5" />Schedule</Button>
        <Button variant={adding === 'api' ? 'secondary' : 'ghost'} size="sm" onclick={() => (adding = 'api')}><Zap class="size-3.5" />API</Button>
      </div>
      <label class="block space-y-1.5">
        <span class="text-xs font-medium">Label</span>
        <Input bind:value={label} placeholder={adding === 'schedule' ? 'Schedule' : 'API'} />
      </label>
      {#if adding === 'schedule'}
        <ScheduleEditor bind:cron bind:timezone bind:valid />
      {:else}
        <p class="text-xs text-muted-foreground">An API trigger runs the Routine when a request with an API token calls it. Its card shows the call.</p>
      {/if}
      <div class="flex justify-end gap-2">
        <Button variant="ghost" size="sm" onclick={() => (adding = null)}>Cancel</Button>
        <Button size="sm" disabled={saving || (adding === 'schedule' && !valid)} onclick={add}>{saving ? 'Adding…' : 'Add trigger'}</Button>
      </div>
    </div>
  {:else if editable}
    <div class="flex gap-2">
      <Button variant="outline" size="sm" onclick={() => start('schedule')}><Plus class="size-3.5" />Add schedule</Button>
      <Button variant="outline" size="sm" onclick={() => start('api')}><Plus class="size-3.5" />Add API trigger</Button>
    </div>
  {/if}
</div>
