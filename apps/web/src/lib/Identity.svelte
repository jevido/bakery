<script lang="ts" module>
  /** A name's initials: the first and last word's first letters, or its first two. */
  export function initials(name: string): string {
    const parts = name.trim().split(/\s+/)
    if (parts.length >= 2) return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
    return name.slice(0, 2).toUpperCase()
  }
</script>

<script lang="ts">
  // Paperclip's Identity (ui/src/components/Identity.tsx; MIT, see NOTICE):
  // an initials avatar and the name beside it. Members have no avatar image.
  import * as Avatar from '$lib/components/ui/avatar'

  let { name, size = 'default', class: className = '' }: { name: string; size?: 'xs' | 'sm' | 'default' | 'lg'; class?: string } = $props()
</script>

<span class={['inline-flex min-w-0 items-center gap-1.5', size === 'xs' && 'gap-1', size === 'lg' && 'gap-2', className]} title={name}>
  <Avatar.Root {size}>
    <Avatar.Fallback>{initials(name)}</Avatar.Fallback>
  </Avatar.Root>
  <span class={['truncate', size === 'sm' ? 'text-xs' : 'text-sm']}>{name}</span>
</span>
