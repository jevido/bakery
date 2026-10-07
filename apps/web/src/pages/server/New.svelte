<script lang="ts">
  // Coolify's New server page (resources/views/livewire/server/create.blade.php
  // and server/new/by-ip.blade.php; app/Livewire/Server/New/ByIp.php;
  // Apache-2.0, see NOTICE), with only "Add a server": The Bakery provisions
  // nothing at a cloud provider. It generates one key per Server instead of
  // picking a Private key, and has no build servers, so neither is asked.
  // User has no default, so the collapsible that holds it starts open.
  //
  // Laid out as a Paperclip settings page (ui/src/pages/CompanySettings.tsx,
  // MIT, see NOTICE): one group, its fields.
  import { Server as ServerIcon } from '@lucide/svelte'
  import { api, ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import Icon from '../../lib/Icon.svelte'
  import { go, href, serverPath } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import SettingsPage from '../../lib/settings/SettingsPage.svelte'
  import type { Server } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Input from '../../lib/ui/Input.svelte'

  let name = $state('')
  let description = $state('')
  let host = $state('')
  let port = $state<string | number>('22')
  let user = $state('')
  let advanced = $state(true)
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)

  async function add(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    try {
      const { server } = await api<{ server: Server }>('POST', '/servers', {
        name,
        description,
        host,
        port: Number(port) || 0,
        user,
      })
      go(serverPath(server.id, 'private-key'))
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
      if (errors.user || errors.port) advanced = true
    } finally {
      busy = false
    }
  }

  $effect(() => breadcrumb.set({ label: 'Servers', href: href('/servers') }, { label: 'New server' }))
</script>

{#if !session.can('manage_servers')}
  <div class="chrome w-full">
    <p class="text-sm text-muted-foreground">Only an admin of this guild adds servers.</p>
  </div>
{:else}
  <SettingsPage icon={ServerIcon} title="New server">
    <form onsubmit={add}>
      <SettingsGroup id="new-server-section" label="Connect a server" hint="Add an existing Linux server using its SSH connection details.">
        <Input
          label="IP address or domain"
          helper="For example 127.0.0.1 or server.example.com."
          bind:value={host}
          error={errors.host}
          required
        />

        <div class="grid gap-4 border-t border-border pt-4 sm:grid-cols-2">
          <Input label="Name" bind:value={name} error={errors.name} required />
          <Input label="Description" bind:value={description} error={errors.description} />
        </div>

        <div class="flex flex-col gap-4 border-t border-border pt-4">
          <button
            type="button"
            onclick={() => (advanced = !advanced)}
            class="flex cursor-pointer items-center gap-2 text-left text-sm font-medium hover:underline"
            aria-expanded={advanced}
          >
            <svg class={['size-4 transition-transform', advanced && 'rotate-90']} viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
              <path
                fill-rule="evenodd"
                d="M7.21 14.77a.75.75 0 0 1 .02-1.06L11.168 10 7.23 6.29a.75.75 0 1 1 1.04-1.08l4.5 4.25a.75.75 0 0 1 0 1.08l-4.5 4.25a.75.75 0 0 1-1.06-.02Z"
                clip-rule="evenodd"
              />
            </svg>
            Advanced settings
          </button>
          <div class={['rounded-lg border border-border p-4', !advanced && 'hidden']}>
            <div class="grid gap-4 sm:grid-cols-2">
              <Input
                label="User"
                helper="A Linux user with rootless Podman, e.g. bakery. The Bakery runs everything as this user."
                bind:value={user}
                error={errors.user}
                placeholder="bakery"
                required
              />
              <Input type="number" label="Port" bind:value={port} error={errors.port} required />
            </div>
          </div>
        </div>

        <div class="flex justify-end border-t border-border pt-4">
          <Button type="submit" variant="highlighted" loading={busy}>
            Add server
            <Icon name="arrow-right" class="size-3.5" />
          </Button>
        </div>
      </SettingsGroup>
    </form>
  </SettingsPage>
{/if}
