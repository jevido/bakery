<script lang="ts">
  // One ghost icon button of the log toolbar with its tooltip, as Paperclip's
  // toolbars draw them (ui/src/components/transcript/RunTranscriptView.tsx;
  // MIT, see NOTICE). Other attributes, such as a menu trigger's, pass
  // through the tooltip's trigger, which merges them with its own.
  import type { Tooltip as TooltipPrimitive } from 'bits-ui'
  import { Button } from '@bakery/ui/components/ui/button'
  import * as Tooltip from '@bakery/ui/components/ui/tooltip'
  import { cn } from '@bakery/ui/utils'
  import Icon, { type IconName } from '../../lib/Icon.svelte'

  let {
    label,
    icon,
    active = false,
    class: className = '',
    ...rest
  }: Omit<TooltipPrimitive.TriggerProps, 'child' | 'children'> & { label: string; icon: IconName; active?: boolean } = $props()
</script>

<Tooltip.Root>
  <Tooltip.Trigger {...rest}>
    {#snippet child({ props })}
      <Button
        {...props}
        variant="ghost"
        size="icon-sm"
        aria-label={label}
        class={cn('text-muted-foreground sm:size-7', active && 'bg-accent text-accent-foreground', className)}
      >
        <Icon name={icon} />
      </Button>
    {/snippet}
  </Tooltip.Trigger>
  <Tooltip.Content>{label}</Tooltip.Content>
</Tooltip.Root>
