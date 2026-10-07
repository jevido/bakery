<script lang="ts">
  // Paperclip's loading glyph (lucide's Loader2, spinning) with optional text.
  import LoaderCircle from '@lucide/svelte/icons/loader-circle'

  let { text, class: className = '' }: { text?: string; class?: string } = $props()
</script>

<span class={['inline-flex shrink-0 items-center gap-1.5', className]}>
  <LoaderCircle class="size-3.5 shrink-0 animate-spin" aria-hidden="true" />
  {#if text}<span>{text}</span>{/if}
</span>
