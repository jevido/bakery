<script lang="ts">
  // Coolify's toast stack (resources/views/components/toast.blade.php),
  // bottom-right. The `html` toasts Coolify can show are left out: Bakery's
  // toasts carry plain text only.
  import { fly } from 'svelte/transition'
  import Icon, { type IconName } from '../Icon.svelte'
  import { dismiss, hold, schedule, toasts, type ToastType } from './toast.svelte'

  const badge: Record<ToastType, string> = {
    success: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-400',
    info: 'bg-coollabs/10 text-coollabs dark:bg-warning/10 dark:text-warning',
    warning: 'bg-amber-100 text-amber-700 dark:bg-warning/10 dark:text-warning',
    danger: 'bg-red-100 text-red-700 dark:bg-red-500/10 dark:text-red-400',
    default: 'bg-neutral-100 text-neutral-600 dark:bg-white/[0.06] dark:text-fg-dim',
  }

  const icon: Record<ToastType, IconName> = {
    success: 'check-circle',
    info: 'info-circle',
    default: 'info-circle',
    warning: 'alert-triangle',
    danger: 'alert-circle',
  }

  let copied = $state<number | null>(null)

  async function copy(id: number, text: string) {
    await navigator.clipboard.writeText(text)
    copied = id
    setTimeout(() => copied === id && (copied = null), 2000)
  }
</script>

<ul class="chrome fixed right-4 bottom-4 z-9999 flex w-[calc(100%-2rem)] flex-col-reverse gap-2.5 sm:max-w-[26rem]" aria-live="polite">
  {#each toasts as item (item.id)}
    <li
      in:fly={{ y: 8, duration: 200 }}
      out:fly={{ y: 4, duration: 150 }}
      onmouseenter={() => hold(item.id)}
      onmouseleave={() => schedule(item.id)}
      class="surface-popover group relative flex w-full items-start rounded-lg p-3.5 pr-20"
    >
      <div class="flex min-w-0 items-start gap-3">
        <div class={['flex size-8 shrink-0 items-center justify-center rounded-lg', badge[item.type]]}>
          <Icon name={icon[item.type]} class="size-4" />
        </div>
        <div class="min-w-0 flex-1 pt-0.5">
          <p class="text-sm leading-5 font-semibold text-neutral-950 dark:text-fg">{item.title}</p>
          {#if item.text}
            <div class="mt-0.5 w-full text-xs leading-5 break-words whitespace-pre-wrap text-neutral-600 dark:text-fg-dim">{item.text}</div>
          {/if}
        </div>
      </div>
      {#if item.text}
        <button
          type="button"
          onclick={() => copy(item.id, item.text)}
          title={copied === item.id ? 'Copied' : 'Copy details'}
          class={[
            'absolute top-2.5 right-10 flex size-7 items-center justify-center rounded-md text-neutral-400 opacity-0 transition-colors group-hover:opacity-100 hover:bg-black/5 hover:text-neutral-700 dark:text-fg-faint dark:hover:bg-white/[0.06] dark:hover:text-fg',
            copied === item.id && 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-400',
          ]}
        >
          {#if copied === item.id}
            <Icon name="check" class="size-3.5" />
          {:else}
            <svg class="size-3.5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.7" stroke="currentColor">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M8.25 7.5V6a2.25 2.25 0 012.25-2.25h7.5A2.25 2.25 0 0120.25 6v7.5A2.25 2.25 0 0118 15.75h-1.5m-8.25-8.25H6A2.25 2.25 0 003.75 9.75v7.5A2.25 2.25 0 006 19.5h7.5a2.25 2.25 0 002.25-2.25V15m-7.5-7.5h5.25A2.25 2.25 0 0115.75 9.75V15"
              />
            </svg>
          {/if}
        </button>
      {/if}
      <button
        type="button"
        onclick={() => dismiss(item.id)}
        aria-label="Dismiss"
        class="absolute top-2.5 right-2.5 flex size-7 items-center justify-center rounded-md text-neutral-400 transition-colors hover:bg-black/5 hover:text-neutral-700 dark:text-fg-faint dark:hover:bg-white/[0.06] dark:hover:text-fg"
      >
        <Icon name="x" class="size-3.5" />
      </button>
    </li>
  {/each}
</ul>
