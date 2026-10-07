<script lang="ts">
  // Coolify's forms/textarea (resources/views/components/forms/textarea.blade.php)
  // on Paperclip's textarea: `font-mono` for code, and Tab inserting two
  // spaces when `allowTab` is set. Sized by `rows` rather than its content.
  import type { HTMLTextareaAttributes } from 'svelte/elements'
  import { Textarea as UiTextarea } from '$lib/components/ui/textarea'
  import FieldError from './FieldError.svelte'
  import FieldLabel from './FieldLabel.svelte'

  type Props = Omit<HTMLTextareaAttributes, 'value'> & {
    value?: string | null
    label?: string
    helper?: string
    error?: string
    monospace?: boolean
    allowTab?: boolean
  }

  let {
    value = $bindable(''),
    label,
    helper,
    error,
    monospace = false,
    allowTab = false,
    id,
    required = false,
    disabled = false,
    spellcheck = !monospace,
    rows = 4,
    class: className = '',
    ...rest
  }: Props = $props()

  const generated = $props.id()
  const htmlId = $derived(id ?? generated)

  function tab(e: KeyboardEvent) {
    if (!allowTab || e.key !== 'Tab') return
    e.preventDefault()
    const el = e.currentTarget as HTMLTextAreaElement
    el.setRangeText('  ', el.selectionStart, el.selectionStart, 'end')
    value = el.value
  }
</script>

<div class="chrome form-control flex-1">
  {#if label}
    <FieldLabel {label} for={htmlId} required={!!required} {helper} disabled={!!disabled} />
  {/if}
  <UiTextarea
    aria-invalid={error ? true : undefined}
    {...rest}
    bind:value
    id={htmlId}
    {rows}
    {required}
    {disabled}
    {spellcheck}
    onkeydown={tab}
    class={['field-sizing-fixed', monospace && 'font-mono', className]}
  />
  <FieldError {error} />
</div>
