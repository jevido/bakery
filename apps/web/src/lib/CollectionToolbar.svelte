<script lang="ts">
  // Paperclip's CollectionToolbar (components/CollectionToolbar.tsx, MIT, see
  // NOTICE): the geometry of a list page's controls. Context and search on the
  // left, controls and actions pushed right, an optional feedback row under
  // them. State and behaviour stay with the page that fills the snippets.
  import type { Snippet } from 'svelte'

  let {
    context,
    search,
    controls,
    actions,
    feedback,
    class: className = '',
    ariaLabel = 'Collection controls',
  }: {
    context?: Snippet
    search?: Snippet
    controls?: Snippet
    actions?: Snippet
    feedback?: Snippet
    class?: string
    ariaLabel?: string
  } = $props()
</script>

<div data-slot="collection-toolbar" class={['flex flex-col gap-2', className]} role="toolbar" aria-label={ariaLabel}>
  <div class="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
    {#if context}
      <div data-slot="collection-toolbar-context" class="min-w-0 shrink-0">{@render context()}</div>
    {/if}
    {#if search}
      <div data-slot="collection-toolbar-search" class="min-w-0 flex-1">{@render search()}</div>
    {/if}
    {#if controls || actions}
      <div class="flex min-w-0 flex-wrap items-center gap-1 sm:ml-auto sm:flex-nowrap">
        {#if controls}
          <div data-slot="collection-toolbar-controls" class="flex min-w-0 flex-wrap items-center gap-1">{@render controls()}</div>
        {/if}
        {#if actions}
          <div data-slot="collection-toolbar-actions" class="flex shrink-0 items-center gap-1">{@render actions()}</div>
        {/if}
      </div>
    {/if}
  </div>
  {#if feedback}
    <div data-slot="collection-toolbar-feedback" class="min-w-0">{@render feedback()}</div>
  {/if}
</div>
