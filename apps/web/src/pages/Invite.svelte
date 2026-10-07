<script lang="ts">
  // Coolify's invitation/accept.blade.php in Paperclip's invite landing
  // (ui/src/pages/InviteLanding.tsx; MIT, see NOTICE): the Invitation on the
  // left, what to do with it on the right. Coolify's invitee already has an
  // account; a Bakery invitee may not, so for a new email the panel asks for
  // a name and a password before "Accept invitation". An existing Member
  // joins the guild with one click while signed in as that email. Anyone
  // with the link can decline it, which makes it stop working.
  import { api, ApiError } from '../lib/api'
  import GuildIcon from '../lib/GuildIcon.svelte'
  import { go, returnAfterLogin } from '../lib/router.svelte'
  import { session, type Account, type RoleRef } from '../lib/session.svelte'
  import AuthAlert from '../lib/ui/AuthAlert.svelte'
  import Button from '../lib/ui/Button.svelte'
  import Input from '../lib/ui/Input.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'

  let { token }: { token: string } = $props()

  type Invitation = { email: string; roles: RoleRef[]; expires_at: string }

  let invitation = $state.raw<Invitation | null>(null)
  let guild = $state('')
  // Whether the email already has a Member, who accepts while signed in.
  let existingMember = $state(false)
  // Why the link cannot be used (unknown, expired, used or revoked).
  let refusal = $state('')
  let name = $state('')
  let password = $state('')
  let errors = $state<Record<string, string>>({})
  let message = $state('')
  let busy = $state(false)
  let declining = $state(false)
  let declined = $state(false)

  $effect(() => {
    invitation = null
    refusal = ''
    api<{ invitation: Invitation; guild: { name: string }; existing_member: boolean }>(
      'GET',
      `/invitations/by-token/${encodeURIComponent(token)}`,
    )
      .then((r) => {
        guild = r.guild.name
        existingMember = r.existing_member
        invitation = r.invitation
      })
      .catch((e) => (refusal = e instanceof ApiError && e.status === 404 ? 'This invitation link is not valid.' : e.message))
  })

  async function accept(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    message = ''
    try {
      const body = existingMember ? {} : { name, password }
      await api<{ member: Account }>('POST', `/invitations/by-token/${encodeURIComponent(token)}/accept`, body)
      // The guild joined is now the Current guild, with another Role.
      await session.refresh()
      go('/')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      if (err.status === 410 || err.status === 404) {
        refusal = err.message
        return
      }
      errors = err.errors
      if (Object.keys(err.errors).length === 0) message = err.message
    } finally {
      busy = false
    }
  }


  async function decline() {
    declining = true
    message = ''
    try {
      await api('POST', `/invitations/by-token/${encodeURIComponent(token)}/decline`)
      declined = true
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      if (err.status === 410 || err.status === 404) refusal = err.message
      else message = err.message
    } finally {
      declining = false
    }
  }

  function signInToAccept() {
    returnAfterLogin(`/invite/${token}`)
    go('/login')
  }
</script>

{#snippet card(heading: string, text: string, testid: string)}
  <main class="chrome min-h-screen bg-background px-6 py-12 text-foreground">
    <div class="mx-auto max-w-xl border border-border bg-card p-6" data-testid={testid}>
      <h1 class="text-lg font-semibold">{heading}</h1>
      <p class="mt-2 text-sm text-muted-foreground">{text}</p>
    </div>
  </main>
{/snippet}

{#snippet declineButton()}
  <Button class="w-full" onclick={decline} loading={declining} disabled={busy}>Decline</Button>
{/snippet}

{#if refusal}
  {@render card('Invitation not available', `${refusal} Ask whoever invited you for a new link.`, 'invite-refused')}
{:else if declined}
  {@render card('Invitation declined', `You did not join ${guild}. The link no longer works.`, 'invite-declined')}
{:else if invitation === null}
  <main class="chrome mx-auto max-w-xl px-6 py-10 text-sm text-muted-foreground"><Spinner text="Loading invitation…" /></main>
{:else}
  <main class="chrome min-h-screen bg-background px-6 py-12 text-foreground">
    <div class="mx-auto max-w-5xl">
      <div class="grid gap-6 lg:grid-cols-(--gtc-36)">
        <section class="space-y-6 border border-border bg-card p-6">
          <div class="flex items-start gap-4">
            <GuildIcon name={guild} class="size-16 shrink-0 rounded-none border border-border" />
            <div class="min-w-0">
              <p class="text-xs tracking-(--tracking-caps) text-muted-foreground uppercase">
                You've been invited to join The Bakery
              </p>
              <h1 class="mt-2 text-2xl font-semibold" data-testid="invite-guild">Join {guild}</h1>
              <p class="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
                {#if existingMember}
                  You already have an account. Review the invitation, then accept it while signed in as {invitation.email}.
                {:else}
                  Create your account to accept. Review the invitation, then pick a name and a password.
                {/if}
              </p>
            </div>
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            {#each [['Guild', guild], ['Email', invitation.email], ['Roles', invitation.roles.length ? invitation.roles.map((r) => r.name).join(', ') : '@everyone'], ['Invitation expires', new Date(invitation.expires_at).toLocaleString()]] as [label, value] (label)}
              <div class="border border-border p-3">
                <div class="text-xs tracking-(--tracking-caps) text-muted-foreground uppercase">{label}</div>
                <div class="mt-1 truncate text-sm">{value}</div>
              </div>
            {/each}
          </div>
          {#if session.member}
            <AuthAlert type={session.member.email === invitation.email ? 'success' : 'info'}>
              Signed in as <span class="font-medium">{session.member.email}</span>.
            </AuthAlert>
          {/if}
        </section>

        <section class="h-fit border border-border bg-card p-6">
          {#if existingMember && session.member?.email === invitation.email}
            <form class="space-y-4" onsubmit={accept}>
              <div>
                <h2 class="text-lg font-semibold">Accept guild invite</h2>
                <p class="mt-1 text-sm text-muted-foreground">This will give you access to {guild}.</p>
              </div>
              {#if message}<p role="alert" class="text-xs text-destructive">{message}</p>{/if}
              <Button class="w-full" type="submit" variant="highlighted" loading={busy} disabled={declining}>Join {guild}</Button>
              {@render declineButton()}
            </form>
          {:else if existingMember || session.member}
            <div class="space-y-4">
              <div>
                <h2 class="text-lg font-semibold">Sign in to continue</h2>
                <p class="mt-1 text-sm text-muted-foreground">
                  {#if session.member}
                    You are signed in as {session.member.email}. {existingMember
                      ? `Sign in as ${invitation.email} to accept this invitation.`
                      : 'Sign out to accept this invitation.'}
                  {:else}
                    You already have an account. Sign in as {invitation.email} to accept this invitation.
                  {/if}
                </p>
              </div>
              {#if message}<p role="alert" class="text-xs text-destructive">{message}</p>{/if}
              {#if session.member}
                <Button class="w-full" variant="highlighted" onclick={() => session.logout()}>Sign out</Button>
              {:else}
                <Button class="w-full" variant="highlighted" onclick={signInToAccept}>Sign in to accept</Button>
              {/if}
              {@render declineButton()}
            </div>
          {:else}
            <form class="space-y-4" onsubmit={accept}>
              <div>
                <h2 class="text-lg font-semibold">Create your account</h2>
                <p class="mt-1 text-sm text-muted-foreground">
                  Your account uses {invitation.email}. After that you are in {guild}.
                </p>
              </div>
              <Input label="Name" name="name" bind:value={name} error={errors.name} autocomplete="name" required />
              <Input
                label="Password"
                type="password"
                name="password"
                bind:value={password}
                error={errors.password}
                autocomplete="new-password"
                required
              />
              <p class="text-xs text-muted-foreground">Use at least 12 characters.</p>
              {#if message}<p role="alert" class="text-xs text-destructive">{message}</p>{/if}
              <Button class="w-full" type="submit" variant="highlighted" loading={busy} disabled={declining}>
                Accept invitation
              </Button>
              {@render declineButton()}
            </form>
          {/if}
        </section>
      </div>
    </div>
  </main>
{/if}
