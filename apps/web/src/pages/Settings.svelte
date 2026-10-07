<script lang="ts">
  // Coolify's Settings page (resources/views/livewire/settings/index.blade.php,
  // Apache-2.0, see NOTICE), its Known hosts only, as a Paperclip settings page
  // (MIT, see NOTICE): one row per host in the style of
  // src/pages/server/Resources.svelte, and Forget through ConfirmationModal
  // in place of window.confirm.
  import { Settings as SettingsIcon } from '@lucide/svelte'
  import { breadcrumb } from '../lib/breadcrumb.svelte'
  import { api } from '../lib/api'
  import { session } from '../lib/session.svelte'
  import SettingsGroup from '../lib/settings/SettingsGroup.svelte'
  import SettingsPage from '../lib/settings/SettingsPage.svelte'
  import type { KnownHost } from '../lib/types'
  import ConfirmationModal from '../lib/ui/ConfirmationModal.svelte'
  import Empty from '../lib/ui/Empty.svelte'
  import Spinner from '../lib/ui/Spinner.svelte'

  let hosts = $state.raw<KnownHost[] | null>(null)
  let error = $state('')

  async function load() {
    const r = await api<{ known_hosts: KnownHost[] }>('GET', '/known-hosts')
    hosts = r.known_hosts
  }
  // Everything here needs manage_servers; the API refuses the rest.
  if (session.can('manage_servers')) load().catch((e) => (error = e.message))

  async function forget(h: KnownHost) {
    await api('DELETE', `/known-hosts/${h.id}`)
    await load()
  }

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })

  $effect(() => breadcrumb.set({ label: 'Settings' }))
</script>

<SettingsPage icon={SettingsIcon} title="Settings">
  {#if !session.can('manage_servers')}
    <Empty title="Settings need the Manage servers permission" description="Ask someone in this guild who has it to manage known git host keys." icon="settings" />
  {:else}
    <SettingsGroup
      id="known-hosts-section"
      label="Known hosts"
      hint="The SSH host keys of git hosts, trusted on the first clone from each. A clone fails if a host later shows another key; forget the host only if you know why its key changed."
    >
      <div class="overflow-hidden rounded-md border border-border">
        {#if error}
          <p class="p-4 text-sm text-destructive">{error}</p>
        {:else if hosts === null}
          <div class="p-4"><Spinner text="Loading…" /></div>
        {:else if hosts.length === 0}
          <div class="p-4" data-testid="known-hosts-empty">
            <Empty size="sm" title="No known hosts yet" description="They appear after the first clone of an SSH repository." icon="settings" />
          </div>
        {:else}
          <div data-testid="known-hosts">
            {#each hosts as h (h.id)}
              <div class="flex items-center gap-3 border-b border-border px-4 py-2.5 last:border-b-0" data-testid="known-host">
                <div class="min-w-0 flex-1">
                  <div class="truncate font-mono text-sm text-foreground">{h.host}</div>
                  {#each h.fingerprints as f (f)}<div class="truncate font-mono text-xs text-muted-foreground">{f}</div>{/each}
                </div>
                <span class="hidden w-32 shrink-0 text-xs text-muted-foreground sm:block">{when.format(new Date(h.created_at))}</span>
                <ConfirmationModal
                  title="Forget this host key?"
                  buttonTitle="Forget"
                  actions={[`The host key of ${h.host} is forgotten.`, 'The next clone trusts whatever key it then presents.']}
                  confirmWithText={false}
                  onconfirm={() => forget(h)}
                >
                  {#snippet trigger(show)}
                    <button
                      type="button"
                      class="inline-flex h-7 shrink-0 items-center rounded-md px-2 text-xs font-medium text-destructive transition-colors hover:bg-destructive/10"
                      onclick={show}
                      data-testid="forget-known-host">Forget</button
                    >
                  {/snippet}
                </ConfirmationModal>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </SettingsGroup>
  {/if}
</SettingsPage>
