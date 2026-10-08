<script lang="ts">
  // Writing Markdown: a Textarea with Write and Preview tabs, in place of
  // Paperclip's MDXEditor (ui/src/components/MarkdownEditor.tsx; MIT, see
  // NOTICE). What is stored is the Markdown either way. Ctrl/⌘+Enter calls
  // `onsubmit`, as Paperclip's dialogs and comment box do.
  import * as Tabs from '@bakery/ui/components/ui/tabs'
  import Markdown from '@bakery/ui/Markdown.svelte'
  import Textarea from './ui/Textarea.svelte'

  let {
    value = $bindable(''),
    label,
    placeholder = 'Add description...',
    rows = 6,
    autofocus = false,
    onsubmit,
  }: {
    value?: string
    label?: string
    placeholder?: string
    rows?: number
    autofocus?: boolean
    onsubmit?: () => void
  } = $props()

  let tab = $state<'write' | 'preview'>('write')
</script>

<div class="space-y-1.5">
  <Tabs.Root bind:value={tab}>
    <div class="flex items-center justify-between gap-2">
      {#if label}<span class="text-sm font-medium">{label}</span>{:else}<span></span>{/if}
      <Tabs.List class="h-7">
        <Tabs.Trigger value="write" class="text-xs">Write</Tabs.Trigger>
        <Tabs.Trigger value="preview" class="text-xs">Preview</Tabs.Trigger>
      </Tabs.List>
    </div>
    <Tabs.Content value="write">
      <!-- svelte-ignore a11y_autofocus -->
      <Textarea
        bind:value
        {rows}
        {placeholder}
        {autofocus}
        aria-label={label ?? 'Description'}
        onkeydown={(e: KeyboardEvent) => {
          if (e.key === 'Enter' && (e.metaKey || e.ctrlKey) && onsubmit) {
            e.preventDefault()
            onsubmit()
          }
        }}
      />
    </Tabs.Content>
    <Tabs.Content value="preview">
      <div class="min-h-24 rounded-md border px-3 py-2">
        {#if value.trim()}
          <Markdown source={value} class="text-sm" />
        {:else}
          <p class="text-sm text-muted-foreground italic">Nothing to preview.</p>
        {/if}
      </div>
    </Tabs.Content>
  </Tabs.Root>
</div>
