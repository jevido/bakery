<script lang="ts">
  // The label row Coolify's form fields share (forms/input.blade.php): a
  // fixed-height row so side-by-side fields align, the required mark, and the
  // helper outside the <label>.
  import type { Snippet } from 'svelte'
  import Helper from './Helper.svelte'

  let {
    label,
    for: htmlFor,
    required = false,
    helper,
    disabled = false,
  }: { label: string; for: string; required?: boolean; helper?: string | Snippet; disabled?: boolean } = $props()
</script>

<div class="mb-1.5 flex h-4 w-full items-center gap-1.5">
  <label for={htmlFor} class={['mb-0! flex items-center gap-1 text-sm font-medium leading-4', disabled && 'text-muted-foreground']}>
    {label}
    {#if required}<span class="inline-block font-semibold text-foreground">*</span>{/if}
  </label>
  {#if helper}<Helper {helper} />{/if}
</div>
