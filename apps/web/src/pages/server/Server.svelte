<script lang="ts">
  // Coolify's Server page (the layout every resources/views/livewire/server/*.blade.php
  // page shares, Apache-2.0, see NOTICE) as Paperclip's detail page: the
  // heading, the configuration nav and the sub-page the URL names.
  import { api } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import { href, type ServerPage } from '../../lib/router.svelte'
  import type { Server } from '../../lib/types'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import ConfigurationSidebar from './ConfigurationSidebar.svelte'
  import Danger from './Danger.svelte'
  import DockerCleanup from './DockerCleanup.svelte'
  import General from './General.svelte'
  import Heading from './Heading.svelte'
  import Metrics from './Metrics.svelte'
  import PrivateKey from './PrivateKey.svelte'
  import Resources from './Resources.svelte'

  let { id, page }: { id: number; page: ServerPage } = $props()

  let server = $state.raw<Server | null>(null)
  let loadError = $state('')

  async function load() {
    const r = await api<{ server: Server }>('GET', `/servers/${id}`)
    server = r.server
  }

  $effect(() => {
    server = null
    loadError = ''
    load().catch((e) => (loadError = e.message))
    // The status and the checks change when the nightly checks or another
    // tab validate it.
    const t = setInterval(() => load().catch(() => {}), 10000)
    return () => clearInterval(t)
  })

  // Servers › the Server's name, once it has loaded. Every route change
  // clears it, a new sub-page included.
  const name = $derived(server?.name)
  $effect(() => {
    void [id, page]
    if (name) breadcrumb.set({ label: 'Servers', href: href('/servers') }, { label: name })
    else breadcrumb.set({ label: 'Servers', href: href('/servers') })
  })
</script>

{#if loadError}
  <p class="chrome text-sm text-destructive">{loadError}</p>
{:else if !server}
  <div class="chrome"><Spinner text="Loading…" /></div>
{:else}
  <div class="chrome w-full space-y-6">
    <Heading {server} {page} />
    <div class="grid min-w-0 gap-6 xl:grid-cols-[210px_minmax(0,1fr)] xl:gap-8">
      <ConfigurationSidebar {server} {page} />
      <div class="min-w-0">
        {#if page === ''}
          <General {server} onchange={(s) => (server = s)} />
        {:else if page === 'private-key'}
          <PrivateKey {server} onchange={(s) => (server = s)} />
        {:else if page === 'resources'}
          <Resources {server} />
        {:else if page === 'metrics'}
          <Metrics {server} />
        {:else if page === 'docker-cleanup'}
          <DockerCleanup {server} onchange={(s) => (server = s)} />
        {:else if page === 'danger'}
          <Danger {server} />
        {/if}
      </div>
    </div>
  </div>
{/if}
