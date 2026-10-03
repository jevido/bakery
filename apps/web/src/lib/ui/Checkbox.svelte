<script lang="ts">
  // Coolify's forms/checkbox (resources/views/components/forms/checkbox.blade.php):
  // the whole row is the hit area, the box is drawn over an invisible input.
  // `instantSave` stands in for wire:click="instantSave": it is called with
  // the new value on every change.
  import Helper from './Helper.svelte'

  let {
    checked = $bindable(false),
    label,
    helper,
    disabled = false,
    fullWidth = false,
    id,
    instantSave,
  }: {
    checked?: boolean
    label?: string
    helper?: string
    disabled?: boolean
    fullWidth?: boolean
    id?: string
    instantSave?: (checked: boolean) => void
  } = $props()

  const generated = $props.id()
  const htmlId = $derived(id ?? generated)
</script>

<div
  class={[
    'chrome form-control group flex min-h-9 max-w-full items-center rounded-lg px-2.5 py-1.5 transition-colors',
    fullWidth && 'w-full',
    !disabled && 'cursor-pointer hover:bg-neutral-100/80 dark:hover:bg-white/[0.035]',
    disabled && 'opacity-55',
  ]}
>
  <label
    for={htmlId}
    class={['label flex w-full max-w-full min-w-0 items-center gap-3 px-0', disabled ? 'cursor-not-allowed' : 'cursor-pointer']}
  >
    <span class="flex min-w-0 grow items-center gap-1.5 text-[12px] break-words text-neutral-600 dark:text-fg-dim">
      {#if label}
        {label}
        {#if helper}<Helper {helper} />{/if}
      {/if}
    </span>
    <span class="relative flex size-[18px] shrink-0">
      <input
        type="checkbox"
        id={htmlId}
        bind:checked
        {disabled}
        onchange={() => instantSave?.(checked)}
        class="peer absolute inset-0 z-10 m-0 h-full w-full cursor-pointer appearance-none opacity-0 disabled:cursor-not-allowed"
      />
      <span
        class="pointer-events-none absolute inset-0 rounded-[5px] border border-neutral-300 bg-white shadow-[inset_0_1px_1px_rgb(0_0_0/0.04)] transition-[color,background-color,border-color,box-shadow] duration-150 ease-out group-hover:border-neutral-400 peer-checked:border-coollabs peer-checked:bg-coollabs peer-focus-visible:ring-2 peer-focus-visible:ring-coollabs/25 peer-focus-visible:ring-offset-2 peer-disabled:opacity-50 dark:border-white/[0.14] dark:bg-white/[0.045] dark:shadow-none dark:group-hover:border-white/[0.22] dark:peer-checked:border-warning dark:peer-checked:bg-warning dark:peer-focus-visible:ring-warning/30 dark:peer-focus-visible:ring-offset-base"
      ></span>
      <svg
        class="pointer-events-none absolute inset-0 m-auto size-3 scale-75 text-white opacity-0 transition-[opacity,transform] peer-checked:scale-100 peer-checked:opacity-100 dark:text-black"
        viewBox="0 0 12 12"
        fill="none"
        aria-hidden="true"
      >
        <path d="m2.25 6.15 2.35 2.3 5.15-5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    </span>
  </label>
</div>
