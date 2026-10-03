<script lang="ts">
  // Coolify's forms/input (resources/views/components/forms/input.blade.php):
  // label row, `input`, the password eye, and the validation message.
  import type { HTMLInputAttributes } from 'svelte/elements'
  import Icon from '../Icon.svelte'
  import FieldError from './FieldError.svelte'
  import Helper from './Helper.svelte'
  import FieldLabel from './FieldLabel.svelte'

  type Props = Omit<HTMLInputAttributes, 'value'> & {
    value?: string | number | null
    label?: string
    helper?: string
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
      <input
        {...rest}
        bind:value
        id={htmlId}
        type={revealed ? 'text' : 'password'}
        {required}
        {disabled}
        {autocomplete}
        class={['input pr-10', revealed && !disabled && 'truncate', className]}
      />
      {#if allowToPeak}
        <button
          type="button"
          onclick={() => (revealed = !revealed)}
          class="password-toggle absolute inset-y-0 right-0 z-10 flex cursor-pointer items-center pr-2 text-neutral-500 hover:text-black dark:text-neutral-400 dark:hover:text-white"
          aria-label="Toggle password visibility"
        >
          <Icon name={revealed ? 'eye-off2' : 'eye'} class="size-[18px]" />
        </button>
      {/if}
    </div>
  {:else}
    <input {...rest} bind:value id={htmlId} {type} {required} {disabled} {autocomplete} class={['input', className]} />
  {/if}
  {#if !label && helper}<Helper {helper} />{/if}
  <FieldError {error} />
</div>
