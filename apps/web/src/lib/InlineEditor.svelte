<script lang="ts">
  // Paperclip's InlineEditor (ui/src/components/InlineEditor.tsx; MIT, see
  // NOTICE): a title or description shown as text, edited in place on a
  // click. A single line saves on Enter or when it loses focus; a multiline
  // one shows its Markdown rendered and saves when it loses focus. Escape
  // throws the change away. Unchanged text is not saved. Without `editable`
  // it is plain text. Left out: autosave while typing and image uploads.
  import Markdown from './Markdown.svelte'

  let {
    value,
    onsave,
    as = 'p',
    multiline = false,
    editable = true,
    placeholder = 'Click to edit...',
    class: className = '',
    label,
  }: {
    value: string
    onsave: (value: string) => unknown
    as?: 'h2' | 'p'
    multiline?: boolean
    editable?: boolean
    placeholder?: string
    class?: string
    /** Names the field for assistive technology, e.g. "Title". */
    label: string
  } = $props()

  let editing = $state(false)
  let draft = $state('')

  function start() {
    if (!editable) return
    draft = value
    editing = true
  }

  function autosize(el: HTMLTextAreaElement) {
    el.style.height = 'auto'
    el.style.height = `${el.scrollHeight}px`
  }

  function commit() {
    if (!editing) return
    editing = false
    const next = multiline ? draft : draft.trim()
    if (next !== value && (multiline || next)) onsave(next)
  }
</script>

{#if editing}
  <textarea
    bind:value={draft}
    rows={1}
    aria-label={label}
    class={['w-full resize-none overflow-hidden rounded bg-transparent px-1 py-0.5 outline-none focus-visible:ring-1 focus-visible:ring-ring', className]}
    {@attach (el: HTMLTextAreaElement) => {
      autosize(el)
      el.focus()
      el.setSelectionRange(el.value.length, el.value.length)
    }}
    oninput={(e) => autosize(e.currentTarget)}
    onblur={commit}
    onkeydown={(e) => {
      if (e.key === 'Enter' && !multiline) {
        e.preventDefault()
        commit()
      }
      if (e.key === 'Escape') {
        e.preventDefault()
        editing = false
      }
    }}
  ></textarea>
{:else if !editable}
  {#if multiline && value}
    <Markdown source={value} class={['px-1 py-0.5', className].join(' ')} />
  {:else}
    <svelte:element this={as} class={['px-1 py-0.5', !value && 'text-muted-foreground italic', className]}>{value}</svelte:element>
  {/if}
{:else}
  <div
    role="button"
    tabindex="0"
    aria-label="Edit {label.toLowerCase()}"
    data-inline-editor={label}
    class={['cursor-pointer overflow-hidden rounded px-1 py-0.5 transition-colors hover:bg-accent/50', !value && 'text-muted-foreground italic', className]}
    onclick={start}
    onkeydown={(e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault()
        start()
      }
    }}
  >
    {#if multiline && value}
      <Markdown source={value} />
    {:else if value}
      <svelte:element this={as}>{value}</svelte:element>
    {:else}
      {placeholder}
    {/if}
  </div>
{/if}
