<script lang="ts">
  // Paperclip's SidebarAccountMenu and ThemeToggle (ui/src/components/
  // SidebarAccountMenu.tsx, ThemeToggle.tsx; MIT, see NOTICE): the signed-in
  // Member at the foot of the sidebar, opening their name, email and what
  // they go by in the Current guild, then Profile, Settings, the theme and
  // Sign out. Left out: Documentation and the feedback link. The theme row
  // offers Dark, Light and System where Paperclip's toggles two.
  import { LogOut, Monitor, Moon, Settings, Sun, UserRound } from '@lucide/svelte'
  import type { Component } from 'svelte'
  import * as Avatar from '@bakery/ui/components/ui/avatar'
  import * as Popover from '@bakery/ui/components/ui/popover'
  import { guildPath, href } from './router.svelte'
  import { session } from './session.svelte'
  import { RAIL_HIDDEN_LABEL, sidebar } from './sidebar.svelte'
  import { theme, type Theme } from './theme.svelte'
  import { cn } from '@bakery/ui/utils'

  let open = $state(false)

  const name = $derived(session.member?.name?.trim() || 'Account')
  const initials = $derived.by(() => {
    const parts = name.split(/\s+/).filter(Boolean)
    if (parts.length >= 2) return `${parts[0][0]}${parts[parts.length - 1][0]}`.toUpperCase()
    return name.slice(0, 2).toUpperCase()
  })

  const ROW =
    'flex h-(--profile-popover-row-height) w-full items-center gap-(--profile-popover-row-gap) rounded-lg px-2.5 text-left text-(length:--text-compact) font-medium leading-(--profile-popover-label-line-height) text-foreground transition-colors'

  const themes: { value: Theme; label: string; icon: Component<{ class?: string }> }[] = [
    { value: 'dark', label: 'Dark', icon: Moon },
    { value: 'light', label: 'Light', icon: Sun },
    { value: 'system', label: 'System', icon: Monitor },
  ]
  const ThemeIcon = $derived(themes.find((t) => t.value === theme.current)?.icon ?? Moon)

  function close() {
    open = false
    sidebar.closeDrawer()
  }
</script>

{#snippet action(label: string, Icon: Component<{ class?: string }>, to: string)}
  <a href={to} class={cn(ROW, 'hover:bg-accent')} onclick={close}>
    <span class="flex size-5 shrink-0 items-center justify-center text-muted-foreground"><Icon class="size-4" /></span>
    <span class="min-w-0 flex-1 truncate">{label}</span>
  </a>
{/snippet}

<div class="chrome bg-border/50 px-3 py-2 dark:bg-muted">
  <div class={cn('flex items-center gap-0.5', !sidebar.rail && 'px-2')}>
    <Popover.Root bind:open>
      <Popover.Trigger
        class={cn(
          'flex min-w-0 items-center gap-2.5 rounded-lg text-left text-(length:--text-compact) font-medium text-foreground/80 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
          sidebar.rail ? 'w-full justify-center gap-0 px-0 py-2' : 'flex-1 px-2 py-1.5',
        )}
        aria-label="Open account menu"
        data-testid="account-menu"
      >
        <Avatar.Root size="sm"><Avatar.Fallback>{initials}</Avatar.Fallback></Avatar.Root>
        <span class={sidebar.rail ? RAIL_HIDDEN_LABEL : 'min-w-0 flex-1 truncate'}>{name}</span>
      </Popover.Trigger>
      <Popover.Content
        side="top"
        align="start"
        sideOffset={10}
        class="w-(--profile-popover-width) max-w-(--sz-calc-24) overflow-hidden rounded-xl border-border bg-popover p-0 shadow-(--shadow-profile-popover)"
      >
        <div class="relative flex h-(--profile-popover-header-height) shrink-0 items-center gap-2.5 px-3.5">
          <a
            href={href('/profile')}
            aria-label="View profile"
            onclick={close}
            class="absolute inset-0 transition-colors hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none focus-visible:ring-inset"
          ></a>
          <Avatar.Root class="pointer-events-none relative size-9">
            <Avatar.Fallback class="text-xs text-foreground">{initials}</Avatar.Fallback>
          </Avatar.Root>
          <div class="pointer-events-none relative min-w-0 flex-1">
            <p class="truncate text-sm leading-(--profile-popover-label-line-height) font-semibold text-foreground">{name}</p>
            <p class="truncate text-(length:--text-micro) leading-(--profile-popover-meta-line-height) text-muted-foreground">{session.member?.email}</p>
            <p class="truncate text-(length:--text-micro) leading-(--profile-popover-meta-line-height) text-muted-foreground" data-testid="account-role">
              {session.roleName}
            </p>
          </div>
        </div>

        <div class="flex flex-1 flex-col gap-0.5 border-t border-border px-2.5 pt-2 pb-2.5">
          {@render action('Profile', UserRound, href('/profile'))}
          {@render action('Settings', Settings, href(guildPath()))}
          <div class={ROW} role="group" aria-label="Theme">
            <span class="flex size-5 shrink-0 items-center justify-center text-muted-foreground"><ThemeIcon class="size-4" /></span>
            <span class="min-w-0 flex-1 truncate">Theme</span>
            <span class="flex shrink-0 items-center gap-0.5 rounded-md bg-muted p-0.5">
              {#each themes as t (t.value)}
                <button
                  type="button"
                  aria-label={t.label}
                  title={t.label}
                  aria-pressed={theme.current === t.value}
                  onclick={() => theme.set(t.value)}
                  class={cn(
                    'flex size-6 items-center justify-center rounded-sm text-muted-foreground transition-colors hover:text-foreground',
                    theme.current === t.value && 'bg-background text-foreground shadow-xs',
                  )}
                >
                  <t.icon class="size-3.5" />
                </button>
              {/each}
            </span>
          </div>
          <button type="button" class={cn(ROW, 'hover:bg-destructive/10')} onclick={() => session.logout()}>
            <span class="flex size-5 shrink-0 items-center justify-center text-muted-foreground"><LogOut class="size-4" /></span>
            <span class="min-w-0 flex-1 truncate">Sign out</span>
          </button>
        </div>
      </Popover.Content>
    </Popover.Root>
  </div>
</div>
