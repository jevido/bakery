<script lang="ts">
  // Coolify's application settings section
  // (resources/views/components/application/settings-section.blade.php,
  // Apache-2.0, see NOTICE): a layer card with the title (its helper behind
  // the underlined title), optional actions on the right and the body.
  import type { Snippet } from 'svelte'
  import Helper from '../../lib/ui/Helper.svelte'

  let {
    id,
    title,
    helper,
    flush = false,
    actions,
    children,
  }: { id: string; title: string; helper?: string; flush?: boolean; actions?: Snippet; children: Snippet } = $props()
</script>

<section {id} class="application-settings-section">
  <header>
    <div class="min-w-0 py-0.5">
      {#if helper}
        <h3>
          <Helper {helper} label={`More information about ${title}`}>
            {#snippet icon()}<span class="underline underline-offset-4">{title}</span>{/snippet}
          </Helper>
        </h3>
      {:else}
        <h3>{title}</h3>
      {/if}
    </div>
    {#if actions}
      <div class="flex max-w-full min-w-0 flex-wrap items-center gap-2">{@render actions()}</div>
    {/if}
  </header>
  <div class={['application-settings-section-body', flush && 'is-flush']}>
    {@render children()}
  </div>
</section>
