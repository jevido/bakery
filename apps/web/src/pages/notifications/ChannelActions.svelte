<script lang="ts">
  // Coolify's notification channel actions (resources/views/components/notification/channel-actions.blade.php,
  // Apache-2.0, see NOTICE): Enable/Disable, highlighted while disabled, and
  // Send test, disabled while the channel is. Enabling and testing first ask
  // the form whether it is valid, as Coolify's reportValidity() does.
  import Icon from '../../lib/Icon.svelte'
  import Button from '../../lib/ui/Button.svelte'

  let {
    enabled,
    busy = false,
    testing = false,
    ontoggle,
    ontest,
    plainWhenDisabled = false,
  }: {
    enabled: boolean
    busy?: boolean
    testing?: boolean
    ontoggle: () => unknown
    ontest: () => unknown
    /** Email: a disabled Send test is a bare button, as in Coolify's email.blade.php. */
    plainWhenDisabled?: boolean
  } = $props()

  const valid = (e: MouseEvent) => (e.currentTarget as HTMLElement).closest('form')?.reportValidity() ?? true
</script>

<div class="flex items-center gap-2">
  <Button
    variant={enabled ? 'default' : 'highlighted'}
    loading={busy}
    data-testid="channel-toggle"
    onclick={(e: MouseEvent) => {
      if (!enabled && !valid(e)) return
      ontoggle()
    }}
  >
    {enabled ? 'Disable' : 'Enable'}
  </Button>
  <Button
    disabled={!enabled}
    loading={testing}
    data-testid="channel-test"
    onclick={(e: MouseEvent) => {
      if (valid(e)) ontest()
    }}
  >
    {#if !testing && (enabled || !plainWhenDisabled)}<Icon name="notifications" class="size-3.5" />{/if}
    Send test
  </Button>
</div>
