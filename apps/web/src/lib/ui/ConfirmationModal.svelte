<script lang="ts" module>
  export type ConfirmationCheckbox = {
    id: string
    label: string
    /** Ticked when the modal opens. */
    checked?: boolean
    /** Listed as an action when the box is left unticked. */
    defaultWarning?: string
  }
</script>

<script lang="ts">
  // Coolify's modal-confirmation (resources/views/components/modal-confirmation.blade.php).
  // Step 1 lists the `checkboxes` (what will also be removed), step 2 lists
  // the actions and enables its button only once `confirmationText` is typed.
  // Coolify's step 3 asking for the password is left out: Bakery's delete
  // endpoints do not take one, so step 2's button is the final "Confirm", as
  // Coolify's is when the password step is skipped.
  // Like Coolify, confirming closes the dialog and runs `onconfirm`; if that
  // throws, the dialog opens again.
  import type { Snippet } from 'svelte'
  import { fade, scale } from 'svelte/transition'
  import Icon from '../Icon.svelte'
  import Button from './Button.svelte'
  import Callout from './Callout.svelte'
  import Checkbox from './Checkbox.svelte'
  import CopyText from './CopyText.svelte'
  import { focusTrap } from './focusTrap'
  import { portal } from './portal'

  let {
    title = 'Are you sure?',
    buttonTitle = 'Confirm Action',
    variant = 'default',
    disabled = false,
    fullWidth = false,
    checkboxes = [],
    actions = [],
    warningMessage = 'This operation is permanent and cannot be undone. Please think again before proceeding!',
    confirmWithText = true,
    confirmationText = 'Confirm Deletion',
    confirmationLabel = 'Please confirm the execution of the actions by entering the Name below',
    shortConfirmationLabel = 'Name',
    step1ButtonText = 'Continue',
    step2ButtonText = 'Confirm',
    onconfirm,
    trigger,
  }: {
    title?: string
    buttonTitle?: string
    variant?: 'default' | 'highlighted' | 'error'
    disabled?: boolean
    fullWidth?: boolean
    checkboxes?: ConfirmationCheckbox[]
    actions?: string[]
    warningMessage?: string
    confirmWithText?: boolean
    confirmationText?: string
    confirmationLabel?: string
    shortConfirmationLabel?: string
    step1ButtonText?: string
    step2ButtonText?: string
    /** Called with the ids of the ticked checkboxes. */
    onconfirm: (selected: string[]) => unknown
    trigger?: Snippet<[() => void]>
  } = $props()

  const initialStep = $derived(checkboxes.length > 0 ? 1 : 2)
  const inputId = $props.id()

  let open = $state(false)
  let step = $state(2)
  let typed = $state('')
  let submitting = $state(false)
  let selected = $state<Record<string, boolean>>({})

  const chosen = $derived(checkboxes.filter((c) => selected[c.id]))
  const listed = $derived([
    ...actions,
    ...chosen.map((c) => c.label),
    ...checkboxes.filter((c) => !selected[c.id] && c.defaultWarning).map((c) => c.defaultWarning as string),
  ])
  const canConfirm = $derived(!submitting && (!confirmWithText || typed === confirmationText))

  function reset() {
    step = initialStep
    typed = ''
    submitting = false
    selected = Object.fromEntries(checkboxes.map((c) => [c.id, c.checked ?? false]))
  }

  function show() {
    reset()
    open = true
  }

  function close() {
    open = false
    reset()
  }

  async function confirm() {
    submitting = true
    open = false
    try {
      await onconfirm(chosen.map((c) => c.id))
      reset()
    } catch {
      submitting = false
      open = true
    }
  }
</script>

<svelte:window onkeydown={(e) => open && e.key === 'Escape' && close()} />

<div class={['chrome relative h-auto max-w-full', fullWidth ? 'flex w-full' : 'inline-flex w-auto']}>
  {#if trigger}
    {@render trigger(show)}
  {:else}
    <Button {variant} {disabled} onclick={show} class={['flex gap-2', fullWidth && 'w-full'].filter(Boolean).join(' ')}>
      {buttonTitle}
    </Button>
  {/if}
  <div class="chrome" {@attach portal}>
    {#if open}
      <div class="fixed inset-0 z-99 flex min-h-full items-center justify-center overflow-y-auto p-4">
        <div class="absolute inset-0 bg-black/50 backdrop-blur-[2px]" transition:fade={{ duration: 200 }}></div>
        <div
          role="dialog"
          aria-modal="true"
          aria-label={title}
          {@attach focusTrap}
          transition:scale={{ start: 0.95, duration: 200 }}
          class="application-settings-form application-settings-section relative flex max-h-[calc(100dvh-2rem)] w-full flex-col lg:max-w-2xl lg:min-w-[36rem]"
          style="box-shadow: 0 0 0 1px var(--coollabs-hairline), var(--shadow-modal)"
        >
          <header class="flex-nowrap!">
            <h3 class="min-w-0 flex-1 truncate">{title}</h3>
            <button
              type="button"
              onclick={close}
              aria-label="Close"
              class="flex size-7 shrink-0 items-center justify-center rounded-md text-neutral-500 transition-colors hover:bg-neutral-100 hover:text-black dark:text-fg-faint dark:hover:bg-white/[0.06] dark:hover:text-fg"
            >
              <Icon name="x" class="size-4" />
            </button>
          </header>
          <div class="application-settings-section-body min-h-0 flex-1 overflow-y-auto" style="-webkit-overflow-scrolling: touch;">
            {#if step === 1}
              <div>
                {#each checkboxes as checkbox (checkbox.id)}
                  <div class="mb-2 flex items-center justify-between">
                    <Checkbox fullWidth label={checkbox.label} bind:checked={selected[checkbox.id]} />
                  </div>
                {/each}
                <div class="mt-4 flex flex-wrap justify-end gap-2 border-t border-neutral-200 pt-4 dark:border-white/[0.08]">
                  <Button variant="error" class="w-auto" onclick={() => step++}>{step1ButtonText}</Button>
                </div>
              </div>
            {:else}
              <div>
                <Callout type="danger" title="Warning" class="mb-4">{warningMessage}</Callout>
                <div class="mb-2 text-[12px] font-medium text-neutral-700 dark:text-fg-dim">The following actions will be performed:</div>
                <ul class="mb-4 space-y-2">
                  {#each listed as action, i (i)}
                    <li class="flex items-start gap-2 text-[12px] leading-5 text-red-600 dark:text-red-400">
                      <Icon name="trash" class="mt-0.5 size-3.5 shrink-0" />
                      <span>{action}</span>
                    </li>
                  {/each}
                </ul>
                {#if confirmWithText}
                  <div class="mb-4">
                    <h4 class="mb-1 text-[12px] font-semibold">Confirm actions</h4>
                    <p class="mb-2 text-[12px] leading-5 text-neutral-500 dark:text-fg-dim">{confirmationLabel}</p>
                    <CopyText text={confirmationText} />
                    <label for={inputId} class="mt-4 mb-1.5 block text-[12px] font-medium text-neutral-700 dark:text-fg-dim">
                      {shortConfirmationLabel}
                    </label>
                    <input id={inputId} type="text" bind:value={typed} class="input w-full" autocomplete="off" />
                  </div>
                {/if}
                <div class="mt-4 flex flex-wrap justify-end gap-2 border-t border-neutral-200 pt-4 dark:border-white/[0.08]">
                  {#if checkboxes.length > 0}
                    <Button onclick={() => step--}>Back</Button>
                  {:else}
                    <Button onclick={close}>Cancel</Button>
                  {/if}
                  <Button variant="error" class="w-auto" disabled={!canConfirm} loading={submitting} onclick={confirm}>
                    {step2ButtonText}
                  </Button>
                </div>
              </div>
            {/if}
          </div>
        </div>
      </div>
    {/if}
  </div>
</div>
