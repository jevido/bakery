<script lang="ts">
  // Coolify's forms/select (resources/views/components/forms/select.blade.php).
  import type { Snippet } from 'svelte'
  import type { HTMLSelectAttributes } from 'svelte/elements'
  import FieldError from './FieldError.svelte'
  import FieldLabel from './FieldLabel.svelte'

  type Props = Omit<HTMLSelectAttributes, 'value'> & {
    value?: unknown
    label?: string
    helper?: string
    error?: string
    children?: Snippet
  }

  let {
    value = $bindable(),
    label,
    helper,
    error,
    id,
    required = false,
    disabled = false,
    class: className = '',
    children,
    ...rest
  }: Props = $props()

  const generated = $props.id()
  const htmlId = $derived(id ?? generated)
</script>

<div class="chrome w-full">
  {#if label}
    <FieldLabel {label} for={htmlId} required={!!required} {helper} disabled={!!disabled} />
  {/if}
  <select {...rest} bind:value id={htmlId} {required} {disabled} class={['select w-full', className]}>
    {@render children?.()}
  </select>
  <FieldError {error} />
</div>
