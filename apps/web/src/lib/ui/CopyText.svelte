<script lang="ts">
  // The read-only field with a copy button that Coolify's confirmation modal
  // shows the name to type in, on Paperclip's input.
  import Check from '@lucide/svelte/icons/check'
  import Copy from '@lucide/svelte/icons/copy'
  import { Input as UiInput } from '$lib/components/ui/input'

  let { text }: { text: string } = $props()
  let copied = $state(false)

  async function copy() {
    await navigator.clipboard.writeText(text)
    copied = true
    setTimeout(() => (copied = false), 1000)
  }
</script>

<div class="relative mb-2">
  <UiInput type="text" value={text} readonly class="pr-10" aria-label="Text to type" />
  {#if window.isSecureContext}
    <button
      type="button"
      onclick={copy}
      title="Copy to clipboard"
      class="absolute inset-y-0 right-0 flex cursor-pointer items-center rounded-r-md px-2.5 text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
    >
      {#if copied}<Check class="size-4 text-success" />{:else}<Copy class="size-4" />{/if}
    </button>
  {/if}
</div>
