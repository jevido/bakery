<script lang="ts">
  // Coolify's modal-input (resources/views/components/modal-input.blade.php),
  // which is what its pages open for every form in a dialog: a trigger button
  // (or the `trigger` snippet, or none when `open` is driven from outside),
  // and a dialog with a title, optional subtitle, header actions and footer,
  // drawn as Paperclip's dialog. Escape, the close button and a click beside
  // the card close it. Its own portal and focus trap stay rather than bits-ui's
  // Dialog: `closeOutside`, `onclose` and the focus of the first field are
  // what callers rely on.
  import type { Snippet } from 'svelte'
  import { fade, scale } from 'svelte/transition'
  import { motion } from './motion'
  import X from '@lucide/svelte/icons/x'
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
        <div class="absolute inset-0 h-full w-full bg-black/50" transition:fade={{ duration: motion(150) }}></div>
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
            transition:scale={{ start: 0.97, duration: motion(150) }}
            class={[
              'relative flex max-h-[calc(100dvh-2rem)] w-full flex-col overflow-hidden rounded-lg border bg-background text-foreground shadow-lg',
              isLarge ? 'lg:w-[95vw]! lg:max-w-7xl!' : 'lg:w-auto lg:max-w-4xl lg:min-w-2xl',
            ]}
          >
            <header class="flex flex-wrap items-start gap-2 px-6 pt-6 pb-4 sm:flex-nowrap">
              <div class="min-w-0 flex-1">
                <h3 class="truncate text-lg leading-none font-semibold">{title}</h3>
                {#if subtitle}
                  <p class="mt-2 text-sm text-muted-foreground">{subtitle}</p>
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
                class="order-2 -mt-1 -mr-2 flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground opacity-70 transition-opacity outline-none hover:opacity-100 focus-visible:ring-(length:--rad-3) focus-visible:ring-ring/50 sm:order-none"
              >
                <X class="size-4" />
              </button>
            </header>
            <div class="min-h-0 flex-1 overflow-y-auto px-6 pb-6" style="-webkit-overflow-scrolling: touch;">
              {@render children?.()}
            </div>
            {#if footer}
              <footer class="flex shrink-0 flex-col-reverse gap-2 border-t px-6 py-4 sm:flex-row sm:flex-wrap sm:items-center sm:justify-end">
                {@render footer()}
              </footer>
            {/if}
          </div>
        </div>
      </div>
    {/if}
  </div>
</div>
