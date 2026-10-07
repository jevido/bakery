<script lang="ts">
  // The header of Paperclip's detail pages (ui/src/pages/ProjectDetail.tsx;
  // MIT, see NOTICE): a leading tile, the name in bold with a line or chips
  // under it, and the page's actions pushed right. Below `sm` the actions
  // wrap under the title. `trailing` sits right after the name, as a
  // switcher does.
  import type { Snippet } from 'svelte'

  let {
    leading,
    title,
    description,
    meta,
    actions,
    trailing,
    titleTestid,
    class: className = '',
  }: {
    leading?: Snippet
    title: string
    description?: string
    meta?: Snippet
    actions?: Snippet
    trailing?: Snippet
    titleTestid?: string
    class?: string
  } = $props()
</script>

<header data-slot="page-header" class={['flex flex-col gap-3 sm:flex-row sm:items-start', className]}>
  <div class="flex min-w-0 items-start gap-3">
    {#if leading}<div class="flex h-7 shrink-0 items-center">{@render leading()}</div>{/if}
    <div class="min-w-0 space-y-1">
      {#if trailing}
        <div class="flex min-w-0 items-center gap-1">
          <h1 class="truncate text-xl font-bold" {title} data-testid={titleTestid}>{title}</h1>
          {@render trailing()}
        </div>
      {:else}
        <h1 class="truncate text-xl font-bold" {title} data-testid={titleTestid}>{title}</h1>
      {/if}
      {#if description}<p class="text-sm text-muted-foreground">{description}</p>{/if}
      {#if meta}<div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">{@render meta()}</div>{/if}
    </div>
  </div>
  {#if actions}<div class="flex shrink-0 flex-wrap items-center gap-2 sm:ml-auto">{@render actions()}</div>{/if}
</header>
