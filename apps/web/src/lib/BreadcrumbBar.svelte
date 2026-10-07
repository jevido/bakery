<script lang="ts">
  // Paperclip's BreadcrumbBar (ui/src/components/BreadcrumbBar.tsx; MIT, see
  // NOTICE): the 60px bar over the page. A single crumb is the page's title;
  // several are a trail whose first link is set in caps. A Resource's crumb
  // keeps its status beside its name. The sidebar toggle leads it (the
  // drawer's hamburger on a phone), a Server page's switcher docks after the
  // crumbs, and an Application's Links and Actions on the right.
  import { Menu, PanelLeft } from '@lucide/svelte'
  import * as Breadcrumb from '$lib/components/ui/breadcrumb'
  import { Button } from '$lib/components/ui/button'
  import { breadcrumb, type Crumb } from './breadcrumb.svelte'
  import { sidebar } from './sidebar.svelte'
  import StatusBadge from './ui/StatusBadge.svelte'
  import StatusSummary from './ui/StatusSummary.svelte'
  import { cn } from './utils'

  const crumbs = $derived(breadcrumb.crumbs)
</script>

{#snippet status(crumb: Crumb)}
  {#if crumb.summary}
    <StatusSummary status={crumb.summary} title={crumb.summaryTitle} />
  {:else if crumb.status}
    <StatusBadge status={crumb.status.label} type={crumb.status.type} class="shrink-0" />
  {/if}
{/snippet}

<div class="chrome flex h-(--sz-60px) shrink-0 items-center border-b border-border px-4 md:px-6" data-testid="breadcrumb-bar">
  <Button
    variant="ghost"
    size="icon-sm"
    class="mr-2 -ml-2 shrink-0 text-muted-foreground"
    onclick={() => sidebar.toggle()}
    aria-label={sidebar.mobile ? 'Open sidebar' : sidebar.collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
    aria-pressed={sidebar.mobile ? undefined : sidebar.collapsed}
    title={sidebar.mobile ? 'Open sidebar' : 'Toggle sidebar ([)'}
    data-testid="sidebar-toggle"
  >
    {#if sidebar.mobile}<Menu class="size-5" />{:else}<PanelLeft class="size-4" />{/if}
  </Button>
  <div class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
    {#if crumbs.length === 1}
      <h1 class="flex min-w-0 items-center gap-2 text-sm font-semibold tracking-wider uppercase">
        {#if crumbs[0].href}
          <a href={crumbs[0].href} class="truncate hover:text-muted-foreground">{crumbs[0].label}</a>
        {:else}
          <span class="truncate">{crumbs[0].label}</span>
        {/if}
        {@render status(crumbs[0])}
      </h1>
    {:else if crumbs.length > 1}
      <Breadcrumb.Root class="min-w-0 overflow-hidden">
        <Breadcrumb.List class="flex-nowrap">
          {#each crumbs as crumb, i (i)}
            {@const last = i === crumbs.length - 1}
            {#if i > 0}<Breadcrumb.Separator />{/if}
            <Breadcrumb.Item class={last ? 'min-w-0' : 'shrink-0'}>
              {#if last || !crumb.href}
                <Breadcrumb.Page class="truncate">{crumb.label}</Breadcrumb.Page>
              {:else}
                <Breadcrumb.Link
                  href={crumb.href}
                  class={cn('min-w-0 truncate', i === 0 && 'font-semibold tracking-wider text-muted-foreground uppercase hover:text-foreground')}
                >
                  {crumb.label}
                </Breadcrumb.Link>
              {/if}
              {@render status(crumb)}
            </Breadcrumb.Item>
          {/each}
        </Breadcrumb.List>
      </Breadcrumb.Root>
    {/if}
    <!-- A Server page's switcher and status summary dock here, after the breadcrumb. -->
    <div id="server-topbar-context" class="min-w-0"></div>
  </div>
  <!-- Resource actions dock here on desktop (the Application heading's Links and Actions). -->
  <div id="resource-action-hud-slot" class="hidden shrink-0 items-center xl:flex"></div>
</div>
