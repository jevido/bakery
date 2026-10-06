<script lang="ts">
  // Coolify's New server page (resources/views/livewire/server/create.blade.php
  // and server/new/by-ip.blade.php; app/Livewire/Server/New/ByIp.php;
  // Apache-2.0, see NOTICE), with only "Add a server": The Bakery provisions
  // nothing at a cloud provider. It generates one key per Server instead of
  // picking a Private key, and has no build servers, so neither is asked.
  // User has no default, so the collapsible that holds it starts open.
  import { api, ApiError } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import Icon from '../../lib/Icon.svelte'
  import { go, href, serverPath } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import type { Server } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'

  let selected = $state(false)
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

<div class="chrome w-full">
  <div class="mb-5 flex min-h-9 flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
    <h1 class="min-w-0 text-[24px]! leading-7! font-semibold! tracking-tight!">New server</h1>
    {#if selected}
      <div class="flex flex-wrap items-center gap-2">
        <Button onclick={() => (selected = false)}>Change method</Button>
      </div>
    {/if}
  </div>

  {#if !session.isAdmin}
    <p class="text-sm text-neutral-500 dark:text-fg-dim">Only an admin of this guild adds servers.</p>
  {:else if !selected}
    <div class="application-settings-form flex flex-col gap-6">
      <section class="application-settings-section">
        <div class="application-settings-section-header">
          <h2 class="application-settings-section-title">Add a server</h2>
          <p class="application-settings-section-description">Connect a server you already manage.</p>
        </div>
        <div class="application-settings-section-body is-flush">
          <div class="grid grid-cols-1 gap-3 p-3 sm:grid-cols-2 lg:grid-cols-4">
            <button
              type="button"
              onclick={() => (selected = true)}
              class="group flex min-h-32 cursor-pointer flex-col rounded-xl border border-neutral-200 bg-white p-3 text-left shadow-sm transition-all hover:-translate-y-px hover:border-neutral-300 hover:shadow-md dark:border-white/[0.08] dark:bg-white/[0.05] dark:hover:border-white/[0.14]"
              data-testid="add-by-ip"
            >
              <div class="flex items-start">
                <span
                  class="flex size-8 items-center justify-center rounded-lg border border-neutral-200 bg-neutral-50 text-neutral-500 dark:border-white/[0.08] dark:bg-white/[0.04] dark:text-fg-dim"
                >
                  <Icon name="servers" class="size-4" />
                </span>
              </div>
              <div class="mt-auto pt-5">
                <h3 class="text-[13px]! font-semibold! text-black dark:text-fg">IP address or domain</h3>
                <p class="mt-1 text-[11px] leading-4 text-neutral-500 dark:text-fg-faint">Connect an existing server over SSH.</p>
              </div>
            </button>
          </div>
        </div>
      </section>
    </div>
  {:else}
    <form class="application-settings-form" onsubmit={add}>
      <SettingsSection id="new-server-section" title="Connect a server" helper="Add an existing Linux server using its SSH connection details.">
        {#snippet actions()}
          <Button type="submit" variant="highlighted" loading={busy}>
            Continue
            <Icon name="arrow-right" class="size-3.5" />
          </Button>
        {/snippet}

        <div class="mb-5">
          <Input
            label="IP address or domain"
            helper="For example 127.0.0.1 or server.example.com."
            bind:value={host}
            error={errors.host}
            required
          />
        </div>

        <div class="grid gap-4 border-t border-neutral-200 pt-4 lg:grid-cols-2 dark:border-white/[0.08]">
          <Input label="Name" bind:value={name} error={errors.name} required />
          <Input label="Description" bind:value={description} error={errors.description} />
        </div>

        <div class="mt-5 flex flex-col gap-4 border-t border-neutral-200 pt-4 dark:border-white/[0.08]">
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
          <div class={['rounded-lg border border-neutral-200 p-4 dark:border-coolgray-400', !advanced && 'hidden']}>
            <div class="grid gap-4 lg:grid-cols-2">
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
      </SettingsSection>
    </form>
  {/if}
</div>
