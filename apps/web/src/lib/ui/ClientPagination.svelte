<script lang="ts">
  // Coolify's client-pagination (resources/views/components/client-pagination.blade.php):
  // "start-end of total", the page size, and previous/next, for a list paged
  // in the browser.
  import Icon from '../Icon.svelte'
  import PageSizeSelect from './PageSizeSelect.svelte'

  let {
    page = $bindable(1),
    pageSize = $bindable(),
    total,
    options = [10, 25, 50, 100],
    storageKey,
    class: className = '',
  }: {
    page?: number
    pageSize: number
    total: number
    options?: number[]
    storageKey?: string
    class?: string
  } = $props()

  const totalPages = $derived(Math.max(1, Math.ceil(total / pageSize)))
  const start = $derived(total === 0 ? 0 : (page - 1) * pageSize + 1)
  const end = $derived(Math.min(page * pageSize, total))

  const arrow =
    'flex size-7 items-center justify-center rounded-md border border-neutral-200 text-neutral-500 transition-colors hover:bg-neutral-100 hover:text-black disabled:pointer-events-none disabled:opacity-35 dark:border-white/[0.08] dark:text-fg-dim dark:hover:bg-white/[0.06] dark:hover:text-fg'
</script>

<footer
  class={[
    'flex min-h-11 items-center justify-between border-t border-neutral-200 px-4 text-[11px] text-neutral-500 dark:border-white/[0.08] dark:text-fg-faint',
    className,
  ]}
>
  <div class="flex items-center gap-3">
    <span class="inline-flex h-7 items-center whitespace-nowrap tabular-nums">{start}-{end} of {total}</span>
    <PageSizeSelect bind:value={pageSize} {options} {storageKey} onchange={() => (page = 1)} />
  </div>
  <div class="flex items-center gap-1">
    <button type="button" class={arrow} disabled={page === 1} onclick={() => (page = Math.max(1, page - 1))} aria-label="Previous page">
      <Icon name="arrow-right" class="size-3.5 rotate-180" />
    </button>
    <button
      type="button"
      class={arrow}
      disabled={page >= totalPages}
      onclick={() => (page = Math.min(totalPages, page + 1))}
      aria-label="Next page"
    >
      <Icon name="arrow-right" class="size-3.5" />
    </button>
  </div>
</footer>
