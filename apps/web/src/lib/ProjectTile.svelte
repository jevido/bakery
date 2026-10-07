<script lang="ts">
  // Paperclip's ProjectTile (components/ProjectTile.tsx, MIT, see NOTICE): a
  // neutral rounded square with a folder icon, tinted when given a color.
  // Projects here have no color or icon of their own, so pages pass neither.
  import { Folder } from '@lucide/svelte'
  import Icon, { type IconName } from './Icon.svelte'

  type Size = 'xs' | 'sm' | 'md' | 'lg'

  let {
    color,
    icon,
    size = 'md',
    class: className = '',
  }: { color?: string | null; icon?: IconName; size?: Size; class?: string } = $props()

  const sizes: Record<Size, { box: string; icon: string }> = {
    xs: { box: 'h-4 w-4 rounded-sm', icon: 'h-2.5 w-2.5' },
    sm: { box: 'h-6 w-6 rounded-md', icon: 'h-3.5 w-3.5' },
    md: { box: 'h-7 w-7 rounded-lg', icon: 'h-4 w-4' },
    lg: { box: 'h-9 w-9 rounded-lg', icon: 'h-5 w-5' },
  }
  const dims = $derived(sizes[size])
</script>

<span
  aria-hidden="true"
  class={['inline-flex shrink-0 items-center justify-center', dims.box, color ? 'text-white' : 'bg-muted text-muted-foreground', className]}
  style:background-color={color || undefined}
>
  {#if icon}<Icon name={icon} class={dims.icon} />{:else}<Folder class={dims.icon} />{/if}
</span>
