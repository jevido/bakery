<script lang="ts">
  // Coolify's forms/copy-button (resources/views/components/forms/copy-button.blade.php)
  // on Paperclip's input: a read-only field with a label and a copy button
  // inside it on the right. A secret is masked until its eye, left of the
  // copy button, reveals it.
  import Check from '@lucide/svelte/icons/check'
  import Copy from '@lucide/svelte/icons/copy'
  import Eye from '@lucide/svelte/icons/eye'
  import EyeOff from '@lucide/svelte/icons/eye-off'
  import { Input as UiInput } from '$lib/components/ui/input'
  import { Label } from '$lib/components/ui/label'

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
    <Label for={id} class="mb-1.5">{label}</Label>
  {/if}
  <div class="relative">
    <UiInput
      {id}
      type={secret && !revealed ? 'password' : 'text'}
      value={text}
      class={['input-with-copy-button', secret ? 'pr-16' : 'pr-9']}
      readonly
      data-testid={testid}
      onfocus={(e) => e.currentTarget.select()}
    />
    {#if secret}
      <button
        type="button"
        onclick={() => (revealed = !revealed)}
        class="password-toggle absolute inset-y-0 right-9 z-10 flex cursor-pointer items-center px-1 text-muted-foreground transition-colors hover:text-foreground"
        aria-label="Toggle password visibility"
      >
        {#if revealed}<EyeOff class="size-4" />{:else}<Eye class="size-4" />{/if}
      </button>
    {/if}
    <button
      type="button"
      onclick={copy}
      class="copy-button absolute inset-y-0 right-0 z-10 flex cursor-pointer items-center rounded-r-md px-2.5 text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-(length:--rad-3) focus-visible:ring-ring/50"
      title="Copy to clipboard"
      aria-label="Copy to clipboard"
    >
      {#if copied}<Check class="size-4 text-success" />{:else}<Copy class="size-4" />{/if}
    </button>
  </div>
</div>
