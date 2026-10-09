<script lang="ts">
  // Paperclip's DocumentDiffModal (ui/src/components/DocumentDiffModal.tsx;
  // MIT, see NOTICE): two bodies of an Issue document line by line, removed
  // lines red, added lines green, unchanged ones plain, each with its old and
  // new line number. Paperclip picks both sides from Select menus of the
  // Revisions; here the caller names them, because the Issue page compares
  // one Revision with the current one, or the newest with an unsaved draft,
  // and the Routine page's History one Routine revision's description with
  // the current one, with its changed fields above the lines.
  import { Badge } from '@bakery/ui/components/ui/badge'
  import * as Dialog from '@bakery/ui/components/ui/dialog'
  import type { Snippet } from 'svelte'
  import { buildLineDiff, type DiffRowKind } from './lineDiff'

  /** One side of the comparison: its label (like "rev 1") and its text. */
  type Side = { label: string; body: string }

  let {
    open = $bindable(false),
    heading,
    old: before,
    new: after,
    children,
    footer,
  }: {
    open?: boolean
    /** The dialog's title. */
    heading: Snippet
    old: Side
    new: Side
    /** Shown above the lines, like the fields that changed. */
    children?: Snippet
    footer?: Snippet
  } = $props()

  const rows = $derived(buildLineDiff(before.body, after.body))

  const lineClasses: Record<DiffRowKind, string> = {
    context: 'bg-transparent',
    removed: 'bg-red-500/10 text-red-900 dark:text-red-100',
    added: 'bg-green-500/10 text-green-900 dark:text-green-100',
  }
  const markers: Record<DiffRowKind, string> = { context: ' ', removed: '-', added: '+' }
</script>

<Dialog.Root bind:open>
  <Dialog.Content class="flex max-h-[85vh] w-full flex-col overflow-hidden sm:max-w-[90vw]" data-testid="document-diff">
    <div class="flex flex-wrap items-center justify-between gap-4 pr-8">
      <Dialog.Header class="shrink-0">
        <Dialog.Title>{@render heading()}</Dialog.Title>
      </Dialog.Header>
      <div class="flex shrink-0 items-center gap-4 text-xs">
        <span class="flex items-center gap-2">
          <Badge variant="outline" class="border-red-500/30 bg-red-500/10 text-[10px] tracking-wider text-red-400 uppercase">Old</Badge>
          {before.label}
        </span>
        <span class="flex items-center gap-2">
          <Badge variant="outline" class="border-green-500/30 bg-green-500/10 text-[10px] tracking-wider text-green-400 uppercase">New</Badge>
          {after.label}
        </span>
      </div>
    </div>

    {@render children?.()}
    <div class="flex-1 overflow-auto rounded-md border border-border text-xs">
      <div class="font-mono text-xs leading-6">
        <div class="grid grid-cols-[3rem_3rem_2rem_1fr] border-b border-border/60 bg-muted/30 px-3 py-2 text-[11px] tracking-wide text-muted-foreground uppercase">
          <span>Old</span>
          <span>New</span>
          <span></span>
          <span>Content</span>
        </div>
        {#each rows as row, n (n)}
          <div class={['grid grid-cols-[3rem_3rem_2rem_1fr] border-b border-border/30 px-3', lineClasses[row.kind]]} data-diff={row.kind}>
            <span class="border-r border-border/30 pr-3 text-right text-muted-foreground select-none">{row.oldLineNumber ?? ''}</span>
            <span class="border-r border-border/30 px-3 text-right text-muted-foreground select-none">{row.newLineNumber ?? ''}</span>
            <span class="px-3 text-center text-muted-foreground select-none">{markers[row.kind]}</span>
            <pre class="overflow-x-auto px-3 py-0 break-words whitespace-pre-wrap text-inherit">{row.text.length > 0 ? row.text : ' '}</pre>
          </div>
        {/each}
      </div>
    </div>
    {#if footer}<Dialog.Footer>{@render footer()}</Dialog.Footer>{/if}
  </Dialog.Content>
</Dialog.Root>
