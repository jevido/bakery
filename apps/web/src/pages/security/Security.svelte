<script lang="ts">
  // Coolify's Keys & Tokens layout (resources/views/components/security/settings-layout.blade.php,
  // Apache-2.0, see NOTICE): the header below xl, the Keys & Tokens sidebar
  // and the page. The Bakery has only API Tokens; Private Keys, Cloud Tokens
  // and Cloud-Init Scripts are left out rather than shown as dead links.
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import Icon, { type IconName } from '../../lib/Icon.svelte'
  import { href, securityPath, type SecurityPage } from '../../lib/router.svelte'
  import ApiTokens from './ApiTokens.svelte'

  let { page }: { page: SecurityPage } = $props()

  const items: { page: SecurityPage; label: string; icon: IconName }[] = [{ page: 'api-tokens', label: 'API Tokens', icon: 'code' }]

  $effect(() => {
    void page
    breadcrumb.set({ label: 'Keys & Tokens' })
  })
</script>

<section class="chrome application-settings-workspace w-full max-w-none">
  <header class="settings-mobile-header xl:hidden">
    <h1 class="settings-mobile-title">Keys & Tokens</h1>
    <p class="settings-mobile-description">Manage SSH keys, cloud credentials, and API access tokens.</p>
  </header>
  <div class="grid min-w-0 gap-8 xl:grid-cols-[210px_minmax(0,1fr)] xl:gap-8">
    <aside class="application-settings-navigation min-w-0 xl:self-start">
      <nav
        aria-label="Keys and tokens"
        class="grid grid-cols-2 gap-0.5 border-y border-neutral-200 py-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-1 xl:border-y-0 xl:py-0 dark:border-white/[0.06]"
      >
        <div class="nav-section hidden xl:block">Keys & Tokens</div>
        {#each items as item (item.page)}
          <a
            class={['menu-item', item.page === page && 'menu-item-active']}
            href={href(securityPath(item.page))}
            aria-current={item.page === page ? 'page' : undefined}
            data-testid="security-menu-item"
          >
            <Icon name={item.icon} class="menu-item-icon" />
            <span class="menu-item-label">{item.label}</span>
          </a>
        {/each}
      </nav>
    </aside>

    <div class="min-w-0">
      <ApiTokens />
    </div>
  </div>
</section>
