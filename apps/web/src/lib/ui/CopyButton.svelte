<script lang="ts">
  // Coolify's forms/copy-button (resources/views/components/forms/copy-button.blade.php):
  // a read-only input with a label and a copy button inside it on the right.
  let { text, label, testid }: { text: string; label?: string; testid?: string } = $props()

  const id = $props.id()
  let copied = $state(false)

  async function copy() {
    await navigator.clipboard?.writeText(text)
    copied = true
    setTimeout(() => (copied = false), 1000)
  }
</script>

<div class="chrome w-full">
  {#if label}
    <label for={id} class="mb-1 flex items-center gap-1 text-sm font-medium text-black dark:text-white">{label}</label>
  {/if}
  <div class="relative">
    <input
      {id}
      type="text"
      value={text}
      class="input input-with-copy-button bg-white dark:bg-coolgray-100 dark:read-only:bg-coolgray-100 dark:read-only:text-white"
      readonly
      data-testid={testid}
      onfocus={(e) => e.currentTarget.select()}
    />
    <button
      type="button"
      onclick={copy}
      class="copy-button absolute inset-y-0 right-0 z-10 flex cursor-pointer items-center pr-2 text-neutral-500 transition-colors hover:text-black focus-visible:ring-2 focus-visible:ring-coollabs focus-visible:ring-offset-2 dark:text-neutral-400 dark:hover:text-white dark:focus-visible:ring-warning dark:focus-visible:ring-offset-base"
      title="Copy to clipboard"
      aria-label="Copy to clipboard"
    >
      {#if copied}
        <svg class="size-[18px] text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
        </svg>
      {:else}
        <svg class="size-[18px]" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
          />
        </svg>
      {/if}
    </button>
  </div>
</div>
