<script lang="ts">
  // Coolify's application links (resources/views/components/applications/links.blade.php,
  // Apache-2.0, see NOTICE): a Links button and a panel of the Application's
  // addresses. `compact` is the pill beside the status on phones. A Service's
  // (components/services/links.blade.php) are the same without the Production
  // badge.
  import Icon from '../../lib/Icon.svelte'

  let {
    urls,
    compact = false,
    resource = 'application',
  }: { urls: string[]; compact?: boolean; resource?: 'application' | 'service' } = $props()

  let open = $state(false)
  let root = $state<HTMLDivElement>()
</script>

<svelte:window
  onclick={(e) => open && root && !root.contains(e.target as Node) && (open = false)}
  onkeydown={(e) => open && e.key === 'Escape' && (open = false)}
/>

<div class={compact ? 'static' : 'relative'} bind:this={root}>
  <button
    type="button"
    onclick={() => (open = !open)}
    title={`Open ${resource} links`}
    aria-expanded={open}
    aria-haspopup="menu"
    class={compact
      ? 'inline-flex h-6 shrink-0 items-center gap-1.5 rounded-full border border-neutral-200 bg-neutral-100 px-2 text-xs leading-none font-medium text-neutral-700 dark:border-white/[0.12] dark:bg-white/[0.07] dark:text-white'
      : 'app-tab shrink-0 gap-1'}
  >
    <span class="inline-flex items-center gap-2">
      {#if !compact}<Icon name="external-link" class="size-3.5 shrink-0 opacity-70" />{/if}
      Links
    </span>
    <span class={['inline-flex transition-transform', open && 'rotate-180']}>
      <Icon name="chevron-down" class="size-3 opacity-55" />
    </span>
  </button>
  {#if open}
    <div
      role="menu"
      class={[
        'listbox-panel top-full! mt-1!',
        compact ? 'right-auto! left-1/2! w-[calc(100vw-2rem)]! max-w-md! min-w-0! -translate-x-1/2' : 'right-0! left-auto! max-w-96! min-w-60!',
      ]}
    >
      {#each urls as url (url)}
        <a role="menuitem" class="listbox-option justify-start! gap-2.5!" target="_blank" rel="noreferrer" href={url}>
          {#if resource === 'application'}
            <span
              class="shrink-0 rounded-md bg-success/10 px-1.5 py-0.5 text-[10px] font-semibold tracking-wide text-success uppercase ring-1 ring-success/20"
            >
              Production
            </span>
          {/if}
          <span class="min-w-0 truncate">{url}</span>
        </a>
      {:else}
        <div class="listbox-option cursor-default! justify-start!">No links available</div>
      {/each}
    </div>
  {/if}
</div>
