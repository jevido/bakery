<script lang="ts">
  // Coolify's unsaved-bar (resources/views/components/unsaved-bar.blade.php):
  // the floating pill that shows while a form has changes not yet saved, with
  // Reset and "Save changes", and Enter saving from anywhere but a textarea,
  // button, link or select. It shows 300 ms after the form turns dirty and
  // hides at once while saving, as Coolify's delayed classes do. On a phone
  // it sits above the on-screen keyboard.
  let {
    dirty,
    saving = false,
    label = "You have changes that haven't been saved yet.",
    onsave,
    onreset,
  }: {
    dirty: boolean
    saving?: boolean
    label?: string
    onsave: () => unknown
    onreset: () => void
  } = $props()

  let keyboardInset = $state(0)

  $effect(() => {
    const update = () => {
      const viewport = window.visualViewport
      keyboardInset =
        window.innerWidth < 640 && viewport ? Math.max(0, window.innerHeight - viewport.height - viewport.offsetTop) : 0
    }
    update()
    window.visualViewport?.addEventListener('resize', update)
    window.visualViewport?.addEventListener('scroll', update)
    window.addEventListener('resize', update)
    return () => {
      window.visualViewport?.removeEventListener('resize', update)
      window.visualViewport?.removeEventListener('scroll', update)
      window.removeEventListener('resize', update)
    }
  })

  function onkeydown(e: KeyboardEvent) {
    if (e.key !== 'Enter' || !dirty || saving) return
    if (e.repeat || e.isComposing || e.shiftKey || e.ctrlKey || e.altKey || e.metaKey) return
    if (!(e.target instanceof Element)) return
    if (e.target.closest('textarea, button, a, select, [contenteditable=true]')) return
    // A form with its own submit button (another section's Save) submits itself.
    if (e.target.closest('form')?.querySelector('button:not([type]), button[type=submit], input[type=submit]')) return
    e.preventDefault()
    onsave()
  }
</script>

<svelte:window {onkeydown} />

<div
  role="status"
  aria-hidden={!dirty || saving}
  style="--keyboard-inset: {keyboardInset}px"
  class={[
    'pointer-events-none fixed inset-x-3 bottom-[calc(var(--keyboard-inset,0px)+max(1.5rem,env(safe-area-inset-bottom,0px)+0.75rem))] z-[1000] flex max-w-full translate-y-6 scale-95 flex-col items-stretch gap-2 rounded-2xl border border-neutral-200 bg-white py-2.5 pr-2.5 pl-4 opacity-0 shadow-modal transition-[opacity,transform,scale] duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] delay-0 dark:border-white/10 dark:bg-surface [&.is-dirty]:pointer-events-auto [&.is-dirty]:translate-y-0 [&.is-dirty]:scale-100 [&.is-dirty]:opacity-100 [&.is-dirty]:delay-300 [&.is-saving]:pointer-events-none [&.is-saving]:translate-y-6 [&.is-saving]:scale-95 [&.is-saving]:opacity-0 [&.is-saving]:delay-0 [&.is-saving]:duration-200 [&.is-saving]:ease-in sm:inset-x-auto sm:bottom-6 sm:left-1/2 sm:w-max sm:max-w-none sm:-translate-x-1/2 sm:flex-row sm:items-center sm:gap-8 sm:py-2 sm:pr-2 sm:pl-5',
    dirty && 'is-dirty',
    saving && 'is-saving',
  ]}
>
  <span class="text-[13px] leading-snug font-semibold text-neutral-800 sm:whitespace-nowrap dark:text-fg">{label}</span>
  <div class="flex shrink-0 items-center justify-end gap-2">
    <button
      type="button"
      tabindex={dirty ? 0 : -1}
      onclick={onreset}
      class="h-8 rounded-lg bg-neutral-100 px-3.5 text-[13px] font-medium text-neutral-700 transition-colors hover:bg-neutral-200 dark:bg-white/[0.07] dark:text-fg dark:hover:bg-white/[0.12]"
    >
      Reset
    </button>
    <button
      type="button"
      tabindex={dirty ? 0 : -1}
      disabled={saving}
      onclick={onsave}
      class="button-highlighted flex h-8 items-center gap-2 rounded-lg px-4 text-[13px] font-semibold transition-[transform,background-color] active:scale-[0.98]"
    >
      <span>Save changes</span>
      <kbd class="rounded border border-current/20 bg-current/10 px-1.5 py-0.5 text-[10px] leading-none font-medium text-current">Enter</kbd>
    </button>
  </div>
</div>
