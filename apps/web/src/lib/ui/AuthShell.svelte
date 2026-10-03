<script lang="ts">
  // Coolify's auth shell (resources/views/components/auth/shell.blade.php)
  // inside layout-simple: the centred card with a heading, a body and an
  // optional footer. The heading carries The Bakery's logo next to the title.
  import type { Snippet } from 'svelte'

  let {
    title = 'The Bakery',
    description,
    children,
    footer,
  }: { title?: string; description?: string; children?: Snippet; footer?: Snippet } = $props()
</script>

<main class="chrome h-full bg-neutral-50 dark:bg-base">
  <section class="auth-shell">
    <div class="auth-shell-content">
      <div class="auth-card">
        <div class="auth-card-heading">
          <div class="flex items-center gap-2">
            <img src="/favicon.svg" alt="" class="size-6" />
            <h1>{title}</h1>
          </div>
          {#if description}<p>{description}</p>{/if}
        </div>
        <div class="auth-card-body">
          {@render children?.()}
        </div>
        {#if footer}
          <footer class="auth-card-footer">{@render footer()}</footer>
        {/if}
      </div>
    </div>
  </section>
</main>
