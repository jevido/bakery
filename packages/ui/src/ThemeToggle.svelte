<script lang="ts">
  // Paperclip's ThemeToggle (ui/src/components/ThemeToggle.tsx; MIT, see
  // NOTICE), its `icon` variant: the floating toggle on the signed-out pages.
  // It flips between dark and light from what is shown now, so a stored
  // `system` becomes the opposite of what the system gives.
  import Moon from '@lucide/svelte/icons/moon'
  import Sun from '@lucide/svelte/icons/sun'
  import { Button } from './components/ui/button/index.js'
  import { theme } from './theme.svelte'

  let { class: className }: { class?: string } = $props()

  const dark = $derived(
    theme.current === 'dark' || (theme.current === 'system' && matchMedia('(prefers-color-scheme: dark)').matches),
  )
  const label = $derived(dark ? 'Switch to light mode' : 'Switch to dark mode')
</script>

<Button
  type="button"
  variant="ghost"
  size="icon-sm"
  class={['text-muted-foreground', className]}
  onclick={() => theme.set(dark ? 'light' : 'dark')}
  aria-label={label}
  title={label}
>
  {#if dark}<Sun />{:else}<Moon />{/if}
</Button>
