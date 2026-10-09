<script lang="ts">
  // Paperclip's RoutineTriggerCard (ui/src/components/RoutineTriggerCard.tsx;
  // MIT, see NOTICE): one Routine trigger with its kind, label, Schedule in
  // words and time zone, its Next run and the result it last fired with.
  // The Bakery shows the enabled switch on the card, opens the label and
  // ScheduleEditor behind Edit, and gives an API trigger the curl line that
  // runs it with an API token, where Paperclip's webhook trigger shows its
  // URL and signing.
  import { Clock3, Pencil, Trash2, Zap } from '@lucide/svelte'
  import { Badge } from '@bakery/ui/components/ui/badge'
  import { Button } from '@bakery/ui/components/ui/button'
  import { Input } from '@bakery/ui/components/ui/input'
  import { Switch } from '@bakery/ui/components/ui/switch'
  import CopyButton from '../../lib/CopyButton.svelte'
  import { href } from '../../lib/router.svelte'
  import type { RoutineTrigger } from '../../lib/routines'
  import { describeSchedule } from '../../lib/schedule'
  import ScheduleEditor from '../../lib/ScheduleEditor.svelte'

  let {
    trigger,
    routineId,
    editable,
    onsave,
    ondelete,
  }: {
    trigger: RoutineTrigger
    routineId: number
    editable: boolean
    onsave: (patch: { label?: string; cron_expression?: string; timezone?: string; enabled?: boolean }) => Promise<boolean>
    ondelete: () => void
  } = $props()

  let editing = $state(false)
  let label = $state('')
  let cron = $state('')
  let timezone = $state('')
  let valid = $state(true)
  let saving = $state(false)

  const name = $derived(trigger.label || (trigger.kind === 'schedule' ? 'Schedule' : 'API'))
  const failed = $derived(/fail|error|skip/i.test(trigger.last_result ?? ''))
  const curl = $derived(
    `curl -X POST ${location.origin}/api/routines/${routineId}/run \\\n  -H "Authorization: Bearer $BAKERY_API_TOKEN" \\\n  -H "Content-Type: application/json" \\\n  -d '{"trigger_id": ${trigger.id}}'`,
  )

  function edit() {
    label = trigger.label
    cron = trigger.cron_expression ?? ''
    timezone = trigger.timezone ?? ''
    editing = true
  }

  async function save() {
    saving = true
    const patch = trigger.kind === 'schedule' ? { label, cron_expression: cron, timezone } : { label }
    if (await onsave(patch)) editing = false
    saving = false
  }
</script>

<div class="space-y-4 rounded-lg border border-border p-4" role="group" aria-label="Trigger: {name}" data-trigger={trigger.id}>
  <div class="flex items-start justify-between gap-3">
    <div class="min-w-0 space-y-1">
      <div class="flex items-center gap-2 text-sm font-medium">
        {#if trigger.kind === 'schedule'}<Clock3 class="size-3.5 shrink-0" />{:else}<Zap class="size-3.5 shrink-0" />{/if}
        <span class="truncate">{name}</span>
        <span class="text-xs font-normal text-muted-foreground">{trigger.kind === 'schedule' ? 'Schedule' : 'API'}</span>
      </div>
      {#if trigger.kind === 'schedule'}
        <p class="text-xs text-muted-foreground">{describeSchedule(trigger.cron_expression ?? '')} · {trigger.timezone}</p>
      {/if}
    </div>
    <div class="flex shrink-0 items-center gap-3">
      {#if trigger.last_result}
        <Badge variant={failed ? 'destructive' : 'secondary'} title={trigger.last_fired_at ? `Last fired ${new Date(trigger.last_fired_at).toLocaleString()}` : undefined}>
          {trigger.last_result}
        </Badge>
      {/if}
      <span class="text-xs text-muted-foreground">
        {#if trigger.kind === 'schedule'}
          {trigger.enabled && trigger.next_run_at ? `Next: ${new Date(trigger.next_run_at).toLocaleString()}` : 'Not scheduled'}
        {:else}
          {trigger.last_fired_at ? `Last fired ${new Date(trigger.last_fired_at).toLocaleString()}` : 'Never fired'}
        {/if}
      </span>
      <Switch
        checked={trigger.enabled}
        disabled={!editable}
        onCheckedChange={(enabled) => onsave({ enabled })}
        aria-label={trigger.enabled ? `Disable ${name}` : `Enable ${name}`}
      />
    </div>
  </div>

  {#if trigger.kind === 'api'}
    <div class="space-y-1.5">
      <div class="flex items-center justify-between gap-2">
        <p class="text-xs font-medium">Run it from anywhere</p>
        <CopyButton text={curl.replaceAll('\\\n  ', '')} label="Copy curl" />
      </div>
      <pre class="overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs">{curl}</pre>
      <p class="text-xs text-muted-foreground">
        Use an API token from <a href={href('/security/api-tokens')} class="underline">Keys &amp; Tokens</a>. Each call is one Routine run with source "api".
      </p>
    </div>
  {/if}

  {#if editing}
    <div class="grid gap-3">
      <label class="space-y-1.5">
        <span class="text-xs font-medium">Label</span>
        <Input bind:value={label} placeholder={trigger.kind === 'schedule' ? 'Schedule' : 'API'} />
      </label>
      {#if trigger.kind === 'schedule'}
        <div class="space-y-1.5">
          <span class="text-xs font-medium">Schedule</span>
          <ScheduleEditor bind:cron bind:timezone bind:valid />
        </div>
      {/if}
      <div class="flex justify-end gap-2">
        <Button variant="ghost" size="sm" onclick={() => (editing = false)}>Cancel</Button>
        <Button size="sm" disabled={saving || !valid} onclick={save}>{saving ? 'Saving…' : 'Save trigger'}</Button>
      </div>
    </div>
  {:else if editable}
    <div class="flex items-center justify-end gap-2">
      <Button variant="ghost" size="sm" class="mr-auto text-muted-foreground hover:text-destructive" onclick={ondelete}>
        <Trash2 class="size-3.5" />Delete
      </Button>
      <Button variant="outline" size="sm" onclick={edit}><Pencil class="size-3.5" />Edit</Button>
    </div>
  {/if}
</div>
