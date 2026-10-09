<script lang="ts">
  // Paperclip's RoutineTriggerCard (ui/src/components/RoutineTriggerCard.tsx;
  // MIT, see NOTICE): one Routine trigger with its kind, label, Schedule in
  // words and time zone, its Next run and the result it last fired with.
  // The Bakery shows the enabled switch on the card, opens the label and
  // ScheduleEditor behind Edit, and gives an API trigger the curl line that
  // runs it with an API token. A Webhook trigger shows its URL, Signing
  // mode, Replay window, last delivery and a curl line per Signing mode
  // with $SECRET for the secret (Paperclip's WebhookTriggerDetails), and
  // Rotate secret behind a confirmation, as Paperclip's Replace key.
  import { Clock3, KeyRound, Pencil, Trash2, Webhook, Zap } from '@lucide/svelte'
  import * as AlertDialog from '@bakery/ui/components/ui/alert-dialog'
  import { Badge } from '@bakery/ui/components/ui/badge'
  import { Button, buttonVariants } from '@bakery/ui/components/ui/button'
  import { Input } from '@bakery/ui/components/ui/input'
  import { Switch } from '@bakery/ui/components/ui/switch'
  import CopyButton from '../../lib/CopyButton.svelte'
  import { href } from '../../lib/router.svelte'
  import { ago } from '../../lib/format'
  import { replayWindow, signingModeHelp, type RoutineTrigger, type SigningMode } from '../../lib/routines'
  import { describeSchedule } from '../../lib/schedule'
  import ScheduleEditor from '../../lib/ScheduleEditor.svelte'
  import SigningModeSelect from './SigningModeSelect.svelte'

  let {
    trigger,
    routineId,
    editable,
    onsave,
    ondelete,
    onrotate,
  }: {
    trigger: RoutineTrigger
    routineId: number
    editable: boolean
    onsave: (patch: {
      label?: string
      cron_expression?: string
      timezone?: string
      enabled?: boolean
      signing_mode?: SigningMode
      replay_window_sec?: number
    }) => Promise<boolean>
    ondelete: () => void
    onrotate: () => Promise<void>
  } = $props()

  let editing = $state(false)
  let label = $state('')
  let cron = $state('')
  let timezone = $state('')
  let valid = $state(true)
  let signingMode = $state<SigningMode>('bearer')
  let replay = $state<number>(replayWindow.default)
  let saving = $state(false)
  let rotating = $state(false)

  const kindName = { schedule: 'Schedule', api: 'API', webhook: 'Webhook' }
  const name = $derived(trigger.label || kindName[trigger.kind])
  const url = $derived(location.origin + (trigger.webhook_path ?? ''))
  /** How a sender fires it, per Signing mode, with $SECRET for the secret. */
  const webhookCurl = $derived.by(() => {
    const post = `curl -X POST ${url} \\\n  -H "Content-Type: application/json"`
    switch (trigger.signing_mode) {
      case 'bearer':
        return `${post} \\\n  -H "Authorization: Bearer $SECRET" \\\n  -d '{}'`
      case 'github_hmac':
        return `BODY='{}'\nSIG=$(printf %s "$BODY" | openssl dgst -sha256 -hmac "$SECRET" | sed 's/^.* //')\n${post} \\\n  -H "X-Hub-Signature-256: sha256=$SIG" \\\n  -H "X-GitHub-Delivery: $(uuidgen)" \\\n  -d "$BODY"`
      case 'hmac_sha256':
        return `BODY='{}'\nTS=$(date +%s)\nSIG=$(printf %s "$TS.$BODY" | openssl dgst -sha256 -hmac "$SECRET" | sed 's/^.* //')\n${post} \\\n  -H "X-Bakery-Timestamp: $TS" \\\n  -H "X-Bakery-Signature: sha256=$SIG" \\\n  -d "$BODY"`
      default:
        return `${post} \\\n  -d '{}'`
    }
  })
  const failed = $derived(/fail|error|skip/i.test(trigger.last_result ?? ''))
  const curl = $derived(
    `curl -X POST ${location.origin}/api/routines/${routineId}/run \\\n  -H "Authorization: Bearer $BAKERY_API_TOKEN" \\\n  -H "Content-Type: application/json" \\\n  -d '{"trigger_id": ${trigger.id}}'`,
  )

  function edit() {
    label = trigger.label
    cron = trigger.cron_expression ?? ''
    timezone = trigger.timezone ?? ''
    signingMode = trigger.signing_mode ?? 'bearer'
    replay = trigger.replay_window_sec ?? replayWindow.default
    editing = true
  }

  async function save() {
    saving = true
    const patch =
      trigger.kind === 'schedule'
        ? { label, cron_expression: cron, timezone }
        : trigger.kind === 'webhook'
          ? { label, signing_mode: signingMode, ...(signingMode === 'hmac_sha256' ? { replay_window_sec: replay } : {}) }
          : { label }
    if (await onsave(patch)) editing = false
    saving = false
  }
</script>

<div class="space-y-4 rounded-lg border border-border p-4" role="group" aria-label="Trigger: {name}" data-trigger={trigger.id}>
  <div class="flex items-start justify-between gap-3">
    <div class="min-w-0 space-y-1">
      <div class="flex items-center gap-2 text-sm font-medium">
        {#if trigger.kind === 'schedule'}<Clock3 class="size-3.5 shrink-0" />{:else if trigger.kind === 'webhook'}<Webhook class="size-3.5 shrink-0" />{:else}<Zap class="size-3.5 shrink-0" />{/if}
        <span class="truncate">{name}</span>
        <span class="text-xs font-normal text-muted-foreground">{kindName[trigger.kind]}</span>
      </div>
      {#if trigger.kind === 'schedule'}
        <p class="text-xs text-muted-foreground">{describeSchedule(trigger.cron_expression ?? '')} · {trigger.timezone}</p>
      {:else if trigger.kind === 'webhook' && trigger.signing_mode}
        <p class="text-xs text-muted-foreground">
          {signingModeHelp[trigger.signing_mode].label}{trigger.signing_mode === 'hmac_sha256' ? ` · Replay window ${trigger.replay_window_sec}s` : ''}
        </p>
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
        {:else if trigger.kind === 'webhook'}
          <span data-last-delivery title={trigger.last_delivery ? new Date(trigger.last_delivery.received_at).toLocaleString() : undefined}>
            {trigger.last_delivery ? `Last delivery: ${trigger.last_delivery.status} ${ago(trigger.last_delivery.received_at)}` : 'No deliveries yet'}
          </span>
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

  {#if trigger.kind === 'webhook'}
    <div class="space-y-3">
      <div class="space-y-1.5">
        <div class="flex items-center justify-between gap-2">
          <p class="text-xs font-medium">Webhook URL</p>
          <span class="text-xs text-muted-foreground hover:text-foreground"><CopyButton text={url} label="Copy URL" /></span>
        </div>
        <code class="block truncate rounded-md border border-border px-3 py-2 text-xs" title={url} data-webhook-url>{url}</code>
      </div>
      <div class="space-y-1.5">
        <div class="flex items-center justify-between gap-2">
          <p class="text-xs font-medium">Fire it from anywhere</p>
          <CopyButton text={webhookCurl} label="Copy curl" />
        </div>
        <pre class="overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs">{webhookCurl}</pre>
        <p class="text-xs text-muted-foreground">
          {trigger.signing_mode === 'none'
            ? 'This webhook uses its URL as the shared secret.'
            : 'The secret key is hidden. Set SECRET to the key shown when the trigger was made or its secret last rotated.'}
          A JSON body's fields fill the Routine's variables. Each delivery is one Routine run with source "webhook".
        </p>
      </div>
    </div>
  {/if}

  {#if editing}
    <div class="grid gap-3">
      <label class="space-y-1.5">
        <span class="text-xs font-medium">Label</span>
        <Input bind:value={label} placeholder={kindName[trigger.kind]} />
      </label>
      {#if trigger.kind === 'webhook'}
        <SigningModeSelect bind:value={signingMode} />
        {#if signingMode === 'hmac_sha256'}
          <label class="space-y-1.5">
            <span class="text-xs font-medium">Replay window (seconds)</span>
            <Input type="number" min={replayWindow.min} max={replayWindow.max} bind:value={replay} />
          </label>
        {/if}
      {/if}
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
      {#if trigger.kind === 'webhook'}
        <Button variant="outline" size="sm" onclick={() => (rotating = true)}><KeyRound class="size-3.5" />Rotate secret</Button>
      {/if}
      <Button variant="outline" size="sm" onclick={edit}><Pencil class="size-3.5" />Edit</Button>
    </div>
  {/if}

  {#if trigger.kind === 'webhook'}
    <AlertDialog.Root open={rotating} onOpenChange={(open) => (rotating = open)}>
      <AlertDialog.Content>
        <AlertDialog.Header>
          <AlertDialog.Title>Rotate the webhook secret?</AlertDialog.Title>
          <AlertDialog.Description>The old secret stops working at once. Update the sending app with the new one.</AlertDialog.Description>
        </AlertDialog.Header>
        <AlertDialog.Footer>
          <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
          <AlertDialog.Action class={buttonVariants({ variant: 'destructive' })} onclick={() => ((rotating = false), onrotate())}>Rotate secret</AlertDialog.Action>
        </AlertDialog.Footer>
      </AlertDialog.Content>
    </AlertDialog.Root>
  {/if}
</div>
