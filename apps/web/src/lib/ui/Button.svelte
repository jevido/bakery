<script lang="ts">
  // Coolify's forms/button (resources/views/components/forms/button.blade.php):
  // `button`, plus `button-highlighted` or `button-error`. `loading` stands in
  // for wire:loading: it disables the button and shows the spinner.
  import type { Snippet } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'
  import Spinner from './Spinner.svelte'

  type Props = HTMLButtonAttributes & {
    variant?: 'default' | 'highlighted' | 'error'
    loading?: boolean
    children?: Snippet
  }

  let {
    variant = 'default',
    loading = false,
    disabled = false,
    type = 'button',
    class: className = '',
    children,
    ...rest
  }: Props = $props()
</script>

<button
  {...rest}
  {type}
  disabled={disabled || loading}
  class={[
    'chrome button',
    variant === 'highlighted' && 'button-highlighted',
    variant === 'error' && 'button-error',
    className,
  ]}
>
  {#if loading}<Spinner />{/if}
  {@render children?.()}
</button>
