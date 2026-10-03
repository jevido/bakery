<script lang="ts">
  // Coolify's modal-input (resources/views/components/modal-input.blade.php),
  // which is what its pages open for every form in a dialog: a trigger button
  // (or the `trigger` snippet, or none when `open` is driven from outside),
  // and the layer-card dialog with a title, optional subtitle, header actions
  // and footer. Escape, the close button and a click beside the card close it.
  import type { Snippet } from 'svelte'
  import { fade, scale } from 'svelte/transition'
  import Icon from '../Icon.svelte'
  import Button from './Button.svelte'
  import { focusTrap } from './focusTrap'
  import { portal } from './portal'

  let {
    open = $bindable(false),
    title = 'Are you sure?',
    subtitle,
    buttonTitle = 'Open Modal',
    variant = 'default',
    disabled = false,
    closeOutside = true,
    isLarge = false,
    fullWidth = false,
    onclose,
    trigger,
    headerActions,
    footer,
    children,
  }: {
    open?: boolean
    title?: string
    subtitle?: string
    buttonTitle?: string
    variant?: 'default' | 'highlighted' | 'error' | 'none'
    disabled?: boolean
    closeOutside?: boolean
    isLarge?: boolean
    fullWidth?: boolean
    onclose?: () => void
    trigger?: Snippet<[() => void]>
    headerActions?: Snippet
    footer?: Snippet
    children?: Snippet
  } = $props()

  function show() {
    open = true
  }

  function close() {
    open = false
    onclose?.()
  }
</script>

<svelte:window onkeydown={(e) => open && e.key === 'Escape' && close()} />

<div class={['chrome relative', fullWidth ? 'h-full w-full' : 'contents']}>
  {#if trigger}
    {@render trigger(show)}
  {:else if variant !== 'none'}
    <Button variant={variant} {disabled} onclick={show} class={fullWidth ? 'w-full' : ''}>{buttonTitle}</Button>
  {/if}
  <div class="chrome" {@attach portal}>
    {#if open}
      <div class="fixed inset-0 z-99 overflow-hidden">
        <div class="absolute inset-0 h-full w-full bg-black/50 backdrop-blur-[2px]" transition:fade={{ duration: 200 }}></div>
        <!-- A click beside the card closes it; Escape does the same from the keyboard. -->
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div
          role="presentation"
          class="relative flex min-h-full items-start justify-center p-2 sm:items-center sm:p-4"
          onclick={(e) => closeOutside && e.target === e.currentTarget && close()}
        >
          <div
            role="dialog"
            aria-modal="true"
            aria-label={title}
            {@attach focusTrap}
            transition:scale={{ start: 0.95, duration: 200 }}
            class={[
              'application-settings-form application-settings-section relative flex max-h-[calc(100dvh-2rem)] w-full flex-col overflow-hidden',
              isLarge ? 'lg:w-[95vw]! lg:max-w-7xl!' : 'lg:w-auto lg:max-w-4xl lg:min-w-2xl',
            ]}
            style="box-shadow: 0 0 0 1px var(--coollabs-hairline), var(--shadow-modal)"
          >
            <header class="flex-wrap! sm:flex-nowrap!">
              <div class="min-w-0 flex-1 py-0.5">
                <h3 class="truncate">{title}</h3>
                {#if subtitle}
                  <p class="mt-0.5 text-xs text-neutral-500 dark:text-fg-dim">{subtitle}</p>
                {/if}
              </div>
              {#if headerActions}
                <div class="order-3 flex w-full shrink-0 items-center gap-2 sm:order-none sm:w-auto">
                  {@render headerActions()}
                </div>
              {/if}
              <button
                type="button"
                onclick={close}
                aria-label="Close"
                class="order-2 flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md text-neutral-500 outline-0 transition-colors hover:bg-neutral-100 hover:text-black focus-visible:ring-1 focus-visible:ring-accent focus-visible:outline-none sm:order-none dark:text-fg-faint dark:hover:bg-white/[0.06] dark:hover:text-fg"
              >
                <Icon name="x" class="size-4" />
              </button>
            </header>
            <div
              class={['application-settings-section-body min-h-0 flex-1 overflow-y-auto', headerActions && 'mt-2 sm:mt-0']}
              style="-webkit-overflow-scrolling: touch;"
            >
              {@render children?.()}
            </div>
            {#if footer}
              <footer class="flex shrink-0 flex-wrap items-center justify-end gap-2 border-t border-neutral-200 px-4 py-3 dark:border-white/[0.08]">
                {@render footer()}
              </footer>
            {/if}
          </div>
        </div>
      </div>
    {/if}
  </div>
</div>
