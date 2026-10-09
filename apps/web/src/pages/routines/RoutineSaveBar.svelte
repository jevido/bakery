<script lang="ts">
  // Paperclip's RoutineSaveBar (ui/src/components/RoutineSaveBar.tsx; MIT,
  // see NOTICE): sticky under the form once something changed, naming how
  // many fields and which, with Discard (asked first) and Save changes.
  // Ctrl/⌘+S saves and Escape asks to discard. Left out: its conflict
  // surface, since a Routine has no revisions to conflict on.
  import * as AlertDialog from '@bakery/ui/components/ui/alert-dialog'
  import { Button } from '@bakery/ui/components/ui/button'
  import * as Popover from '@bakery/ui/components/ui/popover'

  let {
    dirty,
    saving,
    disabled = false,
    onsave,
    ondiscard,
  }: { dirty: string[]; saving: boolean; disabled?: boolean; onsave: () => void; ondiscard: () => void } = $props()

  let confirming = $state(false)

  function onkeydown(e: KeyboardEvent) {
    if (dirty.length === 0) return
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
      e.preventDefault()
      if (!saving && !disabled) onsave()
    } else if (e.key === 'Escape' && !confirming) {
      e.preventDefault()
      confirming = true
    }
  }
</script>

<svelte:window {onkeydown} />

{#if dirty.length > 0}
  <div class="sticky bottom-0 z-10 -mx-4 mt-6 flex h-14 items-center justify-between border-t border-border bg-background/95 px-4 backdrop-blur md:-mx-8 md:px-8" role="region" aria-label="Unsaved changes">
    <Popover.Root>
      <Popover.Trigger class="flex items-center gap-2 text-sm text-foreground">
        <span class="size-1.5 rounded-full bg-amber-500"></span>
        <span class="font-medium">{dirty.length} unsaved {dirty.length === 1 ? 'change' : 'changes'}</span>
      </Popover.Trigger>
      <Popover.Content align="start" class="w-64">
        <p class="mb-2 text-xs font-medium text-muted-foreground">Pending changes</p>
        <ul class="space-y-1 text-sm">
          {#each dirty as field (field)}
            <li class="flex items-center gap-2"><span class="size-1 rounded-full bg-amber-500"></span><span class="capitalize">{field}</span></li>
          {/each}
        </ul>
      </Popover.Content>
    </Popover.Root>
    <div class="flex items-center gap-2">
      <Button variant="ghost" size="sm" disabled={saving} onclick={() => (confirming = true)}>Discard</Button>
      <Button size="sm" disabled={saving || disabled} onclick={onsave}>
        {saving ? 'Saving…' : 'Save changes'}
        <kbd class="ml-2 hidden rounded bg-foreground/10 px-1 text-(length:--text-nano) font-medium sm:inline">⌘S</kbd>
      </Button>
    </div>
  </div>
{/if}

<AlertDialog.Root bind:open={confirming}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>Discard changes?</AlertDialog.Title>
      <AlertDialog.Description>Your unsaved edits to this routine will be lost.</AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>Keep editing</AlertDialog.Cancel>
      <AlertDialog.Action
        onclick={() => {
          confirming = false
          ondiscard()
        }}>Discard</AlertDialog.Action
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
