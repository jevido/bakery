<script lang="ts">
  // Coolify's Server page (the layout every resources/views/livewire/server/*.blade.php
  // page shares, Apache-2.0, see NOTICE): the navbar, the configuration
  // sidebar and the sub-page the URL names.
  import { api } from '../../lib/api'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import { href, type ServerPage } from '../../lib/router.svelte'
  import type { Server } from '../../lib/types'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import ConfigurationSidebar from './ConfigurationSidebar.svelte'
  import General from './General.svelte'
  import Interim from './Interim.svelte'
  import Metrics from './Metrics.svelte'
  import Navbar from './Navbar.svelte'
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

  // Coolify's top-breadcrumb says "Servers"; the Server's name follows in the
  // switcher the navbar puts beside it. Every route change clears it, a new
  // sub-page included.
  $effect(() => {
    void [id, page]
    breadcrumb.set({ label: 'Servers', href: href('/servers') })
  })
</script>

{#if loadError}
  <p class="chrome text-sm text-error">{loadError}</p>
{:else if !server}
  <div class="chrome"><Spinner text="Loading…" /></div>
{:else}
  <Navbar {server} {page} />
  <section class="mt-4 w-full max-w-none lg:mt-0">
    <div class="grid min-w-0 gap-8 xl:grid-cols-[210px_minmax(0,1fr)] xl:gap-8">
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
        {:else}
          <Interim {server} {page} onchange={(s) => (server = s)} />
        {/if}
      </div>
    </div>
  </section>
{/if}
