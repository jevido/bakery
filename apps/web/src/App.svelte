<script lang="ts">
  import { setUnauthorizedHandler } from './lib/api'
  import Layout from './lib/Layout.svelte'
  import { router } from './lib/router.svelte'
  import { session } from './lib/session.svelte'
  import Login from './pages/Login.svelte'
  import Invite from './pages/Invite.svelte'
  import Members from './pages/Members.svelte'
  import ApiTokens from './pages/ApiTokens.svelte'
  import NotFound from './pages/NotFound.svelte'
  import Application from './pages/Application.svelte'
  import Database from './pages/Database.svelte'
  import Project from './pages/Project.svelte'
  import Projects from './pages/Projects.svelte'
  import Server from './pages/Server.svelte'
  import Servers from './pages/Servers.svelte'
  import Service from './pages/Service.svelte'
  import Settings from './pages/Settings.svelte'
  import Setup from './pages/Setup.svelte'

  setUnauthorizedHandler(() => session.signedOut())

  let failed = $state('')
  session.load().catch((e) => (failed = e instanceof Error ? e.message : String(e)))
</script>

{#if failed}
  <main class="auth"><p class="error">Cannot reach the API: {failed}</p></main>
{:else if session.state === 'loading'}
  <main class="auth"><p class="muted">Loading…</p></main>
{:else if router.route.name === 'invite'}
  <!-- An Invitation link opens for anyone, signed in or not. -->
  <Invite token={router.route.token} />
{:else if session.state === 'setup'}
  <Setup />
{:else if session.state === 'signed-out' || router.route.name === 'login'}
  <Login />
{:else}
  <Layout>
    {#if router.route.name === 'projects'}
      <Projects />
    {:else if router.route.name === 'project'}
      <Project id={router.route.id} />
    {:else if router.route.name === 'application'}
      <Application id={router.route.id} />
    {:else if router.route.name === 'database'}
      <Database id={router.route.id} />
    {:else if router.route.name === 'service'}
      <Service id={router.route.id} />
    {:else if router.route.name === 'servers'}
      <Servers />
    {:else if router.route.name === 'server'}
      <Server id={router.route.id} />
    {:else if router.route.name === 'settings'}
      <Settings />
    {:else if router.route.name === 'members'}
      <Members />
    {:else if router.route.name === 'api-tokens'}
      <ApiTokens />
    {:else}
      <NotFound />
    {/if}
  </Layout>
{/if}
