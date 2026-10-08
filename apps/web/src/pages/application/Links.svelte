<script lang="ts">
  // Coolify's application links (resources/views/components/applications/links.blade.php,
  // Apache-2.0, see NOTICE): a Links button and a menu of the Application's
  // addresses, drawn as Paperclip's outline button and DropdownMenu. A
  // Service's (components/services/links.blade.php) are the same without the
  // Production badge.
  import { ChevronDown, ExternalLink } from '@lucide/svelte'
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import * as DropdownMenu from '@bakery/ui/components/ui/dropdown-menu'

  let { urls, resource = 'application' }: { urls: string[]; resource?: 'application' | 'service' } = $props()
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger class={buttonVariants({ variant: 'outline', size: 'sm' })} title={`Open ${resource} links`}>
    <ExternalLink />
    Links
    <ChevronDown class="opacity-60" />
  </DropdownMenu.Trigger>
  <DropdownMenu.Content align="end" class="w-[min(calc(100vw-2rem),24rem)]">
    {#each urls as url (url)}
      <DropdownMenu.Item>
        {#snippet child({ props })}
          <a {...props} target="_blank" rel="noreferrer" href={url}>
            {#if resource === 'application'}
              <span
                class="shrink-0 rounded-md bg-success/10 px-1.5 py-0.5 text-[10px] font-semibold tracking-wide text-success uppercase ring-1 ring-success/20"
              >
                Production
              </span>
            {/if}
            <span class="min-w-0 truncate">{url}</span>
          </a>
        {/snippet}
      </DropdownMenu.Item>
    {:else}
      <DropdownMenu.Item disabled>No links available</DropdownMenu.Item>
    {/each}
  </DropdownMenu.Content>
</DropdownMenu.Root>
