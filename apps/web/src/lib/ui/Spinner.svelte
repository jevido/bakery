<script lang="ts">
  // Coolify's loading-on-button (resources/views/components/loading-on-button.blade.php).
  let { text, class: className = '' }: { text?: string; class?: string } = $props()
</script>

<span class={['inline-flex shrink-0 items-center gap-1.5', className]}>
  <svg class="size-3.5 shrink-0 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <circle class="opacity-25" cx="12" cy="12" r="9" stroke="currentColor" stroke-width="3" />
    <path class="opacity-75" d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
  </svg>
  {#if text}<span>{text}</span>{/if}
</span>
