<script lang="ts">
  // Coolify's Server Private Key page (resources/views/livewire/server/private-key/show.blade.php,
  // app/Livewire/Server/PrivateKey/Show.php; Apache-2.0, see NOTICE). The
  // Bakery generates one key per Server instead of picking one of the team's
  // Private Keys, so the key list is one card for the Server's own key, with
  // the command that authorises it and the pinned host key below it.
  import { Badge } from '$lib/components/ui/badge'
  import { api, ApiError } from '../../lib/api'
  import Icon from '../../lib/Icon.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Server } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import CopyButton from '../../lib/ui/CopyButton.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let { server, onchange }: { server: Server; onchange: (s: Server) => void } = $props()

  const publicKey = $derived(server.public_key ?? '')
  const keyType = $derived(publicKey.split(' ')[0] || 'ssh key')
  const command = $derived(`echo '${publicKey}' >> ~/.ssh/authorized_keys`)

  let checking = $state(false)

  // Coolify's checkConnection: "Server is reachable." once SSH answers. The
  // Bakery's validation also checks Podman, so a reachable Server whose other
  // checks fail says which.
  async function checkConnection() {
    if (checking) return
    checking = true
    try {
      const r = await api<{ server: Server }>('POST', `/servers/${server.id}/validate`)
      onchange(r.server)
      const checks = r.server.validation.checks
      const ssh = checks.find((c) => c.name === 'ssh')
      const failed = checks.find((c) => !c.ok && c.required)
      if (ssh && !ssh.ok) toast.error('Server is not reachable.', ssh.detail)
      else if (failed) toast.error('Server is reachable, but a check failed.', failed.detail)
      else toast.success('Server is reachable.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Server is not reachable.', err.message)
    } finally {
      checking = false
    }
  }

  async function forgetHostKey() {
    try {
      const r = await api<{ server: Server }>('DELETE', `/servers/${server.id}/host-key`)
      onchange(r.server)
      toast.success('Host key forgotten.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error(err.message)
    }
  }
</script>

<div class="space-y-8">
  {#if server.kind !== 'remote'}
    <Empty size="sm" title="The Local server has no Private key." description="The Bakery reaches it through the local Podman socket, not SSH." icon="keys" />
  {:else}
    <SettingsGroup id="server-private-keys-section" label="Private key" hint="The SSH key The Bakery uses to connect to this server.">
      {#snippet actions()}
        {#if session.can('manage_servers')}
          <Button loading={checking} onclick={checkConnection} data-testid="check-connection">
            {#if !checking}<Icon name="refresh" class="size-3.5" />{/if}
            Check connection
          </Button>
        {/if}
      {/snippet}

      {#if !session.can('manage_servers')}
        <p class="text-sm text-muted-foreground" data-testid="private-key-admin-only">Only an admin can see this server's key.</p>
      {:else if !publicKey}
        <Empty size="sm" title="No private key" description="This server has no key yet." icon="keys" />
      {:else}
        <div class="flex min-w-0 items-start gap-3">
          <div class="flex size-9 shrink-0 items-center justify-center rounded-md bg-accent text-muted-foreground">
            <Icon name="keys" class="size-4" />
          </div>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <p class="truncate text-sm font-medium">{server.name}</p>
              <Badge variant="outline" class="font-mono" data-testid="key-type">{keyType}</Badge>
            </div>
            <p class="mt-1 text-xs text-muted-foreground">
              Add it for <span class="font-mono">{server.user}</span> on the server, then Check connection.
            </p>
          </div>
        </div>
        <CopyButton text={publicKey} label="Public key" testid="private-key" />
        <CopyButton text={command} label="Command" testid="authorize-command" />

        <div class="flex flex-col gap-3 border-t border-border pt-4 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <p class="text-sm font-medium">Host key</p>
            {#if server.host_key_fingerprint}
              <p class="mt-1 font-mono text-xs break-all text-muted-foreground" data-testid="host-key">{server.host_key_fingerprint}</p>
            {:else}
              <p class="mt-1 text-xs text-muted-foreground" data-testid="host-key-none">
                Not pinned yet; the next connection pins the key the server presents.
              </p>
            {/if}
          </div>
          {#if server.host_key_fingerprint}
            <ConfirmationModal
              title="Forget host key?"
              buttonTitle="Forget host key"
              variant="error"
              actions={['Forget the host key? The next connection trusts whatever key the server then presents.']}
              warningMessage="Only forget it when the server's host key changed on purpose."
              confirmWithText={false}
              step2ButtonText="Forget host key"
              onconfirm={forgetHostKey}
            />
          {/if}
        </div>
      {/if}
    </SettingsGroup>
  {/if}
</div>
