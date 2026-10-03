<script lang="ts">
  // The read-only field with a copy button that Coolify's confirmation modal
  // shows the name to type in.
  let { text }: { text: string } = $props()
  let copied = $state(false)

  async function copy() {
    await navigator.clipboard.writeText(text)
    copied = true
    setTimeout(() => (copied = false), 1000)
  }
</script>

<div class="relative mb-2">
  <input type="text" value={text} readonly class="input pr-10" aria-label="Text to type" />
  {#if window.isSecureContext}
    <button
      type="button"
      onclick={copy}
      title="Copy to clipboard"
      class="absolute top-1/2 right-2 -translate-y-1/2 p-1.5 text-gray-400 transition-colors hover:text-gray-300"
    >
      {#if copied}
        <svg class="h-5 w-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"
          ><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" /></svg
        >
      {:else}
        <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"
          ><path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
          /></svg
        >
      {/if}
    </button>
  {/if}
</div>
