<script lang="ts">
  // Paperclip's toast stack (components/ToastViewport.tsx): a tinted card per
  // toast with a status dot, title, body and dismiss button, here in the
  // status tokens and kept bottom-right where Coolify has it. Coolify's copy
  // button for the details stays. Bakery's toasts carry plain text only.
  import { fly } from 'svelte/transition'
  import { motion } from './motion'
  import Check from '@lucide/svelte/icons/check'
  import Copy from '@lucide/svelte/icons/copy'
  import X from '@lucide/svelte/icons/x'
  import { dismiss, hold, schedule, toasts, type ToastType } from './toast.svelte'

  const tone: Record<ToastType, string> = {
    success: 'border-success/30 bg-success/10',
    info: 'border-border bg-popover',
    warning: 'border-warning/30 bg-warning/10',
    danger: 'border-destructive/30 bg-destructive/10',
    default: 'border-border bg-popover',
  }

  const dot: Record<ToastType, string> = {
    success: 'bg-success',
    info: 'bg-primary',
    warning: 'bg-warning',
    danger: 'bg-destructive',
    default: 'bg-muted-foreground',
  }

  let copied = $state<number | null>(null)

  async function copy(id: number, text: string) {
    await navigator.clipboard.writeText(text)
    copied = id
    setTimeout(() => copied === id && (copied = null), 2000)
  }
</script>

<ul class="chrome pointer-events-none list-none fixed right-3 bottom-3 z-9999 flex w-[calc(100%-1.5rem)] flex-col-reverse gap-2 sm:max-w-sm" aria-live="polite">
  {#each toasts as item (item.id)}
    <li
      in:fly={{ y: 12, duration: motion(200) }}
      out:fly={{ y: 4, duration: motion(150) }}
      onmouseenter={() => hold(item.id)}
      onmouseleave={() => schedule(item.id)}
      class={['group pointer-events-auto rounded-sm border bg-background text-foreground shadow-lg backdrop-blur-xl', tone[item.type]]}
    >
      <div class="flex items-start gap-3 px-3 py-2.5">
        <span class={['mt-1.5 size-2 shrink-0 rounded-full', dot[item.type]]}></span>
        <div class="min-w-0 flex-1">
          <p class="text-sm leading-5 font-semibold">{item.title}</p>
          {#if item.text}
            <p class="mt-1 text-xs leading-4 break-words whitespace-pre-wrap opacity-70">{item.text}</p>
          {/if}
        </div>
        {#if item.text}
          <button
            type="button"
            onclick={() => copy(item.id, item.text)}
            title={copied === item.id ? 'Copied' : 'Copy details'}
            class={[
              'mt-0.5 shrink-0 rounded p-1 opacity-0 transition-opacity group-hover:opacity-50 hover:bg-accent hover:opacity-100! focus-visible:opacity-100',
              copied === item.id && 'text-success opacity-100!',
            ]}
          >
            {#if copied === item.id}<Check class="size-3.5" />{:else}<Copy class="size-3.5" />{/if}
          </button>
        {/if}
        <button
          type="button"
          onclick={() => dismiss(item.id)}
          aria-label="Dismiss"
          class="mt-0.5 shrink-0 rounded p-1 opacity-50 hover:bg-accent hover:opacity-100"
        >
          <X class="size-3.5" />
        </button>
      </div>
    </li>
  {/each}
</ul>
