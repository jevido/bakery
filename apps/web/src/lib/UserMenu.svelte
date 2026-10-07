<script lang="ts">
  // Coolify's top-user-menu (resources/views/components/top-user-menu.blade.php)
  // with its theme-controls "menu" variant. Left out: auto-collapse (no page has
  // a settings rail yet), Documentation, Feedback and Sponsor.
  import Icon from './Icon.svelte'
  import { href } from './router.svelte'
  import { session } from './session.svelte'
  import { theme, type Theme } from './theme.svelte'

  let { sidebar = false, collapsed = false }: { sidebar?: boolean; collapsed?: boolean } = $props()

  let open = $state(false)
  let appearanceOpen = $state(false)
  let root: HTMLDivElement

  const name = $derived(session.member?.name || 'Account')
  const initial = $derived((session.member?.name || session.member?.email || 'A').slice(0, 1).toUpperCase())
  const role = $derived(session.member ? session.roleName : '')

  const themes: { value: Theme; label: string }[] = [
    { value: 'light', label: 'Light' },
    { value: 'system', label: 'System' },
    { value: 'dark', label: 'Dark' },
  ]

  function toggle() {
    appearanceOpen = false
    open = !open
  }
</script>

<svelte:window
  onkeydown={(e) => e.key === 'Escape' && (open = false)}
  onclick={(e) => open && !root.contains(e.target as Node) && (open = false)}
/>

<div class={['relative', sidebar && 'min-w-0']} bind:this={root}>
  <button
    type="button"
    onclick={toggle}
    title={name}
    aria-label="Account menu for {name}"
    aria-expanded={open}
    class={[
      'flex h-8 items-center gap-1.5 rounded-full border border-neutral-200 bg-neutral-100 px-2 shadow-sm transition-colors hover:bg-neutral-200 dark:border-white/[0.08] dark:bg-white/[0.06] dark:hover:bg-white/[0.1]',
      sidebar && 'max-w-36',
      sidebar && collapsed && 'w-8 justify-center px-0',
    ]}
  >
    <span
      class="flex size-5 shrink-0 items-center justify-center rounded-full bg-neutral-200 text-[11px] font-semibold text-neutral-700 dark:bg-white/[0.1] dark:text-fg"
    >
      {initial}
    </span>
    {#if sidebar}
      <span class={['min-w-0 truncate text-xs font-medium', collapsed && 'hidden']}>{name}</span>
    {/if}
    <svg
      class={['size-3.5 shrink-0 text-neutral-400 transition-transform dark:text-fg-faint', open && 'rotate-180', sidebar && collapsed && 'hidden']}
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <path d="M6 9l6 6 6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
  </button>

  {#if open}
    <div
      class={[
        'top-user-menu-panel listbox-panel z-[90]! max-h-none! w-52! min-w-0! overflow-visible!',
        sidebar ? 'top-auto! right-auto! bottom-full! left-0! mb-1! origin-bottom-left' : 'right-0! left-auto! origin-top-right',
      ]}
    >
      <div class="min-w-0 px-2 py-1.5">
        <div class="truncate text-[13px] font-semibold text-black dark:text-fg">{name}</div>
        <div class="truncate text-[11px] text-neutral-500 dark:text-fg-faint">{session.member?.email}</div>
        <div class="truncate text-[11px] text-neutral-500 dark:text-fg-faint">{role}</div>
      </div>
      <div class="my-1 h-px bg-neutral-200 dark:bg-white/[0.07]"></div>

      <a href={href('/profile')} class="listbox-option" onclick={() => (open = false)}>
        <span class="flex items-center gap-2">
          <Icon name="profile" class="size-4 opacity-80" />
          Profile
        </span>
      </a>
      <button
        type="button"
        class="listbox-option w-full"
        onclick={() => (appearanceOpen = !appearanceOpen)}
        aria-expanded={appearanceOpen}
      >
        <span class="flex items-center gap-2">
          <Icon name="settings" class="size-4 opacity-80" />
          Appearance
        </span>
        <svg
          class={['size-3.5 text-neutral-400 transition-transform dark:text-fg-faint', appearanceOpen && 'rotate-180']}
          viewBox="0 0 24 24"
          fill="none"
          aria-hidden="true"
        >
          <path d="M6 9l6 6 6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </button>
      {#if appearanceOpen}
        <div class="mx-1 grid gap-0.5 pb-1 pl-6">
          {#each themes as option (option.value)}
            <button
              type="button"
              onclick={() => theme.set(option.value)}
              aria-pressed={theme.current === option.value}
              class="flex h-8 w-full items-center justify-between rounded-md px-2 text-left text-xs text-neutral-600 transition-colors hover:bg-neutral-200 hover:text-neutral-950 dark:text-fg-dim dark:hover:bg-white/[0.06] dark:hover:text-fg"
            >
              <span>{option.label}</span>
              {#if theme.current === option.value}
                <svg class="size-3.5 text-coollabs" viewBox="0 0 12 12" fill="none" aria-hidden="true">
                  <path d="m2.5 6.25 2.1 2.1 4.9-5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
              {/if}
            </button>
          {/each}
        </div>
      {/if}

      <div class="my-1 h-px bg-neutral-200 dark:bg-white/[0.07]"></div>

      <button type="button" class="listbox-option w-full text-left text-error!" onclick={() => session.logout()}>
        <span class="flex items-center gap-2">
          <Icon name="logout" class="size-4 opacity-90" />
          Log out
        </span>
      </button>
    </div>
  {/if}
</div>
