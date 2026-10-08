<script lang="ts">
  // Every Desktop signed in as the person (Paperclip has no page for its
  // board API keys; this one follows the API Tokens list beside it): each
  // Desktop's name, when it was connected and last seen, and Sign out, which
  // makes its Desktop key stop working at once. This is where a lost laptop
  // is signed out.
  import { ago } from '../../lib/format'
  import { listDesktops, signOutDesktop, type Desktop } from '../../lib/desktops'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let desktops = $state.raw<Desktop[] | null>(null)
  let loadError = $state('')

  async function load() {
    // The most recently seen first, as a person looks for the one in use.
    desktops = [...(await listDesktops())].sort((a, b) => (b.last_seen_at ?? b.created_at).localeCompare(a.last_seen_at ?? a.created_at))
  }
  load().catch((e) => (loadError = e.message))

  async function signOut(d: Desktop) {
    try {
      await signOutDesktop(d.id)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
      throw err
    }
    toast.success(`${d.name} is signed out.`)
    await load()
  }
</script>

{#if loadError}
  <p class="text-sm text-destructive">{loadError}</p>
{:else if desktops === null}
  <Spinner text="Loading…" />
{:else}
  <SettingsGroup id="desktops" label="Desktops">
    {#if desktops.length === 0}
      <div class="rounded-md border border-border p-4">
        <Empty
          title="No desktop apps signed in."
          description="Install the desktop app and connect it to this Bakery."
          icon="keys"
          size="sm"
        />
      </div>
    {:else}
      <div class="overflow-hidden rounded-md border border-border">
        {#each desktops as desktop (desktop.id)}
          <div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 border-b border-border px-4 py-2.5 last:border-b-0" data-testid="desktop" data-id={desktop.id}>
            <span class="min-w-0 flex-1 truncate text-sm font-medium text-foreground" data-testid="desktop-name">{desktop.name}</span>
            <span class="shrink-0 text-xs text-muted-foreground">Connected {ago(desktop.created_at)}</span>
            <span class="shrink-0 text-xs text-muted-foreground" data-testid="desktop-last-seen">
              {desktop.last_seen_at ? `Last seen ${ago(desktop.last_seen_at)}` : 'Never seen'}
            </span>
            <div class="ml-auto shrink-0">
              <ConfirmationModal
                title="Sign out this desktop app?"
                actions={[`${desktop.name} will be signed out of this Bakery.`, 'It has to be approved again to connect.']}
                warningMessage="Its Desktop key stops working at once."
                confirmWithText={false}
                step2ButtonText="Sign out"
                onconfirm={() => signOut(desktop)}
              >
                {#snippet trigger(show)}
                  <button
                    type="button"
                    class="inline-flex h-7 items-center rounded-md px-2 text-xs font-medium text-destructive transition-colors hover:bg-destructive/10"
                    onclick={show}
                    data-testid="sign-out-desktop">Sign out</button
                  >
                {/snippet}
              </ConfirmationModal>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </SettingsGroup>
{/if}
