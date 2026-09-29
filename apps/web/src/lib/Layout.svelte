<script lang="ts">
  import type { Snippet } from 'svelte'
  import { href, router } from './router.svelte'
  import { session } from './session.svelte'

  let { children }: { children: Snippet } = $props()

  let section = $derived(
    router.route.name === 'notfound' ? '' : router.route.name === 'settings' ? 'settings' : 'projects',
  )
</script>

<div class="shell">
  <aside>
    <a class="brand" href={href('/projects')}>
      <img src="/favicon.svg" alt="" width="22" height="22" />
      Bakery
    </a>
    <nav>
      <a href={href('/projects')} aria-current={section === 'projects' ? 'page' : undefined}>Projects</a>
      <a href={href('/settings')} aria-current={section === 'settings' ? 'page' : undefined}>Settings</a>
    </nav>
  </aside>
  <div class="main">
    <header>
      <span class="muted">{session.owner?.name} · {session.owner?.email}</span>
      <button onclick={() => session.logout()}>Log out</button>
    </header>
    <main>
      {@render children()}
    </main>
  </div>
</div>

<style>
  .shell {
    display: grid;
    grid-template-columns: 13rem 1fr;
    min-height: 100vh;
  }
  aside {
    background: var(--panel);
    border-right: 1px solid var(--border);
    padding: 1rem 0.75rem;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-weight: 700;
    color: var(--text);
    text-decoration: none;
    padding: 0 0.5rem 1rem;
  }
  nav {
    display: grid;
    gap: 0.15rem;
  }
  nav a {
    padding: 0.4rem 0.5rem;
    border-radius: 6px;
    color: var(--muted);
    text-decoration: none;
  }
  nav a:hover,
  nav a[aria-current='page'] {
    background: var(--hover);
    color: var(--text);
  }
  .main {
    display: grid;
    grid-template-rows: auto 1fr;
    min-width: 0;
  }
  header {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 1rem;
    padding: 0.6rem 1.25rem;
    border-bottom: 1px solid var(--border);
  }
  main {
    padding: 1.25rem;
    min-width: 0;
  }
  @media (max-width: 640px) {
    .shell {
      grid-template-columns: 1fr;
    }
    aside {
      border-right: 0;
      border-bottom: 1px solid var(--border);
    }
  }
</style>
