<script lang="ts">
  // Coolify's forms/select (resources/views/components/forms/select.blade.php).
  // The native <select> stays (callers pass <option>s and bind the value),
  // drawn with the classes of Paperclip's select trigger and its chevron.
  import type { Snippet } from 'svelte'
  import type { HTMLSelectAttributes } from 'svelte/elements'
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
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
  <div class="relative">
    <!-- bg-none drops the chevron @tailwindcss/forms paints on every select. -->
    <select
      aria-invalid={error ? true : undefined}
      {...rest}
      bind:value
      id={htmlId}
      {required}
      {disabled}
      data-slot="select-trigger"
      class={[
        'flex h-9 w-full appearance-none items-center rounded-md border border-input bg-transparent bg-none py-2 pr-9 pl-3 text-base whitespace-nowrap text-foreground shadow-xs transition-(--tp-color-box-shadow) outline-none focus-visible:border-ring focus-visible:ring-(length:--rad-3) focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 md:text-sm dark:bg-input/30 dark:hover:bg-input/50 dark:aria-invalid:ring-destructive/40 [&>option]:bg-popover [&>option]:text-popover-foreground',
        className,
      ]}
    >
      {@render children?.()}
    </select>
    <ChevronDown class="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-muted-foreground opacity-50" />
  </div>
  <FieldError {error} />
</div>
