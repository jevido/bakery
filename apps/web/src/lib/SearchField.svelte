<script lang="ts">
  // The search box of Paperclip's list toolbars (ui/src/pages/Projects.tsx;
  // MIT, see NOTICE): a magnifier, the input, and a clear button once
  // something is typed.
  import { Input } from '@bakery/ui/components/ui/input'
  import Icon from './Icon.svelte'

  let {
    value = $bindable(''),
    ref = $bindable(null),
    label,
    autofocus = false,
    oninput,
  }: { value?: string; ref?: HTMLInputElement | null; label: string; autofocus?: boolean; oninput?: () => void } = $props()
</script>

<div class="relative w-full sm:max-w-sm">
  <Icon name="search" class="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
  <!-- svelte-ignore a11y_autofocus -->
  <Input
    bind:value
    bind:ref
    {autofocus}
    {oninput}
    type="search"
    placeholder={label}
    aria-label={label}
    class="h-8 pr-8 pl-8 text-sm [&::-webkit-search-cancel-button]:hidden"
  />
  {#if value}
    <button
      type="button"
      onclick={() => {
        value = ''
        oninput?.()
      }}
      class="absolute top-1/2 right-1.5 flex size-5 -translate-y-1/2 items-center justify-center rounded-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
      aria-label="Clear search"
    >
      <Icon name="x" class="size-3" />
    </button>
  {/if}
</div>
