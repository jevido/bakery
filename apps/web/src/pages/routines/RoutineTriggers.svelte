<script lang="ts">
  // Paperclip's TriggersSection (ui/src/components/routine-triggers/
  // RoutineTriggers.tsx; MIT, see NOTICE): a card per Routine trigger, then
  // Add trigger with its kind, Schedule, API or Webhook. Delete hides the
  // card and offers Undo in a toast; the trigger is only deleted once the
  // toast has gone (or the page is left), as Paperclip's removal with Undo.
  // A Webhook trigger's URL and secret show once, after adding or rotating,
  // in Paperclip's secret message. Left out: the trigger wizard's steps and
  // its setup-pending test delivery.
  import { Clock3, Plus, Webhook, Zap } from '@lucide/svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import { Input } from '@bakery/ui/components/ui/input'
  import { onDestroy } from 'svelte'
  import { ApiError } from '../../lib/api'
  import {
    addTrigger,
    deleteTrigger,
    replayWindow,
    rotateTriggerSecret,
    updateTrigger,
    type RoutineDetail,
    type RoutineTrigger,
    type RoutineTriggerKind,
    type SecretMaterial,
    type SigningMode,
  } from '../../lib/routines'
  import { localTimeZone } from '../../lib/schedule'
  import ScheduleEditor from '../../lib/ScheduleEditor.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import RoutineTriggerCard from './RoutineTriggerCard.svelte'
  import RoutineWebhookSecret from './RoutineWebhookSecret.svelte'
  import SigningModeSelect from './SigningModeSelect.svelte'

  let { routine, editable, onchange }: { routine: RoutineDetail; editable: boolean; onchange: () => Promise<unknown> } = $props()

  /** How long Undo is offered, as the toast stays. */
  const undoFor = 5000

  let adding = $state<RoutineTriggerKind | null>(null)
  let label = $state('')
  let cron = $state('0 10 * * *')
  let timezone = $state(localTimeZone())
  let valid = $state(true)
  let signingMode = $state<SigningMode>('bearer')
  let replay = $state<number>(replayWindow.default)
  let saving = $state(false)
  /** The secret message, shown once after a Webhook trigger is added or rotated. */
  let revealed = $state<{ title: string; material: SecretMaterial } | null>(null)
  /** Triggers deleted but still undoable, each with the timer that sends the delete. */
  let pending = $state(new Map<number, ReturnType<typeof setTimeout>>())

  const shown = $derived(routine.triggers.filter((t) => !pending.has(t.id)))
  const kindName: Record<RoutineTriggerKind, string> = { schedule: 'Schedule', api: 'API', webhook: 'Webhook' }
  const added: Record<RoutineTriggerKind, string> = {
    schedule: 'The routine schedule was saved.',
    api: 'The routine can now be run through the API.',
    webhook: 'Copy its URL and secret key now.',
  }
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
    toast.show('Trigger deleted', t.label || kindName[t.kind], {
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
    signingMode = 'bearer'
    replay = replayWindow.default
  }

  function input(kind: RoutineTriggerKind) {
    if (kind === 'schedule') return { kind, label, cron_expression: cron, timezone }
    if (kind === 'webhook') return { kind, label, signing_mode: signingMode, ...(signingMode === 'hmac_sha256' ? { replay_window_sec: replay } : {}) }
    return { kind, label }
  }

  async function rotate(t: RoutineTrigger) {
    try {
      const { secret_material } = await rotateTriggerSecret(t.id)
      revealed = { title: 'Webhook secret rotated', material: secret_material }
      await onchange()
    } catch (e) {
      toast.error('Secret not rotated', message(e))
    }
  }

  async function add() {
    if (!adding) return
    saving = true
    try {
      const { secret_material } = await addTrigger(routine.id, input(adding))
      if (secret_material) revealed = { title: 'Webhook trigger added', material: secret_material }
      toast.success('Trigger added', added[adding])
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
  {#if revealed}
    <RoutineWebhookSecret
      title={revealed.title}
      url={location.origin + revealed.material.webhook_path}
      secret={revealed.material.webhook_secret}
      ondone={() => (revealed = null)}
    />
  {/if}
  {#if shown.length === 0 && !adding}
    <div class="py-6">
      <Empty title="No triggers yet. Add a schedule, an API trigger or a webhook, or press Run." icon="routines" />
    </div>
  {/if}
  {#each shown as t (t.id)}
    <RoutineTriggerCard trigger={t} routineId={routine.id} {editable} onsave={(patch) => save(t, patch)} ondelete={() => remove(t)} onrotate={() => rotate(t)} />
  {/each}

  {#if adding}
    <div class="space-y-3 rounded-lg border border-dashed border-border p-4" role="group" aria-label="New trigger">
      <div class="flex gap-2">
        <Button variant={adding === 'schedule' ? 'secondary' : 'ghost'} size="sm" onclick={() => (adding = 'schedule')}><Clock3 class="size-3.5" />Schedule</Button>
        <Button variant={adding === 'api' ? 'secondary' : 'ghost'} size="sm" onclick={() => (adding = 'api')}><Zap class="size-3.5" />API</Button>
        <Button variant={adding === 'webhook' ? 'secondary' : 'ghost'} size="sm" onclick={() => (adding = 'webhook')}><Webhook class="size-3.5" />Webhook</Button>
      </div>
      <label class="block space-y-1.5">
        <span class="text-xs font-medium">Label</span>
        <Input bind:value={label} placeholder={kindName[adding]} />
      </label>
      {#if adding === 'schedule'}
        <ScheduleEditor bind:cron bind:timezone bind:valid />
      {:else if adding === 'webhook'}
        <SigningModeSelect bind:value={signingMode} />
        {#if signingMode === 'hmac_sha256'}
          <label class="block space-y-1.5">
            <span class="text-xs font-medium">Replay window (seconds)</span>
            <Input type="number" min={replayWindow.min} max={replayWindow.max} bind:value={replay} />
          </label>
        {/if}
      {:else}
        <p class="text-xs text-muted-foreground">An API trigger runs the Routine when a request with an API token calls it. Its card shows the call.</p>
      {/if}
      <div class="flex justify-end gap-2">
        <Button variant="ghost" size="sm" onclick={() => (adding = null)}>Cancel</Button>
        <Button size="sm" disabled={saving || (adding === 'schedule' && !valid) || (adding === 'webhook' && signingMode === 'hmac_sha256' && !(replay >= replayWindow.min && replay <= replayWindow.max))} onclick={add}>{saving ? 'Adding…' : 'Add trigger'}</Button>
      </div>
    </div>
  {:else if editable}
    <div class="flex gap-2">
      <Button variant="outline" size="sm" onclick={() => start('schedule')}><Plus class="size-3.5" />Add schedule</Button>
      <Button variant="outline" size="sm" onclick={() => start('api')}><Plus class="size-3.5" />Add API trigger</Button>
      <Button variant="outline" size="sm" onclick={() => start('webhook')}><Plus class="size-3.5" />Add webhook</Button>
    </div>
  {/if}
</div>
