<script lang="ts">
  // Paperclip's Sidebar (ui/src/components/Sidebar.tsx; MIT, see NOTICE) as
  // apps/web/src/lib/Sidebar.svelte lays it out: a 60px header, here the
  // Bakery lockup, the nav, and an account strip at the bottom with the
  // Desktop app's version and the theme toggle.
  import BakeryLockup from '@bakery/ui/BakeryLockup.svelte'
  import ThemeToggle from '@bakery/ui/ThemeToggle.svelte'
  import { Plug } from '@lucide/svelte'
  import { version } from './desktop'

  let current = $state('')
  $effect(() => {
    version().then((v) => (current = v), () => (current = ''))
  })
</script>

<aside class="chrome primary-sidebar-surface flex h-full min-h-0 w-60 shrink-0 flex-col border-r border-border" data-testid="sidebar">
  <div class="flex h-(--sz-60px) shrink-0 items-center px-5">
    <BakeryLockup class="h-5 w-auto" />
  </div>
  <nav class="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-3 py-2" aria-label="Main">
    <span
      class="flex items-center gap-2.5 rounded-md bg-accent px-3 py-2 text-[13px] font-medium text-foreground"
      aria-current="page"
    >
      <Plug class="size-4 shrink-0" />
      Connect a Bakery
    </span>
  </nav>
  <div class="flex h-12 shrink-0 items-center justify-between border-t border-border px-3">
    <span class="text-xs text-muted-foreground" data-testid="version">{current ? `Desktop app ${current}` : ''}</span>
    <ThemeToggle />
  </div>
</aside>
