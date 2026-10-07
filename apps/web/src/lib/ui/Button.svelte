<script lang="ts">
  // Paperclip's button (components/ui/button) under Coolify's variant names:
  // `default` is Paperclip's outline, `highlighted` its default and `error`
  // its destructive. `loading` stands in for wire:loading: it disables the
  // button and shows the spinner.
  import type { Snippet } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'
  import { buttonVariants, type ButtonVariant } from '$lib/components/ui/button'
  import type { ClassValue } from 'clsx'
  import { cn } from '$lib/utils'
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

  const variants: Record<NonNullable<Props['variant']>, ButtonVariant> = {
    default: 'outline',
    highlighted: 'default',
    error: 'destructive',
  }
</script>

<!-- Paperclip's button classes on a plain <button>: its Button component also
     takes anchor attributes, which the spread of button attributes trips. -->
<button
  {...rest}
  {type}
  data-slot="button"
  data-variant={variants[variant]}
  disabled={disabled || loading}
  class={cn('chrome', buttonVariants({ variant: variants[variant] }), className as ClassValue)}
>
  {#if loading}<Spinner />{/if}
  {@render children?.()}
</button>
