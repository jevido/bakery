<script lang="ts">
  import { setUnauthorizedHandler } from './lib/api'
  import { breadcrumb } from './lib/breadcrumb.svelte'
  import Layout from './lib/Layout.svelte'
  import { projectAccess, projectOf } from './lib/projectAccess.svelte'
  import { returnAfterLogin, router } from './lib/router.svelte'
  import { session } from './lib/session.svelte'
  import Login from './pages/Login.svelte'
  import Invite from './pages/Invite.svelte'
  import DesktopSignIn from './pages/DesktopSignIn.svelte'
  import Dashboard from './pages/Dashboard.svelte'
  import Guild from './pages/guild/Guild.svelte'
  import NewGuild from './pages/guild/New.svelte'
  import Profile from './pages/Profile.svelte'
  import Activity from './pages/Activity.svelte'
  import Costs from './pages/Costs.svelte'
  import Inbox from './pages/Inbox.svelte'
  import Approval from './pages/approvals/Approval.svelte'
  import Agent from './pages/agents/Agent.svelte'
  import Agents from './pages/agents/Agents.svelte'
  import Approvals from './pages/approvals/Approvals.svelte'
  import Goal from './pages/goals/Goal.svelte'
  import Goals from './pages/goals/Goals.svelte'
  import Routine from './pages/routines/Routine.svelte'
  import Routines from './pages/routines/Routines.svelte'
  import Issue from './pages/issues/Issue.svelte'
  import Issues from './pages/issues/Issues.svelte'
  import NotFound from './pages/NotFound.svelte'
  import Application from './pages/application/Application.svelte'
  import Database from './pages/database/Database.svelte'
  import Environment from './pages/Environment.svelte'
  import EnvironmentEdit from './pages/EnvironmentEdit.svelte'
  import NewResource from './pages/NewResource.svelte'
  import Project from './pages/Project.svelte'
  import ProjectEdit from './pages/ProjectEdit.svelte'
  import ProjectPermissions from './pages/ProjectPermissions.svelte'
  import Projects from './pages/Projects.svelte'
  import NewServer from './pages/server/New.svelte'
  import Server from './pages/server/Server.svelte'
  import Servers from './pages/server/Servers.svelte'
  import Service from './pages/service/Service.svelte'
  import Settings from './pages/Settings.svelte'
  import Storages from './pages/Storages.svelte'
  import Notifications from './pages/notifications/Notifications.svelte'
  import Security from './pages/security/Security.svelte'
  import Setup from './pages/Setup.svelte'
  import AuthAlert from './lib/ui/AuthAlert.svelte'
  import AuthShell from './lib/ui/AuthShell.svelte'
  import Spinner from './lib/ui/Spinner.svelte'
  import Toaster from './lib/ui/Toaster.svelte'

  // The components page is for development only; behind import.meta.env.DEV
  // the production bundle leaves it out.
  const devComponents = import.meta.env.DEV ? import('./pages/dev/Components.svelte').then((m) => m.default) : null

  setUnauthorizedHandler(() => session.signedOut())

  let failed = $state('')
  session.load().catch((e) => (failed = e instanceof Error ? e.message : String(e)))

  // Each page sets its own breadcrumb; a page that sets none shows none.
  $effect.pre(() => {
    void router.route
    breadcrumb.clear()
  })

  // A Desktop sign-in opened signed out shows the login page, which comes
  // back to it after.
  $effect.pre(() => {
    const route = router.route
    if (route.name === 'desktop-sign-in' && session.state === 'signed-out')
      returnAfterLogin(`/desktop-sign-in/${route.id}?token=${encodeURIComponent(route.token)}`)
  })

  // Pages under a Project ask its Permissions, overrides applied.
  $effect.pre(() => {
    const signedIn = session.state === 'signed-in'
    projectAccess.follow(signedIn ? projectOf(router.route) : null, session.guild?.id ?? null)
  })
</script>

{#if failed}
  <AuthShell>
    <AuthAlert type="error"><p>Cannot reach the API: {failed}</p></AuthAlert>
  </AuthShell>
{:else if session.state === 'loading'}
  <div class="chrome fixed inset-0 flex items-center justify-center bg-background text-sm text-muted-foreground">
    <Spinner text="Loading…" />
  </div>
{:else if router.route.name === 'invite'}
  <!-- An Invitation link opens for anyone, signed in or not. -->
  <Invite token={router.route.token} />
{:else if session.state === 'setup'}
  <Setup />
{:else if session.state === 'signed-out' || router.route.name === 'login'}
  <Login />
{:else if router.route.name === 'desktop-sign-in'}
  <DesktopSignIn id={router.route.id} token={router.route.token} />
{:else}
  <Layout>
    <!-- Every page loads again in another Guild: what it shows is that Guild's. -->
    {#key session.guild?.id}
    {#if session.guild === null && router.route.name !== 'profile'}
      <NewGuild noGuild />
    {:else if projectAccess.missing}
      <!-- A Project that is gone, in another Guild, or not to be viewed: the API answered 404. -->
      <NotFound />
    {:else if router.route.name === 'dashboard'}
      <Dashboard />
    {:else if router.route.name === 'projects'}
      <Projects />
    {:else if router.route.name === 'project'}
      <Project id={router.route.id} />
    {:else if router.route.name === 'project-edit'}
      <ProjectEdit id={router.route.id} />
    {:else if router.route.name === 'project-permissions'}
      <ProjectPermissions id={router.route.id} />
    {:else if router.route.name === 'environment'}
      <Environment projectId={router.route.projectId} id={router.route.id} />
    {:else if router.route.name === 'environment-edit'}
      <EnvironmentEdit projectId={router.route.projectId} id={router.route.id} />
    {:else if router.route.name === 'environment-new'}
      <NewResource
        projectId={router.route.projectId}
        id={router.route.id}
        type={router.route.type}
        server={router.route.server}
      />
    {:else if router.route.name === 'project-first-environment-new' || router.route.name === 'application-legacy' || router.route.name === 'database-legacy' || router.route.name === 'service-legacy'}
      <div class="flex justify-center"><Spinner text="Loading…" /></div>
    {:else if router.route.name === 'application'}
      <Application
        projectId={router.route.projectId}
        environmentId={router.route.environmentId}
        id={router.route.id}
        page={router.route.page}
        deploymentId={router.route.deploymentId}
      />

    {:else if router.route.name === 'database'}
      <Database
        projectId={router.route.projectId}
        environmentId={router.route.environmentId}
        id={router.route.id}
        page={router.route.page}
        scheduledBackupId={router.route.scheduledBackupId}
        backupSection={router.route.backupSection}
      />
    {:else if router.route.name === 'service'}
      <Service projectId={router.route.projectId} environmentId={router.route.environmentId} id={router.route.id} page={router.route.page} />
    {:else if router.route.name === 'servers'}
      <Servers />
    {:else if router.route.name === 'server-new'}
      <NewServer />
    {:else if router.route.name === 'server'}
      <Server id={router.route.id} page={router.route.page} />
    {:else if router.route.name === 'storages'}
      <Storages />
    {:else if router.route.name === 'settings'}
      <Settings />
    {:else if router.route.name === 'guild'}
      <Guild page={router.route.page} roleId={router.route.roleId} />
    {:else if router.route.name === 'guild-new'}
      <NewGuild />
    {:else if router.route.name === 'notifications'}
      <Notifications page={router.route.page} />
    {:else if router.route.name === 'security'}
      <Security page={router.route.page} />
    {:else if router.route.name === 'issues'}
      <Issues />
    {:else if router.route.name === 'issue'}
      {#key router.route.key}
        <Issue key={router.route.key} />
      {/key}
    {:else if router.route.name === 'goals'}
      <Goals />
    {:else if router.route.name === 'goal'}
      {#key router.route.id}
        <Goal id={router.route.id} />
      {/key}
    {:else if router.route.name === 'routines'}
      <Routines tab={router.route.tab} />
    {:else if router.route.name === 'routine'}
      {#key router.route.id}
        <Routine id={router.route.id} section={router.route.section} />
      {/key}
    {:else if router.route.name === 'agents'}
      <Agents tab={router.route.tab} />
    {:else if router.route.name === 'agent'}
      {#key router.route.id}
        <Agent id={router.route.id} />
      {/key}
    {:else if router.route.name === 'activity'}
      <Activity />
    {:else if router.route.name === 'costs'}
      <Costs tab={router.route.tab} />
    {:else if router.route.name === 'inbox'}
      <Inbox tab={router.route.tab} />
    {:else if router.route.name === 'approvals'}
      <Approvals tab={router.route.tab} />
    {:else if router.route.name === 'approval'}
      {#key router.route.id}
        <Approval id={router.route.id} />
      {/key}
    {:else if router.route.name === 'profile'}
      <Profile />
    {:else if router.route.name === 'dev-components' && devComponents}
      {#await devComponents then Components}
        <Components />
      {/await}
    {:else}
      <NotFound />
    {/if}
    {/key}
  </Layout>
{/if}

<Toaster />
