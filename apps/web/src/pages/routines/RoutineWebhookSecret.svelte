<script lang="ts">
  // Paperclip's secret message (ui/src/components/routine-triggers/
  // RoutineTriggers.tsx with WebhookFields.tsx's CopyField; MIT, see
  // NOTICE): a Webhook trigger's full URL and its secret, each with a copy
  // button, shown once after the trigger is made or its secret rotated. It
  // stays until Done.
  import { Button } from '@bakery/ui/components/ui/button'
  import CopyButton from '../../lib/CopyButton.svelte'

  let { title, url, secret, ondone }: { title: string; url: string; secret: string; ondone: () => void } = $props()
</script>

<div class="space-y-3 rounded-md border border-border p-4" role="region" aria-label="Webhook secret">
  <p class="text-sm font-medium">{title}</p>
  {#each [{ label: 'Webhook URL', value: url }, { label: 'Secret key', value: secret }] as field (field.label)}
    <div class="space-y-1.5">
      <span class="text-xs font-medium">{field.label}</span>
      <div class="flex min-w-0 items-center gap-2 rounded-md border border-border bg-background px-3 py-2">
        <code title={field.value} class="min-w-0 flex-1 truncate text-xs" data-field={field.label}>{field.value}</code>
        <span class="text-xs text-muted-foreground hover:text-foreground"><CopyButton text={field.value} label="Copy {field.label}" /></span>
      </div>
    </div>
  {/each}
  <p class="text-xs text-muted-foreground">The secret key is not shown again. Paste it into the sending app now, or rotate it later for a new one.</p>
  <Button variant="outline" size="sm" onclick={ondone}>Done</Button>
</div>
