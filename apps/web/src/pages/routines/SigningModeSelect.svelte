<script lang="ts">
  // A Webhook trigger's Signing mode with its one-line help, as Paperclip's
  // TriggerWizard asks how the sender authenticates.
  import * as Select from '@bakery/ui/components/ui/select'
  import { signingModeHelp, signingModes, type SigningMode } from '../../lib/routines'

  let { value = $bindable() }: { value: SigningMode } = $props()
</script>

<div class="space-y-1.5">
  <span class="text-xs font-medium">Signing mode</span>
  <Select.Root type="single" {value} onValueChange={(v) => (value = v as SigningMode)}>
    <Select.Trigger class="w-full" aria-label="Signing mode">{signingModeHelp[value].label}</Select.Trigger>
    <Select.Content>
      {#each signingModes as mode (mode)}
        <Select.Item value={mode} label={signingModeHelp[mode].label} />
      {/each}
    </Select.Content>
  </Select.Root>
  <p class="text-xs text-muted-foreground">{signingModeHelp[value].help}</p>
</div>
