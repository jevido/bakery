<script lang="ts">
  // The list/grid switch of a list page's toolbar: two ghost icon buttons,
  // the current one filled.
  import { buttonVariants } from '$lib/components/ui/button'
  import Icon from './Icon.svelte'

  type ViewMode = 'table' | 'grid'

  let { value, onchange }: { value: ViewMode; onchange: (mode: ViewMode) => void } = $props()

  const views = [
    { mode: 'table', icon: 'unordered-list', label: 'Table view' },
    { mode: 'grid', icon: 'grid', label: 'Grid view' },
  ] as const
</script>

<div class="flex items-center" role="group" aria-label="View">
  {#each views as v (v.mode)}
    <button
      type="button"
      onclick={() => onchange(v.mode)}
      class={buttonVariants({
        variant: 'ghost',
        size: 'icon-sm',
        class: ['size-8', value === v.mode ? 'bg-accent text-foreground' : 'text-muted-foreground'].join(' '),
      })}
      aria-label={v.label}
      aria-pressed={value === v.mode}
      title={v.label}
    >
      <Icon name={v.icon} class="size-3.5" />
    </button>
  {/each}
</div>
