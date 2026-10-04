<script lang="ts">
  // Coolify's forms/copy-button (resources/views/components/forms/copy-button.blade.php):
  // a read-only input with a label and a copy button inside it on the right.
  // A secret is masked until its eye, left of the copy button, reveals it.
  import Icon from '../Icon.svelte'

  let { text, label, testid, secret = false }: { text: string; label?: string; testid?: string; secret?: boolean } = $props()

  const id = $props.id()
  let copied = $state(false)
  let revealed = $state(false)

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
      type={secret && !revealed ? 'password' : 'text'}
      value={text}
      class="input input-with-copy-button {secret ? 'pr-16!' : ''} bg-white dark:bg-coolgray-100 dark:read-only:bg-coolgray-100 dark:read-only:text-white"
      readonly
      data-testid={testid}
      onfocus={(e) => e.currentTarget.select()}
    />
    {#if secret}
      <button
        type="button"
        onclick={() => (revealed = !revealed)}
        class="password-toggle absolute inset-y-0 right-7 z-10 flex cursor-pointer items-center pr-2 text-neutral-500 hover:text-black dark:text-neutral-400 dark:hover:text-white"
        aria-label="Toggle password visibility"
      >
        <Icon name={revealed ? 'eye-off2' : 'eye'} class="size-[18px]" />
      </button>
    {/if}
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
