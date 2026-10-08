<script lang="ts">
  // Coolify's forms/input (resources/views/components/forms/input.blade.php)
  // on Paperclip's input: label row, the field, the password eye, and the
  // validation message.
  import type { Snippet } from 'svelte'
  import type { HTMLInputAttributes } from 'svelte/elements'
  import Eye from '@lucide/svelte/icons/eye'
  import EyeOff from '@lucide/svelte/icons/eye-off'
  import { Input as UiInput } from '@bakery/ui/components/ui/input'
  import FieldError from './FieldError.svelte'
  import Helper from './Helper.svelte'
  import FieldLabel from './FieldLabel.svelte'

  // `files` is left out: a file input is not drawn here, and Paperclip's
  // input types it apart from every other type.
  type Props = Omit<HTMLInputAttributes, 'value' | 'files'> & {
    value?: string | number | null
    label?: string
    helper?: string | Snippet
    error?: string
    /** Show the eye that reveals a password. */
    allowToPeak?: boolean
  }

  let {
    value = $bindable(''),
    label,
    helper,
    error,
    type = 'text',
    id,
    required = false,
    disabled = false,
    allowToPeak = true,
    autocomplete = 'off',
    class: className = '',
    ...rest
  }: Props = $props()

  const generated = $props.id()
  const htmlId = $derived(id ?? generated)
  let revealed = $state(false)
</script>

<div class="chrome w-full">
  {#if label}
    <FieldLabel {label} for={htmlId} required={!!required} {helper} disabled={!!disabled} />
  {/if}
  {#if type === 'password'}
    <div class="relative">
      <UiInput
        aria-invalid={error ? true : undefined}
        {...rest}
        bind:value
        id={htmlId}
        type={revealed ? 'text' : 'password'}
        {required}
        {disabled}
        {autocomplete}
        class={['pr-10', revealed && !disabled && 'truncate', className]}
      />
      {#if allowToPeak}
        <button
          type="button"
          onclick={() => (revealed = !revealed)}
          class="password-toggle absolute inset-y-0 right-0 z-10 flex cursor-pointer items-center px-3 text-muted-foreground transition-colors hover:text-foreground"
          aria-label="Toggle password visibility"
        >
          {#if revealed}<EyeOff class="size-4" />{:else}<Eye class="size-4" />{/if}
        </button>
      {/if}
    </div>
  {:else}
    <UiInput
      aria-invalid={error ? true : undefined}
      {...rest}
      bind:value
      id={htmlId}
      type={type as 'text'}
      {required}
      {disabled}
      {autocomplete}
      class={className}
    />
  {/if}
  {#if !label && helper}<Helper {helper} />{/if}
  <FieldError {error} />
</div>
