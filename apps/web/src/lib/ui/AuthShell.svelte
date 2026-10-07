<script lang="ts">
  // Paperclip's sign-in page layout (ui/src/pages/Auth.tsx; MIT, see NOTICE):
  // the theme toggle in the corner, the form in the left half under The
  // Bakery's lockup with a heading, a line under it and an optional footer,
  // and the ASCII field in the right half, hidden on a phone.
  import type { Snippet } from 'svelte'
  import AsciiArtAnimation from '../AsciiArtAnimation.svelte'
  import BakeryLockup from '../BakeryLockup.svelte'
  import ThemeToggle from '../ThemeToggle.svelte'

  let {
    title = 'The Bakery',
    description,
    children,
    footer,
  }: { title?: string; description?: string; children?: Snippet; footer?: Snippet } = $props()
</script>

<main class="chrome fixed inset-0 flex bg-background text-foreground">
  <div class="absolute top-4 right-4 z-10">
    <ThemeToggle />
  </div>
  <div class="flex w-full flex-col overflow-y-auto md:w-1/2">
    <div class="mx-auto my-auto w-full max-w-md px-8 py-12">
      <div class="mb-8">
        <BakeryLockup class="h-5 w-auto" />
      </div>
      <h1 class="text-xl font-semibold">{title}</h1>
      {#if description}<p class="mt-1 text-sm text-muted-foreground">{description}</p>{/if}
      <div class="mt-6">
        {@render children?.()}
      </div>
      {#if footer}
        <div class="mt-5 text-sm text-muted-foreground">{@render footer()}</div>
      {/if}
    </div>
  </div>
  <div class="hidden w-1/2 overflow-hidden md:block">
    <AsciiArtAnimation />
  </div>
</main>
