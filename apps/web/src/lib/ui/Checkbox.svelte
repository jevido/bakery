<script lang="ts">
  // Coolify's forms/checkbox (resources/views/components/forms/checkbox.blade.php)
  // on Paperclip's checkbox: the whole row is the hit area, its <label>
  // points at the box. `instantSave` stands in for wire:click="instantSave":
  // it is called with the new value on every change.
  import { Checkbox as UiCheckbox } from '@bakery/ui/components/ui/checkbox'
  import Helper from './Helper.svelte'

  let {
    checked = $bindable(false),
    label,
    helper,
    disabled = false,
    fullWidth = false,
    id,
    instantSave,
  }: {
    checked?: boolean
    label?: string
    helper?: string
    disabled?: boolean
    fullWidth?: boolean
    id?: string
    instantSave?: (checked: boolean) => void
  } = $props()

  const generated = $props.id()
  const htmlId = $derived(id ?? generated)
</script>

<div
  class={[
    'chrome form-control flex min-h-9 max-w-full items-center rounded-md px-2.5 py-1.5 transition-colors',
    fullWidth && 'w-full',
    !disabled && 'cursor-pointer hover:bg-accent/50',
    disabled && 'opacity-50',
  ]}
>
  <label
    for={htmlId}
    class={['flex w-full max-w-full min-w-0 items-center gap-3', disabled ? 'cursor-not-allowed' : 'cursor-pointer']}
  >
    <span class="flex min-w-0 grow items-center gap-1.5 text-sm break-words text-foreground">
      {#if label}
        {label}
        {#if helper}<Helper {helper} />{/if}
      {/if}
    </span>
    <UiCheckbox id={htmlId} bind:checked {disabled} onCheckedChange={(value) => instantSave?.(value)} />
  </label>
</div>
