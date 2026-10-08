<script lang="ts">
  // Coolify's Rollback (resources/views/livewire/project/application/rollback.blade.php
  // and app/Livewire/Project/Application/Rollback.php, Apache-2.0, see
  // NOTICE): the Images the deployments context's Image retention keeps, the
  // running one marked, each other one rolled back to without a rebuild.
  // Coolify lists `docker images` by tag; The Bakery lists the finished
  // Deployments whose Image is still on their Server, so a Rollback names
  // the Deployment it starts again.
  //
  // Listed as Paperclip's EntityRow pattern (as the Environment page and
  // Deployment history are), under a SettingsGroup for the Reload action.
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import { Card } from '@bakery/ui/components/ui/card'
  import { api, ApiError } from '../../lib/api'
  import EntityRow from '@bakery/ui/EntityRow.svelte'
  import { ago } from '../../lib/format'
  import Icon from '../../lib/Icon.svelte'
  import { applicationPath, go } from '../../lib/router.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import SettingsGroup from '../../lib/settings/SettingsGroup.svelte'
  import type { Application, Deployment } from '../../lib/types'
  import Empty from '../../lib/ui/Empty.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import StatusBadge from '@bakery/ui/StatusBadge.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let { application }: { application: Application } = $props()

  type RetainedImage = { image: string; current: boolean; deployment: Deployment }

  let images = $state.raw<RetainedImage[]>([])
  let loading = $state(true)
  let rollingBack = $state(0)

  async function load(showToast = false) {
    loading = true
    try {
      images = (await api<{ images: RetainedImage[] }>('GET', `/applications/${application.id}/images`)).images
      if (showToast) toast.success('Images loaded.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Images not loaded', err.message)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    void application.id
    load().catch(() => {})
  })

  async function rollback(d: Deployment) {
    rollingBack = d.id
    try {
      const r = await api<{ deployment: Deployment }>('POST', `/deployments/${d.id}/rollback`)
      go(`${applicationPath(application, 'deployment')}/${r.deployment.id}`)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Rollback not started', err.message)
    } finally {
      rollingBack = 0
    }
  }

  const tag = (image: string) => image.slice(image.lastIndexOf(':') + 1)
  const builtAt = (d: Deployment) => d.finished_at ?? d.created_at
  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' })
  const listCard = 'block gap-0 overflow-hidden py-0'
</script>

<div class="chrome flex flex-col gap-6">
  <SettingsGroup id="rollback-images-section" label="Available images" hint="Rollback uses an existing local image without rebuilding the application." wide>
    {#snippet actions()}
      <button type="button" class={buttonVariants({ variant: 'outline', size: 'sm' })} onclick={() => load(true)} disabled={loading}>
        {#if loading}<Spinner />{/if}
        Reload images
      </button>
    {/snippet}

    {#if loading}
      <Card class={listCard}>
        <div class="flex items-center justify-center gap-2 px-4 py-10 text-sm text-muted-foreground">
          <Spinner />
          Loading available images…
        </div>
      </Card>
    {:else if images.length === 0}
      <Card class={listCard}>
        <Empty title="No rollback images" description="No previous application images are currently stored on this server." icon="layers" />
      </Card>
    {:else}
      <Card class={listCard}>
        {#each images as image (image.deployment.id)}
          <EntityRow
            title={tag(image.image)}
            subtitle={`Built ${ago(builtAt(image.deployment))} · ${when.format(new Date(builtAt(image.deployment)))} · Deployment #${image.deployment.id}${image.deployment.commit_sha ? ` (${image.deployment.commit_sha.slice(0, 7)})` : ''}`}
            reserveSubtitleSpace
            data-testid="rollback-image"
          >
            {#snippet leading()}
              <span class="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
                <Icon name="layers" class="size-4" />
              </span>
            {/snippet}
            {#snippet trailing()}
              {#if image.current}
                <StatusBadge status="Running image" type="success" />
              {/if}
              {#if projectAccess.can('deploy')}
                {#if image.current}
                  <button type="button" class={buttonVariants({ variant: 'outline', size: 'sm' })} disabled title="This image is currently running.">
                    Rollback
                  </button>
                {:else}
                  <button
                    type="button"
                    class={buttonVariants({ variant: 'outline', size: 'sm' })}
                    disabled={rollingBack !== 0}
                    onclick={() => rollback(image.deployment)}
                  >
                    {#if rollingBack === image.deployment.id}<Spinner />{/if}
                    Roll back to this image
                  </button>
                {/if}
              {/if}
            {/snippet}
          </EntityRow>
        {/each}
      </Card>
    {/if}
  </SettingsGroup>
</div>
