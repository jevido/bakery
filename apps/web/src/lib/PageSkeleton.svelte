<script lang="ts">
  // Paperclip's PageSkeleton (components/PageSkeleton.tsx, MIT, see NOTICE),
  // its `list` and `dashboard` variants. The dashboard one leaves out the
  // banner and the chart row, which The Bakery's Dashboard does not have.
  import { Skeleton } from '$lib/components/ui/skeleton'

  let { variant = 'list' }: { variant?: 'list' | 'dashboard' } = $props()
</script>

{#if variant === 'dashboard'}
  <div class="space-y-6">
    <div class="grid grid-cols-2 gap-2 xl:grid-cols-4">
      {#each { length: 4 }, i (i)}
        <Skeleton class="h-24 w-full" />
      {/each}
    </div>
    <div class="grid gap-4 md:grid-cols-2">
      <Skeleton class="h-72 w-full" />
      <Skeleton class="h-72 w-full" />
    </div>
  </div>
{:else}
  <div class="space-y-4">
    <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
      <Skeleton class="h-9 w-44" />
      <div class="flex items-center gap-2">
        <Skeleton class="h-8 w-20" />
        <Skeleton class="h-8 w-24" />
      </div>
    </div>
    <div class="space-y-1">
      {#each { length: 7 }, i (i)}
        <Skeleton class="h-11 w-full rounded-none" />
      {/each}
    </div>
  </div>
{/if}
